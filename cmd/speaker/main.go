/*
DESCRIPTION
  speaker is a netsender client intended to be run on the Speaker Control Unit for audio playback and control.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

// Package speaker is a netsender client for audio playback, speaker
// control,and speaker health checking.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ausocean/av/revid"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/client/pi/gpio"
	"github.com/ausocean/client/pi/netlogger"
	"github.com/ausocean/client/pi/netsender"
	"github.com/ausocean/utils/logging"
	"github.com/kidoman/embd"
	_ "github.com/kidoman/embd/host/rpi"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// Logging configuration.
const (
	logPath      = "/var/log/netsender/netsender.log"
	logMaxSize   = 500 // MB
	logMaxBackup = 10
	logMaxAge    = 28 // days
	logVerbosity = logging.Info
	logSuppress  = true
)

// Misc constants.
const (
	netSendRetryTime = 5 * time.Second
	defaultSleepTime = 60 // Seconds
	pkg              = "rv: "
	minAmpVolume     = 0
	maxAmpVolume     = 63
	volAddr          = 0x4B
	i2cPort          = 1
	confPath         = "/etc/speaker.json"
	defaultPath      = "/home/pi/audio.wav"
)

// cfgCache contains all the relevant keys used in the var cache.
type cfgCache struct {
	Path   string // Stores the path to the audio file to be played.
	Volume string // Stores the volume of the amplifier.
}

// valid keys to be found in the config.
const (
	cfgPath   = "path"
	cfgVolume = "volume"
)

// Channel modes.
const (
	modeStereo = "Stereo"
	modeLeft   = "LeftMono"
	modeRight  = "RightMono"
	modeMute   = "Mute"
)

const audioCmd = "aplay"

// fileMu is used to ensure safe reading and writing of shared files.
var fileMu sync.Mutex

var errUnsupportedType = errors.New("MIME type not supported")

// Variable map to send to the cloud.
var varMap = map[string]string{
	"SpeakerMode":   "enum:" + strings.Join([]string{modeStereo, modeLeft, modeRight, modeMute}, ","),
	"AudioFilePath": "string",
}

// onlinePrefixes stores the valid prefixes that are assumed to be online sources of data. Sources
// with these prefixes will be accessed using a GET request, all other URIs will be assumed to be local.
var onlinePrefixes = []string{"http://", "https://"}

func initCommand(l logging.Logger) { checkPath(audioCmd, l) }

func main() {

	// Create lumberjack logger to handle logging to file.
	fileLog := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    logMaxSize,
		MaxBackups: logMaxBackup,
		MaxAge:     logMaxAge,
	}

	// Create netlogger to handle logging to cloud.
	netLog := netlogger.New()

	// Create logger that we call methods on to log, which in turn writes to the
	// lumberjack and netloggers.
	log := logging.New(logVerbosity, io.MultiWriter(fileLog, netLog), logSuppress)
	// Create i2c bus to communicate with the amplifier.
	bus := embd.NewI2CBus(i2cPort)

	// Get the cached config.
	cfg, err := getConf()
	if err != nil {
		cfg = &cfgCache{Path: defaultPath, Volume: fmt.Sprint(maxAmpVolume)} // Set some default values.
		log.Error("unable to get cached config", "err", err, "defaulting", cfg)
	}
	log.Debug("got cached config", "config", cfg)

	// Set the volume from the cached value.
	err = setVolume(cfg.Volume, bus)
	if err != nil {
		log.Error("failed to set cached volume", "error", err)
	} else {
		log.Debug("successfully set new volume", "volume", cfg.Volume)
	}

	// The netsender client handles cloud communication and GPIO control.
	log.Debug("initialising netsender client")
	ns, err := netsender.New(log, gpio.InitPin, nil, gpio.WritePin, netsender.WithVarTypes(varMap))
	if err != nil {
		log.Fatal("could not initialise netsender client", "error", err)
	}

	// Revid will handle the recording and sending of audio for sound checking.
	log.Debug("initialising revid")
	rv, err := revid.New(config.Config{Logger: log}, ns)
	if err != nil {
		log.Fatal("could not initialise revid", "error", err)
	}

	// Play the audio (audio will play even while muted).
	log.Debug("Playing the audio")
	go playAudio(&cfg.Path, log)

	// Start the control loop.
	log.Debug("starting control loop")
	run(rv, ns, &cfg.Path, bus, log, netLog)
}

// run starts a control loop that runs netsender, sends logs, checks for var changes, and
// if var changes, changes current mode (paused,audio playback or soundcheck)
func run(rv *revid.Revid, ns *netsender.Sender, file *string, bus embd.I2CBus, l logging.Logger, nl *netlogger.Logger) {
	var vs int
	uri := *file

	for {
		l.Debug("running netsender")
		err := ns.Run()
		if err != nil {
			l.Warning("run failed. Retrying...", "error", err)
			time.Sleep(netSendRetryTime)
			continue
		}

		l.Debug("sending logs")
		err = nl.Send(ns)
		if err != nil {
			l.Warning(pkg+"Logs could not be sent", "error", err)
		}

		l.Debug("checking varsum")
		newVs := ns.VarSum()
		if vs == newVs {
			sleep(ns, l)
			continue
		}
		vs = newVs
		l.Info("varsum changed", "vs", vs)

		l.Debug("getting new vars")
		vars, err := ns.Vars()
		if err != nil {
			l.Error(pkg+"netSender failed to get vars", "error", err)
			time.Sleep(netSendRetryTime)
			continue
		}
		l.Info("got new vars", "vars", vars)

		// Configure revid based on the vars.
		l.Debug("updating revid configuration")
		err = rv.Update(vars)
		if err != nil {
			l.Warning(pkg+"couldn't update revid", "error", err)
			sleep(ns, l)
			continue
		}
		l.Info("revid successfully reconfigured")

		l.Debug("checking audio URI")
		newUri := vars["AudioFilePath"]
		if newUri != "" && newUri != uri {
			err = getAudio(newUri, file, l)
			if err != nil {
				l.Error("call to getAudio failed", "uri", newUri, "error", err)
				continue
			}
			uri = newUri
			l.Debug("updated audio file from uri and cached")
		}

		l.Debug("checking amplifier volume")
		v := vars["AmpVolume"]
		if v != "" {
			err = setVolume(v, bus)
			if err != nil {
				l.Error("unable to set requested volume", "volume", v, "error", err)
				continue
			}
			l.Debug("successfully set requested volume", "volume", v)
		}

		l.Debug("checking mode")
		_ = setChannels(vars["SpeakerMode"], l)
		sleep(ns, l)
	}
}

// setChannels handles the muting of one, both, or neither of the channels. It takes in SpeakerMode
// and sets the relevant volumes.
func setChannels(mode string, l logging.Logger) error {
	l.Info("mode is", "mode", mode)

	// Set the volume of each channel.
	vols := map[string]string{
		modeStereo: "100%,100%",
		modeLeft:   "0%,100%",
		modeRight:  "100%,0%",
		modeMute:   "0%,0%",
	}[mode]
	if vols == "" {
		l.Warning("invalid SpeakerMode", "SpeakerMode", mode)
		return fmt.Errorf("invalid SpeakerMode: %s", mode)
	}

	// Create the command to change the channel volumes.
	cmd := exec.Command("amixer", "sset", "Speaker", vols)

	// Pipe the output to stdout and stderr.
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		l.Error("unable to setup pipe to stdout", "error", err)
		return fmt.Errorf("unable to setup pipe to stdout: %w", err)
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		l.Error("unable to setup pipe to stderr", "error", err)
		return fmt.Errorf("unable to setup pipe to stderr: %w", err)
	}

	// Execute the channel setting command.
	err = cmd.Start()
	if err != nil {
		l.Error("unable to set channel", "error", err)
		return fmt.Errorf("unable to set channel: %w", err)
	}

	// Copy any std out to a buffer for logging.
	var outBuff bytes.Buffer
	go func() {
		_, err = io.Copy(&outBuff, outPipe)
		if err != nil {
			l.Error("failed to copy out pipe", "error", err)
		}
	}()

	// Copy any std error to a buffer for logging.
	var errBuff bytes.Buffer
	go func() {
		_, err = io.Copy(&errBuff, errPipe)
		if err != nil {
			l.Error("failed to copy error pipe", "error", err)
		}
	}()
	if errBuff.String() != "" {
		return fmt.Errorf("channel set command failed: %s", &errBuff)
	}

	l.Info("mode set to", "mode", mode)
	return nil
}

// setVolume sends i2c commands to the amplifier in order to set the volume of the amplifier.
func setVolume(hexVol string, bus embd.I2CBus) error {
	vol, err := strconv.ParseInt(hexVol, 10, 8)
	if err != nil {
		return fmt.Errorf("could not parse hex volume: %w", err)
	}
	if vol < minAmpVolume || vol > maxAmpVolume {
		return fmt.Errorf("volume %d, out of range: [%d, %d]", vol, minAmpVolume, maxAmpVolume)
	}
	err = bus.WriteByte(volAddr, byte(vol))
	if err != nil {
		return fmt.Errorf("unable to write volume to amplifier: %w", err)
	}

	// Cache the volume.
	updateConf(func(c *cfgCache) { c.Volume = hexVol })
	return nil
}

// playAudio is intended to be run as a routine. It will continuously run even while muted.
func playAudio(file *string, l logging.Logger) {
	var numPlays int
	for {
		// Ensure that the file is not currently being updated.
		fileMu.Lock()

		cmd := exec.Command(audioCmd, *file)
		// We'd like to see what the playback software is outputting, so pipe
		// stdout and stderr.
		outPipe, err := cmd.StdoutPipe()
		if err != nil {
			l.Error("failed to pipe stdout", "error", err)
		}
		errPipe, err := cmd.StderrPipe()
		if err != nil {
			l.Error("failed to pipe stderr", "error", err)
		}

		// Start playback of the audio file.
		err = cmd.Start()
		if err != nil {
			l.Error("start failed", "error", err)
			fileMu.Unlock()
			continue
		}
		numPlays++
		l.Debug("playing audio", "numPlays", numPlays)

		// Copy any std out to a buffer for logging.
		var outBuff bytes.Buffer
		go func() {
			_, err = io.Copy(&outBuff, outPipe)
			if err != nil {
				l.Error("failed to copy out pipe", "error", err)
			}
		}()

		// Copy any std error to a buffer for logging.
		var errBuff bytes.Buffer
		go func() {
			_, err = io.Copy(&errBuff, errPipe)
			if err != nil {
				l.Error("failed to copy error pipe", "error", err)
			}
		}()

		// Wait for playback to complete.
		err = cmd.Wait()
		if err != nil {
			l.Error("failed to wait for execution finish", "error", err)
		}
		fileMu.Unlock()
		l.Debug("stdout received", "stdout", string(outBuff.Bytes()))

		// If there was any errors on stderr, log them.
		if errBuff.Len() != 0 {
			l.Error("errors from stderr", "stderr", string(errBuff.Bytes()))
		}
	}
}

// sleep uses a delay to halt the program based on the monitoring period
// netsender parameter (mp) defined in the netsender.conf config.
func sleep(ns *netsender.Sender, l logging.Logger) {
	l.Debug("sleeping")
	t, err := strconv.Atoi(ns.Param("mp"))
	if err != nil {
		l.Error(pkg+"could not get sleep time, using default", "error", err)
		t = defaultSleepTime
	}
	time.Sleep(time.Duration(t) * time.Second)
	l.Debug("finished sleeping")
}

// checkPath wraps the use of lookPath to check the existence of executables
// that will be used by the audio looper.
func checkPath(cmd string, l logging.Logger) {
	path, err := exec.LookPath(cmd)
	if err != nil {
		l.Fatal(fmt.Sprintf("couldn't find %s", cmd), "error", err)
	}
	l.Debug(fmt.Sprintf("found %s", cmd), "path", path)
}

// isOnlineResource checks against the onlinePrefixes slice to determine
// whether a uri is an online or local resource, and returns true for a
// matching prefix, and returns false for no match.
func isOnlineResource(uri string) bool {
	for _, prefix := range onlinePrefixes {
		if strings.HasPrefix(uri, prefix) {
			return true
		}
	}
	return false
}

// getAudio determines the source of the file (local or online),
// fetches the audio into a file to be played from.
func getAudio(uri string, playPath *string, l logging.Logger) error {
	const cachePath = "/home/pi/"

	// Update the target audio file from the specified source (URI).
	if !isOnlineResource(uri) {
		// If we are here, the URI points to a file so we can just change
		// the filePath.
		fileMu.Lock()
		*playPath = uri
		err := setAudioPath(*playPath)
		if err != nil {
			l.Error("Failed to set audio path", "error", err)
			fileMu.Unlock()
			return err
		}
		fileMu.Unlock()

		return nil
	}

	// Check if the uri has been downloaded already.
	urlPath := cachePath + strings.Split(uri, "://")[1]
	_, err := os.Stat(urlPath)
	if err == nil {
		l.Debug("resource cached, changing play path")
		err = setAudioPath(urlPath)
		if err != nil {
			return err
		}
		fileMu.Lock()
		defer fileMu.Unlock()
		*playPath = urlPath
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		l.Debug("File found, or other error", "err", err)
		return fmt.Errorf("File found, or other error %w", err)
	}
	l.Debug("File not found, getting resource from URI", "URI", uri)

	resp, err := http.Get(uri)
	if err != nil {
		l.Error("could not get uri", "error", err)
		return fmt.Errorf("could not get uri: %w", err)
	}
	l.Debug("got resource from URI")

	// Determine the file type. (MIME should be type/subtype).
	// for the speaker, the type should be audio.
	mime := strings.Split(resp.Header.Get("Content-Type"), "/")
	if mime[0] != "audio" {
		l.Debug(errUnsupportedType.Error(), "MIME", mime[0])
		return errUnsupportedType
	}
	l.Debug("correct MIME major type")

	switch mime[1] {
	case "wav", "x-wav", "wave", "pcm", "adpcm", "raw":
		l.Debug("correct MIME subtype")

		// Ensure that the parent folders exist.
		err = os.MkdirAll(filepath.Dir(urlPath), fs.FileMode(os.O_CREATE))
		if err != nil {
			l.Error("unable to make directories", "DIR", filepath.Dir(urlPath), "err", err)
			return fmt.Errorf("unable to make directories: %w", err)
		}
		l.Debug("made directory for file")
		file, err := os.OpenFile(urlPath, os.O_WRONLY|os.O_CREATE, fs.FileMode(os.O_CREATE))
		defer file.Close()
		if err != nil {
			l.Error("unable to open file", "file", urlPath, "err", err)
			return fmt.Errorf("unable to open file: %w", err)
		}
		l.Debug("opened file for downloading audio")

		// Copy the response body into the file.
		_, err = io.Copy(file, resp.Body)
		if err != nil {
			l.Error("unable to copy response body")
			return fmt.Errorf("unable to copy response body: %w", err)
		}
		l.Debug("downloaded file, getting lock")

		// Update the filePath to reflect new path.
		fileMu.Lock()
		defer fileMu.Unlock()
		l.Debug("got lock on file reading writing")
		*playPath = cachePath + urlPath
		err = setAudioPath(*playPath)
		if err != nil {
			l.Error("failed to set audio path", "error", err)
			return fmt.Errorf("failed to set audio path: %w", err)
		}
		l.Debug("New file path saved to file")
		return nil
	default:
		l.Debug("unsupported MIME subtype", "MIME", mime[1])
		return errUnsupportedType
	}
}

// setAudioPath takes the path of the currently playing audio file and saves
// it to /etc/netsender/speaker.conf. This location is used on boot as the
// default path to play from until new vars are received.
func setAudioPath(path string) error {
	return updateConf(func(c *cfgCache) { c.Path = path })
}

// getConf returns the contents (path to audio file) of /etc/netsender/speaker.json.
// If the file is empty, or cannot be read, the function will return the default audio path.
func getConf() (*cfgCache, error) {
	// Open the file.
	file, err := os.Open(confPath)
	defer file.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Parse the JSON.
	var cfg cfgCache
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &cfg, nil
}

// updateConf updates the cached vars for the device stored at /etc/netsender/speaker.json
// It applies the given function to modify the config.
func updateConf(u func(c *cfgCache)) error {
	cfg, err := getConf()
	if err != nil {
		return fmt.Errorf("unable to get config: %w", err)
	}

	u(cfg)

	// Marshall JSON.
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshall JSON: %w", err)
	}

	file, err := os.Create(confPath)
	if err != nil {
		return fmt.Errorf("could not open file: %w", err)
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("could not write data to file: %w", err)
	}
	return err
}

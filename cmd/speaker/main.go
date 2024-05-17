/*
DESCRIPTION
  speaker is a netsender client for audio playback, speaker control,
  and speaker health checking.

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
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
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
)

// Channel modes.
const (
	modeStereo = "Stereo"
	modeLeft   = "LeftMono"
	modeRight  = "RightMono"
	modeMute   = "Mute"
)

// Variable map to send to the cloud.
var varMap = map[string]string{
	"SpeakerMode":   "enum:" + strings.Join([]string{modeStereo, modeLeft, modeRight, modeMute}, ","),
	"AudioFilePath": "string",
}

const audioCmd = "aplay"

func initCommand(l logging.Logger) { checkPath(audioCmd, l) }

func main() {
	// Set up the player command with audio file path.
	filePtr := flag.String("path", "/home/pi/audio.wav", "Path to sound file we wish to play.")
	flag.Parse()

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

	if *filePtr == "" {
		log.Fatal("no file path provided, check usage")
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
	go playAudio(filePtr, log)

	// Start the control loop.
	log.Debug("starting control loop")
	run(rv, ns, filePtr, log, netLog)
}

// run starts a control loop that runs netsender, sends logs, checks for var changes, and
// if var changes, changes current mode (paused,audio playback or soundcheck)
func run(rv *revid.Revid, ns *netsender.Sender, file *string, l logging.Logger, nl *netlogger.Logger) {
	var vs int

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

		l.Debug("checking amplifier volume")
		v := vars["AmpVolume"]
		if v != "" {
			vol, err := strconv.ParseInt(v, 10, 8)
			if err != nil {
				l.Error(pkg+"failed to parse amplifier volume", "error", err)
			} else if vol < minAmpVolume || vol > maxAmpVolume {
				l.Error(fmt.Sprintf("%s invalid amplifier volume, must be between %v and %v", pkg, minAmpVolume, maxAmpVolume), "volume", vol)
			} else {
				bus := embd.NewI2CBus(i2cPort)
				err := bus.WriteByte(volAddr, byte(vol))
				if err != nil {
					l.Error(pkg+"failed to write amplifier volume", "error", err)
				}
			}
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
		l.Warning("invalid SpeakeMode", "SpeakerMode", mode)
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
		l.Info("command run", "stdout", outBuff)
	}()

	// Copy any std error to a buffer for logging.
	var errBuff bytes.Buffer
	go func() {
		_, err = io.Copy(&errBuff, errPipe)
		if err != nil {
			l.Error("failed to copy error pipe", "error", err)
		}
		l.Error("command failed", "stderr", errBuff)
	}()
	if errBuff.String() != "" {
		return fmt.Errorf("channel set command failed: %s", &errBuff)
	}

	l.Info("mode set to", "mode", mode)
	return nil
}

// playAudio is intended to be run as a routine. It will continuously run even while muted.
func playAudio(file *string, l logging.Logger) {
	var numPlays int
	for {
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

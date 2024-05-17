/*
DESCRIPTION
  rvcl is command line interface for revid. The user can provide configuration
  by passing a JSON string directly, or by specifying a file containing the JSON.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Scott Barnard <scott@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package rvcl is a command line interface for revid.
package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"runtime/pprof"

	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/revid"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/client/pi/netsender"
	"github.com/ausocean/utils/logging"
)

// Copyright information prefixed to all metadata.
const (
	metaPreambleKey  = "copyright"
	metaPreambleData = "ausocean.org/license/content2019"
)

// Logging configuration.
const (
	logLevel    = logging.Info
	logSuppress = true
)

// Misc consts.
const (
	pkg         = "rvcl: "
	profilePath = "rvcl.prof"
)

// Netsender conf consts.
const (
	cfgPath = "/etc/netsender.conf"
	fMode   = 0777
)

// Default config parameters.
const (
	defaultInput      = "File"
	defaultInputPath  = "../../../test/test-data/av/input/betterInput.h264"
	defaultFileFPS    = "25"
	defaultOutput     = "RTP"
	defaultRTPAddress = "localhost:6970"
	defaultLoop       = "true"
)

// canProfile is set to false with revid-cli is built with "-tags profile".
var canProfile = false

// The logger that will be used throughout.
var log logging.Logger

// stdoutLogger provides an io.Writer for the purpose of capturing stdout from
// the VLC process and using the logger to capture and print to stdout of
// this process.
type stdoutLogger struct {
	l logging.Logger
	t string
}

func (sl *stdoutLogger) Write(d []byte) (int, error) {
	sl.l.Info(sl.t + ": " + string(d))
	return len(d), nil
}

// stderrLogger provides an io.Writer for the purpose of capturing stderr from
// the VLC process and using the logger to capture and print to stdout of
// this process.
type stderrLogger struct {
	l logging.Logger
	t string
}

func (sl *stderrLogger) Write(d []byte) (int, error) {
	sl.l.Error(sl.t + ": " + string(d))
	return len(d), nil
}

func main() {
	mts.Meta = meta.NewWith([][2]string{{metaPreambleKey, metaPreambleData}})

	// Create logger that methods will be called on by the netsender client and
	// revid to log messages. Logs will go the lumberjack logger to handle file
	// writing of messages.
	log = logging.New(
		logLevel,
		os.Stdout,
		logSuppress,
	)

	// If built with profile tag, we will start CPU profiling.
	if canProfile {
		profile()
		defer pprof.StopCPUProfile()
	}

	// User can provide config through single JSON string flag, or through JSON file.
	var (
		configPtr     = flag.String("config", "", "Provide configuration JSON to revid (see readme for further information).")
		configFilePtr = flag.String("config-file", "", "Location of revid configuration file (see readme for further information).")
		rtpAddrPtr    = flag.String("rtp-addr", defaultRTPAddress, "RTP destination address (<ip>:<port>)(common port=6970)")
	)
	flag.Parse()

	// Create config map according to flags or file, and panic if both are defined.
	var (
		cfg map[string]string
		err error
	)
	switch {
	// This doesn't make sense so panic.
	case *configPtr != "" && *configFilePtr != "":
		panic("cannot define both command-line config and file config")

	// Decode JSON file to map.
	case *configPtr != "":
		err = json.Unmarshal([]byte(*configPtr), &cfg)
		if err != nil {
			log.Fatal("could not decode JSON config", "error", err)
		}

	// Decode JSON string to map from command line flag.
	case *configFilePtr != "":
		f, err := os.Open(*configFilePtr)
		if err != nil {
			log.Fatal("could not open config file", "error", err)
		}
		err = json.NewDecoder(f).Decode(&cfg)
		if err != nil {
			log.Fatal("could not decode JSON config", "error", err)
		}

	// No config information has been provided; provide a default config map.
	default:
		cfg = map[string]string{
			"Input":      defaultInput,
			"InputPath":  defaultInputPath,
			"FileFPS":    defaultFileFPS,
			"Output":     defaultOutput,
			"RTPAddress": *rtpAddrPtr,
			"Loop":       defaultLoop,
		}
	}
	log.Info("got config", "config", cfg)

	// Create a netsender client. This is used only for HTTP sending of media
	// in this binary.
	ns, err := netsender.New(log, nil, nil, nil)
	if err != nil {
		log.Fatal("could not initialise netsender client", "error", err)
	}

	// Create the revid client, responsible for media collection and processing.
	log.Info("got creating revid client")
	rv, err := revid.New(config.Config{Logger: log}, ns)
	if err != nil {
		log.Fatal("could not create revid", "error", err)
	}

	// Configure revid with configuration map obtained through flags or file.
	// If config is empty, defaults will be adopted by revid.
	log.Info("updating revid with config")
	err = rv.Update(cfg)
	if err != nil {
		log.Fatal("could not update revid config", "error", err)
	}

	log.Info("starting revid")
	err = rv.Start()
	if err != nil {
		log.Fatal("could not start revid", "error", err)
	}

	// If output is RTP, open up a VLC window to see stream.
	if v, ok := cfg["Output"]; ok && v == "RTP" {
		log.Info("opening vlc window")
		cmd := exec.Command("vlc", "rtp://"+*rtpAddrPtr)
		cmd.Stdout = &stdoutLogger{log, "VLC STDOUT"}
		cmd.Stderr = &stderrLogger{log, "VLC STDERR"}
		err = cmd.Start()
		if err != nil {
			log.Fatal("could not run vlc command", "error", err)
		}
	}

	// Run indefinitely.
	select {}
}

// profile creates a file to hold CPU profile metrics and begins CPU profiling.
func profile() {
	f, err := os.Create(profilePath)
	if err != nil {
		log.Fatal(pkg+"could not create CPU profile", "error", err.Error())
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal(pkg+"could not start CPU profile", "error", err.Error())
	}
}

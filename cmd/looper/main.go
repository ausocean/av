/*
DESCRIPTION
  Looper is a program that loops an audio file.

AUTHORS
  Ella Pietraroia <ella@ausocean.org>
  Scott Barnard <scott@ausocean.org>
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package looper is a bare bones program for repeated playback of an audio file.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"time"

	"github.com/ausocean/client/pi/netlogger"
	"github.com/ausocean/client/pi/netsender"
	"github.com/ausocean/client/pi/sds"
	"github.com/ausocean/utils/logging"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logging related constants.
const (
	logPath      = "/var/log/audiolooper/audiolooper.log"
	logMaxSize   = 500 // MB
	logMaxBackup = 10
	logMaxAge    = 28 // days
	logVerbosity = logging.Debug
	logSuppress  = true
)

// Netsender related consts.
const (
	netSendRetryTime = 5 * time.Second
	defaultSleepTime = 60 // Seconds
)

// Looper modes.
const (
	modeNormal = "Normal"
	modePaused = "Paused"
)

func main() {
	filePtr := flag.String("path", "", "Path to sound file we wish to play.")
	flag.Parse()

	// Create lumberjack logger to handle logging to file.
	fileLog := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    logMaxSize,
		MaxBackups: logMaxBackup,
		MaxAge:     logMaxAge,
	}

	// Create a netlogger to deal with logging to cloud.
	nl := netlogger.New()

	// Create logger that we call methods on to l.
	l := logging.New(logVerbosity, io.MultiWriter(fileLog, nl), logSuppress)

	// Call initialisation code that is specific to the platform (pi 0 or 3).
	initCommand(l)

	// Create netsender client.
	ns, err := netsender.New(l, nil, readPin(), nil)
	if err != nil {
		l.Fatal("could not initialise netsender client", "error", err)
	}

	// This routine will deal with things that need to happen with the netsender client.
	go run(ns, l, nl)

	// Repeatedly play audio file.
	var numPlays int
	for {
		cmd := exec.Command(audioCmd, *filePtr)

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
			l.Error("start failed", "error", err.Error())
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
			l.Error("failed to wait for execution finish", "error", err.Error())
		}
		l.Debug("stdout received", "stdout", string(outBuff.Bytes()))

		// If there was any errors on stderr, l them.
		if errBuff.Len() != 0 {
			l.Error("errors from stderr", "stderr", string(errBuff.Bytes()))
		}
	}
}

// run is a routine to deal with netsender related tasks.
func run(ns *netsender.Sender, l logging.Logger, nl *netlogger.Logger) {
	var vs int
	for {
		err := ns.Run()
		if err != nil {
			l.Warning("Run Failed. Retrying...", "error", err)
			time.Sleep(netSendRetryTime)
			continue
		}

		err = nl.Send(ns)
		if err != nil {
			l.Warning("Logs could not be sent", "error", err.Error())
		}

		// If var sum hasn't changed we skip rest of loop.
		newVs := ns.VarSum()
		if vs == newVs {
			sleep(ns, l)
			continue
		}
		vs = newVs

		vars, err := ns.Vars()
		if err != nil {
			l.Error("netSender failed to get vars", "error", err)
			time.Sleep(netSendRetryTime)
			continue
		}

		// Configure looper based on vars.
		err = update(vars)
		if err != nil {
			l.Warning("couldn't update with new vars", "error", err)
			sleep(ns, l)
			continue
		}

		// TODO: consider handling of any modes ? We'd likely have paused and
		// normal for the audio looper.
		switch ns.Mode() {
		case modePaused:
		case modeNormal:
		}
	}
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

// sleep uses a delay to halt the program based on the monitoring period
// netsender parameter (mp) defined in the netsender.conf config.
func sleep(ns *netsender.Sender, l logging.Logger) {
	t, err := strconv.Atoi(ns.Param("mp"))
	if err != nil {
		l.Error("could not get sleep time, using default", "error", err)
		t = defaultSleepTime
	}
	time.Sleep(time.Duration(t) * time.Second)
}

// readPin provides a callback function of consistent signature for use by
// netsender to retrieve software defined pin values.
func readPin() func(pin *netsender.Pin) error {
	return func(pin *netsender.Pin) error {
		switch {
		case pin.Name == "X23":
			pin.Value = -1
		case pin.Name[0] == 'X':
			return sds.ReadSystem(pin)
		default:
			pin.Value = -1
		}
		return nil
	}
}

// update is currently a stub, but might used to update looper related params
// in future.
func update(v map[string]string) error {
	return nil
}

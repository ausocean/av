/*
DESCRIPTION
  rv is a netsender client using the revid package to perform media collection
  and forwarding whose behaviour is controllable via the cloud.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Jack Richardson <jack@ausocean.org>
  Trek Hopton <trek@ausocean.org>
  Scott Barnard <scott@ausocean.org>
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

// Package rv is a netsender client for revid.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/pprof"
	"strconv"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/revid"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/client/pi/netlogger"
	"github.com/ausocean/client/pi/netsender"
	"github.com/ausocean/utils/logging"
)

// Current software version.
const version = "v1.6.1"

// Copyright information prefixed to all metadata.
const (
	metaPreambleKey  = "copyright"
	metaPreambleData = "ausocean.org/license/content2019"
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

// Revid modes.
const (
	modeNormal    = "Normal"
	modePaused    = "Paused"
	modeBurst     = "Burst"
	modeLoop      = "Loop"
	modeShutdown  = "Shutdown"
	modeCompleted = "Completed"
)

// Misc constants.
const (
	netSendRetryTime = 5 * time.Second
	defaultSleepTime = 60 // Seconds
	profilePath      = "rv.prof"
	pkg              = "rv: "
	runPreDelay      = 20 * time.Second
	rebootCmd        = "syncreboot"
)

// Software define pin values.
const (
	bitratePin   = "X36"
	sharpnessPin = "X38"
	contrastPin  = "X39"
)

// This is set to true if the 'profile' build tag is provided on build.
var canProfile = false

func main() {
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	mts.Meta = meta.NewWith([][2]string{{metaPreambleKey, metaPreambleData}})

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

	log.Info("starting rv", "version", version)

	// If rv has been built with the profile tag, then we'll start a CPU profile.
	if canProfile {
		profile(log)
		defer pprof.StopCPUProfile()
		log.Info("profiling started")
	}

	var (
		rv *revid.Revid
		p  *turbidityProbe
	)

	p, err := NewTurbidityProbe(log, 60*time.Second)
	if err != nil {
		log.Fatal("could not create new turbidity probe", "error", err.Error())
	}

	log.Debug("initialising netsender client")
	ns, err := netsender.New(
		log,
		nil,
		readPin(p, rv, log),
		nil,
		netsender.WithVarTypes(createVarMap()),
	)
	if err != nil {
		log.Fatal(pkg + "could not initialise netsender client: " + err.Error())
	}

	log.Debug("initialising revid")
	rv, err = revid.New(config.Config{Logger: log}, ns)
	if err != nil {
		log.Fatal(pkg+"could not initialise revid", "error", err.Error())
	}

	err = rv.SetProbe(p)
	if err != nil {
		log.Error(pkg+"could not set probe", "error", err.Error())
	}

	// NB: Problems were encountered with communicating with RTSP inputs. When trying to
	// connect it would fail due to timeout; as if things had not been set up quickly
	// enough before revid tried to do things. This delay fixes this, but there is probably
	// a better way to solve this problem.
	time.Sleep(runPreDelay)

	log.Debug("beginning main loop")
	run(rv, ns, log, netLog, p)
}

// run starts the main loop. This will run netsender on every pass of the loop
// (sleeping inbetween), check vars, and if changed, update revid as appropriate.
func run(rv *revid.Revid, ns *netsender.Sender, l logging.Logger, nl *netlogger.Logger, p *turbidityProbe) {
	var vs int
	for {
		l.Debug("running netsender")
		err := ns.Run()
		if err != nil {
			l.Warning(pkg+"Run Failed. Retrying...", "error", err.Error())
			time.Sleep(netSendRetryTime)
			continue
		}

		l.Debug("sending logs")
		err = nl.Send(ns)
		if err != nil {
			l.Warning(pkg+"Logs could not be sent", "error", err.Error())
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
			l.Error(pkg+"netSender failed to get vars", "error", err.Error())
			time.Sleep(netSendRetryTime)
			continue
		}
		l.Debug("got new vars", "vars", vars)

		// Configure revid based on the vars.
		l.Debug("updating revid's configuration")
		err = rv.Update(vars)
		if err != nil {
			l.Warning(pkg+"couldn't update revid", "error", err.Error())
			sleep(ns, l)
			continue
		}
		l.Info("revid successfully reconfigured")

		// Update transform matrix based on new revid variables.
		err = p.Update(rv.Config().TransformMatrix)
		if err != nil {
			l.Error("could not update turbidity probe", "error", err.Error())
		}

		l.Debug("checking mode")
		switch ns.Mode() {
		case modePaused, modeCompleted:
			l.Debug("mode is Paused or Completed, stopping revid")
			rv.Stop()
		case modeNormal, modeLoop:
			l.Debug("mode is Normal or Loop, starting revid")
			err = rv.Start()
			if err != nil {
				l.Error(pkg+"could not start revid", "error", err.Error())
				ns.SetMode(modePaused)
				sleep(ns, l)
				continue
			}
		case modeBurst:
			l.Debug("mode is Burst, bursting revid")
			err = rv.Burst()
			if err != nil {
				l.Warning(pkg+"could not start burst", "error", err.Error())
				ns.SetMode(modePaused)
				sleep(ns, l)
				continue
			}
			ns.SetMode(modePaused)
		case modeShutdown:
			l.Debug("mode is Shutdown, shutting down")
			rv.Stop()
			ns.SetMode(modePaused)
			out, err := exec.Command(rebootCmd, "-s=true").CombinedOutput()
			if err != nil {
				l.Warning("could not use syncreboot to shutdown, out: %s, err: %w", string(out), err)
			}
		}
		l.Info("revid updated with new mode")

		sleep(ns, l)
	}
}

func createVarMap() map[string]string {
	m := make(map[string]string)
	for _, v := range config.Variables {
		m[v.Name] = v.Type
	}
	return m
}

// profile opens a file to hold CPU profiling metrics and then starts the
// CPU profiler.
func profile(l logging.Logger) {
	f, err := os.Create(profilePath)
	if err != nil {
		l.Fatal(pkg+"could not create CPU profile", "error", err.Error())
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		l.Fatal(pkg+"could not start CPU profile", "error", err.Error())
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

// readPin provides a callback function of consistent signature for use by
// netsender to retrieve software defined pin values e.g. revid bitrate.
func readPin(p *turbidityProbe, rv *revid.Revid, l logging.Logger) func(pin *netsender.Pin) error {
	return func(pin *netsender.Pin) error {
		switch {
		case pin.Name == bitratePin:
			pin.Value = -1
			if rv != nil {
				pin.Value = rv.Bitrate()
			}
		case pin.Name == sharpnessPin:
			pin.Value = -1
			if p != nil {
				l.Debug("setting sharpness value", "sharpness", p.sharpness*1000)
				pin.Value = int(p.sharpness * 1000)
			}
		case pin.Name == contrastPin:
			pin.Value = -1
			if p != nil {
				l.Debug("setting contrast pin", "contrast", p.contrast)
				pin.Value = int(p.contrast * 100)
			}
		}
		return nil
	}
}

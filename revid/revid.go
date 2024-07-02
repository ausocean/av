/*
NAME
  revid.go

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Trek Hopton <trek@ausocean.org>
  Scott Barnard <scott@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

// Package revid provides an API for reading, transcoding, and writing audio/video streams and files.
package revid

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ausocean/av/device"
	"github.com/ausocean/av/filter"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/client/pi/netsender"
	"github.com/ausocean/utils/bitrate"
)

// Misc consts.
const (
	poolStartingElementSize = 10000 // Bytes.
	rtmpConnectionMaxTries  = 5
)

type Logger interface {
	SetLevel(int8)
	Log(level int8, message string, params ...interface{})
}

// Revid provides methods to control a revid session; providing methods
// to start, stop and change the state of an instance using the Config struct.
type Revid struct {
	// config holds the Revid configuration.
	// For historical reasons it also handles logging.
	// FIXME(kortschak): The relationship of concerns
	// in config/ns is weird.
	cfg config.Config

	// ns holds the netsender.Sender responsible for HTTP.
	ns *netsender.Sender

	// input will capture audio or video from which we can read data.
	input device.AVDevice

	// closeInput holds the cleanup function return from setupInput and is called
	// in Revid.Stop().
	closeInput func() error

	// lexTo, encoder and packer handle transcoding the input stream.
	lexTo func(dest io.Writer, src io.Reader, delay time.Duration) error

	// probe allows us to "probe" frames after being lexed before going off to
	// later encoding stages. This is useful if we wish to perform some processing
	// on frames to derive metrics, for example, we might like to probe frames to
	// derive turbidity levels. This is provided through SetProbe.
	probe io.WriteCloser

	// filters will hold the filter interface that will write to the chosen filter from the lexer.
	filters []filter.Filter

	// encoders will hold the multiWriteCloser that writes to encoders from the filter.
	encoders io.WriteCloser

	// running is used to keep track of revid's running state between methods.
	running bool

	// wg will be used to wait for any processing routines to finish.
	wg sync.WaitGroup

	// err will channel errors from revid routines to the handle errors routine.
	err chan error

	// bitrate is used for bitrate calculations.
	bitrate bitrate.Calculator

	// stop is used to signal stopping when looping an input.
	stop chan struct{}
}

// New returns a pointer to a new Revid with the desired configuration, and/or
// an error if construction of the new instance was not successful.
func New(c config.Config, ns *netsender.Sender) (*Revid, error) {
	r := Revid{ns: ns, err: make(chan error)}
	err := r.setConfig(c)
	if err != nil {
		return nil, fmt.Errorf("could not set config, failed with error: %w", err)
	}
	go r.handleErrors()
	return &r, nil
}

// Config returns a copy of revids current config.
func (r *Revid) Config() config.Config {
	return r.cfg
}

// Bitrate returns the result of the  most recent bitrate check.
func (r *Revid) Bitrate() int {
	return r.bitrate.Bitrate()
}

func (r *Revid) Write(p []byte) (int, error) {
	mi, ok := r.input.(*device.ManualInput)
	if !ok {
		return 0, errors.New("cannot write to anything but ManualInput")
	}
	return mi.Write(p)
}

// Start invokes a Revid to start processing video from a defined input
// and packetising (if theres packetization) to a defined output.
func (r *Revid) Start() error {
	if r.running {
		r.cfg.Logger.Warning("start called, but revid already running")
		return nil
	}

	r.stop = make(chan struct{})

	r.cfg.Logger.Debug("resetting revid")
	err := r.reset(r.cfg)
	if err != nil {
		r.Stop()
		return err
	}
	r.cfg.Logger.Info("revid reset")

	// Calculate delay between frames if the FileFPS != 0. Otherwise use no delay.
	d := time.Duration(0)
	if r.cfg.FileFPS != 0 {
		d = time.Duration(1000/r.cfg.FileFPS) * time.Millisecond
	}

	r.cfg.Logger.Debug("starting input processing routine")
	r.wg.Add(1)
	go r.processFrom(d)

	r.running = true
	return nil
}

// Stop closes down the pipeline. This closes encoders and sender output routines,
// connections, and/or files.
func (r *Revid) Stop() {
	if !r.running {
		r.cfg.Logger.Warning("stop called but revid isn't running")
		return
	}

	close(r.stop)

	r.cfg.Logger.Debug("stopping input")
	err := r.input.Stop()
	if err != nil {
		r.cfg.Logger.Error("could not stop input", "error", err.Error())
	} else {
		r.cfg.Logger.Info("input stopped")
	}

	r.cfg.Logger.Debug("closing pipeline")
	err = r.encoders.Close()
	if err != nil {
		r.cfg.Logger.Error("failed to close pipeline", "error", err.Error())
	} else {
		r.cfg.Logger.Info("pipeline closed")
	}

	for _, filter := range r.filters {
		err = filter.Close()
		if err != nil {
			r.cfg.Logger.Error("failed to close filters", "error", err.Error())
		} else {
			r.cfg.Logger.Info("filters closed")
		}
	}

	r.cfg.Logger.Debug("waiting for routines to finish")
	r.wg.Wait()
	r.cfg.Logger.Info("routines finished")

	r.running = false
}

// Burst starts revid, waits for time specified, and then stops revid.
func (r *Revid) Burst() error {
	r.cfg.Logger.Debug("starting revid")
	err := r.Start()
	if err != nil {
		return fmt.Errorf("could not start revid: %w", err)
	}
	r.cfg.Logger.Info("revid started")

	dur := time.Duration(r.cfg.BurstPeriod) * time.Second
	time.Sleep(dur)

	r.cfg.Logger.Debug("stopping revid")
	r.Stop()
	r.cfg.Logger.Info("revid stopped")

	return nil
}

func (r *Revid) Running() bool {
	return r.running
}

// Update takes a map of variables and their values and edits the current config
// if the variables are recognised as valid parameters.
func (r *Revid) Update(vars map[string]string) error {
	if r.running {
		r.cfg.Logger.Debug("revid running; stopping for re-config")
		r.Stop()
		r.cfg.Logger.Info("revid was running; stopped for re-config")
	}

	//look through the vars and update revid where needed
	r.cfg.Logger.Debug("checking vars from server", "vars", vars)
	r.cfg.Update(vars)
	r.cfg.Logger.Info("finished reconfig")
	r.cfg.Logger.Debug("config changed", "config", r.cfg)
	return nil
}

func (r *Revid) SetProbe(p io.WriteCloser) error {
	if r.running {
		return errors.New("cannot set probe when revid is running")
	}
	r.probe = p
	return nil
}

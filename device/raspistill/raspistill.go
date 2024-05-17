/*
DESCRIPTION
  raspistill.go provides an implementation of the AVDevice interface for the
  raspistill raspberry pi camera interfacing utility. This allows for the
  capture of single frames over time, i.e. a timelapse form of capture.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package raspistill rovides an implementation of the AVDevice interface for the
// raspistill raspberry pi camera interfacing utility. This allows for the
// capture of single frames over time, i.e. a timelapse form of capture.
package raspistill

import (
	"errors"
	"fmt"
	"time"

	"github.com/ausocean/av/device"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

// To indicate package when logging.
const pkg = "raspistill: "

// Config field validation bounds.
const (
	minTimelapseDuration = 10 * time.Second    // s
	maxTimelapseDuration = 86400 * time.Second // s = 24 hours
	minTimelapseInterval = 1 * time.Second     // s
	maxTimelapseInterval = 86400 * time.Second // s = 24 hours
)

// Raspistill configuration defaults.
const (
	defaultRotation          = 0                    // degrees
	defaultWidth             = 1280                 // pixels
	defaultHeight            = 720                  // pixels
	defaultJPEGQuality       = 75                   // %
	defaultTimelapseDuration = maxTimelapseDuration // ms
	defaultTimelapseInterval = 3600 * time.Second   // ms
)

// Configuration errors.
var (
	errBadRotation          = fmt.Errorf("Rotation bad or unset, defaulting to: %v", defaultRotation)
	errBadWidth             = fmt.Errorf("Width bad or unset, defaulting to: %v", defaultWidth)
	errBadHeight            = fmt.Errorf("Height bad or unset, defaulting to: %v", defaultHeight)
	errBadJPEGQuality       = fmt.Errorf("JPEGQuality bad or unset, defaulting to: %v", defaultJPEGQuality)
	errBadTimelapseDuration = fmt.Errorf("TimelapseDuration bad or unset, defaulting to: %v", defaultTimelapseDuration)
	errBadTimelapseInterval = fmt.Errorf("TimelapseInterval bad or unset, defaulting to: %v", defaultTimelapseInterval)
)

// Misc errors.
var errNotStarted = errors.New("cannot read, raspistill not started")

// Raspistill is an implementation of AVDevice that provides control over the
// raspistill utility for using the raspberry pi camera for the capture of
// singular images.
type Raspistill struct{ raspistill }

// New returns a new Raspistill.
func New(l logging.Logger) *Raspistill { return &Raspistill{raspistill: new(l)} }

// Start will prepare the arguments for the raspistill command using the
// configuration set using the Set method then call the raspistill command,
// piping the image output from which the Read method will read from.
func (r *Raspistill) Start() error { return r.start() }

// Read implements io.Reader. Calling read before Start has been called will
// result in 0 bytes read and an error.
func (r *Raspistill) Read(p []byte) (int, error) { return r.read(p) }

// Stop will terminate the raspistill process and close the output pipe.
func (r *Raspistill) Stop() error { return r.stop() }

// IsRunning is used to determine if the pi's camera is running.
func (r *Raspistill) IsRunning() bool { return r.isRunning }

// Name returns the name of the device.
func (r *Raspistill) Name() string { return "Raspistill" }

// Set will take a Config struct, check the validity of the relevant fields
// and then performs any configuration necessary. If fields are not valid,
// an error is added to the multiError and a default value is used.
func (r *Raspistill) Set(c config.Config) error {
	var errs device.MultiError

	if c.Rotation > 359 {
		c.Rotation = defaultRotation
		errs = append(errs, errBadRotation)
	}

	if c.Width == 0 {
		c.Width = defaultWidth
		errs = append(errs, errBadWidth)
	}

	if c.Height == 0 {
		c.Height = defaultHeight
		errs = append(errs, errBadHeight)
	}

	if c.JPEGQuality < 0 || c.JPEGQuality > 100 {
		c.JPEGQuality = defaultJPEGQuality
		errs = append(errs, errBadJPEGQuality)
	}

	if c.TimelapseDuration > maxTimelapseDuration || c.TimelapseDuration < minTimelapseDuration {
		c.TimelapseDuration = defaultTimelapseDuration
		errs = append(errs, errBadTimelapseDuration)
	}

	if c.TimelapseInterval > maxTimelapseInterval || c.TimelapseInterval < minTimelapseInterval {
		c.TimelapseInterval = defaultTimelapseInterval
		errs = append(errs, errBadTimelapseInterval)
	}

	r.cfg = c
	if len(errs) != 0 {
		return errs
	}
	return nil
}

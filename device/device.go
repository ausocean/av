/*
DESCRIPTION
  device.go provides AVDevice, an interface that describes a configurable
  audio or video device that can be started and stopped from which data may
  be obtained.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package device provides an interface and implementations for input devices
// that can be started and stopped from which media data can be obtained.
package device

import (
	"fmt"
	"io"
	"errors"

	"github.com/ausocean/av/revid/config"
)

// AVDevice describes a configurable audio or video device from which media data
// can be obtained. AVDevice is an io.Reader.
type AVDevice interface {
	io.Reader

	// Name returns the name of the AVDevice.
	Name() string

	// Set allows for configuration of the AVDevice using a Config struct. All,
	// some or none of the fields of the Config struct may be used for configuration
	// by an implementation. An implementation should specify what fields are
	// considered.
	Set(c config.Config) error

	// Start will start the AVDevice capturing media data; after which the Read
	// method may be called to obtain the data. The format of the data may differ
	// and should be specified by the implementation.
	Start() error

	// Stop will stop the AVDevice from capturing media data. From this point
	// Reads will no longer be successful.
	Stop() error

	// IsRunning is used to determine if the device is running.
	IsRunning() bool
}

// multiError implements the built in error interface. multiError is used here
// to collect multi errors during validation of configruation parameters for o
// AVDevices.
type MultiError []error

func (me MultiError) Error() string {
	if len(me) == 0 {
		panic("device: invalid use of MultiError")
	}
	return fmt.Sprintf("%v", []error(me))
}

// ManualInput is an implementation of the Devices interface that represents
// a manual input mechanism, i.e. data is written to this input manually through
// software (ManualInput also implements io.Writer, unlike other implementations).
// The ManualInput employs an io.Pipe, as such, every write must be accompanied
// by a full read (or reads) of the bytes, otherwise blocking will occur (and
// vice versa). This is intended to make writing of distinct access units easier i.e.
// one whole read (with a big enough buf provided) can represent a distinct frame.
type ManualInput struct {
	isRunning bool
	reader    *io.PipeReader
	writer    *io.PipeWriter
}

// NewManualInput provides a new ManualInput.
func NewManualInput() *ManualInput {
	return &ManualInput{}
}

// Read reads from the manual input and puts the bytes into p.
func (m *ManualInput) Read(p []byte) (int, error) {
	if !m.isRunning {
		return 0, errors.New("manual input has not been started, can't read")
	}
	return m.reader.Read(p)
}

// Name returns the name of ManualInput i.e. "ManualInput".
func (m *ManualInput) Name() string { return "ManualInput" }

// Set is a stub to satisfy the Device interface; no configuration fields are
// required by ManualInput.
func (m *ManualInput) Set(c config.Config) error { return nil }

// Start sets the ManualInput isRunning flag to true. This is mostly here just
// to satisfy the Device interface.
func (m *ManualInput) Start() error {
	m.isRunning = true
	m.reader, m.writer = io.Pipe()
	return nil
}

// Stop closes the pipe and sets the isRunning flag to false.
func (m *ManualInput) Stop() error {
	if m.reader != nil {
		m.reader.Close()
	}
	m.isRunning = false
	return nil
}

// IsRunning returns the value of the isRunning flag to indicate if Start has
// been called (and Stop has not been called after).
func (m *ManualInput) IsRunning() bool { return m.isRunning }

// Write writes p to the ManualInput's writer side of its pipe.
func (m *ManualInput) Write(p []byte) (int, error) {
	if !m.isRunning {
		return 0, errors.New("manual input has not been started, can't write")
	}
	return m.writer.Write(p)
}

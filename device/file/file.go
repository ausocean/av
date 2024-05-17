/*
DESCRIPTION
  file.go provides an implementation of the AVDevice interface for media files.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package file provides an implementation of AVDevice for files.
package file

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

// AVFile is an implementation of the AVDevice interface for a file containg
// audio or video data.
type AVFile struct {
	f         *os.File
	path      string
	loop      bool
	isRunning bool
	log       logging.Logger
	set       bool
	mu        sync.Mutex
}

// NewAVFile returns a new AVFile.
func New(l logging.Logger) *AVFile { return &AVFile{log: l} }

// NewWith returns a new AVFile with required params provided i.e. the Set
// method does not need to be called.
func NewWith(l logging.Logger, path string, loop bool) *AVFile {
	return &AVFile{log: l, path: path, loop: loop, set: true}
}

// Name returns the name of the device.
func (m *AVFile) Name() string {
	return "File"
}

// Set simply sets the AVFile's config to the passed config.
func (m *AVFile) Set(c config.Config) error {
	m.path = c.InputPath
	m.loop = c.Loop
	m.set = true
	return nil
}

// Start will open the file at the location of the InputPath field of the
// config struct.
func (m *AVFile) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var err error
	if !m.set {
		return errors.New("AVFile has not been set with config")
	}
	m.f, err = os.Open(m.path)
	if err != nil {
		return fmt.Errorf("could not open media file: %w", err)
	}
	m.isRunning = true
	return nil
}

// Stop will close the file such that any further reads will fail.
func (m *AVFile) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.f.Close()
	if err == nil {
		m.isRunning = false
		return nil
	}
	return err
}

// Read implements io.Reader. If start has not been called, or Start has been
// called and Stop has since been called, an error is returned.
func (m *AVFile) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.f == nil {
		return 0, errors.New("AV file is closed, AVFile not started")
	}

	n, err := m.f.Read(p)
	if err != nil && err != io.EOF {
		return n, err
	}

	if (n < len(p) || err == io.EOF) && m.loop {
		m.log.Info("looping input file")
		// In the case that we reach end of file but loop is true, we want to
		// seek to start and keep reading from there.
		_, err = m.f.Seek(0, io.SeekStart)
		if err != nil {
			return 0, fmt.Errorf("could not seek to start of file for input loop: %w", err)
		}

		// Now that we've seeked to start, let's try reading again.
		n, err = m.f.Read(p)
		if err != nil {
			return n, fmt.Errorf("could not read after start seek: %w", err)
		}
	}
	return n, err
}

// IsRunning is used to determine if the AVFile device is running.
func (m *AVFile) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.f != nil && m.isRunning
}

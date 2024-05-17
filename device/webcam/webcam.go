/*
DESCRIPTION
  webcam.go provides an implementation of AVDevice for webcams.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package webcam provides an implementation of AVDevice for webcams.
package webcam

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/device"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

// Used to indicate package in logging.
const pkg = "webcam: "

// Configuration defaults.
const (
	defaultInputPath = "/dev/video0"
	defaultFrameRate = 25
	defaultBitrate   = 400
	defaultWidth     = 1280
	defaultHeight    = 720
)

// Configuration field errors.
var (
	errBadFrameRate = errors.New("frame rate bad or unset, defaulting")
	errBadBitrate   = errors.New("bitrate bad or unset, defaulting")
	errBadWidth     = errors.New("width bad or unset, defaulting")
	errBadHeight    = errors.New("height bad or unset, defaulting")
	errBadInputPath = errors.New("input path bad or unset, defaulting")
)

// Webcam is an implementation of the AVDevice interface for a Webcam. Webcam
// uses an ffmpeg process to pipe the video data from the webcam.
type Webcam struct {
	out       io.ReadCloser
	log       logging.Logger
	cfg       config.Config
	cmd       *exec.Cmd
	done      chan struct{}
	isRunning bool
}

// New returns a new Webcam.
func New(l logging.Logger) *Webcam {
	return &Webcam{
		log:  l,
		done: make(chan struct{}),
	}
}

// Name returns the name of the device.
func (w *Webcam) Name() string {
	return "Webcam"
}

// Set will validate the relevant fields of the given Config struct and assign
// the struct to the Webcam's Config. If fields are not valid, an error is
// added to the multiError and a default value is used.
func (w *Webcam) Set(c config.Config) error {
	var errs device.MultiError
	if c.InputPath == "" {
		const defaultInputPath = "/dev/video0"
		errs = append(errs, errBadInputPath)
		c.InputPath = defaultInputPath
	}

	if c.Width == 0 {
		errs = append(errs, errBadWidth)
		c.Width = defaultWidth
	}

	if c.Height == 0 {
		errs = append(errs, errBadHeight)
		c.Height = defaultHeight
	}

	if c.FrameRate == 0 {
		errs = append(errs, errBadFrameRate)
		c.FrameRate = defaultFrameRate
	}

	if c.Bitrate <= 0 {
		errs = append(errs, errBadBitrate)
		c.Bitrate = defaultBitrate
	}
	w.cfg = c
	if len(errs) != 0 {
		return errs
	}
	return nil
}

// Start will build the required arguments for ffmpeg and then execute the
// command, piping video output where we can read using the Read method.
func (w *Webcam) Start() error {
	br := w.cfg.Bitrate * 1000

	args := []string{
		"-i", w.cfg.InputPath,
		"-r", fmt.Sprint(w.cfg.FrameRate),
		"-b:v", fmt.Sprint(br),
		"-s", fmt.Sprintf("%dx%d", w.cfg.Width, w.cfg.Height),
	}

	switch w.cfg.InputCodec {
	default:
		return fmt.Errorf("revid: invalid input codec: %v", w.cfg.InputCodec)
	case codecutil.H264:
		args = append(args,
			"-f", "h264",
			"-maxrate", fmt.Sprint(br),
			"-bufsize", fmt.Sprint(br/2),
		)
	case codecutil.MJPEG:
		args = append(args,
			"-f", "mjpeg",
		)
	}

	args = append(args,
		"-",
	)

	w.log.Info(pkg+"ffmpeg args", "args", strings.Join(args, " "))
	w.cmd = exec.Command("ffmpeg", args...)

	var err error
	w.out, err = w.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	stderr, err := w.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not pipe command error: %w", err)
	}

	w.isRunning = true

	go func() {
		for {
			select {
			case <-w.done:
				w.cfg.Logger.Info("webcam.Stop() called, finished checking stderr")
				return
			default:
				buf, err := ioutil.ReadAll(stderr)
				if err != nil {
					w.cfg.Logger.Error("could not read stderr", "error", err)
					return
				}

				if len(buf) != 0 {
					w.cfg.Logger.Error("error from webcam stderr", "error", string(buf))
					return
				}
			}
		}
	}()

	w.cfg.Logger.Info("starting webcam")
	err = w.cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}
	w.cfg.Logger.Info("webcam started")

	return nil
}

// Stop will kill the ffmpeg process and close the output pipe.
func (w *Webcam) Stop() error {
	if !w.isRunning {
		return nil
	}
	w.isRunning = false
	close(w.done)
	if w.cmd == nil || w.cmd.Process == nil {
		return errors.New("ffmpeg process was never started")
	}
	err := w.cmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("could not kill ffmpeg process: %w", err)
	}
	return w.out.Close()
}

// Read implements io.Reader. If the pipe is nil a read error is returned.
func (w *Webcam) Read(p []byte) (int, error) {
	if w.out != nil {
		return w.out.Read(p)
	}
	return 0, errors.New("webcam not streaming")
}

// IsRunning is used to determine if the webcam is running.
func (w *Webcam) IsRunning() bool {
	return w.isRunning
}

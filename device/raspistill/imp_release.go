// +build !test

/*
DESCRIPTION
  release.go provides implementations for the Raspistill struct for release
  conditions, i.e. we're running on a raspberry pi with access to the actual
  raspistill utility with a pi camera connected. The code here runs a raspistill
  background process and reads captured images from the camera.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package raspistill

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

type raspistill struct {
	cfg       config.Config
	cmd       *exec.Cmd
	out       io.ReadCloser
	log       logging.Logger
	done      chan struct{}
	isRunning bool
}

func new(l logging.Logger) raspistill {
	return raspistill{
		log:  l,
		done: make(chan struct{}),
	}
}

func (r *Raspistill) stop() error {
	if r.isRunning == false {
		return nil
	}
	close(r.done)
	if r.cmd == nil || r.cmd.Process == nil {
		return errors.New("raspistill process was never started")
	}
	err := r.cmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("could not kill raspistill process: %w", err)
	}
	r.isRunning = false
	return r.out.Close()
}

func (r *Raspistill) start() error {
	args := []string{
		"--output", "-",
		"--nopreview",
		"--width", fmt.Sprint(r.cfg.Width),
		"--height", fmt.Sprint(r.cfg.Height),
		"--rotation", fmt.Sprint(r.cfg.Rotation),
		"--timeout", fmt.Sprint(r.cfg.TimelapseDuration),
		"--timelapse", fmt.Sprint(r.cfg.TimelapseInterval),
		"--quality", fmt.Sprint(r.cfg.JPEGQuality),
	}

	r.log.Info(pkg+"raspistill args", "args", strings.Join(args, " "))
	r.cmd = exec.Command("raspistill", args...)

	var err error
	r.out, err = r.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not pipe command output: %w", err)
	}

	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not pipe command error: %w", err)
	}

	go func() {
		errScnr := bufio.NewScanner(stderr)
		for {
			select {
			case <-r.done:
				r.log.Info("raspistill.Stop() called, finished checking stderr")
				return
			default:
			}

			if errScnr.Scan() {
				r.log.Error("error line from raspistill stderr", "error", errScnr.Text())
				continue
			}

			err := errScnr.Err()
			if err != nil {
				r.log.Error("error from stderr scan", "error", err)
			}
		}
	}()

	err = r.cmd.Start()
	if err != nil {
		return fmt.Errorf("could not start raspistill process: %w", err)
	}
	r.isRunning = true

	return nil
}

func (r *Raspistill) read(p []byte) (int, error) {
	if r.out != nil {
		return r.out.Read(p)
	}
	return 0, errNotStarted
}

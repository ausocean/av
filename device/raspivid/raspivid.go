/*
DESCRIPTION
  raspivid.go provides an implementation of the AVDevice interface for raspivid.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package raspivid provides an implementation of AVDevice for the raspberry
// pi camera.
package raspivid

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/device"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
	"github.com/ausocean/utils/sliceutils"
)

// To indicate package when logging.
const pkg = "raspivid: "

// Raspivid configuration defaults.
const (
	defaultRaspividCodec            = codecutil.H264
	defaultRaspividRotation         = 0
	defaultRaspividWidth            = 1280
	defaultRaspividHeight           = 720
	defaultRaspividBrightness       = 50
	defaultRaspividSaturation       = 0
	defaultRaspividExposure         = "auto"
	defaultRaspividAutoWhiteBalance = "auto"
	defaultRaspividMinFrames        = 100
	defaultRaspividQuantization     = 30
	defaultRaspividBitrate          = 4800
	defaultRaspividFramerate        = 25
	defaultRaspividSharpness        = 0
	defaultRaspividContrast         = 0
	defaultRaspividISO              = 100
	defaultRaspividEV               = 0
	defaultRaspividAWBGains         = "1.0,1.0"
)

// Configuration errors.
var (
	errBadCodec            = errors.New("codec bad or unset, defaulting")
	errBadRotation         = errors.New("rotation bad or unset, defaulting")
	errBadWidth            = errors.New("width bad or unset, defaulting")
	errBadHeight           = errors.New("height bad or unset, defaulting")
	errBadFrameRate        = errors.New("framerate bad or unset, defaulting")
	errBadBitrate          = errors.New("bitrate bad or unset, defaulting")
	errBadMinFrames        = errors.New("min frames bad or unset, defaulting")
	errBadSaturation       = errors.New("saturation bad or unset, defaulting")
	errBadBrightness       = errors.New("brightness bad or unset, defaulting")
	errBadExposure         = errors.New("exposure bad or unset, defaulting")
	errBadAutoWhiteBalance = errors.New("auto white balance bad or unset, defaulting")
	errBadQuantization     = errors.New("quantization bad or unset, defaulting")
	errBadAWBGains         = errors.New("auto white balance gains bad or unset, defaulting")
	errBadEV               = errors.New("exposure value bad or unset, defaulting")
	errBadContrast         = errors.New("contrast bad or unset, defaulting")
	errBadSharpness        = errors.New("sharpness bad or unset, defaulting")
	errBadISO              = errors.New("iso bad or unset, defaulting")
)

// Possible modes for raspivid --exposure parameter.
var ExposureModes = [...]string{
	"off",
	"auto",
	"night",
	"nightpreview",
	"backlight",
	"spotlight",
	"sports",
	"snow",
	"beach",
	"verylong",
	"fixedfps",
	"antishake",
	"fireworks",
}

// Possible modes for raspivid --awb parameter.
var AutoWhiteBalanceModes = [...]string{
	"off",
	"auto",
	"sun",
	"cloud",
	"shade",
	"tungsten",
	"fluorescent",
	"incandescent",
	"flash",
	"horizon",
}

// Raspivid is an implementation of AVDevice that provides control over the
// raspivid command to allow reading of data from a Raspberry Pi camera.
type Raspivid struct {
	cfg       config.Config
	cmd       *exec.Cmd
	out       io.ReadCloser
	log       logging.Logger
	done      chan struct{}
	isRunning bool
}

// New returns a new Raspivid.
func New(l logging.Logger) *Raspivid {
	return &Raspivid{
		log:  l,
		done: make(chan struct{}),
	}
}

// Name returns the name of the device.
func (r *Raspivid) Name() string {
	return "Raspivid"
}

// Set will take a Config struct, check the validity of the relevant fields
// and then performs any configuration necessary. If fields are not valid,
// an error is added to the multiError and a default value is used.
func (r *Raspivid) Set(c config.Config) error {
	var errs device.MultiError
	switch c.InputCodec {
	case codecutil.H264, codecutil.MJPEG:
	default:
		c.InputCodec = defaultRaspividCodec
		errs = append(errs, errBadCodec)
	}

	if c.Rotation > 359 {
		c.Rotation = defaultRaspividRotation
		errs = append(errs, errBadRotation)
	}

	if c.Width == 0 {
		c.Width = defaultRaspividWidth
		errs = append(errs, errBadWidth)
	}

	if c.Height == 0 {
		c.Height = defaultRaspividHeight
		errs = append(errs, errBadHeight)
	}

	if c.FrameRate == 0 {
		c.FrameRate = defaultRaspividFramerate
		errs = append(errs, errBadFrameRate)
	}

	if c.CBR || sliceutils.ContainsUint8(c.Outputs, config.OutputRTMP) {
		c.Quantization = 0
		if c.Bitrate <= 0 {
			errs = append(errs, errBadBitrate)
			c.Bitrate = defaultRaspividBitrate
		}
	} else {
		c.Bitrate = 0
		if c.Quantization < 10 || c.Quantization > 40 {
			errs = append(errs, errBadQuantization)
			c.Quantization = defaultRaspividQuantization
		}
	}

	if c.MinFrames <= 0 {
		errs = append(errs, errBadMinFrames)
		c.MinFrames = defaultRaspividMinFrames
	}

	if c.Brightness <= 0 || c.Brightness > 100 {
		errs = append(errs, errBadBrightness)
		c.Brightness = defaultRaspividBrightness
	}

	if c.Saturation < -100 || c.Saturation > 100 {
		errs = append(errs, errBadSaturation)
		c.Saturation = defaultRaspividSaturation
	}

	if c.Exposure == "" || !sliceutils.ContainsString(ExposureModes[:], c.Exposure) {
		errs = append(errs, errBadExposure)
		c.Exposure = defaultRaspividExposure
	}

	if c.EV < -10 || c.EV > 10 {
		errs = append(errs, errBadEV)
		c.EV = defaultRaspividEV
	}

	if c.Contrast < -100 || c.Contrast > 100 {
		errs = append(errs, errBadContrast)
		c.Contrast = defaultRaspividContrast
	}

	if c.Sharpness < -100 || c.Sharpness > 100 {
		errs = append(errs, errBadSharpness)
		c.Sharpness = defaultRaspividSharpness
	}

	if c.AutoWhiteBalance == "" || !sliceutils.ContainsString(AutoWhiteBalanceModes[:], c.AutoWhiteBalance) {
		errs = append(errs, errBadAutoWhiteBalance)
		c.AutoWhiteBalance = defaultRaspividAutoWhiteBalance
	}

	if !goodAWBGains(c.AWBGains) {
		errs = append(errs, errBadAWBGains)
		c.AWBGains = defaultRaspividAWBGains
	}

	if c.ISO == 0 || c.ISO < 100 || c.ISO > 800 {
		errs = append(errs, errBadISO)
		c.ISO = defaultRaspividISO
	}

	r.cfg = c
	if len(errs) != 0 {
		return errs
	}
	return nil
}

func goodAWBGains(g string) bool {
	parts := strings.Split(g, ",")
	if len(parts) != 2 {
		return false
	}

	bg, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return false
	}

	rg, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return false
	}

	if bg < 0 || rg < 0 {
		return false
	}

	return true
}

// Start will prepare the arguments for the raspivid command using the
// configuration set using the Set method then call the raspivid command,
// piping the video output from which the Read method will read from.
func (r *Raspivid) Start() error {
	args, err := r.createArgs()
	if err != nil {
		return fmt.Errorf("could not create raspivid args: %w", err)
	}

	r.cfg.Logger.Info(pkg+"raspivid args", "raspividArgs", strings.Join(args, " "))
	r.cmd = exec.Command("raspivid", args...)

	r.out, err = r.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not pipe command output: %w", err)
	}

	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not pipe command error: %w", err)
	}

	go func() {
		for {
			select {
			case <-r.done:
				r.cfg.Logger.Info("raspivid.Stop() called, finished checking stderr")
				return
			default:
				buf, err := ioutil.ReadAll(stderr)
				if err != nil {
					r.cfg.Logger.Error("could not read stderr", "error", err)
					return
				}

				if len(buf) != 0 {
					r.cfg.Logger.Error("error from raspivid stderr", "error", string(buf))
					return
				}
			}
		}
	}()

	err = r.cmd.Start()
	if err != nil {
		return fmt.Errorf("could not start raspivid command: %w", err)
	}
	r.isRunning = true

	return nil
}

// Read implements io.Reader. Calling read before Start has been called will
// result in 0 bytes read and an error.
func (r *Raspivid) Read(p []byte) (int, error) {
	if r.out != nil {
		return r.out.Read(p)
	}
	return 0, errors.New("cannot read, raspivid has not started")
}

// Stop will terminate the raspivid process and close the output pipe.
func (r *Raspivid) Stop() error {
	if r.isRunning == false {
		return nil
	}
	close(r.done)
	if r.cmd == nil || r.cmd.Process == nil {
		return errors.New("raspivid process was never started")
	}
	err := r.cmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("could not kill raspivid process: %w", err)
	}
	r.isRunning = false
	return r.out.Close()
}

// IsRunning is used to determine if the pi's camera is running.
func (r *Raspivid) IsRunning() bool {
	return r.isRunning
}

func (r *Raspivid) createArgs() ([]string, error) {
	const disabled = "0"
	args := []string{
		"--output", "-",
		"--nopreview",
		"--timeout", disabled,
		"--width", fmt.Sprint(r.cfg.Width),
		"--height", fmt.Sprint(r.cfg.Height),
		"--bitrate", fmt.Sprint(r.cfg.Bitrate * 1000), // Convert from kbps to bps.
		"--framerate", fmt.Sprint(r.cfg.FrameRate),
		"--rotation", fmt.Sprint(r.cfg.Rotation),
		"--brightness", fmt.Sprint(r.cfg.Brightness),
		"--saturation", fmt.Sprint(r.cfg.Saturation),
		"--sharpness", fmt.Sprint(r.cfg.Sharpness),
		"--contrast", fmt.Sprint(r.cfg.Contrast),
		"--awb", fmt.Sprint(r.cfg.AutoWhiteBalance),
		"--exposure", fmt.Sprint(r.cfg.Exposure),
	}

	if r.cfg.ISO != defaultRaspividISO {
		args = append(args, []string{"--ISO", fmt.Sprint(r.cfg.ISO)}...)
	}

	if r.cfg.Exposure == "off" {
		args = append(args, []string{"--ev", fmt.Sprint(r.cfg.EV)}...)
	}

	if r.cfg.AutoWhiteBalance == "off" {
		args = append(args, []string{"--awbgains", fmt.Sprint(r.cfg.AWBGains)}...)
	}

	if r.cfg.HorizontalFlip {
		args = append(args, "--hflip")
	}

	if r.cfg.VerticalFlip {
		args = append(args, "--vflip")
	}
	if r.cfg.HorizontalFlip {
		args = append(args, "--hflip")
	}

	switch r.cfg.InputCodec {
	default:
		return []string{}, fmt.Errorf("revid: invalid input codec: %v", r.cfg.InputCodec)
	case codecutil.H264:
		args = append(args,
			"--codec", "H264",
			"--inline",
			"--intra", fmt.Sprint(r.cfg.MinFrames),
		)
		if !r.cfg.CBR {
			args = append(args, "-qp", fmt.Sprint(r.cfg.Quantization))
		}
	case codecutil.MJPEG:
		args = append(args, "--codec", "MJPEG")
	}
	return args, nil
}

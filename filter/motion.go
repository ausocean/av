//go:build withcv
// +build withcv

/*
DESCRIPTION
  A filter that detects motion and discards frames without motion. This
  filter can use different algorithms for motion detection.

AUTHORS
  Scott Barnard <scott@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package filter

import (
	"fmt"
	"image"
	"io"

	"gocv.io/x/gocv"

	"github.com/ausocean/av/revid/config"
)

const (
	defaultMotionDownscaling = 1
	defaultMotionInterval    = 5
	defaultMotionPadding     = 10
)

// MotionAlgorithm is the interface the motion filter expects for
// motion detection algorithms.
type MotionAlgorithm interface {
	Detect(img *gocv.Mat) bool
	Close() error
}

// Motion is a filter that performs motion detection using a supplied
// motion detection algorithm.
type Motion struct {
	dst       io.WriteCloser  // Destination to which motion containing frames go.
	algorithm MotionAlgorithm // Algorithm to use for motion detection.
	scale     float64         // The factor that frames will be downscaled by for motion detection.
	sample    uint            // Interval that motion detection is performed at.
	padding   uint            // The amount of frames before and after motion that will be kept.

	t    uint // Frame counter.
	send uint // Amount of frames to send.

	frames chan []byte // Used for storing frames.
}

// NewMotion returns a pointer to a new Motion filter struct.
func NewMotion(dst io.WriteCloser, alg MotionAlgorithm, c config.Config) *Motion {

	// Validate parameters.
	if c.MotionPadding == 0 {
		c.LogInvalidField("MotionPadding", defaultMotionPadding)
		c.MotionPadding = defaultMotionPadding
	}
	if c.MotionDownscaling <= 0 {
		c.LogInvalidField("MotionDownscaling", defaultMotionDownscaling)
		c.MotionDownscaling = defaultMotionDownscaling
	}
	if c.MotionInterval <= 0 {
		c.LogInvalidField("MotionInterval", defaultMotionInterval)
		c.MotionInterval = defaultMotionInterval
	}

	return &Motion{
		dst:       dst,
		algorithm: alg,
		scale:     1 / float64(c.MotionDownscaling),
		sample:    uint(c.MotionInterval),
		padding:   c.MotionPadding,
		frames:    make(chan []byte, c.MotionInterval+c.MotionPadding),
	}
}

// Implements io.Closer.
// Close frees resources used by gocv, because it has to be done manually, due to
// it using c-go.
func (m *Motion) Close() error {
	return m.algorithm.Close()
}

// Write applies the motion filter to the video stream. Only frames with motion
// are written to the destination encoder, frames without are discarded.
func (m *Motion) Write(f []byte) (int, error) {
	// Decode image into Mat.
	img, err := gocv.IMDecode(f, gocv.IMReadColor)
	if err != nil {
		return 0, fmt.Errorf("image can't be decoded: %w", err)
	}
	defer img.Close()

	// Downsize image to speed up calculations.
	gocv.Resize(img, &img, image.Point{}, m.scale, m.scale, gocv.InterpolationNearestNeighbor)

	// Filter on an interval.
	if m.t == m.sample/2 {
		if m.algorithm.Detect(&img) {
			m.send = m.sample + 2*m.padding - 1
		}
	}
	m.t = (m.t + 1) % m.sample // Increment counter.

	// Send frames.
	m.frames <- f        // Put current frame into buffer.
	toSend := <-m.frames // Get oldest frame out of circular buffer.

	if m.send > 0 {
		m.send--
		return m.dst.Write(toSend)
	}

	return len(f), nil
}

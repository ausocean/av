//go:build withcv
// +build withcv

/*
DESCRIPTION
  A filter that detects motion and discards frames without motion. The
  algorithm calculates the absolute difference for each pixel between
  two frames, then finds the mean. If the mean is above a given threshold,
  then it is considered motion.

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
	"io"

	"gocv.io/x/gocv"

	"github.com/ausocean/av/revid/config"
)

const defaultDiffThreshold = 3

// NewDiff returns a pointer to a new difference motion filter.
func NewDiff(dst io.WriteCloser, c config.Config) *Motion {

	// Validate parameters.
	if c.MotionThreshold <= 0 {
		c.LogInvalidField("MotionThreshold", defaultDiffThreshold)
		c.MotionThreshold = defaultDiffThreshold
	}

	alg := &Diff{
		thresh:    c.MotionThreshold,
		prev:      gocv.NewMat(),
		debugging: newWindows("DIFF"),
	}

	return NewMotion(dst, alg, c)
}

// Diff is a motion detection algorithm. It calculates the absolute
// difference for each pixel between two frames, then finds the mean.
// If the mean is above a given threshold, it is considered motion.
type Diff struct {
	debugging debugWindows
	thresh    float64
	prev      gocv.Mat
}

// Close frees resources used by gocv. It has to be done manually,
// due to gocv using c-go.
func (d *Diff) Close() error {
	d.debugging.close()
	d.prev.Close()
	return nil
}

// Detect performs the motion detection on a frame. It returns true
// if motion is detected.
func (d *Diff) Detect(img *gocv.Mat) bool {
	if d.prev.Empty() {
		d.prev = img.Clone()
		return false
	}

	imgDelta := gocv.NewMat()
	defer imgDelta.Close()

	// Seperate foreground and background.
	gocv.AbsDiff(*img, d.prev, &imgDelta)
	gocv.CvtColor(imgDelta, &imgDelta, gocv.ColorBGRToGray)

	mean := imgDelta.Mean().Val1

	// Update History.
	d.prev = img.Clone()

	// Draw debug information.
	d.debugging.show(*img, imgDelta, mean > d.thresh, nil, fmt.Sprintf("Mean: %f", mean), fmt.Sprintf("Threshold: %f", d.thresh))

	// Return if there is motion.
	return mean > d.thresh
}

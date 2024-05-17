//go:build withcv
// +build withcv

/*
DESCRIPTION
  A filter that detects motion and discards frames without motion. The
  algorithm uses a Mixture of Gaussians method (MoG) to determine what is
  background and what is foreground.

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
	"image"
	"io"

	"gocv.io/x/gocv"

	"github.com/ausocean/av/revid/config"
)

const (
	defaultMOGMinArea   = 25.0
	defaultMOGThreshold = 20.0
	defaultMOGHistory   = 500
	defaultMOGKernel    = 3
)

// NewMOG returns a pointer to a new MOG motion filter.
func NewMOG(dst io.WriteCloser, c config.Config) *Motion {

	// Validate parameters.
	if c.MotionMinArea <= 0 {
		c.LogInvalidField("MotionMinArea", defaultMOGMinArea)
		c.MotionMinArea = defaultMOGMinArea
	}
	if c.MotionThreshold <= 0 {
		c.LogInvalidField("MotionThreshold", defaultMOGThreshold)
		c.MotionThreshold = defaultMOGThreshold
	}
	if c.MotionHistory == 0 {
		c.LogInvalidField("MotionHistory", defaultMOGHistory)
		c.MotionHistory = defaultMOGHistory
	}
	if c.MotionKernel <= 0 {
		c.LogInvalidField("MotionKernel", defaultMOGKernel)
		c.MotionKernel = defaultMOGKernel
	}

	bs := gocv.NewBackgroundSubtractorMOG2WithParams(int(c.MotionHistory), c.MotionThreshold, false)
	alg := &MOG{
		area:      c.MotionMinArea,
		bs:        &bs,
		knl:       gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3)),
		debugging: newWindows("MOG"),
	}

	return NewMotion(dst, alg, c)
}

// MOG is a motion detection algorithm. MoG is short for
// Mixture of Gaussians method.
type MOG struct {
	debugging debugWindows
	area      float64                        // The minimum area that a contour can be found in.
	bs        *gocv.BackgroundSubtractorMOG2 // Uses the MOG algorithm to find the difference between the current and background frame.
	knl       gocv.Mat                       // Matrix that is used for calculations.
}

// Close frees resources used by gocv. It has to be done manually,
// due to gocv using c-go.
func (m *MOG) Close() error {
	m.bs.Close()
	m.knl.Close()
	m.debugging.close()
	return nil
}

// Detect performs the motion detection on a frame. It returns true
// if motion is detected.
func (m *MOG) Detect(img *gocv.Mat) bool {
	imgDelta := gocv.NewMat()
	defer imgDelta.Close()

	// Seperate foreground and background.
	m.bs.Apply(*img, &imgDelta)

	// Threshold imgDelta.
	gocv.Threshold(imgDelta, &imgDelta, 25, 255, gocv.ThresholdBinary)

	// Remove noise.
	gocv.Erode(imgDelta, &imgDelta, m.knl)
	gocv.Dilate(imgDelta, &imgDelta, m.knl)

	// Fill small holes.
	gocv.Dilate(imgDelta, &imgDelta, m.knl)
	gocv.Erode(imgDelta, &imgDelta, m.knl)

	// Find contours and reject ones with a small area.
	var contours [][]image.Point
	allContours := gocv.FindContours(imgDelta, gocv.RetrievalExternal, gocv.ChainApproxSimple)

	for i := 0; i < allContours.Size(); i++ {
		if gocv.ContourArea(allContours.At(i)) > m.area {
			contours = append(contours, allContours.At(i).ToPoints())
		}
	}

	// Draw debug information.
	m.debugging.show(*img, imgDelta, len(contours) > 0, &contours)

	// Return if there is motion.
	return len(contours) > 0
}

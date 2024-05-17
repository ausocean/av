//go:build withcv
// +build withcv

/*
DESCRIPTION
  A filter that detects motion and discards frames without motion. The
  filter uses a K-Nearest Neighbours (KNN) to determine what is
  background and what is foreground.

AUTHORS
  Ella Pietraroia <ella@ausocean.org>

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
	defaultKNNMinArea   = 25.0
	defaultKNNThreshold = 300
	defaultKNNHistory   = 300
	defaultKNNKernel    = 4
)

// NewKNN returns a pointer to a new KNN motion filter.
func NewKNN(dst io.WriteCloser, c config.Config) *Motion {

	// Validate parameters.
	if c.MotionMinArea <= 0 {
		c.LogInvalidField("MotionMinArea", defaultKNNMinArea)
		c.MotionMinArea = defaultKNNMinArea
	}
	if c.MotionThreshold <= 0 {
		c.LogInvalidField("MotionThreshold", defaultKNNThreshold)
		c.MotionThreshold = defaultKNNThreshold
	}
	if c.MotionHistory == 0 {
		c.LogInvalidField("MotionHistory", defaultKNNHistory)
		c.MotionHistory = defaultKNNHistory
	}
	if c.MotionKernel <= 0 {
		c.LogInvalidField("MotionKernel", defaultKNNKernel)
		c.MotionKernel = defaultKNNKernel
	}

	bs := gocv.NewBackgroundSubtractorKNNWithParams(int(c.MotionHistory), c.MotionThreshold, false)
	alg := &KNN{
		area:      c.MotionMinArea,
		bs:        &bs,
		knl:       gocv.GetStructuringElement(gocv.MorphRect, image.Pt(int(c.MotionKernel), int(c.MotionKernel))),
		debugging: newWindows("KNN"),
	}

	return NewMotion(dst, alg, c)
}

// KNN is motion detection algorithm. KNN is short for
// K-Nearest Neighbours method.
type KNN struct {
	debugging debugWindows
	area      float64                       // The minimum area that a contour can be found in.
	bs        *gocv.BackgroundSubtractorKNN // Uses the KNN algorithm to find the difference between the current and background frame.
	knl       gocv.Mat                      // Matrix that is used for calculations.
}

// Close frees resources used by gocv. It has to be done manually,
// due to gocv using c-go.
func (m *KNN) Close() error {
	m.bs.Close()
	m.knl.Close()
	m.debugging.close()
	return nil
}

// Detect performs the motion detection on a frame. It returns true
// if motion is detected.
func (m *KNN) Detect(img *gocv.Mat) bool {
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

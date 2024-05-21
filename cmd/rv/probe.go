//go:build withcv
// +build withcv

/*
DESCRIPTION
  Provides the methods for the turbidity probe using GoCV. Turbidity probe
  will collect the most recent frames in a buffer and write the latest sharpness
  and contrast scores to the probe.

AUTHORS
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"gocv.io/x/gocv"
	"gonum.org/v1/gonum/stat"

	"github.com/ausocean/av/codec/h264"
	"github.com/ausocean/client/pi/turbidity"
	"github.com/ausocean/utils/logging"
)

// Misc constants.
const (
	maxImages     = 1     // Max number of images read when evaluating turbidity.
	bufferLimit   = 20000 // 20KB
	trimTolerance = 200   // Number of times trim can be called where no keyframe is found.
	transformSize = 9     // Size of the square projective matrix.
)

// Turbidity sensor constants.
const (
	k1, k2     = 4, 4 // Block size, must be divisible by the size template with no remainder.
	filterSize = 3    // Sobel filter size.
	scale      = 1.0  // Amount of scale applied to sobel filter values.
	alpha      = 1.0  // Paramater for contrast equation.
)

// turbidityProbe will hold the latest video data and calculate the sharpness and contrast scores.
// These scores will be sent to the cloud based on the given delay.
type turbidityProbe struct {
	sharpness, contrast float64
	delay               time.Duration
	ticker              time.Ticker
	ts                  *turbidity.TurbiditySensor
	log                 logging.Logger
	buffer              *bytes.Buffer
	transform           []float64
	trimCounter         int
}

// NewTurbidityProbe returns a new turbidity probe.
func NewTurbidityProbe(log logging.Logger, delay time.Duration) (*turbidityProbe, error) {
	tp := new(turbidityProbe)
	tp.log = log
	tp.delay = delay
	tp.ticker = *time.NewTicker(delay)
	tp.buffer = bytes.NewBuffer(*new([]byte))

	tp.transform = make([]float64, transformSize)
	transformMatrix := floatToMat(tp.transform)

	// Create the turbidity sensor.
	template := gocv.IMRead("../../turbidity/images/template.jpg", gocv.IMReadGrayScale)
	ts, err := turbidity.NewTurbiditySensor(template, transformMatrix, k1, k2, filterSize, scale, alpha, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create turbidity sensor: %w", err)
	}
	tp.ts = ts
	return tp, nil
}

// Write, reads input h264 frames in the form of a byte stream and writes the the sharpness and contrast
// scores of a video to the the turbidity probe.
func (tp *turbidityProbe) Write(p []byte) (int, error) {
	if tp.buffer.Len() == 0 {
		// The first entry in the buffer must be a keyframe to speed up decoding.
		video, err := h264.Trim(p)
		if err != nil {
			tp.trimCounter++
			if tp.trimCounter >= trimTolerance {
				return 0, fmt.Errorf("could not trim h264 within tolerance: %w", err)
			}
			return len(p), nil
		} else {
			tp.log.Debug("trim successful", "keyframe error counter", tp.trimCounter)
			tp.trimCounter = 0
		}

		n, err := tp.buffer.Write(video)
		if err != nil {
			tp.buffer.Reset()
			return 0, fmt.Errorf("could not write trimmed video to buffer: %w", err)
		}
		tp.log.Debug("video trimmed, write keyframe complete", "size(bytes)", n)
	} else if tp.buffer.Len() < bufferLimit {
		// Buffer size is limited to speed up decoding.
		_, err := tp.buffer.Write(p)
		if err != nil {
			tp.buffer.Reset()
			return 0, fmt.Errorf("could not write to buffer: %w", err)
		}
	} else {
		// Buffer is large enough to begin turbidity calculation.
		select {
		case <-tp.ticker.C:
			tp.log.Debug("beginning turbidity calculation")
			startTime := time.Now()
			err := tp.turbidityCalculation()
			if err != nil {
				return 0, fmt.Errorf("could not calculate turbidity: %w", err)
			}
			tp.log.Debug("finished turbidity calculation", "total duration (sec)", time.Since(startTime).Seconds())
		default:
		}
	}
	return len(p), nil
}

func (tp *turbidityProbe) Close() error {
	return nil
}

// Update will update the probe and turbidity sensor with the new transformation matrix if it has been changed.
func (tp *turbidityProbe) Update(transformMatrix []float64) error {
	if len(transformMatrix) != transformSize {
		return errors.New("transformation matrix has incorrect size")
	}
	for i := range tp.transform {
		if tp.transform[i] == transformMatrix[i] {
			continue
		}
		// Update the turbidity sensor with new transformation.
		tp.log.Debug("updating the transformation matrix")
		tp.transform = transformMatrix
		newTransform := floatToMat(tp.transform)
		tp.ts.TransformMatrix = newTransform
		return nil
	}
	tp.log.Debug("no change to the transformation matrix")
	return nil
}

func (tp *turbidityProbe) turbidityCalculation() error {
	var imgs []gocv.Mat
	img := gocv.NewMat()

	// Write byte array to a temp file.
	file, err := os.CreateTemp("temp", "video*.h264")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tp.log.Debug("writing to file", "buffer size(bytes)", tp.buffer.Len())

	_, err = file.Write(tp.buffer.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}
	tp.log.Debug("write to file success", "buffer size(bytes)", tp.buffer.Len())
	tp.buffer.Reset()

	// Open the video file.
	startTime := time.Now()
	vc, err := gocv.VideoCaptureFile(file.Name())
	if err != nil {
		return fmt.Errorf("failed to open video file: %w", err)
	}
	tp.log.Debug("video capture open", "total duration (sec)", time.Since(startTime).Seconds())

	// Store each frame until maximum amount is reached.
	startTime = time.Now()
	for vc.Read(&img) && len(imgs) < maxImages {
		imgs = append(imgs, img.Clone())
	}
	if len(imgs) <= 0 {
		return errors.New("no frames found")
	}
	tp.log.Debug("read time", "total duration (sec)", time.Since(startTime).Seconds())

	// Process video data to get saturation and contrast scores.
	res, err := tp.ts.Evaluate(imgs)
	if err != nil {
		err_ := cleanUp(file.Name(), vc)
		if err_ != nil {
			return fmt.Errorf("could not clean up: %v, after evaluation error: %w", err_, err)
		}
		return fmt.Errorf("evaluation error: %w", err)
	}

	tp.contrast = stat.Mean(res.Contrast, nil)
	tp.sharpness = stat.Mean(res.Sharpness, nil)

	err = cleanUp(file.Name(), vc)
	if err != nil {
		return fmt.Errorf("could not clean up: %w", err)
	}
	return nil
}

func cleanUp(file string, vc *gocv.VideoCapture) error {
	err := os.Remove(file)
	if err != nil {
		return fmt.Errorf("could not remove temp file: %w", err)
	}
	err = vc.Close()
	if err != nil {
		return fmt.Errorf("could not close video capture device: %w", err)
	}
	return nil
}

// floatToMat will convert a slice of 9 floats to a gocv.Mat.
func floatToMat(array []float64) gocv.Mat {
	mat := gocv.NewMatWithSize(3, 3, gocv.MatTypeCV64F)
	for i := 0; i < mat.Rows(); i++ {
		for j := 0; j < mat.Cols(); j++ {
			mat.SetDoubleAt(i, j, array[i*mat.Cols()+j])
		}
	}
	return mat
}

/*
DESCRIPTION
  A filter that detects motion and discards frames without motion. The
  filter uses a difference method looking at each individual pixel to
  determine what is background and what is foreground.

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
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"sync"

	"github.com/ausocean/av/revid/config"
)

const (
	defaultBasicThreshold = 45000
	defaultBasicPixels    = 1000
)

type pixel struct{ r, g, b uint32 }

// Basic is a filter that provides basic motion detection via a difference
// method.
type Basic struct {
	debugging debugWindows
	dst       io.WriteCloser
	img       image.Image
	bg        [][]pixel
	bwImg     *image.Gray
	thresh    float64
	pix       uint
	w         int
	h         int
	motion    uint
}

// NewBasic returns a pointer to a new Basic filter struct.
func NewBasic(dst io.WriteCloser, c config.Config) *Basic {
	// Validate parameters.
	if c.MotionThreshold <= 0 {
		c.LogInvalidField("MotionThreshold", defaultBasicThreshold)
		c.MotionThreshold = defaultBasicThreshold
	}

	if c.MotionPixels <= 0 {
		c.LogInvalidField("MotionPixels", defaultBasicPixels)
		c.MotionPixels = defaultBasicPixels
	}

	return &Basic{
		dst:       dst,
		thresh:    c.MotionThreshold,
		pix:       c.MotionPixels,
		debugging: newWindows("BASIC"),
	}
}

// Implements io.Closer.
func (bf *Basic) Close() error {
	return bf.debugging.close()
}

// Implements io.Writer.
// Write applies the motion filter to the video stream. Only frames with motion
// are written to the destination encoder, frames without are discarded.
func (bf *Basic) Write(f []byte) (int, error) {
	// Decode MJPEG.
	var err error
	bf.img, err = jpeg.Decode(bytes.NewReader(f))
	if err != nil {
		return 0, fmt.Errorf("image can't be decoded: %w", err)
	}

	// First frame must be set as the first background image.
	if bf.bg == nil {
		bounds := bf.img.Bounds()
		bf.w = bounds.Max.X
		bf.h = bounds.Max.Y

		bf.bwImg = image.NewGray(image.Rect(0, 0, bf.w, bf.h))

		bf.bg = make([][]pixel, bf.h)
		for j, _ := range bf.bg {
			bf.bg[j] = make([]pixel, bf.w)
			for i, _ := range bf.bg[j] {
				p := bf.img.At(i, j)
				r, g, b, _ := p.RGBA()
				bf.bg[j][i].r = r
				bf.bg[j][i].b = b
				bf.bg[j][i].g = g
			}
		}
		return len(f), nil
	}

	// Use 4x goroutines to each process one row of pixels.
	var j int
	j = 0
	var wg sync.WaitGroup

	for j < bf.h {
		wg.Add(4)
		go bf.process(j, &wg)
		go bf.process(j+1, &wg)
		go bf.process(j+2, &wg)
		go bf.process(j+3, &wg)
		j = j + 4
		wg.Wait()
	}

	// Draw debug information.
	bf.debugging.show(bf.img, bf.bwImg, bf.motion > bf.pix, nil, fmt.Sprintf("Motion: %d", bf.motion), fmt.Sprintf("Pix: %d", bf.pix))

	// If there are not enough motion pixels then discard the frame.
	if bf.motion < bf.pix {
		return len(f), nil
	}

	// Write all motion frames.
	return bf.dst.Write(f)
}

// Go routine for one row of the image to be processed.
func (bf *Basic) process(j int, wg *sync.WaitGroup) {
	for i, _ := range bf.bg[j] {
		n := bf.img.At(i, j)
		r, b, g, _ := n.RGBA()

		// Compare the difference of the RGB values of each pixel to the background image.
		diffR := absDiff(r, bf.bg[j][i].r)
		diffG := absDiff(g, bf.bg[j][i].g)
		diffB := absDiff(g, bf.bg[j][i].b)

		diff := diffR + diffG + diffB

		if diff > int(bf.thresh) {
			bf.motion++
			bf.bwImg.SetGray(i, j, color.Gray{0xff})
		} else {
			bf.bwImg.SetGray(i, j, color.Gray{0x00})
		}

		// Update backgound image.
		bf.bg[j][i].r = r
		bf.bg[j][i].b = b
		bf.bg[j][i].g = g
	}
	wg.Done()
}

// Returns the absolute value of the difference of two uint32 numbers.
func absDiff(a, b uint32) int {
	c := int(a) - int(b)
	if c < 0 {
		return -c
	} else {
		return c
	}
}

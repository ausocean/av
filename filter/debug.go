//go:build debug && withcv
// +build debug,withcv

/*
DESCRIPTION
  Displays debug information for the motion filters.

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
	"image/color"

	"gocv.io/x/gocv"
)

// debugWindows is used for displaying debug information for the motion filters.
type debugWindows struct {
	windows []*gocv.Window
}

// close frees resources used by gocv.
func (d *debugWindows) close() error {
	for _, window := range d.windows {
		err := window.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// newWindows creates debugging windows for the motion filter.
func newWindows(name string) debugWindows {
	return debugWindows{
		windows: []*gocv.Window{
			gocv.NewWindow(name + ": Video"),
			gocv.NewWindow(name + ": Motion Detection"),
		},
	}
}

// show displays debug information for the motion filters.
func (d *debugWindows) show(img, imgDelta interface{}, motion bool, contours *[][]image.Point, text ...string) {
	var im, imD gocv.Mat
	const errMsg = "cannot show frame in window: wrong type"
	var drkRed = color.RGBA{191, 0, 0, 0}
	var lhtRed = color.RGBA{191, 31, 31, 0}

	// Type conversions.
	switch img.(type) {
	case image.Image:
		im, _ = gocv.ImageToMatRGB(img.(image.Image))
	case gocv.Mat:
		im = img.(gocv.Mat)
	default:
		panic(errMsg)
	}
	switch imgDelta.(type) {
	case image.Image:
		imD, _ = gocv.ImageToMatRGB(imgDelta.(image.Image))
	case gocv.Mat:
		imD = imgDelta.(gocv.Mat)
	default:
		panic(errMsg)
	}

	// Draw contours.
	if contours != nil {
		for _, c := range *contours {
			rect := gocv.BoundingRect(c)
			gocv.Rectangle(&im, rect, lhtRed, 1)
		}
	}

	// Draw debugging text.
	if motion {
		text = append(text, "Motion Detected")
	}
	for i, str := range text {
		gocv.PutText(&im, str, image.Pt(32, 32*(i+1)), gocv.FontHersheyPlain, 2.0, drkRed, 2)
	}

	// Display windows.
	d.windows[0].IMShow(im)
	d.windows[1].IMShow(imD)
	d.windows[0].WaitKey(1)
}

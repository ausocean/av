//go:build !debug && withcv
// +build !debug,withcv

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
)

// debugWindows is used for displaying debug information for the motion filters.
type debugWindows struct{}

// close frees resources used by gocv.
func (d *debugWindows) close() error { return nil }

// newWindows creates debugging windows for the motion filter.
func newWindows(name string) debugWindows { return debugWindows{} }

// show displays debug information for the motion filters.
func (d *debugWindows) show(img, imgDelta interface{}, motion bool, contours *[][]image.Point, text ...string) {
}

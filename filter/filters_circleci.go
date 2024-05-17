//go:build !withcv
// +build !withcv

/*
DESCRIPTION
  Replaces filters that use the gocv package when Circle-CI builds revid. This
  is needed because Circle-CI does not have a copy of Open CV installed.

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

	"github.com/ausocean/av/revid/config"
)

// NewMOG returns a pointer to a new NoOp struct for testing purposes only.
func NewMOG(dst io.WriteCloser, c config.Config) *NoOp {
	return &NoOp{dst: dst}
}

// NewKNN returns a pointer to a new NoOp struct for testing purposes only.
func NewKNN(dst io.WriteCloser, c config.Config) *NoOp {
	return &NoOp{dst: dst}
}

// NewDiff returns a pointer to a new NoOp struct for testing purposes only.
func NewDiff(dst io.WriteCloser, c config.Config) *NoOp {
	return &NoOp{dst: dst}
}

// debugWindows is used for displaying debug information for the motion filters.
type debugWindows struct{}

// close frees resources used by gocv.
func (d *debugWindows) close() error { return nil }

// newWindows creates debugging windows for the motion filter.
func newWindows(name string) debugWindows { return debugWindows{} }

// show displays debug information for the motion filters.
func (d *debugWindows) show(img, imgDelta interface{}, motion bool, contours *[][]image.Point, text ...string) {
}

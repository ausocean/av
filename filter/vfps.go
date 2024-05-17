/*
DESCRIPTION
  A motion filter that has a variable frame rate. When motion is detected,
  the filter sends all frames and when it is not, the filter sends frames
  at a reduced rate, as set by a parameter.

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
	"io"
)

// VariableFPS is a filter that has a variable frame rate. When motion is
// detected, the filter sends all frames and when it is not, the filter
// sends frames at a reduced framerate.
type VariableFPS struct {
	filter Filter
	dst    io.WriteCloser
	frames uint
	count  uint
}

// NewVariableFPS returns a pointer to a new VariableFPS filter struct.
func NewVariableFPS(dst io.WriteCloser, minFPS uint, filter Filter) *VariableFPS {
	frames := uint(25 / minFPS)
	return &VariableFPS{filter, dst, frames, 0}
}

// Implements io.Writer.
// Write applies the motion filter to the video stream. Frames are sent
// at a reduced frame rate, except when motion is detected, then all frames
// with motion are sent.
func (v *VariableFPS) Write(f []byte) (int, error) {
	v.count = (v.count + 1) % v.frames

	if v.count == 0 {
		return v.dst.Write(f)
	}

	return v.filter.Write(f)
}

// Implements io.Closer.
// Close calls the motion filter's Close method.
func (v *VariableFPS) Close() error {
	return v.filter.Close()
}

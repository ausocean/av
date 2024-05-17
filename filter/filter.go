/*
NAME
  filter.go

AUTHORS
  Ella Pietraroia <ella@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package filter provides the interface and implementations of the filters
// to be used on video input that has been lexed.
package filter

import (
	"io"
)

// Interface for all filters.
type Filter interface {
	io.WriteCloser
	//NB: Filter interface may evolve with more methods as required.
}

// The NoOp filter will perform no operation on the data that is being recieved,
// it will pass it on to the encoder with no changes.
type NoOp struct {
	dst io.Writer
}

func NewNoOp(dst io.Writer) *NoOp { return &NoOp{dst: dst} }

func (n *NoOp) Write(p []byte) (int, error) { return n.dst.Write(p) }

func (n *NoOp) Close() error { return nil }

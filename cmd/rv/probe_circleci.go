//go:build !withcv
// +build !withcv

/*
DESCRIPTION
  Replaces turbidity probe implementation that uses the gocv package.
  When Circle-CI builds revid this is needed because Circle-CI does not
  have a copy of Open CV installed.

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
	"time"

	"github.com/ausocean/utils/logging"
)

type turbidityProbe struct {
	sharpness, contrast float64
}

// NewTurbidityProbe returns an empty turbidity probe for CircleCI testing only.
func NewTurbidityProbe(log logging.Logger, delay time.Duration) (*turbidityProbe, error) {
	tp := new(turbidityProbe)
	return tp, nil
}

// Write performs no operation for CircleCI testing only.
func (tp *turbidityProbe) Write(p []byte) (int, error) { return 0, nil }

func (tp *turbidityProbe) Update(mat []float64) error { return nil }

func (tp *turbidityProbe) Close() error { return nil }

/*
DESCRIPTION
  Testing function for turbidity probe.

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
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/ausocean/utils/logging"
	"gopkg.in/natefinch/lumberjack.v2"
)

// TestProbe reads a given video file and writes the sharpness and contrast scores to a turbidity probe.
func TestProbe(t *testing.T) {
	// Create lumberjack logger.
	fileLog := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    logMaxSize,
		MaxBackups: logMaxBackup,
		MaxAge:     logMaxAge,
	}
	log := logging.New(logVerbosity, io.MultiWriter(fileLog), logSuppress)
	updatedMatrix := []float64{-0.2731048063, -0.0020501869, 661.0275911942, 0.0014327789, -0.2699443748, 339.3921028016, 0.0000838317, 0.0000476486, 1.0}

	ts, err := NewTurbidityProbe(log, time.Microsecond)
	if err != nil {
		t.Fatalf("failed to create turbidity probe")
	}

	err = ts.Update(updatedMatrix)
	if err != nil {
		t.Fatalf("could not update probe: %v", err)
	}

	video, err := ioutil.ReadFile("logo.h264")
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	_, err = ts.Write(video)
	if err != nil {
		t.Fatalf("failed to write sharpness and contrast: %v", err)
	}
	t.Logf("contrast: %v, sharpness: %v\n", ts.contrast, ts.sharpness)
}

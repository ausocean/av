/*
DESCRIPTION
  filter_test.go contains benchmarking tests for each filter implementation.

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
	"testing"

	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

const downscale = 1

type dumbWriteCloser struct{}

func (d *dumbWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (d *dumbWriteCloser) Close() error                { return nil }

func BenchmarkBasic(b *testing.B) {
	cfg := config.Config{Logger: logging.New(logging.Debug, &bytes.Buffer{}, true)}
	err := cfg.Validate()
	if err != nil {
		b.Fatalf("config struct is bad: %v#", err)
	}

	f := NewBasic(&dumbWriteCloser{}, cfg)
	for n := 0; n < b.N; n++ {
		for _, x := range testPackets {
			_, err := f.Write(x)
			if err != nil {
				b.Fatalf("cannot write to basic filter: %v#", err)
			}
		}
	}

	b.Log("Frames: ", len(testPackets))
}

func BenchmarkDifference(b *testing.B) {
	cfg := config.Config{Logger: logging.New(logging.Debug, &bytes.Buffer{}, true)}
	err := cfg.Validate()
	if err != nil {
		b.Fatalf("config struct is bad: %v#", err)
	}

	f := NewDiff(&dumbWriteCloser{}, cfg)
	for n := 0; n < b.N; n++ {
		for _, x := range testPackets {
			_, err := f.Write(x)
			if err != nil {
				b.Fatalf("cannot write to diff filter: %v#", err)
			}
		}
	}

	b.Log("Frames: ", len(testPackets))
}

func BenchmarkKNN(b *testing.B) {
	cfg := config.Config{Logger: logging.New(logging.Debug, &bytes.Buffer{}, true), MotionDownscaling: downscale}
	err := cfg.Validate()
	if err != nil {
		b.Fatalf("config struct is bad: %v#", err)
	}

	f := NewKNN(&dumbWriteCloser{}, cfg)
	for n := 0; n < b.N; n++ {
		for _, x := range testPackets {
			_, err := f.Write(x)
			if err != nil {
				b.Fatalf("cannot write to KNN filter: %v#", err)
			}
		}
	}

	b.Log("Frames: ", len(testPackets))
}

func BenchmarkMOG(b *testing.B) {
	cfg := config.Config{Logger: logging.New(logging.Debug, &bytes.Buffer{}, true), MotionDownscaling: downscale}
	err := cfg.Validate()
	if err != nil {
		b.Fatalf("config struct is bad: %v#", err)
	}

	f := NewMOG(&dumbWriteCloser{}, cfg)
	for n := 0; n < b.N; n++ {
		for _, x := range testPackets {
			_, err := f.Write(x)
			if err != nil {
				b.Fatalf("cannot write to MOG filter: %v#", err)
			}
		}
	}

	b.Log("Frames: ", len(testPackets))
}

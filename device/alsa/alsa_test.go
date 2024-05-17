/*
NAME
  alsa_test.go

AUTHOR
  Trek Hopton <trek@ausocean.org>
  Scott Barnard <scott@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package alsa

import (
	"bytes"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/device"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

func TestDevice(t *testing.T) {
	// We want to open a device with a standard configuration.
	c := config.Config{
		SampleRate: 8000,
		Channels:   1,
		RecPeriod:  0.3,
		BitDepth:   16,
		InputCodec: codecutil.ADPCM,
	}
	n := 2 // Number of periods to wait while recording.

	// Create a new ALSA device, start, read/lex, and then stop it.
	l := logging.New(logging.Debug, os.Stderr, true)
	ai := New(l)
	err := ai.Setup(c)
	// Log any config errors, otherwise if there was an error opening a device, skip
	// this test since not all testing environments will have recording devices.
	switch err := err.(type) {
	case nil:
		// Do nothing.
	case device.MultiError:
		t.Logf("errors from configuring device: %s", err.Error())
	default:
		t.Skip(err)
	}
	err = ai.Start()
	if err != nil {
		t.Error(err)
	}
	cs := ai.DataSize()
	lexer, err := codecutil.NewByteLexer(cs)
	if err != nil {
		t.Fatalf("unexpected error creating byte lexer: %v", err)
	}
	go lexer.Lex(ioutil.Discard, ai, time.Duration(c.RecPeriod*float64(time.Second)))
	time.Sleep(time.Duration(c.RecPeriod*float64(time.Second)) * time.Duration(n))
	ai.Stop()
}

var powerTests = []struct {
	in  int
	out int
}{
	{36, 32},
	{47, 32},
	{3, 4},
	{46, 32},
	{7, 8},
	{2, 2},
	{36, 32},
	{757, 512},
	{2464, 2048},
	{18980, 16384},
	{70000, 65536},
	{8192, 8192},
	{2048, 2048},
	{65536, 65536},
	{-2048, 1},
	{-127, 1},
	{-1, 1},
	{0, 1},
	{1, 2},
}

func TestNearestPowerOfTwo(t *testing.T) {
	for _, tt := range powerTests {
		t.Run(strconv.Itoa(tt.in), func(t *testing.T) {
			v := nearestPowerOfTwo(tt.in)
			if v != tt.out {
				t.Errorf("got %v, want %v", v, tt.out)
			}
		})
	}
}

func TestIsRunning(t *testing.T) {
	const dur = 250 * time.Millisecond
	const sampleRate = 1000
	const channels = 1
	const bitDepth = 16
	const recPeriod = 1

	l := logging.New(logging.Debug, &bytes.Buffer{}, true) // Discard logs.
	d := New(l)

	err := d.Setup(config.Config{
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   bitDepth,
		RecPeriod:  recPeriod,
		InputCodec: codecutil.ADPCM,
	})
	if err != nil {
		t.Skipf("could not set device: %v", err)
	}

	err = d.Start()
	if err != nil {
		t.Fatalf("could not start device %v", err)
	}

	time.Sleep(dur)

	if !d.IsRunning() {
		t.Error("device isn't running, when it should be")
	}

	err = d.Stop()
	if err != nil {
		t.Error(err.Error())
	}

	time.Sleep(dur)

	if d.IsRunning() {
		t.Error("device is running, when it should not be")
	}
}

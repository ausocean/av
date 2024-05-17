/*
DESCRIPTION
  raspivid_test.go tests the raspivid AVDevice.

AUTHORS
  Scott Barnard <scott@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package raspivid

import (
	"bytes"
	"testing"
	"time"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

func TestIsRunning(t *testing.T) {
	const dur = 250 * time.Millisecond

	l := logging.New(logging.Debug, &bytes.Buffer{}, true) // Discard logs.
	d := New(l)

	err := d.Set(config.Config{
		Logger:     l,
		InputCodec: codecutil.H264,
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

func TestGoodAWBGains(t *testing.T) {
	tests := []struct {
		gains  string
		expect bool
	}{
		{gains: "-0.6,1.7", expect: false},
		{gains: "0.6,-1.6", expect: false},
		{gains: "1.3,0.3", expect: true},
		{gains: "0.8,", expect: false},
		{gains: "0.3", expect: false},
		{gains: "0,0", expect: true},
		{gains: ",1.4", expect: false},
	}

	for i, test := range tests {
		got := goodAWBGains(test.gains)
		if got != test.expect {
			t.Errorf("did not get get expected result for test: %d\nWant: %v, Got: %v\n", i, test.expect, got)
		}
	}
}

func TestCreateArgs(t *testing.T) {
	tests := []struct {
		cfg  config.Config
		want []string
	}{
		{
			cfg: config.Config{
				Height:           1080,
				Width:            1440,
				Bitrate:          1000,
				FrameRate:        25,
				Rotation:         45,
				InputCodec:       codecutil.H264,
				Brightness:       50,
				Saturation:       20,
				Contrast:         30,
				Sharpness:        -30,
				AutoWhiteBalance: "auto",
				Exposure:         "auto",
				EV:               3,
				AWBGains:         "0.9,1.2",
				ISO:              300,
				CBR:              true,
			},
			want: []string{
				"--output", "-",
				"--nopreview",
				"--timeout", "0",
				"--width", "1440",
				"--height", "1080",
				"--bitrate", "1000000", // Convert from kbps to bps.
				"--framerate", "25",
				"--rotation", "45",
				"--brightness", "50",
				"--saturation", "20",
				"--sharpness", "-30",
				"--contrast", "30",
				"--awb", "auto",
				"--exposure", "auto",
				"--ISO", "300",
				"--codec", "H264",
				"--inline",
				"--intra", "0",
			},
		},
		{
			cfg: config.Config{
				Height:           1080,
				Width:            1440,
				Bitrate:          1000,
				FrameRate:        25,
				Rotation:         45,
				InputCodec:       codecutil.H264,
				Brightness:       50,
				Saturation:       20,
				Contrast:         30,
				Sharpness:        -30,
				AutoWhiteBalance: "off",
				Exposure:         "off",
				EV:               3,
				AWBGains:         "0.9,1.2",
				ISO:              300,
				CBR:              true,
			},
			want: []string{
				"--output", "-",
				"--nopreview",
				"--timeout", "0",
				"--width", "1440",
				"--height", "1080",
				"--bitrate", "1000000", // Convert from kbps to bps.
				"--framerate", "25",
				"--rotation", "45",
				"--brightness", "50",
				"--saturation", "20",
				"--sharpness", "-30",
				"--contrast", "30",
				"--awb", "off",
				"--exposure", "off",
				"--ISO", "300",
				"--ev", "3",
				"--awbgains", "0.9,1.2",
				"--codec", "H264",
				"--inline",
				"--intra", "0",
			},
		},
		{
			cfg: config.Config{
				Height:           1080,
				Width:            1440,
				Bitrate:          1000,
				FrameRate:        25,
				Rotation:         45,
				InputCodec:       codecutil.H264,
				Brightness:       50,
				Saturation:       20,
				Contrast:         30,
				Sharpness:        -30,
				AutoWhiteBalance: "off",
				Exposure:         "off",
				EV:               3,
				ISO:              100,
				AWBGains:         "0.9,1.2",
				CBR:              true,
			},
			want: []string{
				"--output", "-",
				"--nopreview",
				"--timeout", "0",
				"--width", "1440",
				"--height", "1080",
				"--bitrate", "1000000", // Convert from kbps to bps.
				"--framerate", "25",
				"--rotation", "45",
				"--brightness", "50",
				"--saturation", "20",
				"--sharpness", "-30",
				"--contrast", "30",
				"--awb", "off",
				"--exposure", "off",
				"--ev", "3",
				"--awbgains", "0.9,1.2",
				"--codec", "H264",
				"--inline",
				"--intra", "0",
			},
		},
	}

	for i, test := range tests {
		got, err := (&Raspivid{cfg: test.cfg}).createArgs()
		if err != nil {
			t.Fatalf("did not expect error from createArgs: %v", err)
		}

		if !cmpStrSlice(got, test.want) {
			t.Errorf("did not get expected args list for test: %d\nGot: %v\nWant: %v", i, got, test.want)
		}
	}
}

func cmpStrSlice(a, b []string) bool {
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

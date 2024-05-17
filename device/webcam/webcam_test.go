/*
DESCRIPTION
  webcam_test.go tests the webcam AVDevice.

AUTHORS
  Scott Barnard <scott@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package webcam

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

/*
DESCRIPTION
  file_test.go tests the file AVDevice.

AUTHORS
  Scott Barnard <scott@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package file

import (
	"testing"
	"time"

	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
)

func TestIsRunning(t *testing.T) {
	const dur = 250 * time.Millisecond
	const path = "../../../test/test-data/av/input/motion-detection/mjpeg/school.mjpeg"

	d := New((*logging.TestLogger)(t))

	err := d.Set(config.Config{
		InputPath: path,
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

/*
DESCRIPTION
  jpeg_test.go provides testing for utilities found in jpeg.go.

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package jpeg

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/ausocean/av/protocol/rtp"
)

func TestParsePayload(t *testing.T) {
	const (
		wantPath = "testdata/expect.mjpeg"
		noOfPkts = 5629
	)

	got := &bytes.Buffer{}
	c := NewContext(got)

	for i, pkt := range testPackets {
		p, err := rtp.Payload(pkt)
		if err != nil {
			t.Fatalf("could not get payload for packet %d: %v", i, err)
		}

		m, err := rtp.Marker(pkt)
		if err != nil {
			t.Fatalf("could not get marker for packet %d: %v", i, err)
		}

		err = c.ParsePayload(p, m)
		if err != nil {
			t.Fatalf("could not parse pyload for packet %d: %v", i, err)
		}
	}

	want, err := ioutil.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("could not read file for wanted MJPEG data: %v", err)
	}

	if !bytes.Equal(got.Bytes(), want) {
		t.Error("did not get expected result")
	}
}

/*
NAME
  parse_test.go

DESCRIPTION
  parse_test.go provides testing for behaviour of functionality in parse.go.

AUTHOR
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package rtp

import (
	"bytes"
	"testing"
)

// TestVersion checks that we can correctly get the version from an RTP packet.
func TestVersion(t *testing.T) {
	const expect = 1
	got := version((&Packet{Version: expect}).Bytes(nil))
	if got != expect {
		t.Errorf("unexpected version for RTP packet. Got: %v\n Want: %v\n", got, expect)
	}
}

// TestCsrcCount checks that we can correctly obtain the csrc count from an
// RTP packet.
func TestCsrcCount(t *testing.T) {
	const ver, expect = 2, 2

	pkt := (&Packet{
		Version:   ver,
		CSRCCount: expect,
		CSRC:      make([][4]byte, expect),
	}).Bytes(nil)

	got := csrcCount(pkt)
	if got != expect {
		t.Errorf("unexpected csrc count for RTP packet. Got: %v\n Want: %v\n", got, expect)
	}
}

// TestHasExt checks the behaviour of hasExt with an RTP packet that has the
// extension indicator true, and one with the extension indicator set to false.
func TestHasExt(t *testing.T) {
	const ver = 2

	// First check for when there is an extension field.
	pkt := &Packet{
		Version:     ver,
		ExtHeadFlag: true,
		Extension: ExtensionHeader{
			ID:     0,
			Header: make([][4]byte, 0),
		},
	}

	got := hasExt(pkt.Bytes(nil))
	if !got {
		t.Error("RTP packet did not have true extension indicator as expected")
	}

	// Now check when there is not an extension field.
	pkt.ExtHeadFlag = false
	got = hasExt(pkt.Bytes(nil))
	if got {
		t.Error("did not expect to have extension indicator as true")
	}
}

// TestPayload checks that we can correctly get the payload of an RTP packet
// using Payload for a variety of RTP packet configurations.
func TestPayload(t *testing.T) {
	const ver = 2
	expect := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	testPkts := [][]byte{
		(&Packet{
			Version: ver,
			Payload: expect,
		}).Bytes(nil),

		(&Packet{
			Version:   ver,
			CSRCCount: 3,
			CSRC:      make([][4]byte, 3),
			Payload:   expect,
		}).Bytes(nil),

		(&Packet{
			Version:     ver,
			ExtHeadFlag: true,
			Extension: ExtensionHeader{
				ID:     0,
				Header: make([][4]byte, 3),
			},
			Payload: expect,
		}).Bytes(nil),

		(&Packet{
			Version:   ver,
			CSRCCount: 3,
			CSRC:      make([][4]byte, 3),
			Extension: ExtensionHeader{
				ID:     0,
				Header: make([][4]byte, 3),
			},
			Payload: expect,
		}).Bytes(nil),
	}

	for i, p := range testPkts {
		got, err := Payload(p)
		if err != nil {
			t.Errorf("unexpected error from Payload with pkt: %v", i)
		}

		if !bytes.Equal(got, expect) {
			t.Errorf("unexpected payload data from RTP packet: %v.\n Got: %v\n Want: %v\n", i, got, expect)
		}
	}
}

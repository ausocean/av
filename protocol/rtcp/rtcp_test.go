/*
NAME
  rtcp_test.go

DESCRIPTION
  rtcp_test.go contains testing utilities for functionality provided in rtcp_test.go.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package rtcp

import (
	"bytes"
	"math"
	"testing"
)

// TestReceiverReportBytes checks that we can correctly obtain a []byte of an
// RTCP receiver report from the struct representation.
func TestReceiverReportBytes(t *testing.T) {
	expect := []byte{
		0x81, 0xc9, 0x00, 0x07,
		0xd6, 0xe0, 0x98, 0xda,
		0x6f, 0xad, 0x40, 0xc6,
		0x00, 0xff, 0xff, 0xff,
		0x00, 0x01, 0x83, 0x08,
		0x00, 0x00, 0x00, 0x20,
		0xb9, 0xe1, 0x25, 0x2a,
		0x00, 0x00, 0x2b, 0xf9,
	}

	report := ReceiverReport{
		Header: Header{
			Version:     2,
			Padding:     false,
			ReportCount: 1,
			Type:        typeReceiverReport,
		},
		SenderSSRC: 3605043418,
		Blocks: []ReportBlock{
			ReportBlock{
				SourceIdentifier:  1873625286,
				FractionLost:      0,
				PacketsLost:       math.MaxUint32,
				HighestSequence:   99080,
				Jitter:            32,
				SenderReportTs:    3118540074,
				SenderReportDelay: 11257,
			},
		},
		Extensions: nil,
	}

	got := report.Bytes(nil)
	if !bytes.Equal(got, expect) {
		t.Errorf("did not get expected result. \nGot: %v\nWant: %v\n", got, expect)
	}
}

// TestSourceDescriptionBytes checks that we can correctly obtain a []byte of an
// RTCP source description from the struct representation.
func TestSourceDescriptionBytes(t *testing.T) {
	expect := []byte{
		0x81, 0xca, 0x00, 0x04,
		0xd6, 0xe0, 0x98, 0xda,
		0x01, 0x08, 0x73, 0x61,
		0x78, 0x6f, 0x6e, 0x2d,
		0x70, 0x63, 0x00, 0x00,
	}

	description := Description{
		Header: Header{
			Version:     2,
			Padding:     false,
			ReportCount: 1,
			Type:        typeDescription,
		},
		Chunks: []Chunk{
			Chunk{
				SSRC: 3605043418,
				Items: []SDESItem{
					SDESItem{
						Type: typeCName,
						Text: []byte("saxon-pc"),
					},
				},
			},
		},
	}
	got := description.Bytes(nil)
	if !bytes.Equal(got, expect) {
		t.Errorf("Did not get expected result.\nGot: %v\n Want: %v\n", got, expect)
	}
}

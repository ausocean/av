/*
NAME
  extract_test.go

DESCRIPTION
  extract_test.go provides tests to check validity of the Extractor found in
	extract.go.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h265

import (
	"io"
	"testing"
)

// rtpReader provides the RTP stream.
type rtpReader struct {
	packets [][]byte
	idx     int
}

// Read implements io.Reader.
func (r *rtpReader) Read(p []byte) (int, error) {
	if r.idx == len(r.packets) {
		return 0, io.EOF
	}
	b := r.packets[r.idx]
	n := copy(p, b)
	if n < len(r.packets[r.idx]) {
		r.packets[r.idx] = r.packets[r.idx][n:]
	} else {
		r.idx++
	}
	return n, nil
}

// destination holds the access units extracted during the extraction process.
type destination [][]byte

// Write implements io.Writer.
func (d *destination) Write(p []byte) (int, error) {
	t := make([]byte, len(p))
	copy(t, p)
	*d = append([][]byte(*d), t)
	return len(p), nil
}

// TestLex checks that the Extractor can correctly extract H265 access units from
// HEVC RTP stream in RTP payload format.
func TestLex(t *testing.T) {
	const rtpVer = 2

	tests := []struct {
		donl    bool
		packets [][]byte
		expect  [][]byte
	}{
		{
			donl: false,
			packets: [][]byte{
				{ // Single NAL unit.
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x01, 0x02, 0x03, 0x04, // NAL Data.
				},
				{ // Fragmentation (start packet).
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x62, 0x00, // NAL header (type49).
					0x80,             // FU header.
					0x01, 0x02, 0x03, // FU payload.
				},
				{ // Fragmentation (middle packet)
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x62, 0x00, // NAL header (type 49).
					0x00,             // FU header.
					0x04, 0x05, 0x06, // FU payload.
				},
				{ // Fragmentation (end packet)
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x62, 0x00, // NAL header (type 49).
					0x40,             // FU header.
					0x07, 0x08, 0x09, // FU payload
				},

				{ // Aggregation. Make last packet of access unit => marker bit true.
					0x80, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x60, 0x00, // NAL header (type 49).
					0x00, 0x04, // NAL 1 size.
					0x01, 0x02, 0x03, 0x04, // NAL 1 data.
					0x00, 0x04, // NAL 2 size.
					0x01, 0x02, 0x03, 0x04, // NAL 2 data.
				},
				{ // Singla NAL
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x01, 0x02, 0x03, 0x04, // NAL data.
				},
				{ // Singla NAL. Make last packet of access unit => marker bit true.
					0x80, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x01, 0x02, 0x03, 0x04, // NAL data.
				},
			},
			expect: [][]byte{
				// First access unit.
				{
					// NAL 1
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x01, 0x02, 0x03, 0x04, // NAL data.
					// NAL 2
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x00, 0x00, 0x01, 0x02, 0x03, // FU payload.
					0x04, 0x05, 0x06, // FU payload.
					0x07, 0x08, 0x09, // FU payload.
					// NAL 3
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x01, 0x02, 0x03, 0x04, // NAL data.
					// NAL 4
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x01, 0x02, 0x03, 0x04, // NAL 2 data
				},
				// Second access unit.
				{
					// NAL 1
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x01, 0x02, 0x03, 0x04, // Data.
					// NAL 2
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x01, 0x02, 0x03, 0x04, // Data.
				},
			},
		},
		{
			donl: true,
			packets: [][]byte{
				{ // Single NAL unit.
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x00, 0x00, // DONL
					0x01, 0x02, 0x03, 0x04, // NAL Data.
				},
				{ // Fragmentation (start packet).
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x62, 0x00, // NAL header (type49).
					0x80,       // FU header.
					0x00, 0x00, // DONL
					0x01, 0x02, 0x03, // FU payload.
				},
				{ // Fragmentation (middle packet)
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x62, 0x00, // NAL header (type 49).
					0x00,       // FU header.
					0x00, 0x00, // DONL
					0x04, 0x05, 0x06, // FU payload.
				},
				{ // Fragmentation (end packet)
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x62, 0x00, // NAL header (type 49).
					0x40,       // FU header.
					0x00, 0x00, // DONL
					0x07, 0x08, 0x09, // FU payload
				},

				{ // Aggregation. Make last packet of access unit => marker bit true.
					0x80, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x60, 0x00, // NAL header (type 49).
					0x00, 0x00, // DONL
					0x00, 0x04, // NAL 1 size.
					0x01, 0x02, 0x03, 0x04, // NAL 1 data.
					0x00,       // DOND
					0x00, 0x04, // NAL 2 size.
					0x01, 0x02, 0x03, 0x04, // NAL 2 data.
				},
				{ // Singla NAL
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x40, 0x00, // NAL header (type=32 VPS)
					0x00, 0x00, // DONL.
					0x01, 0x02, 0x03, 0x04, // NAL data.
				},
				{ // Singla NAL. Make last packet of access unit => marker bit true.
					0x80, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // RTP header.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x00, 0x00, // DONL
					0x01, 0x02, 0x03, 0x04, // NAL data.
				},
			},
			expect: [][]byte{
				// First access unit.
				{
					// NAL 1
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x00, 0x00, // DONL
					0x01, 0x02, 0x03, 0x04, // NAL data.
					// NAL 2
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x00, 0x00, 0x01, 0x02, 0x03, // FU payload.
					0x04, 0x05, 0x06, // FU payload.
					0x07, 0x08, 0x09, // FU payload.
					// NAL 3
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x01, 0x02, 0x03, 0x04, // NAL data.
					// NAL 4
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x01, 0x02, 0x03, 0x04, // NAL 2 data
				},
				// Second access unit.
				{
					// NAL 1
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x00, 0x00, // DONL
					0x01, 0x02, 0x03, 0x04, // Data.
					// NAL 2
					0x00, 0x00, 0x00, 0x01, // Start code.
					0x40, 0x00, // NAL header (type=32 VPS).
					0x00, 0x00, // DONL
					0x01, 0x02, 0x03, 0x04, // Data.
				},
			},
		},
	}

	for testNum, test := range tests {
		r := &rtpReader{packets: test.packets}
		d := &destination{}
		err := NewExtractor(test.donl).Extract(d, r, 0)
		switch err {
		case nil, io.EOF: // Do nothing
		default:
			t.Fatalf("error extracting: %v\n", err)
		}

		for i, accessUnit := range test.expect {
			for j, part := range accessUnit {
				if part != [][]byte(*d)[i][j] {
					t.Fatalf("did not get expected data for test: %v.\nGot: %v\nWant: %v\n", testNum, d, test.expect)
				}
			}
		}
	}
}

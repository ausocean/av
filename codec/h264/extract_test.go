/*
NAME
  extract_test.go

DESCRIPTION
  extract_test.go provides tests for the extracter in extract.go

AUTHOR
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h264

import (
	"io"
	"testing"
)

// rtpReader provides an io.Reader for reading the test RTP stream.
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

// destination holds the access units extracted during the lexing process.
type destination [][]byte

// Write implements io.Writer.
func (d *destination) Write(p []byte) (int, error) {
	tmp := make([]byte, len(p))
	copy(tmp, p)
	*d = append(*d, tmp)
	return len(p), nil
}

// TestLex checks that the Lexer can correctly extract H264 access units from
// h264 RTP stream in RTP payload format.
func TestRTPLex(t *testing.T) {
	const rtpVer = 2

	tests := []struct {
		packets [][]byte
		expect  [][]byte
	}{
		{
			packets: [][]byte{
				{ // Single NAL unit.
					128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // RTP header.
					typeSingleNALULowBound, // NAL header.
					1, 2, 3, 4,             // NAL Data.
				},
				{ // Fragmentation (start packet).
					128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // RTP header.
					typeFUA,                      // FU indicator.
					128 | typeSingleNALULowBound, // FU header.
					1, 2, 3,                      // FU payload.
				},
				{ // Fragmentation (middle packet)
					128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // RTP header.
					typeFUA,                // NAL indicator.
					typeSingleNALULowBound, // FU header.
					4, 5, 6,                // FU payload.
				},
				{ // Fragmentation (end packet)
					128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // RTP header.
					typeFUA,                       // NAL indicator.
					0x40 | typeSingleNALULowBound, // FU header.
					7, 8, 9,                       // FU payload
				},

				{
					128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // RTP header.
					typeSTAPA, // NAL header.
					0, 4,      // NAL 1 size.
					1, 2, 3, 4, // NAL 1 data.
					0, 4, // NAL 2 size.
					1, 2, 3, 4, // NAL 2 data.
				},
				{ // Single NAL unit.
					128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // RTP header.
					typeSingleNALULowBound, // NAL header.
					1, 2, 3, 4,             // NAL Data.
				},
				{
					128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // RTP header.
					typeSingleNALULowBound, // NAL header.
					1, 2, 3, 4,             // NAL data.
				},
			},
			expect: [][]byte{
				{
					0, 0, 1, 9, 240, // AUD
				},
				{
					0, 0, 1, 9, 240, // AUD
					0, 0, 0, 1, // Start code.
					typeSingleNALULowBound, // NAL header.
					1, 2, 3, 4,             // NAL data.
				},
				{
					0, 0, 1, 9, 240, // AUD
					0, 0, 0, 1, // Start code.
					typeSingleNALULowBound,
					1, 2, 3, // FU payload.
					4, 5, 6, // FU payload.
					7, 8, 9, // FU payload.
				},
				{
					0, 0, 1, 9, 240, // AUD
					0, 0, 0, 1, // Start code.
					1, 2, 3, 4, // NAL data.
				},
				{
					0, 0, 1, 9, 240, // AUD
					0, 0, 0, 1, // Start code.
					1, 2, 3, 4, // NAL data.
				},
				{
					0, 0, 1, 9, 240, // AUD
					0, 0, 0, 1, // Start code.
					typeSingleNALULowBound, // NAL header.
					1, 2, 3, 4,             // NAL data.
				},
			},
		},
	}

	for testNum, test := range tests {
		r := &rtpReader{packets: test.packets}
		d := &destination{}
		err := NewExtractor().Extract(d, r, 0)
		if err != nil {
			t.Fatalf("error lexing: %v\n", err)
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

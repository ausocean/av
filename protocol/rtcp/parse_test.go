/*
NAME
  parse_test.go

DESCRIPTION
  parse_test.go provides testing utilities for functionality found in parse.go.

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
	"testing"
)

// TestTimestamp checks that Timestamp correctly returns the most signicicant
// word, and least signiciant word, of a receiver report timestamp.
func TestTimestamp(t *testing.T) {
	const expectedMSW = 2209003992
	const expectedLSW = 1956821460
	report := []byte{
		0x80, 0xc8, 0x00, 0x06,
		0x6f, 0xad, 0x40, 0xc6,
		0x83, 0xaa, 0xb9, 0xd8, // Most significant word of timestamp (2209003992)
		0x74, 0xa2, 0xb9, 0xd4, // Least significant word of timestamp (1956821460)
		0x4b, 0x1c, 0x5a, 0xa5,
		0x00, 0x00, 0x00, 0x66,
		0x00, 0x01, 0xc2, 0xc5,
	}

	ts, err := ParseTimestamp(report)
	if err != nil {
		t.Fatalf("did not expect error: %v", err)
	}

	if ts.Seconds != expectedMSW {
		t.Errorf("most significant word of timestamp is not what's expected. \nGot: %v\n Want: %v\n", ts.Seconds, int64(expectedMSW))
	}

	if ts.Fraction != expectedLSW {
		t.Errorf("least significant word of timestamp is not what's expected. \nGot: %v\n Want: %v\n", ts.Fraction, int64(expectedLSW))
	}
}

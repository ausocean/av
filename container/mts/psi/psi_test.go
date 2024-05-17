/*
NAME
  psi_test.go

DESCRIPTION
  See Readme.md

AUTHOR
  Saxon Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package psi

import (
	"bytes"
	"testing"
)

// Some common manifestations of PSI
var (
	// standardPat is a minimal PAT.
	standardPat = PSI{
		PointerField:    0x00,
		TableID:         0x00,
		SyntaxIndicator: true,
		PrivateBit:      false,
		SectionLen:      0x0d,
		SyntaxSection: &SyntaxSection{
			TableIDExt:  0x01,
			Version:     0,
			CurrentNext: true,
			Section:     0,
			LastSection: 0,
			SpecificData: &PAT{
				Program:       0x01,
				ProgramMapPID: 0x1000,
			},
		},
	}

	// standardPmt is a minimal PMT, without time and location descriptors.
	standardPmt = PSI{
		PointerField:    0x00,
		TableID:         0x02,
		SyntaxIndicator: true,
		SectionLen:      0x12,
		SyntaxSection: &SyntaxSection{
			TableIDExt:  0x01,
			Version:     0,
			CurrentNext: true,
			Section:     0,
			LastSection: 0,
			SpecificData: &PMT{
				ProgramClockPID: 0x0100, // wrong
				ProgramInfoLen:  0,
				StreamSpecificData: &StreamSpecificData{
					StreamType:    0x1b,
					PID:           0x0100,
					StreamInfoLen: 0x00,
				},
			},
		},
	}

	// standardPmtTimeLocation is a standard PMT with time and location
	// descriptors, but time and location fields zeroed out.
	standardPmtWithMeta = PSI{
		PointerField:    0x00,
		TableID:         0x02,
		SyntaxIndicator: true,
		SectionLen:      0x3e,
		SyntaxSection: &SyntaxSection{
			TableIDExt:  0x01,
			Version:     0,
			CurrentNext: true,
			Section:     0,
			LastSection: 0,
			SpecificData: &PMT{
				ProgramClockPID: 0x0100,
				ProgramInfoLen:  PmtTimeLocationPil,
				Descriptors: []Descriptor{
					{
						Tag:  TimeDescTag,
						Len:  TimeDataSize,
						Data: make([]byte, TimeDataSize),
					},
					{
						Tag:  LocationDescTag,
						Len:  LocationDataSize,
						Data: make([]byte, LocationDataSize),
					},
				},
				StreamSpecificData: &StreamSpecificData{
					StreamType:    0x1b,
					PID:           0x0100,
					StreamInfoLen: 0x00,
				},
			},
		},
	}
)

// Times as ints for testing
const (
	tstTime1 = 1235367435 // 0x49a2360b
	tstTime2 = 1735357535 // 0x676f745f
)

// GPS string for testing
// TODO: make these realistic
const (
	locationTstStr1 = "$GPGGA,123519,4807.038,N,01131.0"
	locationTstStr2 = "$GPGGA,183710,4902.048,N,02171.0"
)

// err message
const (
	errCmp = "Incorrect output, for: %v \nwant: %v, \ngot:  %v"
)

// Test time to bytes test Data
var (
	timeSlice = []byte{
		0x00, 0x00, 0x00, 0x00, 0x49, 0xA2, 0x36, 0x0B,
	}
)

// Parts to construct bytes of pmt with time and bytes
var (
	pmtWithMetaHead = []byte{
		0x00, 0x02, 0xb0, 0x12, 0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x0a,
		TimeDescTag,                                    // Descriptor tag for timestamp
		TimeDataSize,                                   // Length of bytes to follow
		0x00, 0x00, 0x00, 0x00, 0x67, 0x6f, 0x74, 0x5f, // Timestamp data
		LocationDescTag,  // Descriptor tag for location
		LocationDataSize, // Length of bytes to follow
	}
	pmtWithMetaTail = []byte{
		0x1b, 0xe1, 0x00, 0xf0, 0x00,
	}
)

var (
	// Bytes representing pmt with tstTime1
	pmtTimeBytes1 = []byte{
		0x00, 0x02, 0xb0, 0x12, 0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x0a,
		TimeDescTag,                                    // Descriptor tag
		TimeDataSize,                                   // Length of bytes to follow
		0x00, 0x00, 0x00, 0x00, 0x49, 0xa2, 0x36, 0x0b, // timestamp
		0x1b, 0xe1, 0x00, 0xf0, 0x00,
	}

	// Bytes representing pmt with tstTime 2
	pmtTimeBytes2 = []byte{
		0x00, 0x02, 0xb0, 0x12, 0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x0a,
		TimeDescTag,                                    // Descriptor tag
		TimeDataSize,                                   // Length of bytes to follow
		0x00, 0x00, 0x00, 0x00, 0x67, 0x6f, 0x74, 0x5f, // timestamp
		0x1b, 0xe1, 0x00, 0xf0, 0x00,
	}

	// Bytes representing pmt with time1 and location1
	pmtWithMetaTst1 = buildPmtWithMeta(locationTstStr1)

	// bytes representing pmt with with time1 and location 2
	pmtWithMetaTst2 = buildPmtWithMeta(locationTstStr2)
)

// bytesTests contains data for testing the Bytes() funcs for the PSI data struct
var bytesTests = []struct {
	name  string
	input PSI
	want  []byte
}{
	// Pat test
	{
		name:  "pat Bytes()",
		input: standardPat,
		want:  StandardPatBytes,
	},

	// Pmt test data no descriptor
	{
		name:  "pmt to Bytes() without descriptors",
		input: standardPmt,
		want:  StandardPmtBytes,
	},

	// Pmt with time descriptor
	{
		name: "pmt to Bytes() with time descriptor",
		input: PSI{
			PointerField:    0x00,
			TableID:         0x02,
			SyntaxIndicator: true,
			SectionLen:      0x12,
			SyntaxSection: &SyntaxSection{
				TableIDExt:  0x01,
				Version:     0,
				CurrentNext: true,
				Section:     0,
				LastSection: 0,
				SpecificData: &PMT{
					ProgramClockPID: 0x0100, // wrong
					ProgramInfoLen:  10,
					Descriptors: []Descriptor{
						{
							Tag:  TimeDescTag,
							Len:  TimeDataSize,
							Data: TimeBytes(tstTime1),
						},
					},
					StreamSpecificData: &StreamSpecificData{
						StreamType:    0x1b,
						PID:           0x0100,
						StreamInfoLen: 0x00,
					},
				},
			},
		},
		want: pmtTimeBytes1,
	},

	// Pmt with time and location
	{
		name: "pmt Bytes() with time and location",
		input: PSI{
			PointerField:    0x00,
			TableID:         0x02,
			SyntaxIndicator: true,
			SectionLen:      0x12,
			SyntaxSection: &SyntaxSection{
				TableIDExt:  0x01,
				Version:     0,
				CurrentNext: true,
				Section:     0,
				LastSection: 0,
				SpecificData: &PMT{
					ProgramClockPID: 0x0100, // wrong
					ProgramInfoLen:  10,
					Descriptors: []Descriptor{
						{
							Tag:  TimeDescTag,
							Len:  TimeDataSize,
							Data: TimeBytes(tstTime2),
						},
						{
							Tag:  LocationDescTag,
							Len:  LocationDataSize,
							Data: []byte(locationTstStr1),
						},
					},
					StreamSpecificData: &StreamSpecificData{
						StreamType:    0x1b,
						PID:           0x0100,
						StreamInfoLen: 0x00,
					},
				},
			},
		},
		want: buildPmtWithMeta(locationTstStr1),
	},
}

// TestBytes ensures that the Bytes() funcs are working correctly to take PSI
// structs and convert them to byte slices
func TestBytes(t *testing.T) {
	for _, test := range bytesTests {
		got := test.input.Bytes()
		if !bytes.Equal(got, AddCRC(test.want)) {
			t.Errorf("unexpected error for test %v: got:%v want:%v", test.name, got,
				test.want)
		}
	}
}

// TestTimestampToBytes is a quick sanity check of the int64 time to []byte func
func TestTimestampToBytes(t *testing.T) {
	tb := TimeBytes(tstTime1)
	if !bytes.Equal(timeSlice, tb) {
		t.Errorf(errCmp, "testTimeStampToBytes", timeSlice, tb)
	}
}

// TestTimeUpdate checks to see if we can correctly update the timstamp in pmt
func TestTimeUpdate(t *testing.T) {
	cpy := make([]byte, len(pmtTimeBytes1))
	copy(cpy, pmtTimeBytes1)
	cpy = AddCRC(cpy)
	err := UpdateTime(cpy, tstTime2)
	cpy = cpy[:len(cpy)-4]
	if err != nil {
		t.Errorf("Update time returned err: %v", err)
	}
	if !bytes.Equal(pmtTimeBytes2, cpy) {
		t.Errorf(errCmp, "TestTimeUpdate", pmtTimeBytes2, cpy)
	}
}

// TestLocationGet checks that we can correctly get location data from a pmt table
func TestLocationGet(t *testing.T) {
	pb := standardPmtWithMeta.Bytes()
	err := UpdateLocation(pb, locationTstStr1)
	if err != nil {
		t.Errorf("Error for TestLocationGet UpdateLocation(pb, locationTstStr1): %v", err)
	}
	g, err := LocationFrom(pb)
	if err != nil {
		t.Errorf("Error for TestLocationGet LocationOf(pb): %v", err)
	}
	if g != locationTstStr1 {
		t.Errorf(errCmp, "TestLocationGet", locationTstStr1, g)
	}
}

// TestLocationUpdate checks to see if we can update the location string in a pmt correctly
func TestLocationUpdate(t *testing.T) {
	cpy := make([]byte, len(pmtWithMetaTst1))
	copy(cpy, pmtWithMetaTst1)
	cpy = AddCRC(cpy)
	err := UpdateLocation(cpy, locationTstStr2)
	cpy = cpy[:len(cpy)-4]
	if err != nil {
		t.Errorf("Update time returned err: %v", err)
	}
	if !bytes.Equal(pmtWithMetaTst2, cpy) {
		t.Errorf(errCmp, "TestLocationUpdate", pmtWithMetaTst2, cpy)
	}
}

func TestTrim(t *testing.T) {
	test := []byte{0xa3, 0x01, 0x03, 0x00, 0xde}
	want := []byte{0xa3, 0x01, 0x03}
	got := trimTo(test, 0x00)
	if !bytes.Equal(got, want) {
		t.Errorf(errCmp, "TestTrim", want, got)
	}
}

// buildPmtTimeLocationBytes returns a PMT with time and location from s.
func buildPmtWithMeta(tstStr string) []byte {
	dst := make([]byte, len(pmtWithMetaHead)+32+len(pmtWithMetaTail))
	copy(dst, pmtWithMetaHead)
	copy(dst[len(pmtWithMetaHead):], tstStr)
	copy(dst[len(pmtWithMetaHead)+32:], pmtWithMetaTail)
	return dst
}

/*
NAME
  descriptor_test.go

DESCRIPTION
  See Readme.md

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

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

const (
	errNotExpectedOut = "Did not get expected output: \ngot : %v, \nwant: %v"
	errUnexpectedErr  = "Unexpected error: %v\n"
)

var (
	tstPsi1 = PSI{
		PointerField:    0x00,
		TableID:         0x02,
		SyntaxIndicator: true,
		SectionLen:      0x1c,
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
						Data: make([]byte, TimeDataSize),
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

	tstPsi2 = PSI{
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
				ProgramClockPID: 0x0100,
				ProgramInfoLen:  0,
				StreamSpecificData: &StreamSpecificData{
					StreamType:    0x1b,
					PID:           0x0100,
					StreamInfoLen: 0x00,
				},
			},
		},
	}

	tstPsi3 = PSI{
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

var (
	pmtTimeBytesResizedBigger = []byte{
		0x00, 0x02, 0xb0, 0x1e, 0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x0c,
		TimeDescTag,                                                // Descriptor tag
		0x0a,                                                       // Length of bytes to follow
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, // timestamp
		0x1b, 0xe1, 0x00, 0xf0, 0x00,
	}

	pmtTimeBytesResizedSmaller = []byte{
		0x00, 0x02, 0xb0, 0x1a, 0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x08,
		TimeDescTag,                        // Descriptor tag
		0x06,                               // Length of bytes to follow
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, // timestamp
		0x1b, 0xe1, 0x00, 0xf0, 0x00,
	}
)

// TestHasDescriptorExists checks that PSIBytes.HasDescriptor performs as expected
// when the PSI we're interested in has the descriptor of interest. HasDescriptor
// should return the descriptor bytes.
// TODO: HasDescriptor also returns index of descriptor - we should check this.
func TestHasDescriptorExists(t *testing.T) {
	p := PSIBytes(tstPsi3.Bytes())
	_, got := p.HasDescriptor(LocationDescTag)
	want := []byte{
		LocationDescTag,
		LocationDataSize,
	}
	want = append(want, make([]byte, LocationDataSize)...)
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestHasDescriptorAbsent checks that PSIBytes.HasDescriptor performs as expected
// when the PSI does not have the descriptor of interest. HasDescriptor should
// return a nil slice and a negative index.
// TODO: check index here as well.
func TestHasDescriptorAbsent(t *testing.T) {
	p := PSIBytes(tstPsi3.Bytes())
	const fakeTag = 236
	_, got := p.HasDescriptor(fakeTag)
	var want []byte
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestHasDescriptorNone checks that PSIBytes.HasDescriptor behaves as expected
// when the PSI does not have any descriptors. HasDescriptor should return a nil
// slice.
// TODO: again check index here.
func TestHasDescriptorNone(t *testing.T) {
	p := PSIBytes(tstPsi2.Bytes())
	_, got := p.HasDescriptor(LocationDescTag)
	var want []byte
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestProgramInfoLen checks that PSIBytes.ProgramInfoLen correctly extracts
// the program info length from a PSI.
func TestProgramInfoLen(t *testing.T) {
	p := PSIBytes(tstPsi1.Bytes())
	got := p.ProgramInfoLen()
	want := 10
	if got != want {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestDescriptors checks that PSIBytes.descriptors correctly returns the descriptors
// from a PSI when descriptors exist.
func TestDescriptors(t *testing.T) {
	p := PSIBytes(tstPsi1.Bytes())
	got := p.descriptors()
	want := []byte{
		TimeDescTag,
		TimeDataSize,
	}
	want = append(want, make([]byte, TimeDataSize)...)
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestDescriptors checks that PSIBYtes.desriptors correctly returns nil when
// we try to get descriptors from a psi without any descriptors.
func TestDescriptorsNone(t *testing.T) {
	p := PSIBytes(tstPsi2.Bytes())
	got := p.descriptors()
	var want []byte
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestCreateDescriptorEmpty checks that PSIBytes.createDescriptor correctly adds
// a descriptor to the descriptors list in a PSI when it has no descriptors already.
func TestCreateDescriptorEmpty(t *testing.T) {
	got := PSIBytes(tstPsi2.Bytes())
	got.createDescriptor(TimeDescTag, make([]byte, TimeDataSize))
	UpdateCrc(got[1:])
	want := PSIBytes(tstPsi1.Bytes())
	if !bytes.Equal(want, got) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestCreateDescriptorNotEmpty checks that PSIBytes.createDescriptor correctly adds
// a descriptor to the descriptors list in a PSI when it already has one with
// a different tag.
func TestCreateDescriptorNotEmpty(t *testing.T) {
	got := PSIBytes(tstPsi1.Bytes())
	got.createDescriptor(LocationDescTag, make([]byte, LocationDataSize))
	UpdateCrc(got[1:])
	want := PSIBytes(tstPsi3.Bytes())
	if !bytes.Equal(want, got) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestAddDescriptorEmpty checks that PSIBytes.AddDescriptor correctly adds a descriptor
// when there are no other descriptors present in the PSI.
func TestAddDescriptorEmpty(t *testing.T) {
	got := PSIBytes(tstPsi2.Bytes())
	if err := got.AddDescriptor(TimeDescTag, make([]byte, TimeDataSize)); err != nil {
		t.Errorf(errUnexpectedErr, err.Error())
	}
	want := PSIBytes(tstPsi1.Bytes())
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestAddDescriptorNonEmpty checks that PSIBytes.AddDescriptor correctly adds a
// descriptor when there is already a descriptor of a differing type in a PSI.
func TestAddDescriptorNonEmpty(t *testing.T) {
	got := PSIBytes(tstPsi1.Bytes())
	if err := got.AddDescriptor(LocationDescTag, make([]byte, LocationDataSize)); err != nil {
		t.Errorf(errUnexpectedErr, err.Error())
	}
	want := PSIBytes(tstPsi3.Bytes())
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestAddDescriptorUpdateSame checks that PSIBytes.AddDescriptor correctly updates data in a descriptor
// with the same given tag, with data being the same size. AddDescriptor should just copy new data into
// the descriptors data field.
func TestAddDescriptorUpdateSame(t *testing.T) {
	newData := [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	want := PSIBytes(tstPsi2.Bytes())
	want.createDescriptor(TimeDescTag, newData[:])
	got := PSIBytes(tstPsi1.Bytes())
	if err := got.AddDescriptor(TimeDescTag, newData[:]); err != nil {
		t.Errorf(errUnexpectedErr, err.Error())
	}
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestAddDescriptorUpdateBigger checks that PSIBytes.AddDescriptor correctly resizes descriptor with same given tag
// to a bigger size and copies in new data. AddDescriptor should find descriptor with same tag, increase size of psi,
// shift data to make room for update descriptor, and then copy in the new data.
func TestAddDescriptorUpdateBigger(t *testing.T) {
	got := PSIBytes(tstPsi1.Bytes())
	if err := got.AddDescriptor(TimeDescTag, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a}); err != nil {
		t.Errorf(errUnexpectedErr, err.Error())
	}
	want := AddCRC(pmtTimeBytesResizedBigger)
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

// TestAddDescriptorUpdateSmaller checks that PSIBytes.AddDescriptor correctly resizes descriptor with same given tag
// in a psi to a smaller size and copies in new data. AddDescriptor should find tag with same descrtiptor, shift data
// after descriptor upwards, trim the psi to new size, and then copy in new data.
func TestAddDescriptorUpdateSmaller(t *testing.T) {
	got := PSIBytes(tstPsi1.Bytes())
	if err := got.AddDescriptor(TimeDescTag, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}); err != nil {
		t.Errorf(errUnexpectedErr, err.Error())
	}
	want := AddCRC(pmtTimeBytesResizedSmaller)
	if !bytes.Equal(got, want) {
		t.Errorf(errNotExpectedOut, got, want)
	}
}

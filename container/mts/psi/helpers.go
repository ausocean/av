/*
NAME
	helpers.go

DESCRIPTION
  helpers.go provides functionality for editing and reading bytes slices
	directly in order to insert/read timestamp and location data in psi.

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
	"encoding/binary"
	"errors"
)

// TimeBytes takes a timestamp as an uint64 and converts to an 8 byte slice -
// allows for updating of timestamp in pmt time descriptor.
func TimeBytes(t uint64) []byte {
	var s [TimeDataSize]byte
	binary.BigEndian.PutUint64(s[:], t)
	return s[:]
}

// HasTime takes a psi as a byte slice and checks to see if it has a time descriptor
// - if so return nil, otherwise return error
func HasTime(p []byte) bool {
	return p[TimeTagIndx] == TimeDescTag
}

// HasLocation takes a psi as a byte slice and checks to see if it has a location descriptor
// - if so return nil, otherwise return error
func HasLocation(p []byte) bool {
	return p[LocationTagIndx] == LocationDescTag
}

// UpdateTime takes the byte slice representation of a psi-pmt as well as a time
// as an integer and attempts to update the time descriptor in the pmt with the
// given time if the time descriptor exists, otherwise an error is returned
func UpdateTime(dst []byte, t uint64) error {
	if !HasTime(dst) {
		return errors.New("pmt does not have time descriptor, cannot update")
	}
	ts := TimeBytes(uint64(t))
	for i := range dst[TimeDataIndx : TimeDataIndx+TimeDataSize] {
		dst[i+TimeDataIndx] = ts[i]
	}
	UpdateCrc(dst[1:])
	return nil
}

// SyntaxSecLenFrom takes a byte slice representation of a psi and extracts
// it's syntax section length
func SyntaxSecLenFrom(p []byte) int {
	return int(((p[SyntaxSecLenIdx1] & SyntaxSecLenMask1) << 8) | p[SyntaxSecLenIdx2])
}

// LocationFrom takes a byte slice representation of a psi-pmt and extracts it's
// timestamp, returning as a uint64 if it exists, otherwise returning 0 and nil
// if it does not exist
func LocationFrom(p []byte) (g string, err error) {
	if !HasLocation(p) {
		return "", errors.New("pmt does not have location descriptor")
	}
	gBytes := p[LocationDataIndx : LocationDataIndx+LocationDataSize]
	gBytes = bytes.Trim(gBytes, "\x00")
	g = string(gBytes)
	return g, nil
}

// UpdateLocation takes a byte slice representation of a psi-pmt containing a location
// descriptor and attempts to update the location data value with the passed string.
// If the psi does not contain a location descriptor, and error is returned.
func UpdateLocation(d []byte, s string) error {
	if !HasLocation(d) {
		return errors.New("pmt does not location descriptor, cannot update")
	}
	loc := d[LocationDataIndx : LocationDataIndx+LocationDataSize]
	n := copy(loc, s)
	loc = loc[n:]
	for i := range loc {
		loc[i] = 0
	}
	UpdateCrc(d[1:])
	return nil
}

func trimTo(d []byte, t byte) []byte {
	for i, b := range d {
		if b == t {
			return d[:i]
		}
	}
	return d
}

// addPadding adds an appropriate amount of padding to a pat or pmt table for
// addition to an MPEG-TS packet
func AddPadding(d []byte) []byte {
	t := make([]byte, PacketSize)
	copy(t, d)
	padding := t[len(d):]
	for i := range padding {
		padding[i] = 0xff
	}
	return t
}

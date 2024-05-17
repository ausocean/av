/*
NAME
  parse.go

DESCRIPTION
  parse.go provides functionality for parsing RTP packets.

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
	"encoding/binary"
	"errors"
)

const badVer = "incompatible RTP version"

// Marker returns the state of the RTP marker bit, and an error if parsing fails.
func Marker(d []byte) (bool, error) {
	if len(d) < defaultHeadSize {
		panic("invalid RTP packet length")
	}

	if version(d) != rtpVer {
		return false, errors.New(badVer)
	}

	return d[1]&0x80 != 0, nil
}

// Payload returns the payload from an RTP packet provided the version is
// compatible, otherwise an error is returned.
func Payload(d []byte) ([]byte, error) {
	err := checkPacket(d)
	if err != nil {
		return nil, err
	}
	extLen := 0
	if hasExt(d) {
		extLen = 4 + 4*(int(binary.BigEndian.Uint16(d[optionalFieldIdx+4*csrcCount(d)+2:])))
	}
	payloadIdx := optionalFieldIdx + 4*csrcCount(d) + extLen
	return d[payloadIdx:], nil
}

// SSRC returns the source identifier from an RTP packet. An error is return if
// the packet is not valid.
func SSRC(d []byte) (uint32, error) {
	err := checkPacket(d)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(d[8:]), nil
}

// Sequence returns the sequence number of an RTP packet. An error is returned
// if the packet is not valid.
func Sequence(d []byte) (uint16, error) {
	err := checkPacket(d)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(d[2:]), nil
}

// Timestamp returns the RTP timestamp of an RTP packet. An error is returned
// if the packet is not valid.
func Timestamp(d []byte) (uint32, error) {
	err := checkPacket(d)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(d[4:]), nil
}

// checkPacket checks the validity of the packet, firstly by checking size and
// then also checking that version is compatible with these utilities.
func checkPacket(d []byte) error {
	if len(d) < defaultHeadSize {
		return errors.New("invalid RTP packet length")
	}
	if version(d) != rtpVer {
		return errors.New(badVer)
	}
	return nil
}

// hasExt returns true if an extension is present in the RTP packet.
func hasExt(d []byte) bool {
	return (d[0] & 0x10 >> 4) == 1
}

// csrcCount returns the number of CSRC fields.
func csrcCount(d []byte) int {
	return int(d[0] & 0x0f)
}

// version returns the version of the RTP packet.
func version(d []byte) int {
	return int(d[0] & 0xc0 >> 6)
}

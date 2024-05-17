/*
NAME
  parse.go

DESCRIPTION
  parse.go contains functionality for parsing RTCP packets.

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
	"encoding/binary"
	"errors"
)

// Timestamp describes an NTP timestamp, see https://tools.ietf.org/html/rfc1305
type Timestamp struct {
	Seconds  uint32
	Fraction uint32
}

// ParseTimestamp gets the timestamp from a receiver report and returns it as
// a Timestamp as defined above. If the given bytes do not represent a valid
// receiver report, an error is returned.
func ParseTimestamp(buf []byte) (Timestamp, error) {
	if len(buf) < 4 {
		return Timestamp{}, errors.New("bad RTCP packet, not of sufficient length")
	}
	if (buf[0]&0xc0)>>6 != rtcpVer {
		return Timestamp{}, errors.New("incompatible RTCP version")
	}

	if buf[1] != typeSenderReport {
		return Timestamp{}, errors.New("RTCP packet is not of sender report type")
	}

	return Timestamp{
		Seconds:  binary.BigEndian.Uint32(buf[8:]),
		Fraction: binary.BigEndian.Uint32(buf[12:]),
	}, nil
}

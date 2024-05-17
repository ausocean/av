/*
DESCRIPTION
  parse.go provides H.264 NAL unit parsing utilities for the extraction of
	syntax elements.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h264

import (
	"errors"

	"github.com/ausocean/av/codec/h264/h264dec"
)

var errNotEnoughBytes = errors.New("not enough bytes to read")

// NALType returns the NAL type of the given NAL unit bytes. The given NAL unit
// may be in byte stream or packet format.
// NB: access unit delimiters are skipped.
func NALType(n []byte) (int, error) {
	sc := frameScanner{buf: n}
	for {
		b, ok := sc.readByte()
		if !ok {
			return 0, errNotEnoughBytes
		}
		for i := 1; b == 0x00 && i != 4; i++ {
			b, ok = sc.readByte()
			if !ok {
				return 0, errNotEnoughBytes
			}
			if b != 0x01 || (i != 2 && i != 3) {
				continue
			}

			b, ok = sc.readByte()
			if !ok {
				return 0, errNotEnoughBytes
			}
			nalType := int(b & 0x1f)
			if nalType != h264dec.NALTypeAccessUnitDelimiter {
				return nalType, nil
			}
		}
	}
}

type frameScanner struct {
	off int
	buf []byte
}

func (s *frameScanner) readByte() (b byte, ok bool) {
	if s.off >= len(s.buf) {
		return 0, false
	}
	b = s.buf[s.off]
	s.off++
	return b, true
}

// Trim will trim down a given byte stream of video data so that a key frame appears first.
func Trim(n []byte) ([]byte, error) {
	sc := frameScanner{buf: n}
	for {
		b, ok := sc.readByte()
		if !ok {
			return nil, errNotEnoughBytes
		}
		for i := 1; b == 0x00 && i != 4; i++ {
			b, ok = sc.readByte()
			if !ok {
				return nil, errNotEnoughBytes
			}
			if b != 0x01 || (i != 2 && i != 3) {
				continue
			}

			b, ok = sc.readByte()
			if !ok {
				return nil, errNotEnoughBytes
			}
			nalType := int(b & 0x1f)
			if nalType == 7 {
				sc.off = sc.off - 4
				return sc.buf[sc.off:], nil
			}
		}
	}
}

/*
NAME
  extract.go

DESCRIPTION
  extract.go provides an Extractor to get access units from an RTP stream.

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package h264 provides functionality for handling the H.264 video codec.
// This includes extraction from an RTP stream, lexing of NAL units from
// byte stream, and decoding.
package h264

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/ausocean/av/codec/h264/h264dec"
	"github.com/ausocean/av/protocol/rtp"
)

// NAL types (from https://tools.ietf.org/html/rfc6184#page-13)
const (
	// Single nal units bounds.
	typeSingleNALULowBound  = 1
	typeSingleNALUHighBound = 23

	// Single-time aggregation packets.
	typeSTAPA = 24
	typeSTAPB = 25

	// Multi-time aggregation packets.
	typeMTAP16 = 26
	typeMTAP24 = 27

	// Fragmentation packets.
	typeFUA = 28
	typeFUB = 29
)

// Min NAL lengths.
const (
	minSingleNALLen = 1
	minSTAPALen     = 4
	minFUALen       = 2
)

// Buffer sizes.
const (
	maxAUSize  = 100000 // Max access unit size in bytes.
	maxRTPSize = 1500   // Max ethernet transmission unit in bytes.
)

// Bytes for an access unit delimeter.
var aud = []byte{0x00, 0x00, 0x01, 0x09, 0xf0}

// Extractor is an Extractor for extracting H264 access units from RTP stream.
type Extractor struct {
	buf     *bytes.Buffer // Holds the current access unit.
	frag    bool          // Indicates if we're currently dealing with a fragmentation packet.
	dst     io.Writer     // The destination we'll be writing extracted NALUs to.
	toWrite []byte        // Holds the current NAL unit with start code to be written.
}

// NewExtractor returns a new Extractor.
func NewExtractor() *Extractor {
	return &Extractor{
		buf: bytes.NewBuffer(make([]byte, 0, maxAUSize))}
}

// Extract extracts H264 access units from an RTP stream. This function
// expects that each read from src will provide a single RTP packet.
func (e *Extractor) Extract(dst io.Writer, src io.Reader, delay time.Duration) error {
	e.toWrite = []byte{0, 0, 0, 1}
	e.buf.Write(aud)
	e.dst = dst
	buf := make([]byte, maxRTPSize)
	for {
		n, err := src.Read(buf)
		switch err {
		case nil: // Do nothing.
		case io.EOF:
			return nil
		default:
			return fmt.Errorf("source read error: %w\n", err)
		}

		// Get payload from RTP packet.
		payload, err := rtp.Payload(buf[:n])
		if err != nil {
			return fmt.Errorf("could not get RTP payload, failed with err: %w\n", err)
		}

		nalType := payload[0] & 0x1f

		// If not currently fragmented then we ignore current write.
		if e.frag && nalType != typeFUA {
			e.buf.Reset()
			e.frag = false
			continue
		}

		if typeSingleNALULowBound <= nalType && nalType <= typeSingleNALUHighBound {
			// If len too small, ignore.
			if len(payload) < minSingleNALLen {
				continue
			}
			e.writeWithPrefix(payload)
		} else {
			switch nalType {
			case typeSTAPA:
				e.handleSTAPA(payload)
			case typeFUA:
				e.handleFUA(payload)
			case typeSTAPB:
				panic("STAP-B type unsupported")
			case typeMTAP16:
				panic("MTAP16 type unsupported")
			case typeMTAP24:
				panic("MTAP24 type unsupported")
			case typeFUB:
				panic("FU-B type unsupported")
			default:
				panic("unsupported type")
			}
		}
	}
}

// handleSTAPA parses NAL units from an aggregation packet and writes
// them to the Extractor's buffer buf.
func (e *Extractor) handleSTAPA(d []byte) {
	// If the length is too small, ignore.
	if len(d) < minSTAPALen {
		return
	}

	for i := 1; i < len(d); {
		size := int(binary.BigEndian.Uint16(d[i:]))

		// Skip over NAL unit size.
		const sizeOfFieldLen = 2
		i += sizeOfFieldLen

		// Get the NALU.
		nalu := d[i : i+size]
		i += size
		e.writeWithPrefix(nalu)
	}
}

// handleFUA parses NAL units from fragmentation packets and writes
// them to the Extractor's buf.
func (e *Extractor) handleFUA(d []byte) {
	// If length is too small, ignore.
	if len(d) < minFUALen {
		return
	}

	// Get start and end indiciators from FU header.
	const FUHeadIdx = 1
	start := d[FUHeadIdx]&0x80 != 0
	end := d[FUHeadIdx]&0x40 != 0

	// If start, form new header, skip FU indicator only and set first byte to
	// new header. Otherwise, skip over both FU indicator and FU header.
	if start {
		newHead := (d[0] & 0xe0) | (d[1] & 0x1f)
		d = d[1:]
		d[0] = newHead
		if end {
			panic("bad fragmentation packet")
		}
		e.frag = true
		e.writeWithPrefix(d)
	} else {
		d = d[2:]
		if end {
			e.frag = false
		}
		e.writeNoPrefix(d)
	}
}

// writeWithPrefix writes a NAL unit to the Extractor's buf in byte stream format
// using the start code, and sends any ready prior access unit stored in the buf
// to the destination.
func (e *Extractor) writeWithPrefix(d []byte) {
	e.toWrite = append(e.toWrite, d...)
	curType, _ := NALType(e.toWrite)
	if e.buf.Len() != 0 && (curType == h264dec.NALTypeSPS || curType == h264dec.NALTypeIDR || curType == h264dec.NALTypeNonIDR) {
		e.buf.WriteTo(e.dst)
		e.buf.Reset()
		e.buf.Write(aud)
	}
	e.buf.Write(e.toWrite)
	e.toWrite = e.toWrite[:4]
}

// writeNoPrefix writes data to the Extractor's buf. This is used for non start
// fragmentations of a NALU.
func (e *Extractor) writeNoPrefix(d []byte) {
	e.buf.Write(d)
}

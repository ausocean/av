/*
NAME
  extract.go

DESCRIPTION
  extract.go provides a extractor for taking RTP HEVC (H265) and extracting
	access units.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package h265 provides an RTP h265 extractor that can extract h265 access units
// from an RTP stream.
package h265

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/ausocean/av/protocol/rtp"
)

// NALU types.
const (
	typeAggregation   = 48
	typeFragmentation = 49
	typePACI          = 50
)

// Buffer sizes.
const (
	maxAUSize  = 100000
	maxRTPSize = 4096
)

// Extractor is an RTP HEVC access unit extractor.
type Extractor struct {
	donl bool          // Indicates whether DONL and DOND will be used for the RTP stream.
	buf  *bytes.Buffer // Holds the current access unit.
	frag bool          // Indicates if we're currently dealing with a fragmentation packet.
}

// NewExtractor returns a new Extractor.
func NewExtractor(donl bool) *Extractor {
	return &Extractor{
		donl: donl,
		buf:  bytes.NewBuffer(make([]byte, 0, maxAUSize)),
	}
}

// Extract continually reads RTP packets from the io.Reader src and extracts
// H.265 access units which are written to the io.Writer dst. Extract expects
// that for each read from src, a single RTP packet is received.
func (e *Extractor) Extract(dst io.Writer, src io.Reader, delay time.Duration) error {
	buf := make([]byte, maxRTPSize)
	for {
		n, err := src.Read(buf)
		switch err {
		case nil: // Do nothing.
		default:
			if err == io.EOF {
				if e.buf.Len() == 0 {
					return io.EOF
				}
				return io.ErrUnexpectedEOF
			}
			return err
		}

		// Get payload from RTP packet.
		payload, err := rtp.Payload(buf[:n])
		if err != nil {
			return fmt.Errorf("could not get rtp payload, failed with err: %w\n", err)
		}
		nalType := (payload[0] >> 1) & 0x3f

		// If not currently fragmented then we ignore current write.
		if e.frag && nalType != typeFragmentation {
			e.buf.Reset()
			e.frag = false
			continue
		}

		switch nalType {
		case typeAggregation:
			e.handleAggregation(payload)
		case typeFragmentation:
			e.handleFragmentation(payload)
		case typePACI:
			e.handlePACI(payload)
		default:
			e.writeWithPrefix(payload)
		}

		markerIsSet, err := rtp.Marker(buf[:n])
		if err != nil {
			return fmt.Errorf("could not get marker bit, failed with err: %w\n", err)
		}

		if markerIsSet {
			_, err := e.buf.WriteTo(dst)
			if err != nil {
				// TODO: work out what to do here.
			}
			e.buf.Reset()
		}
	}
	return nil
}

// handleAggregation parses NAL units from an aggregation packet and writes
// them to the Extractor's buffer buf.
func (e *Extractor) handleAggregation(d []byte) {
	idx := 2
	for idx < len(d) {
		if e.donl {
			switch idx {
			case 2:
				idx += 2
			default:
				idx++
			}
		}
		size := int(binary.BigEndian.Uint16(d[idx:]))
		idx += 2
		nalu := d[idx : idx+size]
		idx += size
		e.writeWithPrefix(nalu)
	}
}

// handleFragmentation parses NAL units from fragmentation packets and writes
// them to the Extractor's buf.
func (e *Extractor) handleFragmentation(d []byte) {
	// Get start and end indiciators from FU header.
	start := d[2]&0x80 != 0
	end := d[2]&0x40 != 0

	b1 := (d[0] & 0x81) | ((d[2] & 0x3f) << 1)
	b2 := d[1]
	if start {
		d = d[1:]
		if e.donl {
			d = d[2:]
		}
		d[0] = b1
		d[1] = b2
	} else {
		d = d[3:]
		if e.donl {
			d = d[2:]
		}
	}

	switch {
	case start && !end:
		e.frag = true
		e.writeWithPrefix(d)
	case !start && end:
		e.frag = false
		fallthrough
	case !start && !end:
		e.writeNoPrefix(d)
	default:
		panic("bad fragmentation packet")
	}
}

// handlePACI will handle PACI packets
//
// TODO: complete this
func (e *Extractor) handlePACI(d []byte) {
	panic("unsupported nal type")
}

// write writes a NAL unit to the Extractor's buf in byte stream format using the
// start code.
func (e *Extractor) writeWithPrefix(d []byte) {
	const prefix = "\x00\x00\x00\x01"
	e.buf.Write([]byte(prefix))
	e.buf.Write(d)
}

// writeNoPrefix writes data to the Extractor's buf. This is used for non start
// fragmentations of a NALU.
func (e *Extractor) writeNoPrefix(d []byte) {
	e.buf.Write(d)
}

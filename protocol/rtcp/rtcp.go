/*
NAME
  rtcp.go

DESCRIPTION
  rtcp.go contains structs to describe RTCP packets, and functionality to form
  []bytes of these structs.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package RTCP provides RTCP data structures and a client for communicating
// with an RTCP service.
package rtcp

import (
	"encoding/binary"
)

// RTCP packet types.
const (
	typeSenderReport   = 200
	typeReceiverReport = 201
	typeDescription    = 202
)

// Source Description Item types.
const (
	typeCName = 1
)

const (
	reportBlockSize  = 6
	senderReportSize = 28
)

// ReceiverReport describes an RTCP receiver report packet.
type ReceiverReport struct {
	Header                   // Standard RTCP packet header.
	SenderSSRC uint32        // SSRC of the sender of this report.
	Blocks     []ReportBlock // Report blocks.
	Extensions [][4]byte     // Contains any extensions to the packet.
}

// Bytes returns a []byte of the ReceiverReport r.
func (r *ReceiverReport) Bytes(buf []byte) []byte {
	l := 8 + 4*reportBlockSize*len(r.Blocks) + 4*len(r.Extensions)
	if buf == nil || cap(buf) < l {
		buf = make([]byte, l)
	}
	buf = buf[:l]
	l = 1 + reportBlockSize*len(r.Blocks) + len(r.Extensions)
	r.writeHeader(buf, l)
	binary.BigEndian.PutUint32(buf[4:], r.SenderSSRC)

	idx := 8
	for _, b := range r.Blocks {
		binary.BigEndian.PutUint32(buf[idx:], b.SourceIdentifier)
		binary.BigEndian.PutUint32(buf[idx+4:], b.PacketsLost)
		buf[idx+4] = b.FractionLost
		binary.BigEndian.PutUint32(buf[idx+8:], b.HighestSequence)
		binary.BigEndian.PutUint32(buf[idx+12:], b.Jitter)
		binary.BigEndian.PutUint32(buf[idx+16:], b.SenderReportTs)
		binary.BigEndian.PutUint32(buf[idx+20:], b.SenderReportDelay)
		idx += 24
	}

	for _, e := range r.Extensions {
		copy(buf[idx:], e[:])
		idx += 4
	}

	return buf
}

// ReportBlock describes an RTCP report block used in Sender/Receiver Reports.
type ReportBlock struct {
	SourceIdentifier  uint32 // Source identifier.
	FractionLost      uint8  // Fraction of packets lost.
	PacketsLost       uint32 // Cumulative number of packets lost.
	HighestSequence   uint32 // Extended highest sequence number received.
	Jitter            uint32 // Interarrival jitter.
	SenderReportTs    uint32 // Last sender report timestamp.
	SenderReportDelay uint32 // Delay since last sender report.
}

// Description describes a source description RTCP packet.
type Description struct {
	Header         // Standard RTCP packet header.
	Chunks []Chunk // Chunks to describe items of each SSRC.
}

// Bytes returns an []byte of the Description d.
func (d *Description) Bytes(buf []byte) []byte {
	bodyLen := d.bodyLen()
	rem := bodyLen % 4
	if rem != 0 {
		bodyLen += 4 - rem
	}

	l := 4 + bodyLen
	if buf == nil || cap(buf) < l {
		buf = make([]byte, l)
	}
	buf = buf[:l]

	d.writeHeader(buf, bodyLen/4)
	idx := 4
	for _, c := range d.Chunks {
		binary.BigEndian.PutUint32(buf[idx:], c.SSRC)
		idx += 4
		for _, i := range c.Items {
			buf[idx] = i.Type
			buf[idx+1] = byte(len(i.Text))
			idx += 2
			copy(buf[idx:], i.Text)
			idx += len(i.Text)
		}
	}
	return buf
}

// bodyLen calculates the body length of a source description packet in bytes.
func (d *Description) bodyLen() int {
	var l int
	for _, c := range d.Chunks {
		l += c.len()
	}
	return l
}

// SenderReport describes an RTCP sender report.
type SenderReport struct {
	Header              // Standard RTCP header.
	SSRC         uint32 // SSRC of sender.
	TimestampMSW uint32 // Most significant word of timestamp.
	TimestampLSW uint32 // Least significant word of timestamp.
	RTPTimestamp uint32 // Current RTP timestamp.
	PacketCount  uint32 // Senders packet count.
	OctetCount   uint32 // Senders octet count.

	// Report blocks (unimplemented)
	// ...
}

// Bytes returns a []byte of the SenderReport.
func (r *SenderReport) Bytes() []byte {
	buf := make([]byte, senderReportSize)
	r.writeHeader(buf, senderReportSize-1)
	for i, w := range []uint32{
		r.SSRC,
		r.TimestampMSW,
		r.TimestampLSW,
		r.RTPTimestamp,
		r.PacketCount,
		r.OctetCount,
	} {
		binary.BigEndian.PutUint32(buf[i+4:], w)
	}
	return buf
}

// Header describes a standard RTCP packet header.
type Header struct {
	Version     uint8 // RTCP version.
	Padding     bool  // Padding indicator.
	ReportCount uint8 // Number of reports contained.
	Type        uint8 // Type of RTCP packet.
}

// SDESItem describes a source description item.
type SDESItem struct {
	Type uint8  // Type of item.
	Text []byte // Item text.
}

// Chunk describes a source description chunk for a given SSRC.
type Chunk struct {
	SSRC  uint32     // SSRC of the source being described by the below items.
	Items []SDESItem // Items describing the source.
}

// len returns the len of a chunk in bytes.
func (c *Chunk) len() int {
	tot := 4
	for _, i := range c.Items {
		tot += 2 + len(i.Text)
	}
	return tot
}

// writeHeader writes the standard RTCP header given a buffer to write to and l
// the RTCP body length that needs to be encoded into the header.
func (h Header) writeHeader(buf []byte, l int) {
	buf[0] = h.Version<<6 | asByte(h.Padding)<<5 | 0x1f&h.ReportCount
	buf[1] = h.Type
	binary.BigEndian.PutUint16(buf[2:], uint16(l))
}

func asByte(b bool) byte {
	if b {
		return 0x01
	}
	return 0x00
}

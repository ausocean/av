/*
NAME
  rtp.go

DESCRIPTION
  See Readme.md

  See https://tools.ietf.org/html/rfc6184 and https://tools.ietf.org/html/rfc3550
  for rtp-h264 and rtp standards.

AUTHOR
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package rtp provides a data structure intended to encapsulate the properties
// of an rtp packet and also functions to allow manipulation of these packets.
package rtp

import (
	"encoding/binary"
)

const (
	rtpVer           = 2                                // Version of RTP that this package is compatible with.
	defaultHeadSize  = 12                               // Header size of an rtp packet.
	defPayloadSize   = sendSize                         // Default payload size for the rtp packet.
	defPktSize       = defaultHeadSize + defPayloadSize // Default packet size is header size + payload size.
	optionalFieldIdx = 12                               // This is the idx of optional fields including CSRC and extension header in an RTP packet.
)

// Pkt provides fields consistent with RFC3550 definition of an rtp packet
// The padding indicator does not need to be set manually, only the padding length
type Packet struct {
	Version     uint8           // Version (currently 2).
	PaddingFlag bool            // Padding indicator (0 => padding, 1 => padding).
	ExtHeadFlag bool            // Extension header indicator.
	CSRCCount   uint8           // CSRC count.
	Marker      bool            // Marker bit.
	PacketType  uint8           // Packet type.
	Sync        uint16          // Sync number.
	Timestamp   uint32          // Timestamp.
	SSRC        uint32          // Synchronisation source identifier.
	CSRC        [][4]byte       // Contributing source identifier.
	Extension   ExtensionHeader // Header extension.
	Payload     []byte          // Payload data.
	Padding     []byte          // No of bytes of padding.
}

// ExtensionHeader header provides fields for an RTP packet extension header.
type ExtensionHeader struct {
	ID     uint16
	Header [][4]byte
}

// Bytes provides a byte slice of the packet
func (p *Packet) Bytes(buf []byte) []byte {
	// Calculate the required length for the RTP packet.
	headerExtensionLen := 0
	if p.ExtHeadFlag {
		headerExtensionLen = int(4 + 4*len(p.Extension.Header))
	}
	requiredPktLen := defaultHeadSize + int(4*p.CSRCCount) + headerExtensionLen + len(p.Payload) + len(p.Padding)

	// Create new space if no buffer is given, or it doesn't have sufficient capacity.
	if buf == nil || requiredPktLen > cap(buf) {
		buf = make([]byte, requiredPktLen, defPktSize)
	}
	buf = buf[:requiredPktLen]

	// Start encoding fields into the buffer.
	buf[0] = p.Version<<6 | asByte(p.PaddingFlag)<<5 | asByte(p.ExtHeadFlag)<<4 | p.CSRCCount
	buf[1] = asByte(p.Marker)<<7 | p.PacketType
	binary.BigEndian.PutUint16(buf[2:4], p.Sync)
	binary.BigEndian.PutUint32(buf[4:8], p.Timestamp)
	binary.BigEndian.PutUint32(buf[8:12], p.SSRC)

	// If there is a CSRC count, add the CSRC to the buffer.
	if p.CSRCCount != 0 {
		if p.CSRCCount != uint8(len(p.CSRC)) {
			panic("CSRC count in RTP packet is incorrect")
		}
		for i := 0; i < int(p.CSRCCount); i++ {
			copy(buf[12+i*4:], p.CSRC[i][:])
		}
	}

	// This is our current index for writing to the buffer.
	idx := int(12 + 4*p.CSRCCount)

	// If there is an extension field, add this to the buffer.
	if p.ExtHeadFlag {
		binary.BigEndian.PutUint16(buf[idx:idx+2], p.Extension.ID)
		idx += 2
		binary.BigEndian.PutUint16(buf[idx:idx+2], uint16(len(p.Extension.Header)))
		idx += 2
		for i := 0; i < len(p.Extension.Header); i++ {
			copy(buf[idx+i*4:], p.Extension.Header[i][:])
		}
		idx += len(p.Extension.Header) * 4
	}

	// If there is payload, add to the buffer.
	if p.Payload != nil {
		copy(buf[idx:], p.Payload)
		idx += len(p.Payload)
	}

	// Finally, if there is padding, add to the buffer.
	if p.Padding != nil {
		copy(buf[idx:], p.Padding)
	}

	return buf
}

func asByte(b bool) byte {
	if b {
		return 0x01
	}
	return 0x00
}

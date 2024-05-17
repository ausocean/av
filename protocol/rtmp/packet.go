/*
NAME
  packet.go

DESCRIPTION
  RTMP packet functionality.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package rtmp

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ausocean/av/protocol/rtmp/amf"
)

// Packet types.
const (
	packetTypeChunkSize        = 0x01
	packetTypeBytesReadReport  = 0x03
	packetTypeControl          = 0x04
	packetTypeServerBW         = 0x05
	packetTypeClientBW         = 0x06
	packetTypeAudio            = 0x08
	packetTypeVideo            = 0x09
	packetTypeFlexStreamSend   = 0x0F // not implemented
	packetTypeFlexSharedObject = 0x10 // not implemented
	packetTypeFlexMessage      = 0x11 // not implemented
	packetTypeInfo             = 0x12
	packetTypeInvoke           = 0x14
	packetTypeFlashVideo       = 0x16 // not implemented
)

// Header sizes.
const (
	headerSizeLarge   = 0
	headerSizeMedium  = 1
	headerSizeSmall   = 2
	headerSizeMinimum = 3
	headerSizeAuto    = 4
)

// Special channels.
const (
	chanBytesRead = 0x02
	chanControl   = 0x03
	chanSource    = 0x04
)

// headerSizes defines header sizes for header types 0, 1, 2 and 3 respectively:
//   0: full header (12 bytes)
//   1: header without message ID (8 bytes)
//   2: basic header + timestamp (4 byes)
//   3: basic header (chunk type and stream ID) (1 byte)
var headerSizes = [...]int{12, 8, 4, 1}

// packet represents an RTMP packet.
type packet struct {
	headerType      uint8
	packetType      uint8
	channel         int32
	hasAbsTimestamp bool
	timestamp       uint32
	streamID        uint32
	bodySize        uint32
	bytesRead       uint32
	buf             []byte
	body            []byte
}

func (pkt *packet) isReady() bool {
	return pkt.bytesRead == pkt.bodySize
}

// readFrom reads a packet from the RTMP connection.
func (pkt *packet) readFrom(c *Conn) error {
	var hbuf [fullHeaderSize]byte
	header := hbuf[:]

	_, err := c.read(header[:1])
	if err != nil {
		c.log(DebugLevel, pkg+"failed to read packet header 1st byte", "error", err.Error())
		if err == io.EOF {
			c.log(WarnLevel, pkg+"EOF error; connection likely terminated")
		}
		return fmt.Errorf("failed to read packet header 1st byte: %w", err)
	}
	pkt.headerType = (header[0] & 0xc0) >> 6
	pkt.channel = int32(header[0] & 0x3f)
	header = header[1:]

	switch {
	case pkt.channel == 0:
		_, err = c.read(header[:1])
		if err != nil {
			c.log(DebugLevel, pkg+"failed to read packet header 2nd byte", "error", err.Error())
			return fmt.Errorf("failed to read packet header second byte: %w", err)
		}
		header = header[1:]
		pkt.channel = int32(header[0]) + 64

	case pkt.channel == 1:
		_, err = c.read(header[:2])
		if err != nil {
			c.log(DebugLevel, pkg+"failed to read packet header 3rd byte", "error", err.Error())
			return fmt.Errorf("failed to read packet header 3rd byte: %w", err)
		}
		header = header[2:]
		pkt.channel = int32(binary.BigEndian.Uint16(header[:2])) + 64
	}

	if pkt.channel >= c.channelsAllocatedIn {
		n := pkt.channel + 10
		timestamp := append(c.channelTimestamp, make([]int32, 10)...)

		var pkts []*packet
		if c.channelsIn == nil {
			pkts = make([]*packet, n)
		} else {
			pkts = append(c.channelsIn[:pkt.channel:pkt.channel], make([]*packet, 10)...)
		}

		c.channelTimestamp = timestamp
		c.channelsIn = pkts

		for i := int(c.channelsAllocatedIn); i < len(c.channelTimestamp); i++ {
			c.channelTimestamp[i] = 0
		}
		for i := int(c.channelsAllocatedIn); i < int(n); i++ {
			c.channelsIn[i] = nil
		}
		c.channelsAllocatedIn = n
	}

	size := headerSizes[pkt.headerType]
	switch {
	case size == fullHeaderSize:
		pkt.hasAbsTimestamp = true
	case size < fullHeaderSize:
		if c.channelsIn[pkt.channel] != nil {
			*pkt = *(c.channelsIn[pkt.channel])
		}
	}
	size--

	if size > 0 {
		_, err = c.read(header[:size])
		if err != nil {
			c.log(DebugLevel, pkg+"failed to read packet header", "error", err.Error())
			return fmt.Errorf("failed to read packet header: %w", err)
		}
	}
	hSize := len(hbuf) - len(header) + size

	if size >= 3 {
		pkt.timestamp = amf.DecodeInt24(header[:3])
		pkt.bytesRead = 0
		if size >= 6 {
			pkt.bodySize = amf.DecodeInt24(header[3:6])

			if size > 6 {
				pkt.packetType = header[6]
				if size == 11 {
					pkt.streamID = amf.DecodeInt32LE(header[7:11])
				}
			}
		}
	}

	extendedTimestamp := pkt.timestamp == 0xffffff
	if extendedTimestamp {
		_, err = c.read(header[size : size+4])
		if err != nil {
			c.log(DebugLevel, pkg+"failed to read extended timestamp", "error", err.Error())
			return fmt.Errorf("failed to read extended timestamp: %w", err)
		}
		pkt.timestamp = amf.DecodeInt32(header[size : size+4])
		hSize += 4
	}

	pkt.resize(pkt.bodySize, pkt.headerType)

	if pkt.bodySize > c.inChunkSize {
		c.log(WarnLevel, pkg+"reading large packet", "size", int(pkt.bodySize))
	}

	nToRead := pkt.bodySize - pkt.bytesRead
	nChunk := c.inChunkSize
	if nToRead < nChunk {
		nChunk = nToRead
	}

	n, err := c.read(pkt.body[pkt.bytesRead : pkt.bytesRead+nChunk])
	if err != nil {
		c.log(DebugLevel, pkg+"failed to read packet body", "error", err.Error())
		return fmt.Errorf("failed to read packet body: %w", err)
	}

	if uint32(n) != nChunk {
		return fmt.Errorf("did not read correct number of bytes, read: %d, expected: %d", n, nChunk)
	}

	pkt.bytesRead += nChunk

	// Keep the packet as a reference for other packets on this channel.
	if c.channelsIn[pkt.channel] == nil {
		c.channelsIn[pkt.channel] = &packet{}
	}
	*(c.channelsIn[pkt.channel]) = *pkt

	if extendedTimestamp {
		c.channelsIn[pkt.channel].timestamp = 0xffffff
	}

	if pkt.isReady() {
		if !pkt.hasAbsTimestamp {
			// Timestamps seem to always be relative.
			pkt.timestamp += uint32(c.channelTimestamp[pkt.channel])
		}
		c.channelTimestamp[pkt.channel] = int32(pkt.timestamp)

		c.channelsIn[pkt.channel].body = nil
		c.channelsIn[pkt.channel].hasAbsTimestamp = false
	}

	return nil
}

// resize adjusts the packet's storage (if necessary) to accommodate a body of the given size and header type.
// When headerSizeAuto is specified, the header type is computed based on packet type.
func (pkt *packet) resize(size uint32, ht uint8) {
	if cap(pkt.buf) < fullHeaderSize+int(size) {
		pkt.buf = make([]byte, fullHeaderSize+size)
	}
	pkt.body = pkt.buf[fullHeaderSize:]
	if ht != headerSizeAuto {
		pkt.headerType = ht
		return
	}
	switch pkt.packetType {
	case packetTypeVideo, packetTypeAudio:
		if pkt.timestamp == 0 {
			pkt.headerType = headerSizeLarge
		} else {
			pkt.headerType = headerSizeMedium
		}
	case packetTypeInfo:
		pkt.headerType = headerSizeLarge
		pkt.bodySize += 16
	default:
		pkt.headerType = headerSizeMedium
	}
}

// writeTo writes a packet to the RTMP connection.
// Packets are written in chunks which are c.chunkSize in length (128 bytes by default).
// We defer sending small audio packets and combine consecutive small audio packets where possible to reduce I/O.
// When queue is true, we expect a response to this request and cache the method on c.methodCalls.
func (pkt *packet) writeTo(c *Conn, queue bool) error {
	if pkt.body == nil || pkt.bodySize == 0 {
		return errInvalidBody
	}

	if pkt.channel >= c.channelsAllocatedOut {
		c.log(DebugLevel, pkg+"growing channelsOut", "channel", pkt.channel)
		n := int(pkt.channel + 10)

		var pkts []*packet
		if c.channelsOut == nil {
			pkts = make([]*packet, n)
		} else {
			pkts = append(c.channelsOut[:pkt.channel:pkt.channel], make([]*packet, 10)...)
		}
		c.channelsOut = pkts

		for i := int(c.channelsAllocatedOut); i < n; i++ {
			c.channelsOut[i] = nil
		}

		c.channelsAllocatedOut = int32(n)
	}

	prevPkt := c.channelsOut[pkt.channel]
	var last int
	if prevPkt != nil && pkt.headerType != headerSizeLarge {
		// Compress header by using the previous packet's attributes.
		if prevPkt.bodySize == pkt.bodySize && prevPkt.packetType == pkt.packetType && pkt.headerType == headerSizeMedium {
			pkt.headerType = headerSizeSmall
		}

		if prevPkt.timestamp == pkt.timestamp && pkt.headerType == headerSizeSmall {
			pkt.headerType = headerSizeMinimum
		}

		last = int(prevPkt.timestamp)
	}

	if pkt.headerType > 3 {
		c.log(WarnLevel, pkg+"unexpected header type", "type", pkt.headerType)
		return errInvalidHeader
	}

	// The complete packet starts from headerSize _before_ the start the body.
	// origIdx is the original offset, which will be 0 for a full (12-byte) header or 11 for a minimum (1-byte) header.
	buf := pkt.buf
	hSize := headerSizes[pkt.headerType]
	origIdx := fullHeaderSize - hSize

	// Adjust 1 or 2 bytes depending on the channel.
	cSize := 0
	switch {
	case pkt.channel > 319:
		cSize = 2
	case pkt.channel > 63:
		cSize = 1
	}

	if cSize != 0 {
		origIdx -= cSize
		hSize += cSize
	}

	// Adjust 4 bytes for the timestamp.
	var ts uint32
	if prevPkt != nil {
		ts = uint32(int(pkt.timestamp) - last)
	}
	if ts >= 0xffffff {
		origIdx -= 4
		hSize += 4
		c.log(DebugLevel, pkg+"larger timestamp than 24 bits", "timestamp", ts)
	}

	headerIdx := origIdx

	ch := pkt.headerType << 6
	switch cSize {
	case 0:
		ch |= byte(pkt.channel)
	case 1:
		// Do nothing.
	case 2:
		ch |= 1
	}
	buf[headerIdx] = ch
	headerIdx++

	if cSize != 0 {
		tmp := pkt.channel - 64
		buf[headerIdx] = byte(tmp & 0xff)
		headerIdx++

		if cSize == 2 {
			buf[headerIdx] = byte(tmp >> 8)
			headerIdx++
		}
	}

	if headerSizes[pkt.headerType] > 1 {
		tmp := ts
		if ts > 0xffffff {
			tmp = 0xffffff
		}
		amf.EncodeInt24(buf[headerIdx:], tmp)
		headerIdx += 3 // 24bits
	}

	if headerSizes[pkt.headerType] > 4 {
		amf.EncodeInt24(buf[headerIdx:], pkt.bodySize)
		headerIdx += 3 // 24bits
		buf[headerIdx] = pkt.packetType
		headerIdx++
	}

	if headerSizes[pkt.headerType] > 8 {
		binary.LittleEndian.PutUint32(buf[headerIdx:headerIdx+4], pkt.streamID)
		headerIdx += 4 // 32bits
	}

	if ts >= 0xffffff {
		amf.EncodeInt32(buf[headerIdx:], ts)
		headerIdx += 4 // 32bits
	}

	size := int(pkt.bodySize)
	chunkSize := int(c.outChunkSize)

	if c.deferred == nil {
		// Defer sending small audio packets (at most once).
		if pkt.packetType == packetTypeAudio && size < chunkSize {
			c.deferred = buf[origIdx:][:size+hSize]
			c.log(DebugLevel, pkg+"deferred sending packet", "size", size, "la", c.link.conn.LocalAddr(), "ra", c.link.conn.RemoteAddr())
			return nil
		}
	} else {
		// Send previously deferred packet if combining it with the next one would exceed the chunk size.
		if len(c.deferred)+size+hSize > chunkSize {
			c.log(DebugLevel, pkg+"sending deferred packet separately", "size", len(c.deferred))
			_, err := c.write(c.deferred)
			if err != nil {
				return fmt.Errorf("could not write deferred packet: %w", err)
			}
			c.deferred = nil
		}
	}

	// TODO(kortschak): Rewrite this horrific peice of premature optimisation.
	c.log(DebugLevel, pkg+"sending packet", "size", size, "la", c.link.conn.LocalAddr(), "ra", c.link.conn.RemoteAddr())
	for size+hSize != 0 {
		if chunkSize > size {
			chunkSize = size
		}
		bytes := buf[origIdx:][:chunkSize+hSize]
		if c.deferred != nil {
			// Prepend the previously deferred packet and write it with the current one.
			c.log(DebugLevel, pkg+"combining deferred packet", "size", len(c.deferred))
			bytes = append(c.deferred, bytes...)
		}
		_, err := c.write(bytes)
		if err != nil {
			return fmt.Errorf("could not write combined packet: %w", err)
		}
		c.deferred = nil

		size -= chunkSize
		origIdx += chunkSize + hSize
		hSize = 0

		if size > 0 {
			// We are writing the 2nd or subsequent chunk.
			origIdx -= 1 + cSize
			hSize = 1 + cSize

			if ts >= 0xffffff {
				origIdx -= 4
				hSize += 4
			}

			buf[origIdx] = 0xc0 | ch

			if cSize != 0 {
				tmp := int(pkt.channel) - 64
				buf[origIdx+1] = byte(tmp)

				if cSize == 2 {
					buf[origIdx+2] = byte(tmp >> 8)
				}
			}
			if ts >= 0xffffff {
				extendedTimestamp := buf[origIdx+1+cSize:]
				amf.EncodeInt32(extendedTimestamp[:4], ts)
			}
		}
	}

	// If we invoked a remote method and queue is true, we queue the method until the result arrives.
	if pkt.packetType == packetTypeInvoke && queue {
		buf := pkt.body[1:]
		meth := amf.DecodeString(buf)
		c.log(DebugLevel, pkg+"queuing method "+meth)
		buf = buf[3+len(meth):]
		txn := int32(amf.DecodeNumber(buf[:8]))
		c.methodCalls = append(c.methodCalls, method{name: meth, num: txn})
	}

	if c.channelsOut[pkt.channel] == nil {
		c.channelsOut[pkt.channel] = &packet{}
	}
	*(c.channelsOut[pkt.channel]) = *pkt

	return nil
}

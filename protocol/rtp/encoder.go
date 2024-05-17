/*
NAME
  encoder.go

DESCRIPTION
  See Readme.md

AUTHOR
  Saxon Nelson-Milton (saxon@ausocean.org)

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package rtp

import (
	"io"
	"math/rand"
	"time"
)

const (
	defaultPktType = 33
	timestampFreq  = 90000 // Hz
	mtsSize        = 188
	bufferSize     = 1000
	sendSize       = 7 * 188
)

// Encoder implements io writer and provides functionality to wrap data into
// rtp packets
type Encoder struct {
	dst           io.Writer
	ssrc          uint32
	seqNo         uint16
	clock         time.Duration
	frameInterval time.Duration
	lastTime      time.Time
	fps           int
	buffer        []byte
	pktSpace      [defPktSize]byte
}

// NewEncoder returns a new Encoder type given an io.Writer - the destination
// after encoding and the desired fps
func NewEncoder(dst io.Writer, fps int) *Encoder {
	return &Encoder{
		dst:           dst,
		ssrc:          rand.Uint32(),
		frameInterval: time.Duration(float64(time.Second) / float64(fps)),
		fps:           fps,
		buffer:        make([]byte, 0),
	}
}

// Write provides an interface between a prior encoder and this rtp encoder,
// so that multiple layers of packetization can occur.
func (e *Encoder) Write(data []byte) (int, error) {
	e.buffer = append(e.buffer, data...)
	if len(e.buffer) < sendSize {
		return len(data), nil
	}
	buf := e.buffer
	for len(buf) != 0 {
		l := min(sendSize, len(buf))
		err := e.Encode(buf[:l])
		if err != nil {
			return len(data), err
		}
		buf = buf[l:]
	}
	e.buffer = e.buffer[:0]
	return len(data), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Encode takes a nalu unit and encodes it into an rtp packet and
// writes to the io.Writer given in NewEncoder
func (e *Encoder) Encode(payload []byte) error {
	pkt := Packet{
		Version:    rtpVer,           // version
		CSRCCount:  0,                // CSRC count
		PacketType: defaultPktType,   // 33 for mpegts
		Sync:       e.nxtSeqNo(),     // sequence number
		Timestamp:  e.nxtTimestamp(), // timestamp
		SSRC:       e.ssrc,           // source identifier
		Payload:    payload,
		Padding:    nil,
	}
	_, err := e.dst.Write(pkt.Bytes(e.pktSpace[:defPktSize]))
	if err != nil {
		return err
	}
	e.tick()
	return nil
}

// tick advances the clock one frame interval.
func (e *Encoder) tick() {
	e.clock += e.frameInterval
}

// nxtTimestamp gets the next timestamp
func (e *Encoder) nxtTimestamp() uint32 {
	return uint32(e.clock.Seconds() * timestampFreq)
}

// nxtSeqNo gets the next rtp packet sequence number
func (e *Encoder) nxtSeqNo() uint16 {
	e.seqNo++
	return e.seqNo - 1
}

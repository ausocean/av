/*
NAME
  encoder.go

DESCRIPTION
  See Readme.md

AUTHOR
  Dan Kortschak <dan@ausocean.org>
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package flv

import (
	"io"
	"time"
)

const (
	inputChanLength  = 500
	outputChanLength = 500
	videoHeaderSize  = 16
	audioHeaderSize  = 2
	sampleRate44Khz  = 3
	cscPacket        = 0
	dataPacket       = 1
)

// Data representing silent audio (required for youtube)
var (
	dummyCSC = []byte{
		0x12, 0x08, 0x56, 0xe5, 0x00,
	}

	dummyDataPacket = []byte{
		0xdc, 0x00, 0x4c, 0x61, 0x76, 0x63, 0x35, 0x38,
		0x2e, 0x36, 0x2e, 0x31, 0x30, 0x32, 0x00, 0x02, 0x30,
		0x40, 0x0e,
	}
)

// Encoder provides properties required for the generation of flv video
// from raw video data
type Encoder struct {
	dst             io.WriteCloser
	fps             int
	start           time.Time
	stereoAudio     bool
	audioConfigSent bool
}

// NewEncoder retuns a new FLV encoder.
func NewEncoder(dst io.WriteCloser, stereoAudio bool, fps int) (*Encoder, error) {
	e := Encoder{
		dst:             dst,
		fps:             fps,
		stereoAudio:     stereoAudio,
		audioConfigSent: false,
	}
	return &e, nil
}

// getNextTimestamp generates and returns the next timestamp based on current time
func (e *Encoder) getNextTimestamp() (timestamp uint32) {
	if e.start.IsZero() {
		e.start = time.Now()
		return 0
	}
	return uint32(time.Now().Sub(e.start).Seconds() * float64(1000))
}

// http://www.itu.int/rec/dologin_pub.asp?lang=e&id=T-REC-H.264-200305-S!!PDF-E&type=items
// Table 7-1 NAL unit type codes
const (
	nonIdrPic   = 1
	idrPic      = 5
	suppEnhInf  = 6
	seqParamSet = 7
	paramSet    = 8
)

// isKeyFrame returns true if the passed frame data represents that of a keyframe
// FIXME(kortschak): Clarify and document the logic of this functions.
func isKeyFrame(frame []byte) bool {
	sc := frameScanner{buf: frame}
	for {
		b, ok := sc.readByte()
		if !ok {
			return false
		}
		for i := 1; b == 0x00 && i < 4; i++ {
			b, ok = sc.readByte()
			if !ok {
				return false
			}
			if b != 0x01 || (i != 3 && i != 2) {
				continue
			}

			b, ok = sc.readByte()
			if !ok {
				return false
			}
			switch nalTyp := b & 0x1f; nalTyp {
			case idrPic:
				return true
			case nonIdrPic:
				return false
			}
		}
	}
	return false
}

// isSequenceHeader returns true if the passed frame data represents that of a
// a sequence header.
// FIXME(kortschak): Clarify and document the logic of this functions.
func isSequenceHeader(frame []byte) bool {
	sc := frameScanner{buf: frame}
	for {
		b, ok := sc.readByte()
		if !ok {
			return false
		}
		for i := 1; b == 0x00 && i != 4; i++ {
			b, ok = sc.readByte()
			if !ok {
				return false
			}
			if b != 0x01 || (i != 2 && i != 3) {
				continue
			}

			b, ok = sc.readByte()
			if !ok {
				return false
			}
			switch nalTyp := b & 0x1f; nalTyp {
			case suppEnhInf, seqParamSet, paramSet:
				return true
			case nonIdrPic, idrPic:
				return false
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

// WriteVideo takes raw h264 and encodes into flv, then
// writes to the encoders io.Writer destination.
func (e *Encoder) WriteVideo(videoFrame []byte) (int, error) {
	var frameType byte
	var packetType byte
	var totalWritten int = 0

	if e.start.IsZero() {
		// This is the first frame, so write the PreviousTagSize0.
		//
		// See https://download.macromedia.com/f4v/video_file_format_spec_v10_1.pdf
		// section E.3.
		var zero [4]byte
		written, err := e.dst.Write(zero[:])
		totalWritten += written
		if err != nil {
			return totalWritten, err
		}
	}
	timeStamp := e.getNextTimestamp()
	if isKeyFrame(videoFrame) {
		frameType = KeyFrameType
	} else {
		frameType = InterFrameType
	}
	if isSequenceHeader(videoFrame) {
		packetType = SequenceHeader
	} else {
		packetType = AVCNALU
	}

	tag := VideoTag{
		TagType:           uint8(VideoTagType),
		DataSize:          uint32(len(videoFrame)) + DataHeaderLength,
		Timestamp:         timeStamp,
		TimestampExtended: NoTimestampExtension,
		FrameType:         frameType,
		Codec:             H264,
		PacketType:        packetType,
		CompositionTime:   0,
		Data:              videoFrame,
		PrevTagSize:       uint32(videoHeaderSize + len(videoFrame)),
	}
	written, err := e.dst.Write(tag.Bytes())
	totalWritten += written
	if err != nil {
		return totalWritten, err
	}

	return totalWritten, nil
}

// An adapter that provides the io.Writer interface for WriteVideo.
type VideoWriterAdapter struct {
	Encoder *Encoder
}

// Write implements the io.Writer interface.
// It takes in a single frame of raw h264 and encodes in in an flv.
func (a *VideoWriterAdapter) Write(p []byte) (n int, err error) {
	n, err = a.Encoder.WriteVideo(p)
	return n, err
}

// A decorator that writes dummy audio at the same time as video.
// Useful for when you don't want any audio.
type DummyAudioDecorator struct {
	Encoder *Encoder
}

// Write implements the io.Writer interface
// It takes in a single frame of raw h264, writes dummy audio, and encodes them into an flv.
func (d *DummyAudioDecorator) Write(frame []byte) (int, error) {
	// If the audio config hasn't been sent yet, then write the dummy CSC
	// Otherwise, write the normal dummy audio packet
	if !d.Encoder.audioConfigSent {
		d.Encoder.WriteAudio(dummyCSC)
	}
	d.Encoder.WriteAudio(dummyDataPacket)
	n, err := d.Encoder.WriteVideo(frame)

	return n, err
}

// WriteAudio takes raw aac and encodes into flv, then
// writes to the encoders io.Writer destination.
func (e *Encoder) WriteAudio(audioFrame []byte) (int, error) {
	var totalWritten int = 0

	if e.start.IsZero() {
		// This is the first frame, so write the PreviousTagSize0.
		//
		// See https://download.macromedia.com/f4v/video_file_format_spec_v10_1.pdf
		// section E.3.
		var zero [4]byte
		written, err := e.dst.Write(zero[:])
		totalWritten += written
		if err != nil {
			return totalWritten, err
		}
	}
	timeStamp := e.getNextTimestamp()
	packetType := dataPacket
	// If we haven't sent the audio config yet, then this frame must be the config.
	if !e.audioConfigSent {
		packetType = cscPacket
		e.audioConfigSent = true
	}
	tag := AudioTag{
		TagType:           uint8(AudioTagType),
		DataSize:          uint32(len(audioFrame)) + audioHeaderSize,
		Timestamp:         timeStamp,
		TimestampExtended: NoTimestampExtension,
		SoundFormat:       AACAudioFormat,
		SoundRate:         sampleRate44Khz,
		SoundSize:         true,
		SoundType:         e.stereoAudio,
		PacketType:        uint8(packetType),
		Data:              audioFrame,
		PrevTagSize:       uint32(len(audioFrame)) + audioHeaderSize + 11,
	}
	written, err := e.dst.Write(tag.Bytes())
	totalWritten += written
	if err != nil {
		return totalWritten, err
	}

	return totalWritten, nil
}

// An adapter that provides the io.Writer interface for WriteAudio.
type AudioWriterAdapter struct {
	Encoder *Encoder
}

// Write implements the io.Writer interface.
// It takes in a single frame of raw aac and encodes in in an flv.
func (a *AudioWriterAdapter) Write(p []byte) (n int, err error) {
	n, err = a.Encoder.WriteAudio(p)
	return n, err
}

// Close will close the encoder destination.
func (e *Encoder) Close() error {
	return e.dst.Close()
}

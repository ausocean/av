/*
NAME
  flv.go

DESCRIPTION
  See Readme.md

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


// See https://wwwimages2.adobe.com/content/dam/acom/en/devnet/flv/video_file_format_spec_v10.pdf
// for format specification.

// Package flv provides FLV encoding and related functions.
package flv

import "encoding/binary"

const (
	maxVideoTagSize = 10000
	maxAudioTagSize = 10000
)

const (
	VideoTagType         = 9
	AudioTagType         = 8
	KeyFrameType         = 1
	InterFrameType       = 2
	H264                 = 7
	AVCNALU              = 1
	SequenceHeader       = 0
	DataHeaderLength     = 5
	NoTimestampExtension = 0
	AACAudioFormat       = 10
	PCMAudioFormat       = 0
)

const (
	sizeofFLVTagHeader = 11
	sizeofPrevTagSize  = 4
)

const version = 0x01

// FLV is big-endian.
var order = binary.BigEndian

// orderPutUint24 is a binary.BigEndian method look-alike for
// writing 24 bit words to a byte slice.
func orderPutUint24(b []byte, v uint32) {
	_ = b[2] // early bounds check to guarantee safety of writes below
	b[0] = byte(v >> 16)
	b[1] = byte(v >> 8)
	b[2] = byte(v)
}

type VideoTag struct {
	TagType           uint8
	DataSize          uint32
	Timestamp         uint32
	TimestampExtended uint8
	FrameType         uint8
	Codec             uint8
	PacketType        uint8
	CompositionTime   uint32
	Data              []byte
	PrevTagSize       uint32
}

func (t *VideoTag) Bytes() []byte {
	// FIXME(kortschak): This should probably be an encoding.BinaryMarshaler.
	// This will allow handling of invalid field values.

	b := make([]byte, t.DataSize+sizeofFLVTagHeader+sizeofPrevTagSize)

	b[0] = t.TagType
	orderPutUint24(b[1:4], t.DataSize)
	orderPutUint24(b[4:7], t.Timestamp)
	b[7] = t.TimestampExtended
	b[11] = t.FrameType<<4 | t.Codec
	b[12] = t.PacketType
	orderPutUint24(b[13:16], t.CompositionTime)
	copy(b[16:], t.Data)
	order.PutUint32(b[len(b)-4:], t.PrevTagSize)

	return b
}

type AudioTag struct {
	TagType           uint8
	DataSize          uint32
	Timestamp         uint32
	TimestampExtended uint8
	SoundFormat       uint8
	SoundRate         uint8
	SoundSize         bool
	SoundType         bool
	Data              []byte
	PrevTagSize       uint32
}

func (t *AudioTag) Bytes() []byte {
	// FIXME(kortschak): This should probably be an encoding.BinaryMarshaler.
	// This will allow handling of invalid field values.

	b := make([]byte, t.DataSize+sizeofFLVTagHeader+sizeofPrevTagSize)

	b[0] = t.TagType
	orderPutUint24(b[1:4], t.DataSize)
	orderPutUint24(b[4:7], t.Timestamp)
	b[7] = t.TimestampExtended
	b[11] = t.SoundFormat<<4 | t.SoundRate<<2 | btb(t.SoundSize)<<1 | btb(t.SoundType)
	copy(b[12:], t.Data)
	order.PutUint32(b[len(b)-4:], t.PrevTagSize)

	return b
}

func btb(b bool) byte {
	if b {
		return 1
	}
	return 0
}

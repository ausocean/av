/*
NAME
  flv_test.go

DESCRIPTION
  flv_test.go provides testing for functionality provided in flv.go.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package flv

import (
	"bytes"
	"testing"
)

// TestVideoTagBytes checks that we can correctly get a []byte representation
// of a VideoTag using VideoTag.Bytes().
func TestVideoTagBytes(t *testing.T) {
	tests := []struct {
		tag      VideoTag
		expected []byte
	}{
		{
			tag: VideoTag{
				TagType:           VideoTagType,
				DataSize:          12,
				Timestamp:         1234,
				TimestampExtended: 56,
				FrameType:         KeyFrameType,
				Codec:             H264,
				PacketType:        AVCNALU,
				CompositionTime:   0,
				Data:              []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			},
			expected: []byte{
				0x09,             // TagType.
				0x00, 0x00, 0x0c, // DataSize.
				0x00, 0x04, 0xd2, // Timestamp.
				0x38,             // TimestampExtended.
				0x00, 0x00, 0x00, // StreamID. (always 0)
				0x17,             // FrameType=0001, Codec=0111
				0x01,             // PacketType.
				0x00, 0x00, 0x00, // CompositionTime
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, // VideoData.
				0x00, 0x00, 0x00, 0x00, // previousTagSize.
			},
		},
	}

	for testNum, test := range tests {
		got := test.tag.Bytes()
		if !bytes.Equal(got, test.expected) {
			t.Errorf("did not get expected result for test: %v.\n Got: %v\n Want: %v\n", testNum, got, test.expected)
		}
	}
}

// TestAudioTagBytes checks that we can correctly get a []byte representation of
// an AudioTag using AudioTag.Bytes().
func TestAudioTagBytes(t *testing.T) {
	tests := []struct {
		tag      AudioTag
		expected []byte
	}{
		{
			tag: AudioTag{
				TagType:           AudioTagType,
				DataSize:          9,
				Timestamp:         1234,
				TimestampExtended: 56,
				SoundFormat:       AACAudioFormat,
				SoundRate:         3,
				SoundSize:         true,
				SoundType:         true,
				PacketType:        1,
				Data:              []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			},
			expected: []byte{
				0x08,             // TagType.
				0x00, 0x00, 0x09, // DataSize.
				0x00, 0x04, 0xd2, // Timestamp.
				0x38,             // TimestampExtended.
				0x00, 0x00, 0x00, // StreamID. (always 0)
				0xaf,                                     // SoundFormat=1010,SoundRate=11,SoundSize=1,SoundType=1
				0x01,                                     // PacketType = dataPacket
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, // AudioData.
				0x00, 0x00, 0x00, 0x00, // previousTagSize.
			},
		},
	}

	for testNum, test := range tests {
		got := test.tag.Bytes()
		if !bytes.Equal(got, test.expected) {
			t.Errorf("did not get expected result for test: %v.\n Got: %v\n Want: %v\n", testNum, got, test.expected)
		}
	}
}

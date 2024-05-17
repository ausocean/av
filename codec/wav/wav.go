/*
NAME
  wav.go

DESCRIPTION
  wav.go contains functions for processing wav.

AUTHOR
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package wav provides functions for converting wav audio.
package wav

import (
	"encoding/binary"
	"fmt"
)

// ConvertFormat converts the common name for a format in a string type to the specific
// integer required by the wav encoder.
var ConvertFormat = map[string]int{"pcm": PCMFormat}

const PCMFormat = 1 // PCMFormat defines the value for pcm audio as defined by the wav std.

var (
	errInvalidFormat   = fmt.Errorf("invalid or no format defined")
	errInvalidRate     = fmt.Errorf("invalid or no sample rate defined")
	errInvalidChannels = fmt.Errorf("invalid or no number of channels defined")
	errInvalidBitDepth = fmt.Errorf("invalid or no bit depth defined")
)

// Metadata defines the format of the audio file for reading.
type Metadata struct {
	AudioFormat int
	Channels    int
	SampleRate  int
	BitDepth    int
}

type WAV struct {
	Metadata Metadata
	Audio    []byte
}

// Write writes the given audio byte slice to the WAV, encoding the appropriate headings.
func (w *WAV) Write(p []byte) (n int, err error) {
	// Create header slice.
	header := make([]byte, 44)

	// Write RIFF type.
	copy(header[0:4], []byte("RIFF"))

	// Write the size of overall file.
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(p)+44))
	copy(header[4:8], buf)

	// Write WAVE type.
	copy(header[8:12], []byte("WAVE"))

	// Write fmt chunk marker.
	copy(header[12:16], []byte("fmt "))

	// Write the subchunk1 Size.
	binary.LittleEndian.PutUint32(buf, 16)
	copy(header[16:20], buf)

	// Write the encoded audio format.
	if w.Metadata.AudioFormat != PCMFormat { // TODO: allow for more encoding formats.
		return 0, errInvalidFormat
	}
	binary.LittleEndian.PutUint16(buf[0:2], 1)
	copy(header[20:22], buf[0:2])

	// Write the number of channels.
	if w.Metadata.Channels == 0 {
		return 0, errInvalidChannels
	}
	binary.LittleEndian.PutUint16(buf[0:2], uint16(w.Metadata.Channels))
	copy(header[22:24], buf[0:2])

	// Write the sample rate.
	if w.Metadata.SampleRate == 0 {
		return 0, errInvalidRate
	}
	binary.LittleEndian.PutUint32(buf[0:4], uint32(w.Metadata.SampleRate))
	copy(header[24:28], buf[0:4])

	// Write bit depth values.
	if w.Metadata.BitDepth == 0 {
		return 0, errInvalidBitDepth
	}
	var val uint32 = uint32((w.Metadata.SampleRate * w.Metadata.BitDepth * w.Metadata.Channels) / 8)
	binary.LittleEndian.PutUint32(buf[0:4], val)
	copy(header[28:32], buf[0:4])

	val = uint32((w.Metadata.BitDepth * w.Metadata.Channels) / 8)
	binary.LittleEndian.PutUint32(buf[0:4], val)
	copy(header[32:34], buf[0:4])

	binary.LittleEndian.PutUint32(buf[0:4], uint32(w.Metadata.BitDepth))
	copy(header[34:36], buf[0:4])

	// Mark start of data.
	copy(header[36:40], []byte("data"))

	// Write size of data chunk.
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(p)))
	copy(header[40:44], buf[0:4])

	// Append audio data.
	w.Audio = header
	w.Audio = append(w.Audio, p...)

	// Return successful write.
	return len(p) + 44, nil

}

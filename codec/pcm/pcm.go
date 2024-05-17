/*
NAME
  pcm.go

DESCRIPTION
  pcm.go contains functions for processing pcm.

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package pcm provides functions for processing and converting pcm audio.
package pcm

import (
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"
)

// SampleFormat is the format that a PCM Buffer's samples can be in.
type SampleFormat int

// Used to represent an unknown format.
const (
	Unknown SampleFormat = -1
)

// Sample formats that we use.
const (
	S16_LE SampleFormat = iota
	S32_LE
	// There are many more:
	// https://linux.die.net/man/1/arecord
	// https://trac.ffmpeg.org/wiki/audio%20types
)

// BufferFormat contains the format for a PCM Buffer.
type BufferFormat struct {
	SFormat  SampleFormat
	Rate     uint
	Channels uint
}

// Buffer contains a buffer of PCM data and the format that it is in.
type Buffer struct {
	Format BufferFormat
	Data   []byte
}

// DataSize takes audio attributes describing PCM audio data and returns the size of that data.
func DataSize(rate, channels, bitDepth uint, period float64) int {
	s := int(float64(channels) * float64(rate) * float64(bitDepth/8) * period)
	return s
}

// Resample takes Buffer c and resamples the pcm audio data to 'rate' Hz and returns a Buffer with the resampled data.
// Notes:
// 	- Currently only downsampling is implemented and c's rate must be divisible by 'rate' or an error will occur.
// 	- If the number of bytes in c.Data is not divisible by the decimation factor (ratioFrom), the remaining bytes will
// 	  not be included in the result. Eg. input of length 480002 downsampling 6:1 will result in output length 80000.
func Resample(c Buffer, rate uint) (Buffer, error) {
	if c.Format.Rate == rate {
		return c, nil
	}
	if c.Format.Rate < 0 {
		return Buffer{}, fmt.Errorf("Unable to convert from: %v Hz", c.Format.Rate)
	}
	if rate < 0 {
		return Buffer{}, fmt.Errorf("Unable to convert to: %v Hz", rate)
	}

	// The number of bytes in a sample.
	var sampleLen int
	switch c.Format.SFormat {
	case S32_LE:
		sampleLen = int(4 * c.Format.Channels)
	case S16_LE:
		sampleLen = int(2 * c.Format.Channels)
	default:
		return Buffer{}, fmt.Errorf("Unhandled ALSA format: %v", c.Format.SFormat)
	}
	inPcmLen := len(c.Data)

	// Calculate sample rate ratio ratioFrom:ratioTo.
	rateGcd := gcd(rate, c.Format.Rate)
	ratioFrom := int(c.Format.Rate / rateGcd)
	ratioTo := int(rate / rateGcd)

	// ratioTo = 1 is the only number that will result in an even sampling.
	if ratioTo != 1 {
		return Buffer{}, fmt.Errorf("unhandled from:to rate ratio %v:%v: 'to' must be 1", ratioFrom, ratioTo)
	}

	newLen := inPcmLen / ratioFrom
	resampled := make([]byte, 0, newLen)

	// For each new sample to be generated, loop through the respective 'ratioFrom' samples in 'c.Data' to add them
	// up and average them. The result is the new sample.
	bAvg := make([]byte, sampleLen)
	for i := 0; i < newLen/sampleLen; i++ {
		var sum int
		for j := 0; j < ratioFrom; j++ {
			switch c.Format.SFormat {
			case S32_LE:
				sum += int(int32(binary.LittleEndian.Uint32(c.Data[(i*ratioFrom*sampleLen)+(j*sampleLen) : (i*ratioFrom*sampleLen)+((j+1)*sampleLen)])))
			case S16_LE:
				sum += int(int16(binary.LittleEndian.Uint16(c.Data[(i*ratioFrom*sampleLen)+(j*sampleLen) : (i*ratioFrom*sampleLen)+((j+1)*sampleLen)])))
			}
		}
		avg := sum / ratioFrom
		switch c.Format.SFormat {
		case S32_LE:
			binary.LittleEndian.PutUint32(bAvg, uint32(avg))
		case S16_LE:
			binary.LittleEndian.PutUint16(bAvg, uint16(avg))
		}
		resampled = append(resampled, bAvg...)
	}

	// Return a new Buffer with resampled data.
	return Buffer{
		Format: BufferFormat{
			Channels: c.Format.Channels,
			SFormat:  c.Format.SFormat,
			Rate:     rate,
		},
		Data: resampled,
	}, nil
}

// StereoToMono returns raw mono audio data generated from only the left channel from
// the given stereo Buffer
func StereoToMono(c Buffer) (Buffer, error) {
	if c.Format.Channels == 1 {
		return c, nil
	}
	if c.Format.Channels != 2 {
		return Buffer{}, fmt.Errorf("Audio is not stereo or mono, it has %v channels", c.Format.Channels)
	}

	var stereoSampleBytes int
	switch c.Format.SFormat {
	case S32_LE:
		stereoSampleBytes = 8
	case S16_LE:
		stereoSampleBytes = 4
	default:
		return Buffer{}, fmt.Errorf("Unhandled sample format %v", c.Format.SFormat)
	}

	recLength := len(c.Data)
	mono := make([]byte, recLength/2)

	// Convert to mono: for each byte in the stereo recording, if it's in the first half of a stereo sample
	// (left channel), add it to the new mono audio data.
	var inc int
	for i := 0; i < recLength; i++ {
		if i%stereoSampleBytes < stereoSampleBytes/2 {
			mono[inc] = c.Data[i]
			inc++
		}
	}

	// Return a new Buffer with resampled data.
	return Buffer{
		Format: BufferFormat{
			Channels: 1,
			SFormat:  c.Format.SFormat,
			Rate:     c.Format.Rate,
		},
		Data: mono,
	}, nil
}

// gcd is used for calculating the greatest common divisor of two positive integers, a and b.
// assumes given a and b are positive.
func gcd(a, b uint) uint {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// String returns the string representation of a SampleFormat.
func (f SampleFormat) String() string {
	switch f {
	case S16_LE:
		return "S16_LE"
	case S32_LE:
		return "S32_LE"
	default:
		return "Unknown"
	}
}

// SFFromString takes a string representing a sample format and returns the corresponding SampleFormat.
func SFFromString(s string) (SampleFormat, error) {
	switch s {
	case "S16_LE":
		return S16_LE, nil
	case "S32_LE":
		return S32_LE, nil
	default:
		return Unknown, errors.Errorf("unknown sample format (%s)", s)
	}
}

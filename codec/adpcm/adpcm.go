/*
NAME
  adpcm.go

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package adpcm provides functions to transcode between PCM and ADPCM.
package adpcm

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

const (
	byteDepth     = 2 // We are working with 16-bit samples. TODO(Trek): make configurable.
	initSamps     = 2 // Number of samples used to initialise the encoder.
	initSize      = initSamps * byteDepth
	headSize      = 8 // Number of bytes in the header of ADPCM.
	samplesPerEnc = 2 // Number of sample encoded at a time eg. 2 16-bit samples get encoded into 1 byte.
	bytesPerEnc   = samplesPerEnc * byteDepth
	chunkLenSize  = 4 // Size of the chunk length in bytes, chunk length is a 32 bit number.
	compFact      = 4 // In general ADPCM compresses by a factor of 4.
)

// Table of index changes (see spec).
var indexTable = []int16{
	-1, -1, -1, -1, 2, 4, 6, 8,
	-1, -1, -1, -1, 2, 4, 6, 8,
}

// Quantize step size table (see spec).
var stepTable = []int16{
	7, 8, 9, 10, 11, 12, 13, 14,
	16, 17, 19, 21, 23, 25, 28, 31,
	34, 37, 41, 45, 50, 55, 60, 66,
	73, 80, 88, 97, 107, 118, 130, 143,
	157, 173, 190, 209, 230, 253, 279, 307,
	337, 371, 408, 449, 494, 544, 598, 658,
	724, 796, 876, 963, 1060, 1166, 1282, 1411,
	1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024,
	3327, 3660, 4026, 4428, 4871, 5358, 5894, 6484,
	7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899,
	15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794,
	32767,
}

// Encoder is used to encode to ADPCM from PCM data.
type Encoder struct {
	// dst is the destination for ADPCM-encoded data.
	dst io.Writer

	est int16 // Estimation of sample based on quantised ADPCM nibble.
	idx int16 // Index to step used for estimation.
}

// Decoder is used to decode from ADPCM to PCM data.
type Decoder struct {
	// dst is the destination for PCM-encoded data.
	dst io.Writer

	est  int16 // Estimation of sample based on quantised ADPCM nibble.
	idx  int16 // Index to step used for estimation.
	step int16
}

// NewEncoder retuns a new ADPCM Encoder.
func NewEncoder(dst io.Writer) *Encoder {
	return &Encoder{dst: dst}
}

// encodeSample takes a single 16 bit PCM sample and
// returns a byte of which the last 4 bits are an encoded ADPCM nibble.
func (e *Encoder) encodeSample(sample int16) byte {
	// Find difference between the sample and the previous estimation.
	delta := capAdd16(sample, -e.est)

	// Create and set sign bit for nibble and find absolute value of difference.
	var nib byte
	if delta < 0 {
		nib = 8
		delta = -delta
	}

	step := stepTable[e.idx]
	diff := step >> 3
	var mask byte = 4

	for i := 0; i < 3; i++ {
		if delta > step {
			nib |= mask
			delta = capAdd16(delta, -step)
			diff = capAdd16(diff, step)
		}
		mask >>= 1
		step >>= 1
	}

	if nib&8 != 0 {
		diff = -diff
	}

	// Adjust estimated sample based on calculated difference.
	e.est = capAdd16(e.est, diff)

	e.idx += indexTable[nib&7]

	// Check for underflow and overflow.
	if e.idx < 0 {
		e.idx = 0
	} else if e.idx > int16(len(stepTable)-1) {
		e.idx = int16(len(stepTable) - 1)
	}

	return nib
}

// calcHead sets the state for the Encoder by running the first sample through
// the Encoder, and writing the first sample to the Encoder's io.Writer (dst).
// It returns the number of bytes written to the Encoder's destination and the first error encountered.
func (e *Encoder) calcHead(sample []byte, pad bool) (int, error) {
	// Check that we are given 1 sample.
	if len(sample) != byteDepth {
		return 0, fmt.Errorf("length of given byte array is: %v, expected: %v", len(sample), byteDepth)
	}

	n, err := e.dst.Write(sample)
	if err != nil {
		return n, err
	}

	_n, err := e.dst.Write([]byte{byte(int16(e.idx))})
	if err != nil {
		return n, err
	}
	n += _n

	if pad {
		_n, err = e.dst.Write([]byte{0x01})
	} else {
		_n, err = e.dst.Write([]byte{0x00})
	}
	n += _n
	if err != nil {
		return n, err
	}
	return n, nil
}

// init initializes the Encoder's estimation to the first uncompressed sample and the index to
// point to a suitable quantizer step size.
// The suitable step size is the closest step size in the stepTable to half the absolute difference of the first two samples.
func (e *Encoder) init(samples []byte) {
	int1 := int16(binary.LittleEndian.Uint16(samples[:byteDepth]))
	int2 := int16(binary.LittleEndian.Uint16(samples[byteDepth:initSize]))
	e.est = int1

	halfDiff := math.Abs(math.Abs(float64(int1)) - math.Abs(float64(int2))/2)
	closest := math.Abs(float64(stepTable[0]) - halfDiff)
	var cInd int16
	for i, step := range stepTable {
		if math.Abs(float64(step)-halfDiff) < closest {
			closest = math.Abs(float64(step) - halfDiff)
			cInd = int16(i)
		}
	}
	e.idx = cInd
}

// Write takes a slice of bytes of arbitrary length representing pcm and encodes it into adpcm.
// It writes its output to the Encoder's dst.
// The number of bytes written out is returned along with any error that occured.
func (e *Encoder) Write(b []byte) (int, error) {
	// Check that pcm has enough data to initialize Decoder.
	pcmLen := len(b)
	if pcmLen < initSize {
		return 0, fmt.Errorf("length of given byte array must be >= %v", initSize)
	}

	// Determine if there will be a byte that won't contain two full nibbles and will need padding.
	pad := false
	if (pcmLen-byteDepth)%bytesPerEnc != 0 {
		pad = true
	}

	// Write the first 4 bytes of the adpcm chunk, which represent its length, ie. the number of bytes following the chunk length.
	chunkLen := EncBytes(pcmLen)
	chunkLenBytes := make([]byte, chunkLenSize)
	binary.LittleEndian.PutUint32(chunkLenBytes, uint32(chunkLen))
	n, err := e.dst.Write(chunkLenBytes)
	if err != nil {
		return n, err
	}

	e.init(b[:initSize])
	_n, err := e.calcHead(b[:byteDepth], pad)
	n += _n
	if err != nil {
		return n, err
	}
	// Skip the first sample and start at the end of the first two samples, then every two samples encode them into a byte of adpcm.
	for i := byteDepth; i+bytesPerEnc-1 < pcmLen; i += bytesPerEnc {
		nib1 := e.encodeSample(int16(binary.LittleEndian.Uint16(b[i : i+byteDepth])))
		nib2 := e.encodeSample(int16(binary.LittleEndian.Uint16(b[i+byteDepth : i+bytesPerEnc])))
		_n, err := e.dst.Write([]byte{byte((nib2 << 4) | nib1)})
		n += _n
		if err != nil {
			return n, err
		}
	}
	// If we've reached the end of the pcm data and there's a sample left over,
	// compress it to a nibble and leave the first half of the byte padded with 0s.
	if pad {
		nib := e.encodeSample(int16(binary.LittleEndian.Uint16(b[pcmLen-byteDepth : pcmLen])))
		_n, err := e.dst.Write([]byte{nib})
		n += _n
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// NewDecoder retuns a new ADPCM Decoder.
func NewDecoder(dst io.Writer) *Decoder {
	return &Decoder{dst: dst}
}

// decodeSample takes a byte, the last 4 bits of which contain a single
// 4 bit ADPCM nibble, and returns a 16 bit decoded PCM sample.
func (d *Decoder) decodeSample(nibble byte) int16 {
	// Calculate difference.
	var diff int16
	if nibble&4 != 0 {
		diff = capAdd16(diff, d.step)
	}
	if nibble&2 != 0 {
		diff = capAdd16(diff, d.step>>1)
	}
	if nibble&1 != 0 {
		diff = capAdd16(diff, d.step>>2)
	}
	diff = capAdd16(diff, d.step>>3)

	// Account for sign bit.
	if nibble&8 != 0 {
		diff = -diff
	}

	// Adjust estimated sample based on calculated difference.
	d.est = capAdd16(d.est, diff)

	// Adjust index into step size lookup table using nibble.
	d.idx += indexTable[nibble]

	// Check for overflow and underflow.
	if d.idx < 0 {
		d.idx = 0
	} else if d.idx > int16(len(stepTable)-1) {
		d.idx = int16(len(stepTable) - 1)
	}

	// Find new quantizer step size.
	d.step = stepTable[d.idx]

	return d.est
}

// Write takes a slice of bytes of arbitrary length representing adpcm and decodes it into pcm.
// It writes its output to the Decoder's dst.
// The number of bytes written out is returned along with any error that occured.
func (d *Decoder) Write(b []byte) (int, error) {
	// Iterate over each chunk and decode it.
	var n int
	var chunkLen int
	for off := 0; off+headSize <= len(b); off += chunkLen {
		// Read length of chunk and check if whole chunk exists.
		chunkLen = int(binary.LittleEndian.Uint32(b[off : off+chunkLenSize]))
		if off+chunkLen > len(b) {
			break
		}

		// Initialize Decoder with header of b.
		d.est = int16(binary.LittleEndian.Uint16(b[off+chunkLenSize : off+chunkLenSize+byteDepth]))
		d.idx = int16(b[off+chunkLenSize+byteDepth])
		d.step = stepTable[d.idx]
		_n, err := d.dst.Write(b[off+chunkLenSize : off+chunkLenSize+byteDepth])
		n += _n
		if err != nil {
			return n, err
		}

		// For each byte, seperate it into two nibbles (each nibble is a compressed sample),
		// then decode each nibble and output the resulting 16-bit samples.
		// If padding flag is true only decode up until the last byte, then decode that separately.
		for i := off + headSize; i < off+chunkLen-int(b[off+chunkLenSize+3]); i++ {
			twoNibs := b[i]
			nib2 := byte(twoNibs >> 4)
			nib1 := byte((nib2 << 4) ^ twoNibs)

			firstBytes := make([]byte, byteDepth)
			binary.LittleEndian.PutUint16(firstBytes, uint16(d.decodeSample(nib1)))
			_n, err := d.dst.Write(firstBytes)
			n += _n
			if err != nil {
				return n, err
			}

			secondBytes := make([]byte, byteDepth)
			binary.LittleEndian.PutUint16(secondBytes, uint16(d.decodeSample(nib2)))
			_n, err = d.dst.Write(secondBytes)
			n += _n
			if err != nil {
				return n, err
			}
		}
		if b[off+chunkLenSize+3] == 0x01 {
			padNib := b[off+chunkLen-1]
			samp := make([]byte, byteDepth)
			binary.LittleEndian.PutUint16(samp, uint16(d.decodeSample(padNib)))
			_n, err := d.dst.Write(samp)
			n += _n
			if err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

// capAdd16 adds two int16s together and caps at max/min int16 instead of overflowing
func capAdd16(a, b int16) int16 {
	c := int32(a) + int32(b)
	switch {
	case c < math.MinInt16:
		return math.MinInt16
	case c > math.MaxInt16:
		return math.MaxInt16
	default:
		return int16(c)
	}
}

// EncBytes will return the number of adpcm bytes that will be generated when encoding the given amount of pcm bytes (n).
func EncBytes(n int) int {
	// For 'n' pcm bytes, 1 sample is left uncompressed, the rest is compressed by a factor of 4
	// and a chunk length (4B), start index (1B) and padding-flag (1B) are added.
	// Also if there are an even number of samples, there will be half a byte of padding added to the last byte.
	if n%bytesPerEnc == 0 {
		return (n-byteDepth)/compFact + headSize + 1
	}
	return (n-byteDepth)/compFact + headSize
}

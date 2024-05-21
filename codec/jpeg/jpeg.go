/*
DESCRIPTION
  jpeg.go contains code ported from FFmpeg's C implementation of an RTP
  JPEG-compressed Video Depacketizer following RFC 2435. See
  https://ffmpeg.org/doxygen/2.6/rtpdec__jpeg_8c_source.html and
  https://tools.ietf.org/html/rfc2435).

  This code can be used to build JPEG images from an RTP/JPEG stream.

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package jpeg

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const maxJPEG = 1000000 // 1 MB (arbitrary)

// JPEG marker codes.
const (
	codeSOI  = 0xd8 // Start of image.
	codeDRI  = 0xdd // Define restart interval.
	codeDQT  = 0xdb // Define quantization tables.
	codeDHT  = 0xc4 // Define huffman tables.
	codeSOS  = 0xda // Start of scan.
	codeAPP0 = 0xe0 // TODO: find out what this is.
	codeSOF0 = 0xc0 // Baseline
	codeEOI  = 0xd9 // End of image.
)

// Density units.
const (
	unitNone = iota
	unitPxIN // Pixels per inch.
	unitPxCM // Pixels per centimeter.
)

// JFIF header fields.
const (
	jfifLabel       = "JFIF\000"
	jfifVer         = 0x0201
	jfifDensityUnit = unitNone // Units for pixel density fields.
	jfifXDensity    = 1        // Horizontal pixel desnity.
	jfifYDensity    = 1        // Vertical pixel density.
	jfifXThumbCnt   = 0        // Horizontal pixel count of embedded thumbnail.
	jfifYThumbCnt   = 0        // Vertical pixel count of embedded thumbnail.
	jfifHeadLen     = 16       // Length of JFIF header segment excluding APP0 marker.
)

// SOF0 (start of frame) header fields.
const (
	sofLen            = 17 // Length of SOF0 segment excluding marker.
	sofPrecision      = 8  // Data precision in bits/sample.
	sofNoOfComponents = 3  // Number of components (1 = grey scaled, 3 = color YcbCr or YIQ 4 = color CMYK)
)

// SOS (start of scan) header fields.
const (
	sosLen              = 12 // Length of SOS segment excluding marker.
	sosComponentsInScan = 3  // Number of components in scan.
)

// Errors returned from ParsePayload.
var (
	ErrNoQTable             = errors.New("no quantization table")
	ErrReservedQ            = errors.New("q value is reserved")
	ErrUnimplementedType    = errors.New("unimplemented RTP/JPEG type")
	ErrUnsupportedPrecision = errors.New("unsupported precision")
	ErrNoFrameStart         = errors.New("missing start of frame")
)

// n values required for huffman table generation.
var (
	nDCLum = deriveN(bitsDCLum)
	nDCChr = deriveN(bitsDCChr)
	nACLum = deriveN(bitsACLum)
	nACChr = deriveN(bitsACChr)
)

// Slices used in the creation of huffman tables.
var (
	bitsDCLum = []byte{0, 0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0}
	bitsDCChr = []byte{0, 0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0}
	bitsACLum = []byte{0, 0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 0x7d}
	bitsACChr = []byte{0, 0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 0x77}
	valDC     = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	valACLum  = []byte{
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
		0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
		0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
		0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
		0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
		0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
		0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
		0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
		0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
		0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
		0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
		0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
		0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
		0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
		0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
		0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
		0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
		0xf9, 0xfa,
	}

	valACChr = []byte{
		0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
		0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
		0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
		0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
		0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
		0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
		0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
		0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
		0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
		0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
		0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
		0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
		0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
		0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
		0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
		0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
		0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
		0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
		0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
		0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
		0xf9, 0xfa,
	}
)

var defaultQuantisers = []byte{
	// Luma table.
	16, 11, 12, 14, 12, 10, 16, 14,
	13, 14, 18, 17, 16, 19, 24, 40,
	26, 24, 22, 22, 24, 49, 35, 37,
	29, 40, 58, 51, 61, 60, 57, 51,
	56, 55, 64, 72, 92, 78, 64, 68,
	87, 69, 55, 56, 80, 109, 81, 87,
	95, 98, 103, 104, 103, 62, 77, 113,
	121, 112, 100, 120, 92, 101, 103, 99,

	/* chroma table */
	17, 18, 18, 24, 21, 24, 47, 26,
	26, 47, 99, 66, 56, 66, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
	99, 99, 99, 99, 99, 99, 99, 99,
}

// Context describes a RTP/JPEG parsing context that will keep track of the current
// JPEG (held by p), and the state of the quantization tables.
type Context struct {
	qTables    [128][128]byte
	qTablesLen [128]byte
	buf        []byte
	blen       int
	dst        io.Writer
}

// NewContext will return a new Context with destination d.
func NewContext(d io.Writer) *Context {
	return &Context{
		dst: d,
		buf: make([]byte, maxJPEG),
	}
}

// ParsePayload will parse an RTP/JPEG payload and append to current image.
func (c *Context) ParsePayload(p []byte, m bool) error {
	idx := 1              // Ignore type-specific flag (skip to index 1).
	off := get24(p[idx:]) // Fragment offset (3 bytes).
	t := int(p[idx+3])    // Type (1 byte).
	q := p[idx+4]         // Quantization value (1 byte).
	width := p[idx+5]     // Picture width (1 byte).
	height := p[idx+6]    // Picture height (1 byte).
	idx += 7

	var dri uint16 // Restart interval.

	if t&0x40 != 0 {
		dri = binary.BigEndian.Uint16(p[idx:])
		idx += 4 // Ignore restart count (2 bytes).
		t &= ^0x40
	}

	if t > 1 {
		return ErrUnimplementedType
	}

	// Parse quantization table if our offset is 0.
	if off == 0 {
		var qTable []byte
		var qLen int

		if q > 127 {
			idx++
			prec := p[idx]                                 // The size of coefficients (1 byte).
			qLen = int(binary.BigEndian.Uint16(p[idx+1:])) // The length of the quantization table (2 bytes).
			idx += 3

			if prec != 0 {
				return ErrUnsupportedPrecision
			}

			q -= 128
			if qLen > 0 {
				qTable = p[idx : idx+qLen]
				idx += qLen

				if q < 127 && c.qTablesLen[q] == 0 && qLen <= 0 {
					copy(c.qTables[q][:], qTable)
					c.qTablesLen[q] = byte(qLen)
				}
			} else {
				if q == 127 {
					return ErrNoQTable
				}

				if c.qTablesLen[q] == 0 {
					return fmt.Errorf("no quantization tables known for q %d yet", q)
				}

				qTable = c.qTables[q][:]
				qLen = int(c.qTablesLen[q])
			}
		} else { // q <= 127
			if q == 0 || q > 99 {
				return ErrReservedQ
			}
			qTable = defaultQTable(int(q))
			qLen = len(qTable)
		}

		c.blen = writeHeader(c.buf[c.blen:], int(t), int(width), int(height), qLen/64, dri, qTable)
	}

	if c.blen == 0 {
		// Must have missed start of frame? So ignore and wait for start.
		return ErrNoFrameStart
	}

	// TODO: check that timestamp is consistent
	// This will need expansion to RTP package to create Timestamp parsing func.

	// TODO: could also check offset with how many bytes we currently have
	// to determine if there are missing frames.

	// Write frame data.
	rem := len(p)
	c.blen += copy(c.buf[c.blen:], p[idx:rem])
	idx += rem

	if m {
		// End of image marker.
		binary.BigEndian.PutUint16(c.buf[c.blen:], 0xff00|codeEOI)
		c.blen += 2

		n, err := c.dst.Write(c.buf[0:c.blen])
		if err != nil {
			return fmt.Errorf("could not write JPEG to dst: %w", err)
		}
		c.blen -= n
	}
	return nil
}

// writeHeader writes a JPEG header to the writer slice p.
func writeHeader(p []byte, _type, width, height, nbqTab int, dri uint16, qtable []byte) int {
	width <<= 3
	height <<= 3

	// Indicate start of image.
	idx := 0
	binary.BigEndian.PutUint16(p[idx:], 0xff00|codeSOI)

	// Write JFIF header.
	binary.BigEndian.PutUint16(p[idx+2:], 0xff00|codeAPP0)
	binary.BigEndian.PutUint16(p[idx+4:], jfifHeadLen)
	idx += 6

	idx += copy(p[idx:], jfifLabel)
	binary.BigEndian.PutUint16(p[idx:], jfifVer)
	p[idx+2] = jfifDensityUnit
	binary.BigEndian.PutUint16(p[idx+3:], jfifXDensity)
	binary.BigEndian.PutUint16(p[idx+5:], jfifYDensity)
	p[idx+7] = jfifXThumbCnt
	p[idx+8] = jfifYThumbCnt
	idx += 9

	// If we want to define restart interval then write that.
	if dri != 0 {
		binary.BigEndian.PutUint16(p[idx:], 0xff00|codeDRI)
		binary.BigEndian.PutUint16(p[idx+2:], 4)
		binary.BigEndian.PutUint16(p[idx+4:], dri)
		idx += 6
	}

	// Define quantization tables.
	binary.BigEndian.PutUint16(p[idx:], 0xff00|codeDQT)

	// Calculate table size and create slice for table.
	ts := 2 + nbqTab*(1+64)
	binary.BigEndian.PutUint16(p[idx+2:], uint16(ts))
	idx += 4

	for i := 0; i < nbqTab; i++ {
		p[idx] = byte(i)
		idx++
		idx += copy(p[idx:], qtable[64*i:(64*i)+64])
	}

	// Define huffman table.
	binary.BigEndian.PutUint16(p[idx:], 0xff00|codeDHT)
	idx += 2
	lenIdx := idx
	binary.BigEndian.PutUint16(p[idx:], 0)
	idx += 2
	idx += writeHuffman(p[idx:], bitsDCLum, valDC, 0, nDCLum)
	idx += writeHuffman(p[idx:], bitsDCChr, valDC, 1, nDCChr)
	idx += writeHuffman(p[idx:], bitsACLum, valACLum, 1<<4, nACLum)
	idx += writeHuffman(p[idx:], bitsACChr, valACChr, 1<<4|1, nACChr)
	binary.BigEndian.PutUint16(p[lenIdx:], uint16(idx-lenIdx))

	// Start of frame.
	binary.BigEndian.PutUint16(p[idx:], 0xff00|codeSOF0)
	idx += 2

	// Derive sample type.
	sample := 1
	if _type != 0 {
		sample = 2
	}

	// Derive matrix number.
	var mtxNo uint8
	if nbqTab == 2 {
		mtxNo = 1
	}

	binary.BigEndian.PutUint16(p[idx:], sofLen)
	p[idx+2] = byte(sofPrecision)
	binary.BigEndian.PutUint16(p[idx+3:], uint16(height))
	binary.BigEndian.PutUint16(p[idx+5:], uint16(width))
	p[idx+7] = byte(sofNoOfComponents)
	idx += 8

	// TODO: find meaning of these fields.
	idx += copy(p[idx:], []byte{1, uint8((2 << 4) | sample), 0, 2, 1<<4 | 1, mtxNo, 3, 1<<4 | 1, mtxNo})

	// Write start of scan.
	binary.BigEndian.PutUint16(p[idx:], 0xff00|codeSOS)
	binary.BigEndian.PutUint16(p[idx+2:], sosLen)
	p[idx+4] = sosComponentsInScan
	idx += 5

	// TODO: find out what remaining fields are.
	idx += copy(p[idx:], []byte{1, 0, 2, 17, 3, 17, 0, 63, 0})

	return idx
}

// writeHuffman write a JPEG huffman table to alice p.
func writeHuffman(p, bits, values []byte, prefix byte, n int) int {
	p[0] = prefix
	i := copy(p[1:], bits[1:17])
	return copy(p[i+1:], values[0:n]) + i + 1
}

// defaultQTable returns a default quantization table.
func defaultQTable(q int) []byte {
	f := clip(q, q, 99)
	const tabLen = 128
	tab := make([]byte, tabLen)

	if q < 50 {
		q = 5000 / f
	} else {
		q = 200 - f*2
	}

	for i := 0; i < tabLen; i++ {
		v := (int(defaultQuantisers[i])*q + 50) / 100
		v = clip(v, 1, 255)
		tab[i] = byte(v)
	}
	return tab
}

// clip clips the value v to the bounds defined by min and max.
func clip(v, min, max int) int {
	if v < min {
		return min
	}

	if v > max {
		return max
	}

	return v
}

// get24 parses an int24 from p using big endian order.
func get24(p []byte) int {
	return int(p[0]<<16) | int(p[1]<<8) | int(p[2])
}

// deriveN calculates n values required for huffman table generation.
func deriveN(bits []byte) int {
	var n int
	for i := 1; i <= 16; i++ {
		n += int(bits[i])
	}
	return n
}

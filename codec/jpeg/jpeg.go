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

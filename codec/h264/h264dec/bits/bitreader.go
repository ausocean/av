/*
DESCRIPTION
  bitreader.go provides a bit reader implementation that can read or peek from
  an io.Reader data source.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>, The Australian Ocean Laboratory (AusOcean)

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package bits provides a bit reader implementation that can read or peek from
// an io.Reader data source.
package bits

import (
	"bufio"
	"io"
)

type bytePeeker interface {
	io.ByteReader
	Peek(int) ([]byte, error)
}

// BitReader is a bit reader that provides methods for reading bits from an
// io.Reader source.
type BitReader struct {
	r     bytePeeker
	n     uint64
	bits  int
	nRead int
}

// NewBitReader returns a new BitReader.
func NewBitReader(r io.Reader) *BitReader {
	byter, ok := r.(bytePeeker)
	if !ok {
		byter = bufio.NewReader(r)
	}
	return &BitReader{r: byter}
}

// ReadBits reads n bits from the source and returns them the least-significant
// part of a uint64.
// For example, with a source as []byte{0x8f,0xe3} (1000 1111, 1110 0011), we
// would get the following results for consequtive reads with n values:
// n = 4, res = 0x8 (1000)
// n = 2, res = 0x3 (0011)
// n = 4, res = 0xf (1111)
// n = 6, res = 0x23 (0010 0011)
func (br *BitReader) ReadBits(n int) (uint64, error) {
	for n > br.bits {
		b, err := br.r.ReadByte()
		if err == io.EOF {
			return 0, io.ErrUnexpectedEOF
		}
		if err != nil {
			return 0, err
		}
		br.nRead++
		br.n <<= 8
		br.n |= uint64(b)
		br.bits += 8
	}

	// br.n looks like this (assuming that br.bits = 14 and bits = 6):
	// Bit: 111111
	//      5432109876543210
	//
	//         (6 bits, the desired output)
	//        |-----|
	//        V     V
	//      0101101101001110
	//        ^            ^
	//        |------------|
	//           br.bits (num valid bits)
	//
	// This the next line right shifts the desired bits into the
	// least-significant places and masks off anything above.
	r := (br.n >> uint(br.bits-n)) & ((1 << uint(n)) - 1)
	br.bits -= n
	return r, nil
}

// PeekBits provides the next n bits returning them in the least-significant
// part of a uint64, without advancing through the source.
// For example, with a source as []byte{0x8f,0xe3} (1000 1111, 1110 0011), we
// would get the following results for consequtive peeks with n values:
// n = 4, res = 0x8 (1000)
// n = 8, res = 0x8f (1000 1111)
// n = 16, res = 0x8fe3 (1000 1111, 1110 0011)
func (br *BitReader) PeekBits(n int) (uint64, error) {
	byt, err := br.r.Peek(int((n-br.bits)+7) / 8)
	bits := br.bits
	if err != nil {
		if err == io.EOF {
			return 0, io.ErrUnexpectedEOF
		}
		return 0, err
	}
	for i := 0; n > bits; i++ {
		b := byt[i]
		if err != nil {
			return 0, err
		}
		br.n <<= 8
		br.n |= uint64(b)
		bits += 8
	}

	r := (br.n >> uint(bits-n)) & ((1 << uint(n)) - 1)
	return r, nil
}

// ByteAligned returns true if the reader position is at the start of a byte,
// and false otherwise.
func (br *BitReader) ByteAligned() bool {
	return br.bits == 0
}

// Off returns the current offset from the starting bit of the current byte.
func (br *BitReader) Off() int {
	return br.bits
}

// BytesRead returns the number of bytes that have been read by the BitReader.
func (br *BitReader) BytesRead() int {
	return br.nRead
}

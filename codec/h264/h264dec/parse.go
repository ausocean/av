/*
NAME
  parse.go

DESCRIPTION
  parse.go provides parsing processes for syntax elements of different
  descriptors specified in 7.2 of ITU-T H.264.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>, The Australian Ocean Laboratory (AusOcean)
  mrmod <mcmoranbjr@gmail.com>
*/

package h264dec

import (
	"math"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
	"github.com/pkg/errors"
)

// mbPartPredMode represents a macroblock partition prediction mode.
// Modes are defined as consts below. These modes are in section 7.4.5.
type mbPartPredMode int8

const (
	intra4x4 mbPartPredMode = iota
	intra8x8
	intra16x16
	predL0
	predL1
	direct
	biPred
	inter
	naMbPartPredMode
)

// fieldReader provides methods for reading bool and int fields from a
// bits.BitReader with a sticky error that may be checked after a series of
// parsing read calls.
type fieldReader struct {
	e  error
	br *bits.BitReader
}

// newFieldReader returns a new fieldReader.
func newFieldReader(br *bits.BitReader) fieldReader {
	return fieldReader{br: br}
}

// readBitsInt returns an int from reading n bits from br. If we have an error
// already, we do not continue with the read.
func (r fieldReader) readBits(n int) uint64 {
	if r.e != nil {
		return 0
	}
	var b uint64
	b, r.e = r.br.ReadBits(n)
	return b
}

// readUe parses a syntax element of ue(v) descriptor, i.e. an unsigned integer
// Exp-Golomb-coded element using method as specified in section 9.1 of ITU-T
// H.264 and return as an int. The read does not happen if the fieldReader
// has a non-nil error.
func (r fieldReader) readUe() uint64 {
	if r.e != nil {
		return 0
	}
	var i uint64
	i, r.e = readUe(r.br)
	return i
}

// readTe parses a syntax element of te(v) descriptor i.e, truncated
// Exp-Golomb-coded syntax element using method as specified in section 9.1
// and returns as an int. The read does not happen if the fieldReader
// has a non-nil error.
func (r fieldReader) readTe(x uint) int64 {
	if r.e != nil {
		return 0
	}
	var i int64
	i, r.e = readTe(r.br, x)
	return i
}

// readSe parses a syntax element with descriptor se(v), i.e. a signed integer
// Exp-Golomb-coded syntax element, using the method described in sections
// 9.1 and 9.1.1 and returns as int. The read does not happen if the fieldReader
// has a non-nil error.
func (r fieldReader) readSe() int {
	if r.e != nil {
		return 0
	}
	var i int
	i, r.e = readSe(r.br)
	return i
}

// readMe parses a syntax element of me(v) descriptor, i.e. mapped
// Exp-Golomb-coded element, using methods described in sections 9.1 and 9.1.2
// and returns as int. The read does not happen if the fieldReader has a
// non-nil error.
func (r fieldReader) readMe(chromaArrayType uint, mpm mbPartPredMode) int {
	if r.e != nil {
		return 0
	}
	var i uint
	i, r.e = readMe(r.br, chromaArrayType, mpm)
	return int(i)
}

// err returns the fieldReader's error e.
func (r fieldReader) err() error {
	return r.e
}

// readUe parses a syntax element of ue(v) descriptor, i.e. an unsigned integer
// Exp-Golomb-coded element using method as specified in section 9.1 of ITU-T H.264.
//
// TODO: this should return uint, but rest of code needs to be changed for this
// to happen.
func readUe(r *bits.BitReader) (uint64, error) {
	nZeros := -1
	var err error
	for b := uint64(0); b == 0; nZeros++ {
		b, err = r.ReadBits(1)
		if err != nil {
			return 0, err
		}
	}
	rem, err := r.ReadBits(int(nZeros))
	if err != nil {
		return 0, err
	}
	return uint64(math.Pow(float64(2), float64(nZeros)) - 1 + float64(rem)), nil
}

// readTe parses a syntax element of te(v) descriptor i.e, truncated
// Exp-Golomb-coded syntax element using method as specified in section 9.1
// Rec. ITU-T H.264 (04/2017).
//
// TODO: this should also return uint.
func readTe(r *bits.BitReader, x uint) (int64, error) {
	if x > 1 {
		ue, err := readUe(r)
		return int64(ue), err
	}

	if x == 1 {
		b, err := r.ReadBits(1)
		if err != nil {
			return 0, errors.Wrap(err, "could not read bit")
		}
		if b == 0 {
			return 1, nil
		}
		return 0, nil
	}

	return 0, errReadTeBadX
}

var errReadTeBadX = errors.New("x must be more than or equal to 1")

// readSe parses a syntax element with descriptor se(v), i.e. a signed integer
// Exp-Golomb-coded syntax element, using the method described in sections
// 9.1 and 9.1.1 in Rec. ITU-T H.264 (04/2017).
func readSe(r *bits.BitReader) (int, error) {
	codeNum, err := readUe(r)
	if err != nil {
		return 0, errors.Wrap(err, "error reading ue(v)")
	}

	return int(math.Pow(-1, float64(codeNum+1)) * math.Ceil(float64(codeNum)/2.0)), nil
}

// readMe parses a syntax element of me(v) descriptor, i.e. mapped
// Exp-Golomb-coded element, using methods described in sections 9.1 and 9.1.2
// in Rec. ITU-T H.264 (04/2017).
func readMe(r *bits.BitReader, chromaArrayType uint, mpm mbPartPredMode) (uint, error) {
	// Indexes to codedBlockPattern map.
	var i1, i2, i3 uint64

	// ChromaArrayType selects first index.
	switch chromaArrayType {
	case 1, 2:
		i1 = 0
	case 0, 3:
		i1 = 1
	default:
		return 0, errInvalidCAT
	}

	// CodeNum from readUe selects second index.
	i2, err := readUe(r)
	if err != nil {
		return 0, errors.Wrap(err, "error from readUe")
	}

	// Need to check that we won't go out of bounds with this index.
	if int(i2) >= len(codedBlockPattern[i1]) {
		return 0, errInvalidCodeNum
	}

	// Macroblock prediction mode selects third index.
	switch mpm {
	case intra4x4, intra8x8:
		i3 = 0
	case inter:
		i3 = 1
	default:
		return 0, errInvalidMPM
	}

	return codedBlockPattern[i1][i2][i3], nil
}

// Errors used by readMe.
var (
	errInvalidCodeNum = errors.New("invalid codeNum")
	errInvalidMPM     = errors.New("invalid macroblock prediction mode")
	errInvalidCAT     = errors.New("invalid chroma array type")
)

// codedBlockPattern contains data from table 9-4 in ITU-T H.264 (04/2017)
// for mapping a chromaArrayType, codeNum and macroblock prediction mode to a
// coded block pattern.
var codedBlockPattern = [][][2]uint{
	// Table 9-4 (a) for ChromaArrayType = 1 or 2
	{
		{47, 0}, {31, 16}, {15, 1}, {0, 2}, {23, 4}, {27, 8}, {29, 32}, {30, 3},
		{7, 5}, {11, 10}, {13, 12}, {14, 15}, {39, 47}, {43, 7}, {45, 11}, {46, 13},
		{16, 14}, {3, 6}, {31, 9}, {10, 31}, {12, 35}, {19, 37}, {21, 42}, {26, 44},
		{28, 33}, {35, 34}, {37, 36}, {42, 40}, {44, 39}, {1, 43}, {2, 45}, {4, 46},
		{8, 17}, {17, 18}, {18, 20}, {20, 24}, {24, 19}, {6, 21}, {9, 26}, {22, 28},
		{25, 23}, {32, 27}, {33, 29}, {34, 30}, {36, 22}, {40, 25}, {38, 38}, {41, 41},
	},
	// Table 9-4 (b) for ChromaArrayType = 0 or 3
	{
		{15, 0}, {0, 1}, {7, 2}, {11, 4}, {13, 8}, {14, 3}, {3, 5}, {5, 10}, {10, 12},
		{12, 15}, {1, 7}, {2, 11}, {4, 13}, {8, 14}, {6, 6}, {9, 9},
	},
}

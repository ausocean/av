/*
TODO: this file should really be in a 'h264enc' package.

DESCRIPTION
  cabacenc.go provides functionality for CABAC encoding.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h264dec

import (
	"errors"
	"fmt"
	"math"
)

// Error used by unaryBinString.
var errNegativeSyntaxVal = errors.New("cannot get unary binary string of negative value")

// unaryBinString returns the unary binary string of a syntax element having
// value v, as specified by setion 9.3.2.1 in the specifications.
func unaryBinString(v int) ([]int, error) {
	if v < 0 {
		return nil, errNegativeSyntaxVal
	}
	r := make([]int, v+1)
	for i := 0; i <= v; i++ {
		if i < v {
			r[i] = 1
		}
	}
	return r, nil
}

// Error used by truncUnaryBinString.
var errInvalidSyntaxVal = errors.New("syntax value cannot be greater than cMax")

// truncUnaryBinString returns the truncated unary binary string of a syntax
// element v given a cMax as specified in section 9.3.2.2 of the specifications.
func truncUnaryBinString(v, cMax int) ([]int, error) {
	if v < 0 {
		return nil, errNegativeSyntaxVal
	}

	if v > cMax {
		return nil, errInvalidSyntaxVal
	}

	if v == cMax {
		b, _ := unaryBinString(v)
		return b[:len(b)-1], nil
	}
	return unaryBinString(v)
}

// Error used by unaryExpGolombBinString.
var errInvalidUCoff = errors.New("uCoff cannot be less than or equal to zero")

// unaryExpGolombBinString returns the concatendated unary/k-th order
// Exp-Golomb (UEGk) binary string of a syntax element using the process defined
// in section 9.3.2.3 of the specifications.
func unaryExpGolombBinString(v, uCoff, k int, signedValFlag bool) ([]int, error) {
	if uCoff <= 0 {
		return nil, errInvalidUCoff
	}

	prefix, err := truncUnaryBinString(mini(uCoff, absi(v)), uCoff)
	if err != nil {
		return nil, err
	}

	return append(prefix, suffix(v, uCoff, k, signedValFlag)...), nil
}

// suffix returns the suffix part of a unary k-th Exp-Golomb binar string
// using the the algorithm as described by pseudo code 9-6 in section 9.3.2.3.
// TODO: could probably reduce allocations.
func suffix(v, uCoff, k int, signedValFlag bool) []int {
	var s []int

	if absi(v) >= uCoff {
		sufS := absi(v) - uCoff
		var stop bool

		for {
			if sufS >= (1 << uint(k)) {
				s = append(s, 1)
				sufS = sufS - (1 << uint(k))
				k++
			} else {
				s = append(s, 0)
				for k = k - 1; k >= 0; k-- {
					s = append(s, (sufS>>uint(k))&1)
				}
				stop = true
			}
			if stop {
				break
			}
		}
	}

	if signedValFlag && v != 0 {
		if v > 0 {
			s = append(s, 0)
		} else {
			s = append(s, 1)
		}
	}

	return s
}

// Error used by fixedLenBinString.
var errNegativeValue = errors.New("cannot get fixed length binary string of negative value")

// fixedLenBinString returns the fixed-length (FL) binary string of the syntax
// element v, given cMax to determine bin length, as specified by section 9.3.2.4
// of the specifications.
func fixedLenBinString(v, cMax int) ([]int, error) {
	if v < 0 {
		return nil, errNegativeValue
	}
	l := int(math.Ceil(math.Log2(float64(cMax + 1))))
	r := make([]int, l)
	for i := l - 1; i >= 0; i-- {
		r[i] = v % 2
		v = v / 2
	}
	return r, nil
}

// Errors used by mbTypeBinString.
var (
	errBadMbType      = errors.New("macroblock type outside of valid range")
	errBadMbSliceType = errors.New("bad slice type for macroblock")
)

// mbTypeBinString returns the macroblock type binary string for the given
// macroblock type value and slice type using the process defined in section
// 9.3.2.5 of the specifications.
func mbTypeBinString(v, slice int) ([]int, error) {
	switch slice {
	case sliceTypeI:
		if v < minIMbType || v > maxIMbType {
			return nil, errBadMbType
		}
		return binOfIMBTypes[v], nil

	case sliceTypeSI:
		if v < minSIMbType || v > maxSIMbType {
			return nil, errBadMbType
		}
		if v == sliceTypeSI {
			return []int{0}, nil
		}
		return append([]int{1}, binOfIMBTypes[v-1]...), nil

	case sliceTypeP, sliceTypeSP:
		if v < minPOrSPMbType || v > maxPOrSPMbType || v == P8x8ref0 {
			return nil, errBadMbType
		}
		if v < 5 {
			return binOfPOrSPMBTypes[v], nil
		}
		return append([]int{1}, binOfIMBTypes[v-5]...), nil

	case sliceTypeB:
		if v < minBMbType || v > maxBMbType {
			return nil, errBadMbType
		}
		if v < 23 {
			return binOfBMBTypes[v], nil
		}
		return append([]int{1, 1, 1, 1, 0, 1}, binOfIMBTypes[v-23]...), nil

	default:
		return nil, errBadMbSliceType
	}
}

// Error used by subMbTypeBinString.
var errBadSubMbSliceType = errors.New("bad slice type for sub-macroblock")

// subMbTypeBinString returns the binary string of a sub-macroblock type
// given the slice in which it is in using the process defined in section
// 9.3.2.5 of the specifications.
func subMbTypeBinString(v, slice int) ([]int, error) {
	switch slice {
	case sliceTypeP, sliceTypeSP:
		if v < minPOrSPSubMbType || v > maxPOrSPSubMbType {
			return nil, errBadMbType
		}
		return binOfPOrSPSubMBTypes[v], nil

	case sliceTypeB:
		if v < minBSubMbType || v > maxBSubMbType {
			return nil, errBadMbType
		}
		return binOfBSubMBTypes[v], nil

	default:
		return nil, errBadSubMbSliceType
	}
}

// codedBlockPatternBinString returns the binarization for the syntax element
// coded_block_pattern as defined by section 9.3.2.6 in specifications.
func codedBlockPatternBinString(luma, chroma, arrayType int) ([]int, error) {
	p, err := fixedLenBinString(luma, 15)
	if err != nil {
		return nil, fmt.Errorf("fixed length binarization failed with error: %w", err)
	}

	if arrayType == 0 || arrayType == 3 {
		return p, nil
	}

	s, err := truncUnaryBinString(chroma, 2)
	if err != nil {
		return nil, fmt.Errorf("truncated unary binarization failed with error: %w", err)
	}

	return append(p, s...), nil
}

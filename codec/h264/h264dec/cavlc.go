/*
DESCRIPTION
  cavlc.go provides utilities for context-adaptive variable-length coding
  for the parsing of H.264 syntax structure fields.

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
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
)

// TODO: find where these are defined in the specifications.
const (
	chromaDCLevel = iota
	intra16x16DCLevel
	intra16x16ACLevel
	cbIntra16x16DCLevel
	cbIntra16x16ACLevel
	crIntra16x16DCLevel
	crIntra16x16ACLevel
	lumaLevel4x4
	cbLevel4x4
	crLevel4x4
)

// Initialize the CAVLC coeff_token mapping table.
func init() {
	lines, err := csv.NewReader(strings.NewReader(coeffTokenTable)).ReadAll()
	if err != nil {
		panic(fmt.Sprintf("could not read lines from coeffTokenTable string, failed with error: %v", err))
	}

	coeffTokenMaps, err = formCoeffTokenMap(lines)
	if err != nil {
		panic(fmt.Sprintf("could not form coeff_token map, failed with err: %v", err))
	}
}

// tokenMap maps coeff_token to values of TrailingOnes(coeff_token) and
// TotalCoeff(coeff_token) given as tokenMap[ number of leading zeros in
// coeff_token][ coeff_token val ][ 0 for trailing ones and 1 for totalCoef ]
type tokenMap map[int]map[int][2]int

// The number of columns in the coeffTokenMap defined below. This is
// representative of the number of defined nC ranges in table 9-5.
const nColumns = 6

// coeffTokenMaps holds a representation of table 9-5 from the specifications, and
// is indexed as follows, coeffToken[ nC group ][ number of coeff_token leading
// zeros ][ value of coeff_token ][ 0 for TrailingOnes(coeff_token) and 1 for
// TotalCoef(coeff_token) ].
var coeffTokenMaps [nColumns]tokenMap

// formCoeffTokenMap populates the global [nColumns]tokenMap coeffTokenMaps
// representation of table 9-5 in the specifications using the coeffTokenTable
// const string defined in cavlctab.go.
func formCoeffTokenMap(lines [][]string) ([nColumns]tokenMap, error) {
	var maps [nColumns]tokenMap

	for i := range maps {
		maps[i] = make(tokenMap)
	}

	for _, line := range lines {
		trailingOnes, err := strconv.Atoi(line[0])
		if err != nil {
			return maps, fmt.Errorf("could not convert trailingOnes string to int, failed with error: %w", err)
		}

		totalCoeff, err := strconv.Atoi(line[1])
		if err != nil {
			return maps, fmt.Errorf("could not convert totalCoeff string to int, failed with error: %w", err)
		}

		// For each column in this row, therefore each nC category, load the
		// coeff_token leading zeros and value into the map.
		for j, v := range line[2:] {
			if v[0] == '-' {
				continue
			}

			// Count the leading zeros.
			var nZeros int
			for _, c := range v {
				if c == '1' {
					break
				}

				if c == '0' {
					nZeros++
				}
			}

			// This will be the value of the coeff_token (without leading zeros).
			val, err := binToInt(v[nZeros:])
			if err != nil {
				return maps, fmt.Errorf("could not get value of remaining binary, failed with error: %w", err)
			}

			// Add the TrailingOnes(coeff_token) and TotalCoeff(coeff_token) values
			// into the map for the coeff_token leading zeros and value.
			if maps[j][nZeros] == nil {
				maps[j][nZeros] = make(map[int][2]int)
			}
			maps[j][nZeros][val] = [2]int{trailingOnes, totalCoeff}
		}
	}
	return maps, nil
}

// TODO: put this somewhere more appropriate once context is understood.
type block struct {
	usingInterMbPredMode bool
	mbType               int
	totalCoef            int
}

// parseTotalCoeffAndTrailingOnes will use logic provided in section 9.2.1 of
// the specifications to obtain a value of nC, parse coeff_token from br and
// then use table 9-5 to find corresponding values of TrailingOnes(coeff_token)
// and TotalCoeff(coeff_token) which are then subsequently returned.
func parseTotalCoeffAndTrailingOnes(br *bits.BitReader, vid *VideoStream, ctx *SliceContext, usingMbPredMode bool, level, maxNumCoef, inBlockIdx int) (totalCoeff, trailingOnes, nC, outBlockIdx int, err error) {
	if level == chromaDCLevel {
		if ctx.chromaArrayType == 1 {
			nC = -1
		} else {
			nC = -2
		}
	} else {
		// Steps 1,2 and 3.
		if level == intra16x16DCLevel || level == cbIntra16x16DCLevel || level == crIntra16x16DCLevel {
			outBlockIdx = 0
		}

		// Step 4 derive blkA and blkB variables (blockA and blockB here).
		var mbAddr, blk [2]block
		switch level {
		case intra16x16DCLevel, intra16x16ACLevel, lumaLevel4x4:
			// TODO: clause 6.4.11.4
			panic("not implemented")
		case cbIntra16x16DCLevel, cbIntra16x16ACLevel, cbLevel4x4:
			// TODO: clause 6.4.11.6
			panic("not implemented")
		case crIntra16x16DCLevel, crIntra16x16ACLevel, crLevel4x4:
			// TODO: clause 6.4.11.6
			panic("not implemented")
		default:
			// TODO: clause 6.4.11.5
			panic("not implemented")
		}

		var availableFlag [2]bool
		var n [2]int
		for i := range availableFlag {
			// Step 5.
			if !(!available(mbAddr[i]) || usingMbPredMode || vid.ConstrainedIntraPred ||
				mbAddr[i].usingInterMbPredMode || ctx.nalType == 2 || ctx.nalType == 3 || ctx.nalType == 4) {
				availableFlag[i] = true
			}

			// Step 6.
			if availableFlag[i] {
				switch {
				case mbAddr[i].mbType == pSkip || mbAddr[i].mbType == bSkip || (mbAddr[i].mbType != iPCM && resTransformCoeffLevelsZero(blk[i])):
					n[i] = 0
				case mbAddr[i].mbType == iPCM:
					n[i] = 16
				default:
					// TODO: how is this set ?
					// "Otherwise, nN is set equal to the value TotalCoeff( coeff_token ) of
					// the neighbouring block blkN."
					// Do we need to run this same process for this block, or assume it's
					// already happened?
					n[i] = blk[i].totalCoef
				}
			}
		}

		// Step 7.
		switch {
		case availableFlag[0] && availableFlag[1]:
			nC = (n[0] + n[1] + 1) >> 1
		case availableFlag[0]:
			nC = n[0]
		case availableFlag[1]:
			nC = n[1]
		default:
			nC = 0
		}
	}

	trailingOnes, totalCoeff, _, err = readCoeffToken(br, nC)
	if err != nil {
		err = fmt.Errorf("could not get trailingOnes and totalCoeff vars, failed with error: %w", err)
		return
	}
	return
}

var (
	errInvalidNC = errors.New("invalid value of nC")
	errBadToken  = errors.New("could not find coeff_token value in map")
)

// readCoeffToken will read the coeff_token from br and find a match in the
// coeff_token mapping table (table 9-5 in the specifications) given also nC.
// The resultant TrailingOnes(coeff_token) and TotalCoeff(coeff_token) are
// returned as well as the value of coeff_token.
func readCoeffToken(br *bits.BitReader, nC int) (trailingOnes, totalCoeff, coeffToken int, err error) {
	// Get the number of leading zeros.
	var b uint64
	nZeros := -1
	for ; b == 0; nZeros++ {
		b, err = br.ReadBits(1)
		if err != nil {
			err = fmt.Errorf("could not read coeff_token leading zeros, failed with error: %w", err)
			return
		}
	}

	// Get the column idx for the map.
	var nCIdx int
	switch {
	case 0 <= nC && nC < 2:
		nCIdx = 0
	case 2 <= nC && nC < 4:
		nCIdx = 1
	case 4 <= nC && nC < 8:
		nCIdx = 2
	case 8 <= nC:
		nCIdx = 3
	case nC == -1:
		nCIdx = 4
	case nC == -2:
		nCIdx = 5
	default:
		err = errInvalidNC
		return
	}

	// Get the value of coeff_token.
	val := b
	nBits := nZeros
	for {
		vars, ok := coeffTokenMaps[nCIdx][nZeros][int(val)]
		if ok {
			trailingOnes = vars[0]
			totalCoeff = vars[1]
			coeffToken = int(val)
			return
		}

		const maxCoeffTokenBits = 16
		if !ok && nBits == maxCoeffTokenBits {
			err = errBadToken
			return
		}

		b, err = br.ReadBits(1)
		if err != nil {
			err = fmt.Errorf("could not read next bit of coeff_token, failed with error: %w", err)
			return
		}

		nBits++
		val <<= 1
		val |= b
	}
}

// TODO: implement this. From step 6 section 9.2.1. Will return true if "AC
// residual transform coefficient levels of the neighbouring block blkN are
// equal to 0 due to the corresponding bit of CodedBlockPatternLuma or
// CodedBlockPatternChroma being equal to 0"
func resTransformCoeffLevelsZero(b block) bool {
	panic("not implemented")
	return true
}

// TODO: implement this
func available(b block) bool {
	panic("not implemented")
	return true
}

// parseLevelPrefix parses the level_prefix variable as specified by the process
// outlined in section 9.2.2.1 in the specifications.
func parseLevelPrefix(br *bits.BitReader) (int, error) {
	zeros := -1
	for b := 0; b != 1; zeros++ {
		_b, err := br.ReadBits(1)
		if err != nil {
			return -1, fmt.Errorf("could not read bit, failed with error: %w", err)
		}
		b = int(_b)
	}
	return zeros, nil
}

// parseLevelInformation parses level information and returns the resultant
// levelVal list using the process defined by section 9.2.2 in the specifications.
func parseLevelInformation(br *bits.BitReader, totalCoeff, trailingOnes int) ([]int, error) {
	var levelVal []int
	var i int
	for ; i < trailingOnes; i++ {
		b, err := br.ReadBits(1)
		if err != nil {
			return nil, fmt.Errorf("could not read trailing_ones_sign_flag, failed with error: %w", err)
		}
		levelVal = append(levelVal, 1-int(b)*2)
	}

	var suffixLen int
	switch {
	case totalCoeff > 10 && trailingOnes < 3:
		suffixLen = 1
	case totalCoeff <= 10 || trailingOnes == 3:
		suffixLen = 0
	default:
		return nil, errors.New("invalid TotalCoeff and TrailingOnes combination")
	}

	for j := 0; j < totalCoeff-trailingOnes; j++ {
		levelPrefix, err := parseLevelPrefix(br)
		if err != nil {
			return nil, fmt.Errorf("could not parse level prefix, failed with error: %w", err)
		}

		var levelSuffixSize int
		switch {
		case levelPrefix == 14 && suffixLen == 0:
			levelSuffixSize = 4
		case levelPrefix >= 15:
			levelSuffixSize = levelPrefix - 3
		default:
			levelSuffixSize = suffixLen
		}

		var levelSuffix int
		if levelSuffixSize > 0 {
			b, err := br.ReadBits(levelSuffixSize)
			if err != nil {
				return nil, fmt.Errorf("could not parse levelSuffix, failed with error: %w", err)
			}
			levelSuffix = int(b)
		} else {
			levelSuffix = 0
		}

		levelCode := (mini(15, levelPrefix) << uint(suffixLen)) + levelSuffix

		if levelPrefix >= 15 && suffixLen == 0 {
			levelCode += 15
		}

		if levelPrefix >= 16 {
			levelCode += (1 << uint(levelPrefix-3)) - 4096
		}

		if i == trailingOnes && trailingOnes < 3 {
			levelCode += 2
		}

		if levelCode%2 == 0 {
			levelVal = append(levelVal, (levelCode+2)>>1)
		} else {
			levelVal = append(levelVal, (-levelCode-1)>>1)
		}

		if suffixLen == 0 {
			suffixLen = 1
		}

		if absi(levelVal[i]) > (3<<uint(suffixLen-1)) && suffixLen < 6 {
			suffixLen++
		}
		i++
	}
	return levelVal, nil
}

// combineLevelRunInfo combines the level and run information obtained prior
// using the process defined in section 9.2.4 of the specifications and returns
// the corresponding coeffLevel list.
func combineLevelRunInfo(levelVal, runVal []int, totalCoeff int) []int {
	coeffNum := -1
	i := totalCoeff - 1
	var coeffLevel []int
	for j := 0; j < totalCoeff; j++ {
		coeffNum += runVal[i] + 1
		if coeffNum >= len(coeffLevel) {
			coeffLevel = append(coeffLevel, make([]int, (coeffNum+1)-len(coeffLevel))...)
		}
		coeffLevel[coeffNum] = levelVal[i]
		i++
	}
	return coeffLevel
}

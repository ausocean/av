/*
DESCRIPTION
  cabac.go provides utilities for context-adaptive binary artihmetic decoding
  for the parsing of H.264 syntax structure fields.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Bruce McMoran <mcmoranbjr@gmail.com>
  Shawn Smith <shawnpsmith@gmail.com>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h264dec

import (
	"github.com/ausocean/av/codec/h264/h264dec/bits"
	"github.com/pkg/errors"
)

const (
	NaCtxId            = 10000
	NA_SUFFIX          = -1
	MbAddrNotAvailable = 10000
)

// G.7.4.3.4 via G.7.3.3.4 via 7.3.2.13 for NalUnitType 20 or 21
// refLayerMbWidthC is equal to MbWidthC for the reference layer representation
func RefMbW(chromaFlag, refLayerMbWidthC int) int {
	if chromaFlag == 0 {
		return 16
	}
	return refLayerMbWidthC
}

// refLayerMbHeightC is equal to MbHeightC for the reference layer representation
func RefMbH(chromaFlag, refLayerMbHeightC int) int {
	if chromaFlag == 0 {
		return 16
	}
	return refLayerMbHeightC
}
func XOffset(xRefMin16, refMbW int) int {
	return (((xRefMin16 - 64) >> 8) << 4) - (refMbW >> 1)
}
func YOffset(yRefMin16, refMbH int) int {
	return (((yRefMin16 - 64) >> 8) << 4) - (refMbH >> 1)
}
func MbWidthC(sps *SPS) int {
	mbWidthC := 16 / SubWidthC(sps)
	if sps.ChromaFormatIDC == chromaMonochrome || sps.SeparateColorPlaneFlag {
		mbWidthC = 0
	}
	return mbWidthC
}
func MbHeightC(sps *SPS) int {
	mbHeightC := 16 / SubHeightC(sps)
	if sps.ChromaFormatIDC == chromaMonochrome || sps.SeparateColorPlaneFlag {
		mbHeightC = 0
	}
	return mbHeightC
}

// G.8.6.2.2.2
func Xr(x, xOffset, refMbW int) int {
	return (x + xOffset) % refMbW
}
func Yr(y, yOffset, refMbH int) int {
	return (y + yOffset) % refMbH
}

// G.8.6.2.2.2
func Xd(xr, refMbW int) int {
	if xr >= refMbW/2 {
		return xr - refMbW
	}
	return xr + 1
}
func Yd(yr, refMbH int) int {
	if yr >= refMbH/2 {
		return yr - refMbH
	}
	return yr + 1
}
func Ya(yd, refMbH, signYd int) int {
	return yd - (refMbH/2+1)*signYd
}

// 6.4.11.1
func MbAddr(xd, yd, predPartWidth int) {
	// TODO: Unfinished
	var n string
	if xd == -1 && yd == 0 {
		n = "A"
	}
	if xd == 0 && yd == -1 {
		n = "B"
	}
	if xd == predPartWidth && yd == -1 {
		n = "C"
	}
	if xd == -1 && yd == -1 {
		n = "D"
	}
	_ = n
}

func CondTermFlag(mbAddr, mbSkipFlag int) int {
	if mbAddr == MbAddrNotAvailable || mbSkipFlag == 1 {
		return 0
	}
	return 1
}

// s9.3.3 p 278: Returns the value of the syntax element
func (bin *Binarization) Decode(sliceContext *SliceContext, b *bits.BitReader, rbsp []byte) {}

// 9.3.3.1.1 : returns ctxIdxInc
func Decoder9_3_3_1_1_1(condTermFlagA, condTermFlagB int) int {
	return condTermFlagA + condTermFlagB
}

// 9-5
// 7-30 p 112
func SliceQPy(pps *PPS, header *SliceHeader) int {
	return 26 + pps.PicInitQpMinus26 + header.SliceQpDelta
}

// 9-5
func PreCtxState(m, n, sliceQPy int) int {
	// slicQPy-subY
	return Clip3(1, 126, ((m*Clip3(0, 51, sliceQPy))>>4)+n)
}

func Clip1y(x, bitDepthY int) int {
	return Clip3(0, (1<<uint(bitDepthY))-1, x)
}
func Clipc(x, bitDepthC int) int {
	return Clip3(0, (1<<uint(bitDepthC))-1, x)
}

// 5-5
func Clip3(x, y, z int) int {
	if z < x {
		return x
	}
	if z > y {
		return y
	}
	return z
}

type CABAC struct {
	PStateIdx int
	ValMPS    int
	Context   *SliceContext
}

// table 9-1
func initCabac(binarization *Binarization, vid *VideoStream, ctx *SliceContext) *CABAC {
	var valMPS, pStateIdx int
	// TODO: When to use prefix, when to use suffix?
	ctxIdx := CtxIdx(
		binarization.binIdx,
		binarization.MaxBinIdxCtx.Prefix,
		binarization.CtxIdxOffset.Prefix)
	mn := MNVars[ctxIdx]

	preCtxState := PreCtxState(mn[0].M, mn[0].N, SliceQPy(vid.PPS, ctx.SliceHeader))
	if preCtxState <= 63 {
		pStateIdx = 63 - preCtxState
		valMPS = 0
	} else {
		pStateIdx = preCtxState - 64
		valMPS = 1
	}
	_ = pStateIdx
	_ = valMPS
	// Initialization of context variables
	// Initialization of decoding engine
	return &CABAC{
		PStateIdx: pStateIdx,
		ValMPS:    valMPS,
		Context:   ctx,
	}
}

// Binarizations for macroblock types in slice types.
var (
	// binOfIMBTypes provides binarization strings for values of macroblock
	// type in I slices as defined in table 9-36 of the specifications.
	binOfIMBTypes = [numOfIMBTypes][]int{
		0:  {0},
		1:  {1, 0, 0, 0, 0, 0},
		2:  {1, 0, 0, 0, 0, 1},
		3:  {1, 0, 0, 0, 1, 0},
		4:  {1, 0, 0, 0, 1, 1},
		5:  {1, 0, 0, 1, 0, 0, 0},
		6:  {1, 0, 0, 1, 0, 0, 1},
		7:  {1, 0, 0, 1, 0, 1, 0},
		8:  {1, 0, 0, 1, 0, 1, 1},
		9:  {1, 0, 0, 1, 1, 0, 0},
		10: {1, 0, 0, 1, 1, 0, 1},
		11: {1, 0, 0, 1, 1, 1, 0},
		12: {1, 0, 0, 1, 1, 1, 1},
		13: {1, 0, 1, 0, 0, 0},
		14: {1, 0, 1, 0, 0, 1},
		15: {1, 0, 1, 0, 1, 0},
		16: {1, 0, 1, 0, 1, 1},
		17: {1, 0, 1, 1, 0, 0, 0},
		18: {1, 0, 1, 1, 0, 0, 1},
		19: {1, 0, 1, 1, 0, 1, 0},
		20: {1, 0, 1, 1, 0, 1, 1},
		21: {1, 0, 1, 1, 1, 0, 0},
		22: {1, 0, 1, 1, 1, 0, 1},
		23: {1, 0, 1, 1, 1, 1, 0},
		24: {1, 0, 1, 1, 1, 1, 1},
		25: {1, 1},
	}

	// binOfPOrSPMBTypes provides binarization strings for values of macroblock
	// type in P or SP slices as defined in table 9-37 of the specifications.
	// NB: binarization of macroblock types 5 to 30 is 1 and not included here.
	binOfPOrSPMBTypes = [5][]int{
		0: {0, 0, 0},
		1: {0, 1, 1},
		2: {0, 1, 0},
		3: {0, 0, 1},
		4: {},
	}

	// binOfBMBTypes provides binarization strings for values of macroblock
	// type in B slice as defined in table 9-37 of the specifications.
	// NB: binarization of macroblock types 23 to 48 is 111101 and is not
	// included here.
	binOfBMBTypes = [23][]int{
		0:  {0},
		1:  {1, 0, 0},
		2:  {1, 0, 1},
		3:  {1, 1, 0, 0, 0, 0},
		4:  {1, 1, 0, 0, 0, 1},
		5:  {1, 1, 0, 0, 1, 0},
		6:  {1, 1, 0, 0, 1, 1},
		7:  {1, 1, 0, 1, 0, 0},
		8:  {1, 1, 0, 1, 0, 1},
		9:  {1, 1, 0, 1, 1, 0},
		10: {1, 1, 0, 1, 1, 1},
		11: {1, 1, 1, 1, 1, 0},
		12: {1, 1, 1, 0, 0, 0, 0},
		13: {1, 1, 1, 0, 0, 0, 1},
		14: {1, 1, 1, 0, 0, 1, 0},
		15: {1, 1, 1, 0, 0, 1, 1},
		16: {1, 1, 1, 0, 1, 0, 0},
		17: {1, 1, 1, 0, 1, 0, 1},
		18: {1, 1, 1, 0, 1, 1, 0},
		19: {1, 1, 1, 0, 1, 1, 1},
		20: {1, 1, 1, 1, 0, 0, 0},
		21: {1, 1, 1, 1, 0, 0, 1},
		22: {1, 1, 1, 1, 1, 1},
	}
)

// Binarizations for sub-macroblock types in slice types.
var (
	// binOfPorSPSubMBTypes provides binarization strings for values of sub-macroblock
	// type in P or SP slices as defined in table 9-38 of the specifications.
	binOfPOrSPSubMBTypes = [4][]int{
		0: {1},
		1: {0, 0},
		2: {0, 1, 1},
		3: {0, 1, 0},
	}

	// binOfBSubMBTypes provides binarization strings for values of sub-macroblock
	// type in B slices as defined in table 9-38 of the specifications.
	binOfBSubMBTypes = [numOfBSubMBTypes][]int{
		0:  {1},
		1:  {1, 0, 0},
		2:  {1, 0, 1},
		3:  {1, 1, 0, 0, 0},
		4:  {1, 1, 0, 0, 1},
		5:  {1, 1, 0, 1, 0},
		6:  {1, 1, 0, 1, 1},
		7:  {1, 1, 1, 0, 0, 0},
		8:  {1, 1, 1, 0, 0, 1},
		9:  {1, 1, 1, 0, 1, 0},
		10: {1, 1, 1, 0, 1, 1},
		11: {1, 1, 1, 1, 0},
		12: {1, 1, 1, 1, 1},
	}
)

// Table 9-34
type MaxBinIdxCtx struct {
	// When false, Prefix is the MaxBinIdxCtx
	IsPrefixSuffix bool
	Prefix, Suffix int
}
type CtxIdxOffset struct {
	// When false, Prefix is the MaxBinIdxCtx
	IsPrefixSuffix bool
	Prefix, Suffix int
}

// Table 9-34
type Binarization struct {
	SyntaxElement string
	BinarizationType
	MaxBinIdxCtx
	CtxIdxOffset
	UseDecodeBypass int
	// TODO: Why are these private but others aren't?
	binIdx    int
	binString []int
}
type BinarizationType struct {
	PrefixSuffix   bool
	FixedLength    bool
	Unary          bool
	TruncatedUnary bool
	CMax           bool
	// 9.3.2.3
	UEGk      bool
	CMaxValue int
}

// 9.3.2.5
func NewBinarization(syntaxElement string, data *SliceData) *Binarization {
	sliceTypeName := data.SliceTypeName
	logger.Printf("debug: binarization of %s in sliceType %s\n", syntaxElement, sliceTypeName)
	binarization := &Binarization{SyntaxElement: syntaxElement}
	switch syntaxElement {
	case "CodedBlockPattern":
		binarization.BinarizationType = BinarizationType{PrefixSuffix: true}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{IsPrefixSuffix: true, Prefix: 3, Suffix: 1}
		binarization.CtxIdxOffset = CtxIdxOffset{IsPrefixSuffix: true, Prefix: 73, Suffix: 77}
	case "IntraChromaPredMode":
		binarization.BinarizationType = BinarizationType{
			TruncatedUnary: true, CMax: true, CMaxValue: 3}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{Prefix: 1}
		binarization.CtxIdxOffset = CtxIdxOffset{Prefix: 64}
	case "MbQpDelta":
		binarization.BinarizationType = BinarizationType{}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{Prefix: 2}
		binarization.CtxIdxOffset = CtxIdxOffset{Prefix: 60}
	case "MvdLnEnd0":
		binarization.UseDecodeBypass = 1
		binarization.BinarizationType = BinarizationType{UEGk: true}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{IsPrefixSuffix: true, Prefix: 4, Suffix: NA_SUFFIX}
		binarization.CtxIdxOffset = CtxIdxOffset{
			IsPrefixSuffix: true,
			Prefix:         40,
			Suffix:         NA_SUFFIX,
		}
	case "MvdLnEnd1":
		binarization.UseDecodeBypass = 1
		binarization.BinarizationType = BinarizationType{UEGk: true}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{
			IsPrefixSuffix: true,
			Prefix:         4,
			Suffix:         NA_SUFFIX,
		}
		binarization.CtxIdxOffset = CtxIdxOffset{
			IsPrefixSuffix: true,
			Prefix:         47,
			Suffix:         NA_SUFFIX,
		}
		// 9.3.2.5
	case "MbType":
		logger.Printf("debug: \tMbType is %s\n", data.MbTypeName)
		switch sliceTypeName {
		case "SI":
			binarization.BinarizationType = BinarizationType{PrefixSuffix: true}
			binarization.MaxBinIdxCtx = MaxBinIdxCtx{IsPrefixSuffix: true, Prefix: 0, Suffix: 6}
			binarization.CtxIdxOffset = CtxIdxOffset{IsPrefixSuffix: true, Prefix: 0, Suffix: 3}
		case "I":
			binarization.BinarizationType = BinarizationType{}
			binarization.MaxBinIdxCtx = MaxBinIdxCtx{Prefix: 6}
			binarization.CtxIdxOffset = CtxIdxOffset{Prefix: 3}
		case "SP":
			fallthrough
		case "P":
			binarization.BinarizationType = BinarizationType{PrefixSuffix: true}
			binarization.MaxBinIdxCtx = MaxBinIdxCtx{IsPrefixSuffix: true, Prefix: 2, Suffix: 5}
			binarization.CtxIdxOffset = CtxIdxOffset{IsPrefixSuffix: true, Prefix: 14, Suffix: 17}
		}
	case "MbFieldDecodingFlag":
		binarization.BinarizationType = BinarizationType{
			FixedLength: true, CMax: true, CMaxValue: 1}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{Prefix: 0}
		binarization.CtxIdxOffset = CtxIdxOffset{Prefix: 70}
	case "PrevIntra4x4PredModeFlag":
		fallthrough
	case "PrevIntra8x8PredModeFlag":
		binarization.BinarizationType = BinarizationType{FixedLength: true, CMax: true, CMaxValue: 1}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{Prefix: 0}
		binarization.CtxIdxOffset = CtxIdxOffset{Prefix: 68}
	case "RefIdxL0":
		fallthrough
	case "RefIdxL1":
		binarization.BinarizationType = BinarizationType{Unary: true}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{Prefix: 2}
		binarization.CtxIdxOffset = CtxIdxOffset{Prefix: 54}
	case "RemIntra4x4PredMode":
		fallthrough
	case "RemIntra8x8PredMode":
		binarization.BinarizationType = BinarizationType{FixedLength: true, CMax: true, CMaxValue: 7}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{Prefix: 0}
		binarization.CtxIdxOffset = CtxIdxOffset{Prefix: 69}
	case "TransformSize8x8Flag":
		binarization.BinarizationType = BinarizationType{FixedLength: true, CMax: true, CMaxValue: 1}
		binarization.MaxBinIdxCtx = MaxBinIdxCtx{Prefix: 0}
		binarization.CtxIdxOffset = CtxIdxOffset{Prefix: 399}
	}
	return binarization
}
func (b *Binarization) IsBinStringMatch(bits []int) bool {
	for i, _b := range bits {
		if b.binString[i] != _b {
			return false
		}
	}
	return len(b.binString) == len(bits)
}

// 9.3.1.2: output is codIRange and codIOffset
func initDecodingEngine(bitReader *bits.BitReader) (int, int, error) {
	logger.Printf("debug: initializing arithmetic decoding engine\n")
	codIRange := 510
	codIOffset, err := bitReader.ReadBits(9)
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not read codIOffset")
	}
	logger.Printf("debug: codIRange: %d :: codIOffsset: %d\n", codIRange, codIOffset)
	return codIRange, int(codIOffset), nil
}

// 9.3.3.2: output is value of the bin
func NewArithmeticDecoding(context *SliceContext, binarization *Binarization, ctxIdx, codIRange, codIOffset int) (ArithmeticDecoding, error) {
	a := ArithmeticDecoding{Context: context, Binarization: binarization}
	logger.Printf("debug: decoding bypass %d, for ctx %d\n", binarization.UseDecodeBypass, ctxIdx)
	// TODO: Implement
	if binarization.UseDecodeBypass == 1 {
		// TODO: 9.3.3.2.3 : DecodeBypass()
		var err error
		codIOffset, a.BinVal, err = a.DecodeBypass(context.SliceData, codIRange, codIOffset)
		if err != nil {
			return ArithmeticDecoding{}, errors.Wrap(err, "error from DecodeBypass getting codIOffset and BinVal")
		}

	} else if binarization.UseDecodeBypass == 0 && ctxIdx == 276 {
		// TODO: 9.3.3.2.4 : DecodeTerminate()
	} else {
		// TODO: 9.3.3.2.1 : DecodeDecision()
	}
	a.BinVal = -1
	return a, nil
}

// 9.3.3.2.3
// Invoked when bypassFlag is equal to 1
func (a ArithmeticDecoding) DecodeBypass(sliceData *SliceData, codIRange, codIOffset int) (int, int, error) {
	// Decoded value binVal
	codIOffset = codIOffset << uint(1)
	// TODO: Concurrency check
	// TODO: Possibly should be codIOffset | ReadOneBit
	shift, err := sliceData.BitReader.ReadBits(1)
	if err != nil {
		return 0, 0, errors.Wrap(err, "coult not read shift bit from sliceData.")
	}
	codIOffset = codIOffset << uint(shift)
	if codIOffset >= codIRange {
		a.BinVal = 1
		codIOffset -= codIRange
	} else {
		a.BinVal = 0
	}
	return codIOffset, a.BinVal, nil
}

// 9.3.3.2.4
// Decodes endOfSliceFlag and I_PCM
// Returns codIRange, codIOffSet, decoded value of binVal
func (a ArithmeticDecoding) DecodeTerminate(sliceData *SliceData, codIRange, codIOffset int) (int, int, int, error) {
	codIRange -= 2
	if codIOffset >= codIRange {
		a.BinVal = 1
		// Terminate CABAC decoding, last bit inserted into codIOffset is = 1
		// this is now also the rbspStopOneBit
		// TODO: How is this denoting termination?
		return codIRange, codIOffset, a.BinVal, nil
	}
	a.BinVal = 0
	var err error
	codIRange, codIOffset, err = a.RenormD(sliceData, codIRange, codIOffset)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "error from RenormD")
	}
	return codIRange, codIOffset, a.BinVal, nil
}

// 9.3.3.2.2 Renormalization process of ADecEngine
// Returns codIRange, codIOffset
func (a ArithmeticDecoding) RenormD(sliceData *SliceData, codIRange, codIOffset int) (int, int, error) {
	if codIRange >= 256 {
		return codIRange, codIOffset, nil
	}
	codIRange = codIRange << uint(1)
	codIOffset = codIOffset << uint(1)
	bit, err := sliceData.BitReader.ReadBits(1)
	if err != nil {
		return 0, 0, errors.Wrap(err, "could not read bit from sliceData")
	}
	codIOffset = codIOffset | int(bit)
	return a.RenormD(sliceData, codIRange, codIOffset)
}

type ArithmeticDecoding struct {
	Context      *SliceContext
	Binarization *Binarization
	BinVal       int
}

// 9.3.3.2.1
// returns: binVal, updated codIRange, updated codIOffset
func (a ArithmeticDecoding) BinaryDecision(ctxIdx, codIRange, codIOffset int) (int, int, int, error) {
	var binVal int
	cabac := initCabac(a.Binarization, nil, a.Context)
	// Derivce codIRangeLPS
	qCodIRangeIdx := (codIRange >> 6) & 3
	pStateIdx := cabac.PStateIdx
	codIRangeLPS, err := retCodIRangeLPS(pStateIdx, qCodIRangeIdx)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "could not get codIRangeLPS from retCodIRangeLPS")
	}

	codIRange = codIRange - codIRangeLPS
	if codIOffset >= codIRange {
		binVal = 1 - cabac.ValMPS
		codIOffset -= codIRange
		codIRange = codIRangeLPS
	} else {
		binVal = cabac.ValMPS
	}

	// TODO: Do StateTransition and then RenormD happen here? See: 9.3.3.2.1
	return binVal, codIRange, codIOffset, nil
}

// 9.3.3.2.1.1
// Returns: pStateIdx, valMPS
func (c *CABAC) StateTransitionProcess(binVal int) {
	if binVal == c.ValMPS {
		c.PStateIdx = stateTransxTab[c.PStateIdx].TransIdxMPS
	} else {
		if c.PStateIdx == 0 {
			c.ValMPS = 1 - c.ValMPS
		}
		c.PStateIdx = stateTransxTab[c.PStateIdx].TransIdxLPS
	}
}

var ctxIdxLookup = map[int]map[int]int{
	3:  {0: NaCtxId, 1: 276, 2: 3, 3: 4, 4: NaCtxId, 5: NaCtxId},
	14: {0: 0, 1: 1, 2: NaCtxId},
	17: {0: 0, 1: 276, 2: 1, 3: 2, 4: NaCtxId},
	27: {0: NaCtxId, 1: 3, 2: NaCtxId},
	32: {0: 0, 1: 276, 2: 1, 3: 2, 4: NaCtxId},
	36: {2: NaCtxId, 3: 3, 4: 3, 5: 3},
	40: {0: NaCtxId},
	47: {0: NaCtxId, 1: 3, 2: 4, 3: 5},
	54: {0: NaCtxId, 1: 4},
	64: {0: NaCtxId, 1: 3, 2: 3},
	69: {0: 0, 1: 0, 2: 0},
	77: {0: NaCtxId, 1: NaCtxId},
}

// 9.3.3.1
// Returns ctxIdx
func CtxIdx(binIdx, maxBinIdxCtx, ctxIdxOffset int) int {
	ctxIdx := NaCtxId
	// table 9-39
	c, ok := ctxIdxLookup[ctxIdxOffset]
	if ok {
		v, ok := c[binIdx]
		if ok {
			return v
		}
	}

	switch ctxIdxOffset {
	case 0:
		if binIdx != 0 {
			return NaCtxId
		}
		// 9.3.3.1.1.3
	case 3:
		return 7
	case 11:
		if binIdx != 0 {
			return NaCtxId
		}

		// 9.3.3.1.1.3
	case 14:
		if binIdx > 2 {
			return NaCtxId
		}
	case 17:
		return 3
	case 21:
		if binIdx < 3 {
			ctxIdx = binIdx
		} else {
			return NaCtxId
		}
	case 24:
		// 9.3.3.1.1.1
	case 27:
		return 5
	case 32:
		return 3
	case 36:
		if binIdx == 0 || binIdx == 1 {
			ctxIdx = binIdx
		}
	case 40:
		fallthrough
	case 47:
		return 6
	case 54:
		if binIdx > 1 {
			ctxIdx = 5
		}
	case 60:
		if binIdx == 0 {
			// 9.3.3.1.1.5
		}
		if binIdx == 1 {
			ctxIdx = 2
		}
		if binIdx > 1 {
			ctxIdx = 3
		}
	case 64:
		return NaCtxId
	case 68:
		if binIdx != 0 {
			return NaCtxId
		}
		ctxIdx = 0
	case 69:
		return NaCtxId
	case 70:
		if binIdx != 0 {
			return NaCtxId
		}
		// 9.3.3.1.1.2
	case 77:
		return NaCtxId
	case 276:
		if binIdx != 0 {
			return NaCtxId
		}
		ctxIdx = 0
	case 399:
		if binIdx != 0 {
			return NaCtxId
		}
		// 9.3.3.1.1.10
	}

	return ctxIdx
}

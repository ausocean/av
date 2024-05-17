/*
DESCRIPTION
  slice.go provides parsing functionality for slice raw byte sequence data.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>, The Australian Ocean Laboratory (AusOcean)
  Bruce McMoran <mcmoranbjr@gmail.com>
*/

package h264dec

import (
	"bytes"
	"fmt"
	"math"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
	"github.com/pkg/errors"
)

// Slice types as defined by table 7-6 in specifications.
const (
	sliceTypeP  = 0
	sliceTypeB  = 1
	sliceTypeI  = 2
	sliceTypeSP = 3
	sliceTypeSI = 4
)

// Chroma formats as defined in section 6.2, tab 6-1.
const (
	chromaMonochrome = iota
	chroma420
	chroma422
	chroma444
)

type picture struct {
	*SliceContext
	isIDR         bool
	isBottomField bool
	isTopField    bool
}

type VideoStream struct {
	*SPS
	*PPS
	Slices []*SliceContext

	ChromaArrayType                  int
	priorPic                         *picture
	currPic                          *picture
	maxPicOrderCntLsb                int
	picOrderCntMsb                   int
	idrPicFlag                       bool
	frameNumOffset                   int
	expectedDeltaPerPicOrderCntCycle int
	topFieldOrderCnt                 int
	bottomFieldOrderCnt              int
}

type SliceContext struct {
	*SPS
	*PPS
	*NALUnit
	*Slice
	chromaArrayType int
	nalType         int
}

type Slice struct {
	*SliceHeader
	*SliceData
}

// RefPicListModification provides elements of a ref_pic_list_modification syntax
// (defined in 7.3.3.1 of specifications) and a ref_pic_list_mvc_modification
// (defined in H.7.3.3.1.1 of specifications).
type RefPicListModification struct {
	RefPicListModificationFlag [2]bool
	ModificationOfPicNums      [2][]int
	AbsDiffPicNumMinus1        [2][]int
	LongTermPicNum             [2][]int
}

// TODO: need to complete this.
// NewRefPicListMVCModification parses elements of a ref_pic_list_mvc_modification
// following the syntax structure defined in section H.7.3.3.1.1, and returns as
// a new RefPicListModification.
func NewRefPicListMVCModifiation(br *bits.BitReader) (*RefPicListModification, error) {
	return nil, nil
}

// NewRefPicListModification parses elements of a ref_pic_list_modification
// following the syntax structure defined in section 7.3.3.1, and returns as
// a new RefPicListModification.
func NewRefPicListModification(br *bits.BitReader, p *PPS, s *SliceHeader) (*RefPicListModification, error) {
	r := &RefPicListModification{}
	r.ModificationOfPicNums[0] = make([]int, p.NumRefIdxL0DefaultActiveMinus1+2)
	r.ModificationOfPicNums[1] = make([]int, p.NumRefIdxL1DefaultActiveMinus1+2)
	r.AbsDiffPicNumMinus1[0] = make([]int, p.NumRefIdxL0DefaultActiveMinus1+2)
	r.AbsDiffPicNumMinus1[1] = make([]int, p.NumRefIdxL1DefaultActiveMinus1+2)
	r.LongTermPicNum[0] = make([]int, p.NumRefIdxL0DefaultActiveMinus1+2)
	r.LongTermPicNum[1] = make([]int, p.NumRefIdxL1DefaultActiveMinus1+2)
	fr := newFieldReader(br)

	// 7.3.3.1
	if s.SliceType%5 != 2 && s.SliceType%5 != 4 {
		r.RefPicListModificationFlag[0] = fr.readBits(1) == 1

		if r.RefPicListModificationFlag[0] {
			for i := 0; ; i++ {
				r.ModificationOfPicNums[0][i] = int(fr.readUe())

				if r.ModificationOfPicNums[0][i] == 0 || r.ModificationOfPicNums[0][i] == 1 {
					r.AbsDiffPicNumMinus1[0][i] = int(fr.readUe())
				} else if r.ModificationOfPicNums[0][i] == 2 {
					r.LongTermPicNum[0][i] = int(fr.readUe())
				}

				if r.ModificationOfPicNums[0][i] == 3 {
					break
				}
			}
		}
	}

	if s.SliceType%5 == 1 {
		r.RefPicListModificationFlag[1] = fr.readBits(1) == 1

		if r.RefPicListModificationFlag[1] {
			for i := 0; ; i++ {
				r.ModificationOfPicNums[1][i] = int(fr.readUe())

				if r.ModificationOfPicNums[1][i] == 0 || r.ModificationOfPicNums[1][i] == 1 {
					r.AbsDiffPicNumMinus1[1][i] = int(fr.readUe())
				} else if r.ModificationOfPicNums[1][i] == 2 {
					r.LongTermPicNum[1][i] = int(fr.readUe())
				}

				if r.ModificationOfPicNums[1][i] == 3 {
					break
				}
			}
		}
	}
	return r, nil
}

// PredWeightTable provides elements of a pred_weight_table syntax structure
// as defined in section 7.3.3.2 of the specifications.
type PredWeightTable struct {
	LumaLog2WeightDenom   int
	ChromaLog2WeightDenom int
	LumaWeightL0Flag      bool
	LumaWeightL0          []int
	LumaOffsetL0          []int
	ChromaWeightL0Flag    bool
	ChromaWeightL0        [][]int
	ChromaOffsetL0        [][]int
	LumaWeightL1Flag      bool
	LumaWeightL1          []int
	LumaOffsetL1          []int
	ChromaWeightL1Flag    bool
	ChromaWeightL1        [][]int
	ChromaOffsetL1        [][]int
}

// NewPredWeightTable parses elements of a pred_weight_table following the
// syntax structure defined in section 7.3.3.2, and returns as a new
// PredWeightTable.
func NewPredWeightTable(br *bits.BitReader, h *SliceHeader, chromaArrayType int) (*PredWeightTable, error) {
	p := &PredWeightTable{}
	r := newFieldReader(br)

	p.LumaLog2WeightDenom = int(r.readUe())

	if chromaArrayType != 0 {
		p.ChromaLog2WeightDenom = int(r.readUe())
	}
	for i := 0; i <= h.NumRefIdxL0ActiveMinus1; i++ {
		p.LumaWeightL0Flag = r.readBits(1) == 1

		if p.LumaWeightL0Flag {
			se, err := readSe(br)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse LumaWeightL0")
			}
			p.LumaWeightL0 = append(p.LumaWeightL0, se)

			se, err = readSe(br)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse LumaOffsetL0")
			}
			p.LumaOffsetL0 = append(p.LumaOffsetL0, se)
		}
		if chromaArrayType != 0 {
			b, err := br.ReadBits(1)
			if err != nil {
				return nil, errors.Wrap(err, "could not read ChromaWeightL0Flag")
			}
			p.ChromaWeightL0Flag = b == 1

			if p.ChromaWeightL0Flag {
				p.ChromaWeightL0 = append(p.ChromaWeightL0, []int{})
				p.ChromaOffsetL0 = append(p.ChromaOffsetL0, []int{})
				for j := 0; j < 2; j++ {
					se, err := readSe(br)
					if err != nil {
						return nil, errors.Wrap(err, "could not parse ChromaWeightL0")
					}
					p.ChromaWeightL0[i] = append(p.ChromaWeightL0[i], se)

					se, err = readSe(br)
					if err != nil {
						return nil, errors.Wrap(err, "could not parse ChromaOffsetL0")
					}
					p.ChromaOffsetL0[i] = append(p.ChromaOffsetL0[i], se)
				}
			}
		}
	}
	if h.SliceType%5 == 1 {
		for i := 0; i <= h.NumRefIdxL1ActiveMinus1; i++ {
			b, err := br.ReadBits(1)
			if err != nil {
				return nil, errors.Wrap(err, "could not read LumaWeightL1Flag")
			}
			p.LumaWeightL1Flag = b == 1

			if p.LumaWeightL1Flag {
				se, err := readSe(br)
				if err != nil {
					return nil, errors.Wrap(err, "could not parse LumaWeightL1")
				}
				p.LumaWeightL1 = append(p.LumaWeightL1, se)

				se, err = readSe(br)
				if err != nil {
					return nil, errors.Wrap(err, "could not parse LumaOffsetL1")
				}
				p.LumaOffsetL1 = append(p.LumaOffsetL1, se)
			}
			if chromaArrayType != 0 {
				b, err := br.ReadBits(1)
				if err != nil {
					return nil, errors.Wrap(err, "could not read ChromaWeightL1Flag")
				}
				p.ChromaWeightL1Flag = b == 1

				if p.ChromaWeightL1Flag {
					p.ChromaWeightL1 = append(p.ChromaWeightL1, []int{})
					p.ChromaOffsetL1 = append(p.ChromaOffsetL1, []int{})
					for j := 0; j < 2; j++ {
						se, err := readSe(br)
						if err != nil {
							return nil, errors.Wrap(err, "could not parse ChromaWeightL1")
						}
						p.ChromaWeightL1[i] = append(p.ChromaWeightL1[i], se)

						se, err = readSe(br)
						if err != nil {
							return nil, errors.Wrap(err, "could not parse ChromaOffsetL1")
						}
						p.ChromaOffsetL1[i] = append(p.ChromaOffsetL1[i], se)
					}
				}
			}
		}
	}
	return p, nil
}

// DecRefPicMarking provides elements of a dec_ref_pic_marking syntax structure
// as defined in section 7.3.3.3 of the specifications.
type DecRefPicMarking struct {
	NoOutputOfPriorPicsFlag       bool
	LongTermReferenceFlag         bool
	AdaptiveRefPicMarkingModeFlag bool
	elements                      []drpmElement
}

type drpmElement struct {
	MemoryManagementControlOperation int
	DifferenceOfPicNumsMinus1        int
	LongTermPicNum                   int
	LongTermFrameIdx                 int
	MaxLongTermFrameIdxPlus1         int
}

// NewDecRefPicMarking parses elements of a dec_ref_pic_marking following the
// syntax structure defined in section 7.3.3.3, and returns as a new
// DecRefPicMarking.
func NewDecRefPicMarking(br *bits.BitReader, idrPic bool) (*DecRefPicMarking, error) {
	d := &DecRefPicMarking{}
	r := newFieldReader(br)
	if idrPic {
		b, err := br.ReadBits(1)
		if err != nil {
			return nil, errors.Wrap(err, "could not read NoOutputOfPriorPicsFlag")
		}
		d.NoOutputOfPriorPicsFlag = b == 1

		b, err = br.ReadBits(1)
		if err != nil {
			return nil, errors.Wrap(err, "could not read LongTermReferenceFlag")
		}
		d.LongTermReferenceFlag = b == 1
	} else {
		b, err := br.ReadBits(1)
		if err != nil {
			return nil, errors.Wrap(err, "could not read AdaptiveRefPicMarkingModeFlag")
		}
		d.AdaptiveRefPicMarkingModeFlag = b == 1

		if d.AdaptiveRefPicMarkingModeFlag {
			for i := 0; ; i++ {
				d.elements = append(d.elements, drpmElement{})

				d.elements[i].MemoryManagementControlOperation = int(r.readUe())

				if d.elements[i].MemoryManagementControlOperation == 1 || d.elements[i].MemoryManagementControlOperation == 3 {
					d.elements[i].DifferenceOfPicNumsMinus1 = int(r.readUe())
				}
				if d.elements[i].MemoryManagementControlOperation == 2 {
					d.elements[i].LongTermPicNum = int(r.readUe())
				}
				if d.elements[i].MemoryManagementControlOperation == 3 || d.elements[i].MemoryManagementControlOperation == 6 {
					d.elements[i].LongTermFrameIdx = int(r.readUe())
				}
				if d.elements[i].MemoryManagementControlOperation == 4 {
					d.elements[i].MaxLongTermFrameIdxPlus1 = int(r.readUe())
				}

				if d.elements[i].MemoryManagementControlOperation == 0 {
					break
				}
			}
		}
	}
	return d, nil
}

type SliceHeader struct {
	FirstMbInSlice          int
	SliceType               int
	PPSID                   int
	ColorPlaneID            int
	FrameNum                int
	FieldPic                bool
	BottomField             bool
	IDRPicID                int
	PicOrderCntLsb          int
	DeltaPicOrderCntBottom  int
	DeltaPicOrderCnt        []int
	RedundantPicCnt         int
	DirectSpatialMvPred     bool
	NumRefIdxActiveOverride bool
	NumRefIdxL0ActiveMinus1 int
	NumRefIdxL1ActiveMinus1 int
	*RefPicListModification
	*PredWeightTable
	*DecRefPicMarking
	CabacInit               int
	SliceQpDelta            int
	SpForSwitch             bool
	SliceQsDelta            int
	DisableDeblockingFilter int
	SliceAlphaC0OffsetDiv2  int
	SliceBetaOffsetDiv2     int
	SliceGroupChangeCycle   int
}

type SliceData struct {
	BitReader                *bits.BitReader
	CabacAlignmentOneBit     int
	MbSkipRun                int
	MbSkipFlag               bool
	MbFieldDecodingFlag      bool
	EndOfSliceFlag           bool
	MbType                   int
	MbTypeName               string
	SliceTypeName            string
	PcmAlignmentZeroBit      int
	PcmSampleLuma            []int
	PcmSampleChroma          []int
	TransformSize8x8Flag     bool
	CodedBlockPattern        int
	MbQpDelta                int
	PrevIntra4x4PredModeFlag []int
	RemIntra4x4PredMode      []int
	PrevIntra8x8PredModeFlag []int
	RemIntra8x8PredMode      []int
	IntraChromaPredMode      int
	RefIdxL0                 []int
	RefIdxL1                 []int
	MvdL0                    [][][]int
	MvdL1                    [][][]int
}

// Table 7-6
var sliceTypeMap = map[int]string{
	0: "P",
	1: "B",
	2: "I",
	3: "SP",
	4: "SI",
	5: "P",
	6: "B",
	7: "I",
	8: "SP",
	9: "SI",
}

func flagVal(b bool) int {
	if b {
		return 1
	}
	return 0
}

// context-adaptive arithmetic entropy-coded element (CABAC)
// 9.3
// When parsing the slice date of a slice (7.3.4) the initialization is 9.3.1
func (d SliceData) ae(v int) int {
	// 9.3.1.1 : CABAC context initialization ctxIdx
	return 0
}

// 8.2.2
func MbToSliceGroupMap(sps *SPS, pps *PPS, header *SliceHeader) []int {
	mbaffFrameFlag := 0
	if sps.MBAdaptiveFrameFieldFlag && !header.FieldPic {
		mbaffFrameFlag = 1
	}
	mapUnitToSliceGroupMap := MapUnitToSliceGroupMap(sps, pps, header)
	mbToSliceGroupMap := []int{}
	for i := 0; i <= PicSizeInMbs(sps, header)-1; i++ {
		if sps.FrameMBSOnlyFlag || header.FieldPic {
			mbToSliceGroupMap = append(mbToSliceGroupMap, mapUnitToSliceGroupMap[i])
			continue
		}
		if mbaffFrameFlag == 1 {
			mbToSliceGroupMap = append(mbToSliceGroupMap, mapUnitToSliceGroupMap[i/2])
			continue
		}
		if !sps.FrameMBSOnlyFlag && !sps.MBAdaptiveFrameFieldFlag && !header.FieldPic {
			mbToSliceGroupMap = append(
				mbToSliceGroupMap,
				mapUnitToSliceGroupMap[(i/(2*PicWidthInMbs(sps)))*PicWidthInMbs(sps)+(i%PicWidthInMbs(sps))])
		}
	}
	return mbToSliceGroupMap

}
func PicWidthInMbs(sps *SPS) int {
	return int(sps.PicWidthInMBSMinus1 + 1)
}
func PicHeightInMapUnits(sps *SPS) int {
	return int(sps.PicHeightInMapUnitsMinus1 + 1)
}
func PicSizeInMapUnits(sps *SPS) int {
	return int(PicWidthInMbs(sps) * PicHeightInMapUnits(sps))
}
func FrameHeightInMbs(sps *SPS) int {
	return int((2 - flagVal(sps.FrameMBSOnlyFlag)) * PicHeightInMapUnits(sps))
}
func PicHeightInMbs(sps *SPS, header *SliceHeader) int {
	return int(FrameHeightInMbs(sps) / (1 + flagVal(header.FieldPic)))
}
func PicSizeInMbs(sps *SPS, header *SliceHeader) int {
	return int(PicWidthInMbs(sps) * PicHeightInMbs(sps, header))
}

// table 6-1
func SubWidthC(sps *SPS) int {
	n := 17
	if sps.SeparateColorPlaneFlag {
		if sps.ChromaFormatIDC == chroma444 {
			return n
		}
	}

	switch sps.ChromaFormatIDC {
	case chromaMonochrome:
		return n
	case chroma420:
		n = 2
	case chroma422:
		n = 2
	case chroma444:
		n = 1

	}
	return n
}
func SubHeightC(sps *SPS) int {
	n := 17
	if sps.SeparateColorPlaneFlag {
		if sps.ChromaFormatIDC == chroma444 {
			return n
		}
	}
	switch sps.ChromaFormatIDC {
	case chromaMonochrome:
		return n
	case chroma420:
		n = 2
	case chroma422:
		n = 1
	case chroma444:
		n = 1

	}
	return n
}

// 7-36
func CodedBlockPatternLuma(data *SliceData) int {
	return data.CodedBlockPattern % 16
}
func CodedBlockPatternChroma(data *SliceData) int {
	return data.CodedBlockPattern / 16
}

// dependencyId see Annex G.8.8.1
// Also G7.3.1.1 nal_unit_header_svc_extension
func DQId(nalUnit *NALUnit) int {
	return int((nalUnit.SVCExtension.DependencyID << 4)) + int(nalUnit.SVCExtension.QualityID)
}

// Annex G p527
func NumMbPart(nalUnit *NALUnit, sps *SPS, header *SliceHeader, data *SliceData) int {
	sliceType := sliceTypeMap[header.SliceType]
	numMbPart := 0
	if MbTypeName(sliceType, CurrMbAddr(sps, header)) == "B_SKIP" || MbTypeName(sliceType, CurrMbAddr(sps, header)) == "B_Direct_16x16" {
		if DQId(nalUnit) == 0 && nalUnit.Type != 20 {
			numMbPart = 4
		} else if DQId(nalUnit) > 0 && nalUnit.Type == 20 {
			numMbPart = 1
		}
	} else if MbTypeName(sliceType, CurrMbAddr(sps, header)) != "B_SKIP" && MbTypeName(sliceType, CurrMbAddr(sps, header)) != "B_Direct_16x16" {
		numMbPart = CurrMbAddr(sps, header)

	}
	return numMbPart
}

func MbPred(chromaArrayType int, vid *VideoStream, sliceContext *SliceContext, br *bits.BitReader, rbsp []byte) error {
	var cabac *CABAC
	r := newFieldReader(br)

	sliceType := sliceTypeMap[sliceContext.Slice.SliceHeader.SliceType]
	mbPartPredMode, err := MbPartPredMode(sliceContext.Slice.SliceData, sliceType, sliceContext.Slice.SliceData.MbType, 0)
	if err != nil {
		return errors.Wrap(err, "could not get mbPartPredMode")
	}
	if mbPartPredMode == intra4x4 || mbPartPredMode == intra8x8 || mbPartPredMode == intra16x16 {
		if mbPartPredMode == intra4x4 {
			for luma4x4BlkIdx := 0; luma4x4BlkIdx < 16; luma4x4BlkIdx++ {
				var v int
				if vid.PPS.EntropyCodingMode == 1 {
					// TODO: 1 bit or ae(v)
					binarization := NewBinarization(
						"PrevIntra4x4PredModeFlag",
						sliceContext.Slice.SliceData)
					binarization.Decode(sliceContext, br, rbsp)

					// TODO: fix videostream should be nil.
					cabac = initCabac(binarization, nil, sliceContext)
					_ = cabac
					logger.Printf("TODO: ae for PevIntra4x4PredModeFlag[%d]\n", luma4x4BlkIdx)
				} else {
					b, err := br.ReadBits(1)
					if err != nil {
						return errors.Wrap(err, "could not read PrevIntra4x4PredModeFlag")
					}
					v = int(b)
				}
				sliceContext.Slice.SliceData.PrevIntra4x4PredModeFlag = append(
					sliceContext.Slice.SliceData.PrevIntra4x4PredModeFlag,
					v)
				if sliceContext.Slice.SliceData.PrevIntra4x4PredModeFlag[luma4x4BlkIdx] == 0 {
					if vid.PPS.EntropyCodingMode == 1 {
						// TODO: 3 bits or ae(v)
						binarization := NewBinarization(
							"RemIntra4x4PredMode",
							sliceContext.Slice.SliceData)
						binarization.Decode(sliceContext, br, rbsp)

						logger.Printf("TODO: ae for RemIntra4x4PredMode[%d]\n", luma4x4BlkIdx)
					} else {
						b, err := br.ReadBits(3)
						if err != nil {
							return errors.Wrap(err, "could not read RemIntra4x4PredMode")
						}
						v = int(b)
					}
					if len(sliceContext.Slice.SliceData.RemIntra4x4PredMode) < luma4x4BlkIdx {
						sliceContext.Slice.SliceData.RemIntra4x4PredMode = append(
							sliceContext.Slice.SliceData.RemIntra4x4PredMode,
							make([]int, luma4x4BlkIdx-len(sliceContext.Slice.SliceData.RemIntra4x4PredMode)+1)...)
					}
					sliceContext.Slice.SliceData.RemIntra4x4PredMode[luma4x4BlkIdx] = v
				}
			}
		}
		if mbPartPredMode == intra8x8 {
			for luma8x8BlkIdx := 0; luma8x8BlkIdx < 4; luma8x8BlkIdx++ {
				sliceContext.Update(sliceContext.Slice.SliceHeader, sliceContext.Slice.SliceData)
				var v int
				if vid.PPS.EntropyCodingMode == 1 {
					// TODO: 1 bit or ae(v)
					binarization := NewBinarization("PrevIntra8x8PredModeFlag", sliceContext.Slice.SliceData)
					binarization.Decode(sliceContext, br, rbsp)

					logger.Printf("TODO: ae for PrevIntra8x8PredModeFlag[%d]\n", luma8x8BlkIdx)
				} else {
					b, err := br.ReadBits(1)
					if err != nil {
						return errors.Wrap(err, "could not read PrevIntra8x8PredModeFlag")
					}
					v = int(b)
				}
				sliceContext.Slice.SliceData.PrevIntra8x8PredModeFlag = append(
					sliceContext.Slice.SliceData.PrevIntra8x8PredModeFlag, v)
				if sliceContext.Slice.SliceData.PrevIntra8x8PredModeFlag[luma8x8BlkIdx] == 0 {
					if vid.PPS.EntropyCodingMode == 1 {
						// TODO: 3 bits or ae(v)
						binarization := NewBinarization(
							"RemIntra8x8PredMode",
							sliceContext.Slice.SliceData)
						binarization.Decode(sliceContext, br, rbsp)

						logger.Printf("TODO: ae for RemIntra8x8PredMode[%d]\n", luma8x8BlkIdx)
					} else {
						b, err := br.ReadBits(3)
						if err != nil {
							return errors.Wrap(err, "could not read RemIntra8x8PredMode")
						}
						v = int(b)
					}
					if len(sliceContext.Slice.SliceData.RemIntra8x8PredMode) < luma8x8BlkIdx {
						sliceContext.Slice.SliceData.RemIntra8x8PredMode = append(
							sliceContext.Slice.SliceData.RemIntra8x8PredMode,
							make([]int, luma8x8BlkIdx-len(sliceContext.Slice.SliceData.RemIntra8x8PredMode)+1)...)
					}
					sliceContext.Slice.SliceData.RemIntra8x8PredMode[luma8x8BlkIdx] = v
				}
			}

		}
		if chromaArrayType == 1 || chromaArrayType == 2 {
			if vid.PPS.EntropyCodingMode == 1 {
				// TODO: ue(v) or ae(v)
				binarization := NewBinarization(
					"IntraChromaPredMode",
					sliceContext.Slice.SliceData)
				binarization.Decode(sliceContext, br, rbsp)

				logger.Printf("TODO: ae for IntraChromaPredMode\n")
			} else {
				sliceContext.Slice.SliceData.IntraChromaPredMode = int(r.readUe())
			}
		}

	} else if mbPartPredMode != direct {
		for mbPartIdx := 0; mbPartIdx < NumMbPart(sliceContext.NALUnit, vid.SPS, sliceContext.Slice.SliceHeader, sliceContext.Slice.SliceData); mbPartIdx++ {
			sliceContext.Update(sliceContext.Slice.SliceHeader, sliceContext.Slice.SliceData)
			m, err := MbPartPredMode(sliceContext.Slice.SliceData, sliceType, sliceContext.Slice.SliceData.MbType, mbPartIdx)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("could not get mbPartPredMode for loop 1 mbPartIdx: %d", mbPartIdx))
			}
			if (sliceContext.Slice.SliceHeader.NumRefIdxL0ActiveMinus1 > 0 || sliceContext.Slice.SliceData.MbFieldDecodingFlag != sliceContext.Slice.SliceHeader.FieldPic) && m != predL1 {
				logger.Printf("\tTODO: refIdxL0[%d] te or ae(v)\n", mbPartIdx)
				if len(sliceContext.Slice.SliceData.RefIdxL0) < mbPartIdx {
					sliceContext.Slice.SliceData.RefIdxL0 = append(
						sliceContext.Slice.SliceData.RefIdxL0, make([]int, mbPartIdx-len(sliceContext.Slice.SliceData.RefIdxL0)+1)...)
				}
				if vid.PPS.EntropyCodingMode == 1 {
					// TODO: te(v) or ae(v)
					binarization := NewBinarization(
						"RefIdxL0",
						sliceContext.Slice.SliceData)
					binarization.Decode(sliceContext, br, rbsp)

					logger.Printf("TODO: ae for RefIdxL0[%d]\n", mbPartIdx)
				} else {
					// TODO: Only one reference picture is used for inter-prediction,
					// then the value should be 0
					if MbaffFrameFlag(vid.SPS, sliceContext.Slice.SliceHeader) == 0 || !sliceContext.Slice.SliceData.MbFieldDecodingFlag {
						sliceContext.Slice.SliceData.RefIdxL0[mbPartIdx] = int(r.readTe(uint(sliceContext.Slice.SliceHeader.NumRefIdxL0ActiveMinus1)))
					} else {
						rangeMax := 2*sliceContext.Slice.SliceHeader.NumRefIdxL0ActiveMinus1 + 1
						sliceContext.Slice.SliceData.RefIdxL0[mbPartIdx] = int(r.readTe(uint(rangeMax)))
					}
				}
			}
		}
		for mbPartIdx := 0; mbPartIdx < NumMbPart(sliceContext.NALUnit, vid.SPS, sliceContext.Slice.SliceHeader, sliceContext.Slice.SliceData); mbPartIdx++ {
			m, err := MbPartPredMode(sliceContext.Slice.SliceData, sliceType, sliceContext.Slice.SliceData.MbType, mbPartIdx)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("could not get mbPartPredMode for loop 2 mbPartIdx: %d", mbPartIdx))
			}
			if m != predL1 {
				for compIdx := 0; compIdx < 2; compIdx++ {
					if len(sliceContext.Slice.SliceData.MvdL0) < mbPartIdx {
						sliceContext.Slice.SliceData.MvdL0 = append(
							sliceContext.Slice.SliceData.MvdL0,
							make([][][]int, mbPartIdx-len(sliceContext.Slice.SliceData.MvdL0)+1)...)
					}
					if len(sliceContext.Slice.SliceData.MvdL0[mbPartIdx][0]) < compIdx {
						sliceContext.Slice.SliceData.MvdL0[mbPartIdx][0] = append(
							sliceContext.Slice.SliceData.MvdL0[mbPartIdx][0],
							make([]int, compIdx-len(sliceContext.Slice.SliceData.MvdL0[mbPartIdx][0])+1)...)
					}
					if vid.PPS.EntropyCodingMode == 1 {
						// TODO: se(v) or ae(v)
						if compIdx == 0 {
							binarization := NewBinarization(
								"MvdLnEnd0",
								sliceContext.Slice.SliceData)
							binarization.Decode(sliceContext, br, rbsp)

						} else if compIdx == 1 {
							binarization := NewBinarization(
								"MvdLnEnd1",
								sliceContext.Slice.SliceData)
							binarization.Decode(sliceContext, br, rbsp)

						}
						logger.Printf("TODO: ae for MvdL0[%d][0][%d]\n", mbPartIdx, compIdx)
					} else {
						sliceContext.Slice.SliceData.MvdL0[mbPartIdx][0][compIdx], _ = readSe(br)
					}
				}
			}
		}
		for mbPartIdx := 0; mbPartIdx < NumMbPart(sliceContext.NALUnit, vid.SPS, sliceContext.Slice.SliceHeader, sliceContext.Slice.SliceData); mbPartIdx++ {
			sliceContext.Update(sliceContext.Slice.SliceHeader, sliceContext.Slice.SliceData)
			m, err := MbPartPredMode(sliceContext.Slice.SliceData, sliceType, sliceContext.Slice.SliceData.MbType, mbPartIdx)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("could not get mbPartPredMode for loop 3 mbPartIdx: %d", mbPartIdx))
			}
			if m != predL0 {
				for compIdx := 0; compIdx < 2; compIdx++ {
					if len(sliceContext.Slice.SliceData.MvdL1) < mbPartIdx {
						sliceContext.Slice.SliceData.MvdL1 = append(
							sliceContext.Slice.SliceData.MvdL1,
							make([][][]int, mbPartIdx-len(sliceContext.Slice.SliceData.MvdL1)+1)...)
					}
					if len(sliceContext.Slice.SliceData.MvdL1[mbPartIdx][0]) < compIdx {
						sliceContext.Slice.SliceData.MvdL1[mbPartIdx][0] = append(
							sliceContext.Slice.SliceData.MvdL0[mbPartIdx][0],
							make([]int, compIdx-len(sliceContext.Slice.SliceData.MvdL1[mbPartIdx][0])+1)...)
					}
					if vid.PPS.EntropyCodingMode == 1 {
						if compIdx == 0 {
							binarization := NewBinarization(
								"MvdLnEnd0",
								sliceContext.Slice.SliceData)
							binarization.Decode(sliceContext, br, rbsp)

						} else if compIdx == 1 {
							binarization := NewBinarization(
								"MvdLnEnd1",
								sliceContext.Slice.SliceData)
							binarization.Decode(sliceContext, br, rbsp)

						}
						// TODO: se(v) or ae(v)
						logger.Printf("TODO: ae for MvdL1[%d][0][%d]\n", mbPartIdx, compIdx)
					} else {
						sliceContext.Slice.SliceData.MvdL1[mbPartIdx][0][compIdx], _ = readSe(br)
					}
				}
			}
		}
	}
	return nil
}

// 8.2.2.1
func MapUnitToSliceGroupMap(sps *SPS, pps *PPS, header *SliceHeader) []int {
	mapUnitToSliceGroupMap := []int{}
	picSizeInMapUnits := PicSizeInMapUnits(sps)
	if pps.NumSliceGroupsMinus1 == 0 {
		// 0 to PicSizeInMapUnits -1 inclusive
		for i := 0; i <= picSizeInMapUnits-1; i++ {
			mapUnitToSliceGroupMap = append(mapUnitToSliceGroupMap, 0)
		}
	} else {
		switch pps.SliceGroupMapType {
		case 0:
			// 8.2.2.1
			i := 0
			for i < picSizeInMapUnits {
				// iGroup should be incremented in the pps.RunLengthMinus1 index operation. There may be a bug here
				for iGroup := 0; iGroup <= pps.NumSliceGroupsMinus1 && i < picSizeInMapUnits; i += pps.RunLengthMinus1[iGroup+1] + 1 {
					for j := 0; j < pps.RunLengthMinus1[iGroup] && i+j < picSizeInMapUnits; j++ {
						if len(mapUnitToSliceGroupMap) < i+j {
							mapUnitToSliceGroupMap = append(
								mapUnitToSliceGroupMap,
								make([]int, (i+j)-len(mapUnitToSliceGroupMap)+1)...)
						}
						mapUnitToSliceGroupMap[i+j] = iGroup
					}
				}
			}
		case 1:
			// 8.2.2.2
			for i := 0; i < picSizeInMapUnits; i++ {
				v := ((i % PicWidthInMbs(sps)) + (((i / PicWidthInMbs(sps)) * (pps.NumSliceGroupsMinus1 + 1)) / 2)) % (pps.NumSliceGroupsMinus1 + 1)
				mapUnitToSliceGroupMap = append(mapUnitToSliceGroupMap, v)
			}
		case 2:
			// 8.2.2.3
			for i := 0; i < picSizeInMapUnits; i++ {
				mapUnitToSliceGroupMap = append(mapUnitToSliceGroupMap, pps.NumSliceGroupsMinus1)
			}
			for iGroup := pps.NumSliceGroupsMinus1 - 1; iGroup >= 0; iGroup-- {
				yTopLeft := pps.TopLeft[iGroup] / PicWidthInMbs(sps)
				xTopLeft := pps.TopLeft[iGroup] % PicWidthInMbs(sps)
				yBottomRight := pps.BottomRight[iGroup] / PicWidthInMbs(sps)
				xBottomRight := pps.BottomRight[iGroup] % PicWidthInMbs(sps)
				for y := yTopLeft; y <= yBottomRight; y++ {
					for x := xTopLeft; x <= xBottomRight; x++ {
						idx := y*PicWidthInMbs(sps) + x
						if len(mapUnitToSliceGroupMap) < idx {
							mapUnitToSliceGroupMap = append(
								mapUnitToSliceGroupMap,
								make([]int, idx-len(mapUnitToSliceGroupMap)+1)...)
							mapUnitToSliceGroupMap[idx] = iGroup
						}
					}
				}
			}

		case 3:
			// 8.2.2.4
			// TODO
		case 4:
			// 8.2.2.5
			// TODO
		case 5:
			// 8.2.2.6
			// TODO
		case 6:
			// 8.2.2.7
			// TODO
		}
	}
	// 8.2.2.8
	// Convert mapUnitToSliceGroupMap to MbToSliceGroupMap
	return mapUnitToSliceGroupMap
}
func nextMbAddress(n int, sps *SPS, pps *PPS, header *SliceHeader) int {
	i := n + 1
	// picSizeInMbs is the number of macroblocks in picture 0
	// 7-13
	// PicWidthInMbs = sps.PicWidthInMBSMinus1 + 1
	// PicHeightInMapUnits = sps.PicHeightInMapUnitsMinus1 + 1
	// 7-29
	// picSizeInMbs = PicWidthInMbs * PicHeightInMbs
	// 7-26
	// PicHeightInMbs = FrameHeightInMbs / (1 + header.fieldPicFlag)
	// 7-18
	// FrameHeightInMbs = (2 - ps.FrameMBSOnlyFlag) * PicHeightInMapUnits
	picWidthInMbs := sps.PicWidthInMBSMinus1 + 1
	picHeightInMapUnits := sps.PicHeightInMapUnitsMinus1 + 1
	frameHeightInMbs := (2 - flagVal(sps.FrameMBSOnlyFlag)) * int(picHeightInMapUnits)
	picHeightInMbs := frameHeightInMbs / (1 + flagVal(header.FieldPic))
	picSizeInMbs := int(picWidthInMbs) * picHeightInMbs
	mbToSliceGroupMap := MbToSliceGroupMap(sps, pps, header)
	for i < picSizeInMbs && mbToSliceGroupMap[i] != mbToSliceGroupMap[i] {
		i++
	}
	return i
}

func CurrMbAddr(sps *SPS, header *SliceHeader) int {
	mbaffFrameFlag := 0
	if sps.MBAdaptiveFrameFieldFlag && !header.FieldPic {
		mbaffFrameFlag = 1
	}

	return header.FirstMbInSlice * (1 * mbaffFrameFlag)
}

func MbaffFrameFlag(sps *SPS, header *SliceHeader) int {
	if sps.MBAdaptiveFrameFieldFlag && !header.FieldPic {
		return 1
	}
	return 0
}

func NewSliceData(chromaArrayType int, vid *VideoStream, sliceContext *SliceContext, br *bits.BitReader) (*SliceData, error) {
	r := newFieldReader(br)
	var cabac *CABAC
	sliceContext.Slice.SliceData = &SliceData{BitReader: br}
	// TODO: Why is this being initialized here?
	// initCabac(sliceContext)
	if vid.PPS.EntropyCodingMode == 1 {
		for !br.ByteAligned() {
			b, err := br.ReadBits(1)
			if err != nil {
				return nil, errors.Wrap(err, "could not read CabacAlignmentOneBit")
			}
			sliceContext.Slice.SliceData.CabacAlignmentOneBit = int(b)
		}
	}
	mbaffFrameFlag := 0
	if vid.SPS.MBAdaptiveFrameFieldFlag && !sliceContext.Slice.SliceHeader.FieldPic {
		mbaffFrameFlag = 1
	}
	currMbAddr := sliceContext.Slice.SliceHeader.FirstMbInSlice * (1 * mbaffFrameFlag)

	moreDataFlag := true
	prevMbSkipped := 0
	sliceContext.Slice.SliceData.SliceTypeName = sliceTypeMap[sliceContext.Slice.SliceHeader.SliceType]
	sliceContext.Slice.SliceData.MbTypeName = MbTypeName(sliceContext.Slice.SliceData.SliceTypeName, sliceContext.Slice.SliceData.MbType)
	logger.Printf("debug: \tSliceData: Processing moreData: %v\n", moreDataFlag)
	for moreDataFlag {
		logger.Printf("debug: \tLooking for more sliceContext.Slice.SliceData in slice type %s\n", sliceContext.Slice.SliceData.SliceTypeName)
		if sliceContext.Slice.SliceData.SliceTypeName != "I" && sliceContext.Slice.SliceData.SliceTypeName != "SI" {
			logger.Printf("debug: \tNonI/SI slice, processing moreData\n")
			if vid.PPS.EntropyCodingMode == 0 {
				sliceContext.Slice.SliceData.MbSkipRun = int(r.readUe())

				if sliceContext.Slice.SliceData.MbSkipRun > 0 {
					prevMbSkipped = 1
				}
				for i := 0; i < sliceContext.Slice.SliceData.MbSkipRun; i++ {
					// nextMbAddress(currMbAdd
					currMbAddr = nextMbAddress(currMbAddr, vid.SPS, vid.PPS, sliceContext.Slice.SliceHeader)
				}
				if sliceContext.Slice.SliceData.MbSkipRun > 0 {
					moreDataFlag = moreRBSPData(br)
				}
			} else {
				b, err := br.ReadBits(1)
				if err != nil {
					return nil, errors.Wrap(err, "could not read MbSkipFlag")
				}
				sliceContext.Slice.SliceData.MbSkipFlag = b == 1

				moreDataFlag = !sliceContext.Slice.SliceData.MbSkipFlag
			}
		}
		if moreDataFlag {
			if mbaffFrameFlag == 1 && (currMbAddr%2 == 0 || (currMbAddr%2 == 1 && prevMbSkipped == 1)) {
				if vid.PPS.EntropyCodingMode == 1 {
					// TODO: ae implementation
					binarization := NewBinarization("MbFieldDecodingFlag", sliceContext.Slice.SliceData)
					// TODO: this should take a BitReader where the nil is.
					binarization.Decode(sliceContext, br, nil)

					logger.Printf("TODO: ae for MbFieldDecodingFlag\n")
				} else {
					b, err := br.ReadBits(1)
					if err != nil {
						return nil, errors.Wrap(err, "could not read MbFieldDecodingFlag")
					}
					sliceContext.Slice.SliceData.MbFieldDecodingFlag = b == 1
				}
			}

			// BEGIN: macroblockLayer()
			if vid.PPS.EntropyCodingMode == 1 {
				// TODO: ae implementation
				binarization := NewBinarization("MbType", sliceContext.Slice.SliceData)
				cabac = initCabac(binarization, nil, sliceContext)
				_ = cabac
				// TODO: remove bytes parameter from this function.
				binarization.Decode(sliceContext, br, nil)
				if binarization.PrefixSuffix {
					logger.Printf("debug: MBType binarization has prefix and suffix\n")
				}
				bits := []int{}
				for binIdx := 0; binarization.IsBinStringMatch(bits); binIdx++ {
					newBit, err := br.ReadBits(1)
					if err != nil {
						return nil, errors.Wrap(err, "could not read bit")
					}
					if binarization.UseDecodeBypass == 1 {
						// DecodeBypass
						logger.Printf("TODO: decodeBypass is set: 9.3.3.2.3")
						codIRange, codIOffset, err := initDecodingEngine(sliceContext.Slice.SliceData.BitReader)
						if err != nil {
							return nil, errors.Wrap(err, "could not initialise decoding engine")
						}
						// Initialize the decoder
						// TODO: When should the suffix of MaxBinIdxCtx be used and when just the prefix?
						// TODO: When should the suffix of CtxIdxOffset be used?
						arithmeticDecoder, err := NewArithmeticDecoding(
							sliceContext,
							binarization,
							CtxIdx(
								binarization.binIdx,
								binarization.MaxBinIdxCtx.Prefix,
								binarization.CtxIdxOffset.Prefix,
							),
							codIRange,
							codIOffset,
						)
						if err != nil {
							return nil, errors.Wrap(err, "error from NewArithmeticDecoding")
						}
						// Bypass decoding
						codIOffset, _, err = arithmeticDecoder.DecodeBypass(
							sliceContext.Slice.SliceData,
							codIRange,
							codIOffset,
						)
						if err != nil {
							return nil, errors.Wrap(err, "could not DecodeBypass")
						}
						// End DecodeBypass

					} else {
						// DO 9.3.3.1
						ctxIdx := CtxIdx(
							binIdx,
							binarization.MaxBinIdxCtx.Prefix,
							binarization.CtxIdxOffset.Prefix)
						if binarization.MaxBinIdxCtx.IsPrefixSuffix {
							logger.Printf("TODO: Handle PrefixSuffix binarization\n")
						}
						logger.Printf("debug: MBType ctxIdx for %d is %d\n", binIdx, ctxIdx)
						// Then 9.3.3.2
						codIRange, codIOffset, err := initDecodingEngine(br)
						if err != nil {
							return nil, errors.Wrap(err, "error from initDecodingEngine")
						}
						logger.Printf("debug: coding engine initialized: %d/%d\n", codIRange, codIOffset)
					}
					bits = append(bits, int(newBit))
				}

				logger.Printf("TODO: ae for MBType\n")
			} else {
				sliceContext.Slice.SliceData.MbType = int(r.readUe())
			}
			if sliceContext.Slice.SliceData.MbTypeName == "I_PCM" {
				for !br.ByteAligned() {
					_, err := br.ReadBits(1)
					if err != nil {
						return nil, errors.Wrap(err, "could not read PCMAlignmentZeroBit")
					}
				}
				// 7-3 p95
				bitDepthY := 8 + vid.SPS.BitDepthLumaMinus8
				for i := 0; i < 256; i++ {
					s, err := br.ReadBits(int(bitDepthY))
					if err != nil {
						return nil, errors.Wrap(err, fmt.Sprintf("could not read PcmSampleLuma[%d]", i))
					}
					sliceContext.Slice.SliceData.PcmSampleLuma = append(
						sliceContext.Slice.SliceData.PcmSampleLuma,
						int(s))
				}
				// 9.3.1 p 246
				// cabac = initCabac(binarization, sliceContext)
				// 6-1 p 47
				mbWidthC := 16 / SubWidthC(vid.SPS)
				mbHeightC := 16 / SubHeightC(vid.SPS)
				// if monochrome
				if vid.SPS.ChromaFormatIDC == chromaMonochrome || vid.SPS.SeparateColorPlaneFlag {
					mbWidthC = 0
					mbHeightC = 0
				}

				bitDepthC := 8 + vid.SPS.BitDepthChromaMinus8
				for i := 0; i < 2*mbWidthC*mbHeightC; i++ {
					s, err := br.ReadBits(int(bitDepthC))
					if err != nil {
						return nil, errors.Wrap(err, fmt.Sprintf("could not read PcmSampleChroma[%d]", i))
					}
					sliceContext.Slice.SliceData.PcmSampleChroma = append(
						sliceContext.Slice.SliceData.PcmSampleChroma,
						int(s))
				}
				// 9.3.1 p 246
				// cabac = initCabac(binarization, sliceContext)

			} else {
				noSubMbPartSizeLessThan8x8Flag := 1
				m, err := MbPartPredMode(sliceContext.Slice.SliceData, sliceContext.Slice.SliceData.SliceTypeName, sliceContext.Slice.SliceData.MbType, 0)
				if err != nil {
					return nil, errors.Wrap(err, "could not get mbPartPredMode")
				}
				if sliceContext.Slice.SliceData.MbTypeName == "I_NxN" && m != intra16x16 && NumMbPart(sliceContext.NALUnit, vid.SPS, sliceContext.Slice.SliceHeader, sliceContext.Slice.SliceData) == 4 {
					logger.Printf("\tTODO: subMbPred\n")
					/*
						subMbType := SubMbPred(sliceContext.Slice.SliceData.MbType)
						for mbPartIdx := 0; mbPartIdx < 4; mbPartIdx++ {
							if subMbType[mbPartIdx] != "B_Direct_8x8" {
								if NumbSubMbPart(subMbType[mbPartIdx]) > 1 {
									noSubMbPartSizeLessThan8x8Flag = 0
								}
							} else if !vid.SPS.Direct8x8InferenceFlag {
								noSubMbPartSizeLessThan8x8Flag = 0
							}
						}
					*/
				} else {
					if vid.PPS.Transform8x8Mode == 1 && sliceContext.Slice.SliceData.MbTypeName == "I_NxN" {
						// TODO
						// 1 bit or ae(v)
						// If vid.PPS.EntropyCodingMode == 1, use ae(v)
						if vid.PPS.EntropyCodingMode == 1 {
							binarization := NewBinarization("TransformSize8x8Flag", sliceContext.Slice.SliceData)
							cabac = initCabac(binarization, nil, sliceContext)
							binarization.Decode(sliceContext, br, nil)

							logger.Println("TODO: ae(v) for TransformSize8x8Flag")
						} else {
							b, err := br.ReadBits(1)
							if err != nil {
								return nil, errors.Wrap(err, "could not read TransformSize8x8Flag")
							}
							sliceContext.Slice.SliceData.TransformSize8x8Flag = b == 1
						}
					}
					// TODO: fix nil argument for.
					MbPred(chromaArrayType, nil, sliceContext, br, nil)
				}
				m, err = MbPartPredMode(sliceContext.Slice.SliceData, sliceContext.Slice.SliceData.SliceTypeName, sliceContext.Slice.SliceData.MbType, 0)
				if err != nil {
					return nil, errors.Wrap(err, "could not get mbPartPredMode")
				}
				if m != intra16x16 {
					// TODO: me, ae
					logger.Printf("TODO: CodedBlockPattern pending me/ae implementation\n")
					if vid.PPS.EntropyCodingMode == 1 {
						binarization := NewBinarization("CodedBlockPattern", sliceContext.Slice.SliceData)
						cabac = initCabac(binarization, nil, sliceContext)
						// TODO: fix nil argument.
						binarization.Decode(sliceContext, br, nil)

						logger.Printf("TODO: ae for CodedBlockPattern\n")
					} else {
						me, _ := readMe(
							br,
							uint(chromaArrayType),
							// TODO: fix this
							//MbPartPredMode(sliceContext.Slice.SliceData, sliceContext.Slice.SliceData.SliceTypeName, sliceContext.Slice.SliceData.MbType, 0)))
							0)
						sliceContext.Slice.SliceData.CodedBlockPattern = int(me)
					}

					// sliceContext.Slice.SliceData.CodedBlockPattern = me(v) | ae(v)
					if CodedBlockPatternLuma(sliceContext.Slice.SliceData) > 0 && vid.PPS.Transform8x8Mode == 1 && sliceContext.Slice.SliceData.MbTypeName != "I_NxN" && noSubMbPartSizeLessThan8x8Flag == 1 && (sliceContext.Slice.SliceData.MbTypeName != "B_Direct_16x16" || vid.SPS.Direct8x8InferenceFlag) {
						// TODO: 1 bit or ae(v)
						if vid.PPS.EntropyCodingMode == 1 {
							binarization := NewBinarization("Transform8x8Flag", sliceContext.Slice.SliceData)
							cabac = initCabac(binarization, nil, sliceContext)
							// TODO: fix nil argument.
							binarization.Decode(sliceContext, br, nil)

							logger.Printf("TODO: ae for TranformSize8x8Flag\n")
						} else {
							b, err := br.ReadBits(1)
							if err != nil {
								return nil, errors.Wrap(err, "coult not read TransformSize8x8Flag")
							}
							sliceContext.Slice.SliceData.TransformSize8x8Flag = b == 1
						}
					}
				}
				m, err = MbPartPredMode(sliceContext.Slice.SliceData, sliceContext.Slice.SliceData.SliceTypeName, sliceContext.Slice.SliceData.MbType, 0)
				if err != nil {
					return nil, errors.Wrap(err, "could not get mbPartPredMode")
				}
				if CodedBlockPatternLuma(sliceContext.Slice.SliceData) > 0 || CodedBlockPatternChroma(sliceContext.Slice.SliceData) > 0 || m == intra16x16 {
					// TODO: se or ae(v)
					if vid.PPS.EntropyCodingMode == 1 {
						binarization := NewBinarization("MbQpDelta", sliceContext.Slice.SliceData)
						cabac = initCabac(binarization, nil, sliceContext)
						// TODO; fix nil argument
						binarization.Decode(sliceContext, br, nil)

						logger.Printf("TODO: ae for MbQpDelta\n")
					} else {
						sliceContext.Slice.SliceData.MbQpDelta, _ = readSe(br)
					}

				}
			}

		} // END MacroblockLayer
		if vid.PPS.EntropyCodingMode == 0 {
			moreDataFlag = moreRBSPData(br)
		} else {
			if sliceContext.Slice.SliceData.SliceTypeName != "I" && sliceContext.Slice.SliceData.SliceTypeName != "SI" {
				if sliceContext.Slice.SliceData.MbSkipFlag {
					prevMbSkipped = 1
				} else {
					prevMbSkipped = 0
				}
			}
			if mbaffFrameFlag == 1 && currMbAddr%2 == 0 {
				moreDataFlag = true
			} else {
				// TODO: ae implementation
				b, err := br.ReadBits(1)
				if err != nil {
					return nil, errors.Wrap(err, "could not read EndOfSliceFlag")
				}
				sliceContext.Slice.SliceData.EndOfSliceFlag = b == 1
				moreDataFlag = !sliceContext.Slice.SliceData.EndOfSliceFlag
			}
		}
		currMbAddr = nextMbAddress(currMbAddr, vid.SPS, vid.PPS, sliceContext.Slice.SliceHeader)
	} // END while moreDataFlag
	return sliceContext.Slice.SliceData, nil
}

func (c *SliceContext) Update(header *SliceHeader, data *SliceData) {
	c.Slice = &Slice{SliceHeader: header, SliceData: data}
}
func NewSliceContext(vid *VideoStream, nalUnit *NALUnit, rbsp []byte, showPacket bool) (*SliceContext, error) {
	var err error
	sps := vid.SPS
	pps := vid.PPS
	logger.Printf("debug: %s RBSP %d bytes %d bits == \n", NALUnitType[int(nalUnit.Type)], len(rbsp), len(rbsp)*8)
	logger.Printf("debug: \t%#v\n", rbsp[0:8])
	var idrPic bool
	if nalUnit.Type == 5 {
		idrPic = true
	}
	header := SliceHeader{}
	if sps.SeparateColorPlaneFlag {
		vid.ChromaArrayType = 0
	} else {
		vid.ChromaArrayType = int(sps.ChromaFormatIDC)
	}
	br := bits.NewBitReader(bytes.NewReader(rbsp))
	r := newFieldReader(br)

	header.FirstMbInSlice = int(r.readUe())
	header.SliceType = int(r.readUe())

	sliceType := sliceTypeMap[header.SliceType]
	logger.Printf("debug: %s (%s) slice of %d bytes\n", NALUnitType[int(nalUnit.Type)], sliceType, len(rbsp))
	header.PPSID = int(r.readUe())
	if sps.SeparateColorPlaneFlag {
		b, err := br.ReadBits(2)
		if err != nil {
			return nil, errors.Wrap(err, "could not read ColorPlaneID")
		}
		header.ColorPlaneID = int(b)
	}
	// TODO: See 7.4.3
	// header.FrameNum = b.NextField("FrameNum", 0)
	if !sps.FrameMBSOnlyFlag {
		b, err := br.ReadBits(1)
		if err != nil {
			return nil, errors.Wrap(err, "could not read FieldPic")
		}
		header.FieldPic = b == 1
		if header.FieldPic {
			b, err := br.ReadBits(1)
			if err != nil {
				return nil, errors.Wrap(err, "could not read BottomField")
			}
			header.BottomField = b == 1
		}
	}
	if idrPic {
		header.IDRPicID = int(r.readUe())
	}
	if sps.PicOrderCountType == 0 {
		b, err := br.ReadBits(int(sps.Log2MaxPicOrderCntLSBMin4 + 4))
		if err != nil {
			return nil, errors.Wrap(err, "could not read PicOrderCntLsb")
		}
		header.PicOrderCntLsb = int(b)

		if pps.BottomFieldPicOrderInFramePresent && !header.FieldPic {
			header.DeltaPicOrderCntBottom, err = readSe(br)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse DeltaPicOrderCntBottom")
			}
		}
	}
	if sps.PicOrderCountType == 1 && !sps.DeltaPicOrderAlwaysZeroFlag {
		header.DeltaPicOrderCnt[0], err = readSe(br)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse DeltaPicOrderCnt")
		}

		if pps.BottomFieldPicOrderInFramePresent && !header.FieldPic {
			header.DeltaPicOrderCnt[1], err = readSe(br)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse DeltaPicOrderCnt")
			}
		}
	}
	if pps.RedundantPicCntPresent {
		header.RedundantPicCnt = int(r.readUe())
	}
	if sliceType == "B" {
		b, err := br.ReadBits(1)
		if err != nil {
			return nil, errors.Wrap(err, "could not read DirectSpatialMvPred")
		}
		header.DirectSpatialMvPred = b == 1
	}
	if sliceType == "B" || sliceType == "SP" {
		b, err := br.ReadBits(1)
		if err != nil {
			return nil, errors.Wrap(err, "could not read NumRefIdxActiveOverride")
		}
		header.NumRefIdxActiveOverride = b == 1

		if header.NumRefIdxActiveOverride {
			header.NumRefIdxL0ActiveMinus1 = int(r.readUe())
			if sliceType == "B" {
				header.NumRefIdxL1ActiveMinus1 = int(r.readUe())
			}
		}
	}

	if nalUnit.Type == 20 || nalUnit.Type == 21 {
		// Annex H
		// H.7.3.3.1.1
		// refPicListMvcModifications()
	} else {
		header.RefPicListModification, err = NewRefPicListModification(br, pps, &header)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse RefPicListModification")
		}
	}

	if (pps.WeightedPred && (sliceType == "P" || sliceType == "SP")) || (pps.WeightedBipred == 1 && sliceType == "B") {
		header.PredWeightTable, err = NewPredWeightTable(br, &header, vid.ChromaArrayType)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse PredWeightTable")
		}
	}
	if nalUnit.RefIdc != 0 {
		// devRefPicMarking()
		header.DecRefPicMarking, err = NewDecRefPicMarking(br, idrPic)
		if err != nil {
			return nil, errors.Wrap(err, "could not parse DecRefPicMarking")
		}
	}
	if pps.EntropyCodingMode == 1 && sliceType != "I" && sliceType != "SI" {
		header.CabacInit = int(r.readUe())
	}
	header.SliceQpDelta = int(r.readSe())

	if sliceType == "SP" || sliceType == "SI" {
		if sliceType == "SP" {
			header.SpForSwitch = r.readBits(1) == 1
		}
		header.SliceQsDelta = int(r.readSe())
	}
	if pps.DeblockingFilterControlPresent {
		header.DisableDeblockingFilter = int(r.readUe())
		if header.DisableDeblockingFilter != 1 {
			header.SliceAlphaC0OffsetDiv2, err = readSe(br)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse SliceAlphaC0OffsetDiv2")
			}

			header.SliceBetaOffsetDiv2, err = readSe(br)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse SliceBetaOffsetDiv2")
			}
		}
	}
	if pps.NumSliceGroupsMinus1 > 0 && pps.SliceGroupMapType >= 3 && pps.SliceGroupMapType <= 5 {
		b, err := br.ReadBits(int(math.Ceil(math.Log2(float64(pps.PicSizeInMapUnitsMinus1/pps.SliceGroupChangeRateMinus1 + 1)))))
		if err != nil {
			return nil, errors.Wrap(err, "could not read SliceGruopChangeCycle")
		}
		header.SliceGroupChangeCycle = int(b)
	}

	sliceContext := &SliceContext{
		NALUnit: nalUnit,
		Slice: &Slice{
			SliceHeader: &header,
		},
	}
	sliceContext.Slice.SliceData, err = NewSliceData(vid.ChromaArrayType, nil, sliceContext, br)
	if err != nil {
		return nil, errors.Wrap(err, "could not create slice data")
	}

	return sliceContext, nil
}

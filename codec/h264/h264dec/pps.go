package h264dec

import (
	"math"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
)

// import "strings"

// Specification Page 46 7.3.2.2

type PPS struct {
	ID, SPSID                         int
	EntropyCodingMode                 int
	BottomFieldPicOrderInFramePresent bool
	NumSliceGroupsMinus1              int
	SliceGroupMapType                 int
	RunLengthMinus1                   []int
	TopLeft                           []int
	BottomRight                       []int
	SliceGroupChangeDirection         bool
	SliceGroupChangeRateMinus1        int
	PicSizeInMapUnitsMinus1           int
	SliceGroupId                      []int
	NumRefIdxL0DefaultActiveMinus1    int
	NumRefIdxL1DefaultActiveMinus1    int
	WeightedPred                      bool
	WeightedBipred                    int
	PicInitQpMinus26                  int
	PicInitQsMinus26                  int
	ChromaQpIndexOffset               int
	DeblockingFilterControlPresent    bool
	ConstrainedIntraPred              bool
	RedundantPicCntPresent            bool
	Transform8x8Mode                  int
	PicScalingMatrixPresent           bool
	PicScalingListPresent             []bool
	SecondChromaQpIndexOffset         int
}

func NewPPS(br *bits.BitReader, chromaFormat int) (*PPS, error) {
	pps := PPS{}
	r := newFieldReader(br)

	pps.ID = int(r.readUe())
	pps.SPSID = int(r.readUe())
	pps.EntropyCodingMode = int(r.readBits(1))
	pps.BottomFieldPicOrderInFramePresent = r.readBits(1) == 1
	pps.NumSliceGroupsMinus1 = int(r.readUe())

	if pps.NumSliceGroupsMinus1 > 0 {
		pps.SliceGroupMapType = int(r.readUe())

		if pps.SliceGroupMapType == 0 {
			for iGroup := 0; iGroup <= pps.NumSliceGroupsMinus1; iGroup++ {
				pps.RunLengthMinus1 = append(pps.RunLengthMinus1, int(r.readUe()))
			}
		} else if pps.SliceGroupMapType == 2 {
			for iGroup := 0; iGroup < pps.NumSliceGroupsMinus1; iGroup++ {
				pps.TopLeft[iGroup] = int(r.readUe())
				pps.BottomRight[iGroup] = int(r.readUe())
			}
		} else if pps.SliceGroupMapType > 2 && pps.SliceGroupMapType < 6 {
			pps.SliceGroupChangeDirection = r.readBits(1) == 1
			pps.SliceGroupChangeRateMinus1 = int(r.readUe())
		} else if pps.SliceGroupMapType == 6 {
			pps.PicSizeInMapUnitsMinus1 = int(r.readUe())

			for i := 0; i <= pps.PicSizeInMapUnitsMinus1; i++ {
				pps.SliceGroupId[i] = int(r.readBits(int(math.Ceil(math.Log2(float64(pps.NumSliceGroupsMinus1 + 1))))))
			}
		}

	}
	pps.NumRefIdxL0DefaultActiveMinus1 = int(r.readUe())
	pps.NumRefIdxL1DefaultActiveMinus1 = int(r.readUe())
	pps.WeightedPred = r.readBits(1) == 1
	pps.WeightedBipred = int(r.readBits(2))
	pps.PicInitQpMinus26 = int(r.readSe())
	pps.PicInitQsMinus26 = int(r.readSe())
	pps.ChromaQpIndexOffset = int(r.readSe())
	pps.DeblockingFilterControlPresent = r.readBits(1) == 1
	pps.ConstrainedIntraPred = r.readBits(1) == 1
	pps.RedundantPicCntPresent = r.readBits(1) == 1

	logger.Printf("debug: \tChecking for more PPS data")
	if moreRBSPData(br) {
		logger.Printf("debug: \tProcessing additional PPS data")
		pps.Transform8x8Mode = int(r.readBits(1))
		pps.PicScalingMatrixPresent = r.readBits(1) == 1

		if pps.PicScalingMatrixPresent {
			v := 6
			if chromaFormat != chroma444 {
				v = 2
			}
			for i := 0; i < 6+(v*pps.Transform8x8Mode); i++ {
				pps.PicScalingListPresent[i] = r.readBits(1) == 1
				if pps.PicScalingListPresent[i] {
					if i < 6 {
						scalingList(
							br,
							ScalingList4x4[i],
							16,
							DefaultScalingMatrix4x4[i])

					} else {
						scalingList(
							br,
							ScalingList8x8[i],
							64,
							DefaultScalingMatrix8x8[i-6])

					}
				}
			}
		}
		pps.SecondChromaQpIndexOffset = r.readSe()
		moreRBSPData(br)
		// rbspTrailingBits()
	}
	return &pps, nil
}

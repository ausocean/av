/*
DESCRIPTION
  decode.go provides slice decoding functionality.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package h264dec provides a decoder for h264 frames.
package h264dec

import (
	"errors"
	"fmt"
)

// NB: this is a placeholder.
func decode(vid *VideoStream, ctx *SliceContext) error {
	var err error
	vid.topFieldOrderCnt, vid.bottomFieldOrderCnt, err = decodePicOrderCnt(vid, ctx)
	if err != nil {
		return fmt.Errorf("could not derive topFieldOrderCnt and bottomFieldOrderCnt, failed with error: %w", err)
	}

	// According to 8.2.1 after decoding picture.
	if ctx.elements[0].MemoryManagementControlOperation == 5 {
		tempPicOrderCnt := picOrderCnt(ctx)
		vid.topFieldOrderCnt = vid.topFieldOrderCnt - tempPicOrderCnt
		vid.bottomFieldOrderCnt = vid.bottomFieldOrderCnt - tempPicOrderCnt
	}
	return nil
}

// TODO: complete this. Unsure how to determine if pic is frame or a
// complementary field pair.
// picOrderCnt as defined by section 8.2.1.
func picOrderCnt(ctx *SliceContext) int {
	panic("not implemented")
}

// decodePicOrderCnt derives topFieldOrderCnt and bottomFieldOrderCnt based
// on the PicOrderCntType using the process defined in section 8.2.1 of the
// specifications.
func decodePicOrderCnt(vid *VideoStream, ctx *SliceContext) (topFieldOrderCnt, bottomFieldOrderCnt int, err error) {
	// There are some steps listed in 8.2.1 regarding cases when not an IDR frame,
	// but we're not yet dealing with that.
	// TODO: write steps dealing with not IDR frame.
	if !vid.idrPicFlag {
		panic("not implemented")
	}

	switch ctx.PicOrderCountType {
	case 0:
		topFieldOrderCnt, bottomFieldOrderCnt = decodePicOrderCntType0(vid, ctx)
	case 1:
		topFieldOrderCnt, bottomFieldOrderCnt = decodePicOrderCntType1(vid, ctx)
	case 2:
		topFieldOrderCnt, bottomFieldOrderCnt = decodePicOrderCntType2(vid, ctx)
	default:
		err = errors.New("invalid PicOrderCountType")
	}

	// TODO: check DiffPicOrderCnt( picA, picB ) once picOrderCnt( picX ) is
	// worked out.
	return
}

// picOrderCntType0 is used to return topFieldOrderCnt and bottomFieldOrderCnt
// when pic_order_cnt_type i.e vid.PicOrderCntType == 0, using the process
// defined in section 8.2.1.1 of the specifications. If topFieldOrderCnt or
// bottomFieldOrderCnt are -1 they are unset.
func decodePicOrderCntType0(vid *VideoStream, ctx *SliceContext) (topFieldOrderCnt, bottomFieldOrderCnt int) {
	prevPicOrderCntMsb, prevPicOrderCntLsb := 0, 0
	topFieldOrderCnt, bottomFieldOrderCnt = -1, -1

	// NB: We're currently only handling IDRs so panic.
	if !vid.idrPicFlag {
		panic("not implemented")
	}

	vid.picOrderCntMsb = prevPicOrderCntMsb
	if ctx.PicOrderCntLsb < prevPicOrderCntLsb && (prevPicOrderCntLsb-ctx.PicOrderCntLsb) >= (vid.maxPicOrderCntLsb/2) {
		vid.picOrderCntMsb = prevPicOrderCntMsb + vid.maxPicOrderCntLsb
	} else if ctx.PicOrderCntLsb > prevPicOrderCntLsb && (ctx.PicOrderCntLsb-prevPicOrderCntLsb) > (vid.maxPicOrderCntLsb/2) {
		vid.picOrderCntMsb = prevPicOrderCntMsb - vid.maxPicOrderCntLsb
	}

	if !ctx.BottomField {
		topFieldOrderCnt = vid.picOrderCntMsb + ctx.PicOrderCntLsb
	} else if ctx.FieldPic {
		bottomFieldOrderCnt = vid.picOrderCntMsb + ctx.PicOrderCntLsb
	} else {
		bottomFieldOrderCnt = topFieldOrderCnt + ctx.DeltaPicOrderCntBottom
	}
	return
}

// picOrderCntType1 is used to return topFieldOrderCnt and bottomFieldOrderCnt
// when vic.PicOrderCntType == 1 according to logic defined in section 8.2.1.2
// of the specifications. If topFieldOrderCnt or bottomFieldOrderCnt are -1,
// then they are considered unset.
func decodePicOrderCntType1(vid *VideoStream, ctx *SliceContext) (topFieldOrderCnt, bottomFieldOrderCnt int) {
	topFieldOrderCnt, bottomFieldOrderCnt = -1, -1

	// TODO: this will be prevFrameNum when we do frames other than IDR.
	_ = vid.priorPic.FrameNum

	if !vid.idrPicFlag {
		panic("not implemented")
	}

	vid.frameNumOffset = 0

	absFrameNum := 0
	if ctx.NumRefFramesInPicOrderCntCycle != 0 {
		absFrameNum = vid.frameNumOffset + ctx.FrameNum
	}

	if ctx.RefIdc == 0 && absFrameNum > 0 {
		absFrameNum = absFrameNum - 1
	}

	var expectedPicOrderCnt int
	if absFrameNum > 0 {
		picOrderCntCycleCnt := (absFrameNum - 1) / int(ctx.NumRefFramesInPicOrderCntCycle)
		frameNumInPicOrderCntCycle := (absFrameNum - 1) % int(ctx.NumRefFramesInPicOrderCntCycle)
		expectedPicOrderCnt = picOrderCntCycleCnt * vid.expectedDeltaPerPicOrderCntCycle
		for i := 0; i <= frameNumInPicOrderCntCycle; i++ {
			expectedPicOrderCnt = expectedPicOrderCnt + ctx.OffsetForRefFrameList[i]
		}
	}

	if ctx.RefIdc == 0 {
		expectedPicOrderCnt = expectedPicOrderCnt + int(ctx.OffsetForNonRefPic)
	}

	if !ctx.FieldPic {
		topFieldOrderCnt = expectedPicOrderCnt + ctx.DeltaPicOrderCnt[0]
		bottomFieldOrderCnt = topFieldOrderCnt + int(ctx.OffsetForTopToBottomField) + ctx.DeltaPicOrderCnt[1]
	} else if ctx.BottomField {
		bottomFieldOrderCnt = expectedPicOrderCnt + int(ctx.OffsetForTopToBottomField) + ctx.DeltaPicOrderCnt[0]
	} else {
		topFieldOrderCnt = expectedPicOrderCnt + ctx.DeltaPicOrderCnt[0]
	}
	return
}

// picOrderCntType2 is used to return topFieldOrderCnt and bottomFieldOrderCnt
// when vic.PicOrderCntType == 1 according to logic defined in section 8.2.1.3
// of the specifications. If topFieldOrderCnt or bottomFieldOrderCnt are -1,
// then they are considered unset.
func decodePicOrderCntType2(vid *VideoStream, ctx *SliceContext) (topFieldOrderCnt, bottomFieldOrderCnt int) {
	topFieldOrderCnt, bottomFieldOrderCnt = -1, -1

	// TODO: this will be prevFrameNum when we do frames other than IDR.
	_ = vid.priorPic.FrameNum

	if !vid.idrPicFlag {
		panic("not implemented")
	}
	vid.frameNumOffset = 0

	// TODO: handle tempPicOrderCnt calculation for when not IDR.
	var tempPicOrderCnt int

	if !ctx.FieldPic {
		topFieldOrderCnt = tempPicOrderCnt
		bottomFieldOrderCnt = tempPicOrderCnt
	} else if ctx.BottomField {
		bottomFieldOrderCnt = tempPicOrderCnt
	} else {
		topFieldOrderCnt = tempPicOrderCnt
	}

	return
}

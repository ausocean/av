/*
DESCRIPTION
  slice_test.go provides testing for parsing functionality found in slice.go.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Shawn Smith <shawn@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h264dec

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
)

var subWidthCTests = []struct {
	in   SPS
	want int
}{
	{SPS{}, 17},
	{SPS{ChromaFormatIDC: 0}, 17},
	{SPS{ChromaFormatIDC: 1}, 2},
	{SPS{ChromaFormatIDC: 2}, 2},
	{SPS{ChromaFormatIDC: 3}, 1},
	{SPS{ChromaFormatIDC: 3, SeparateColorPlaneFlag: true}, 17},
	{SPS{ChromaFormatIDC: 999}, 17},
}

// TestSubWidthC tests that the correct SubWidthC is returned given
// SPS inputs with various chroma formats.
func TestSubWidthC(t *testing.T) {
	for _, tt := range subWidthCTests {
		if got := SubWidthC(&tt.in); got != tt.want {
			t.Errorf("SubWidthC(%#v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

var subHeightCTests = []struct {
	in   SPS
	want int
}{
	{SPS{}, 17},
	{SPS{ChromaFormatIDC: 0}, 17},
	{SPS{ChromaFormatIDC: 1}, 2},
	{SPS{ChromaFormatIDC: 2}, 1},
	{SPS{ChromaFormatIDC: 3}, 1},
	{SPS{ChromaFormatIDC: 3, SeparateColorPlaneFlag: true}, 17},
	{SPS{ChromaFormatIDC: 999}, 17},
}

// TestSubHeightC tests that the correct SubHeightC is returned given
// SPS inputs with various chroma formats.
func TestSubHeightC(t *testing.T) {
	for _, tt := range subHeightCTests {
		if got := SubHeightC(&tt.in); got != tt.want {
			t.Errorf("SubHeight(%#v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestNewRefPicListModification(t *testing.T) {
	tests := []struct {
		in   string
		s    SliceHeader
		p    PPS
		want RefPicListModification
	}{
		{
			in: "1" + // u(1) ref_pic_list_modification_flag_l0=true
				// First modification for list0
				"1" + // ue(v) modification_of_pic_nums_idc[0][0] = 0
				"010" + // ue(v) abs_diff_pic_num_minus1[0][0] = 1

				// Second modification for list0
				"010" + // ue(v) modification_of_pic_nums_idc[0][1] = 1
				"011" + // ue(v) abs_diff_pic_num_minus1[0][1] = 2

				// Third modification for list0
				"011" + // ue(v) modification_of_pic_nums_idc[0][2] = 2
				"010" + // ue(v) long_term_pic_num = 1

				// Fourth modification does not exist
				"00100", // ue(v) modification_of_pic_nums_idc[0][3] = 3

			s: SliceHeader{
				SliceType: 3,
			},

			p: PPS{
				NumRefIdxL0DefaultActiveMinus1: 2,
			},

			want: RefPicListModification{
				RefPicListModificationFlag: [2]bool{true, false},
				ModificationOfPicNums:      [2][]int{{0, 1, 2, 3}, {0, 0}},
				AbsDiffPicNumMinus1:        [2][]int{{1, 2, 0, 0}, {0, 0}},
				LongTermPicNum:             [2][]int{{0, 0, 1, 0}, {0, 0}},
			},
		},
	}

	for i, test := range tests {
		inBytes, err := binToSlice(test.in)
		if err != nil {
			t.Fatalf("unexpected error %v for binToSlice in test %d", err, i)
		}

		got, err := NewRefPicListModification(bits.NewBitReader(bytes.NewReader(inBytes)), &test.p, &test.s)
		if err != nil {
			t.Fatalf("unexpected error %v for NewRefPicListModification in test %d", err, i)
		}

		if !reflect.DeepEqual(*got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

func TestNewPredWeightTable(t *testing.T) {
	tests := []struct {
		in              string
		sliceHeader     SliceHeader
		chromaArrayType int
		want            PredWeightTable
	}{
		{
			in: "011" + // ue(v) luma_log2_weight_denom = 2
				"00100" + // ue(v) chroma_log2_weigght_denom = 3

				// list0
				// i = 0
				"1" + // u(1) luma_weight_l0_flag = true
				"011" + // se(v) luma_weight_l0[0] = -1
				"010" + // se(v) luma_offset_l0[0] = 1
				"1" + // u(1) chroma_weight_l0_flag = true

				// i = 0, j = 0
				"010" + // se(v) chroma_weight_l0[0][0] = 1
				"010" + // se(v) chroma_offset_l0[0][0] = 1

				// i = 0, j = 1
				"010" + // se(v) chroma_weight_l0[0][1] = 1
				"010" + // se(v) chroma_offset_l0[0][1] = 1

				// i = 1
				"1" + // u(1) luma_weight_l0_flag = true
				"011" + // se(v) luma_weight_l0[1] = -1
				"00100" + // se(v) luma_offset_l0[1] = 2
				"1" + // u(1) chroma_weight_l0_flag = true

				// i = 1, j = 0
				"011" + // se(v) chroma_weight_l0[1][0] = -1
				"00101" + // se(v) chroma_offset_l0[1][0] = -2

				// i = 1, j = 1
				"011" + // se(v) chroma_weight_l0[1][1] = -1
				"011" + // se(v) chroma_offset_l0[1][1] = -1

				// list1
				// i = 0
				"1" + // u(1) luma_weight_l1_flag = true
				"011" + // se(v) luma_weight_l1[0] = -1
				"010" + // se(v) luma_offset_l1[0] = 1
				"1" + // u(1) chroma_weight_l1_flag = true

				// i = 0, j = 0
				"010" + // se(v) chroma_weight_l1[0][0] = 1
				"010" + // se(v) chroma_offset_l1[0][0] = 1

				// i = 0, j = 1
				"010" + // se(v) chroma_weight_l1[0][1] = 1
				"010" + // se(v) chroma_offset_l1[0][1] = 1

				// i = 1
				"1" + // u(1) luma_weight_l1_flag = true
				"011" + // se(v) luma_weight_l1[1] = -1
				"00100" + // se(v) luma_offset_l1[1] = 2
				"1" + // u(1) chroma_weight_l1_flag = true

				// i = 1, j = 0
				"011" + // se(v) chroma_weight_l0[1][0] = -1
				"00101" + // se(v) chroma_offset_l0[1][0] = -2

				// i = 1, j = 1
				"011" + // se(v) chroma_weight_l0[1][1] = -1
				"011", // se(v) chroma_offset_l0[1][1] = -1

			sliceHeader: SliceHeader{
				NumRefIdxL0ActiveMinus1: 1,
				NumRefIdxL1ActiveMinus1: 1,
				SliceType:               1,
			},

			chromaArrayType: 1,

			want: PredWeightTable{
				LumaLog2WeightDenom:   2,
				ChromaLog2WeightDenom: 3,

				LumaWeightL0Flag:   true,
				LumaWeightL0:       []int{-1, -1},
				LumaOffsetL0:       []int{1, 2},
				ChromaWeightL0Flag: true,
				ChromaWeightL0:     [][]int{{1, 1}, {-1, -1}},
				ChromaOffsetL0:     [][]int{{1, 1}, {-2, -1}},

				LumaWeightL1Flag:   true,
				LumaWeightL1:       []int{-1, -1},
				LumaOffsetL1:       []int{1, 2},
				ChromaWeightL1Flag: true,
				ChromaWeightL1:     [][]int{{1, 1}, {-1, -1}},
				ChromaOffsetL1:     [][]int{{1, 1}, {-2, -1}},
			},
		},
	}

	for i, test := range tests {
		inBytes, err := binToSlice(test.in)
		if err != nil {
			t.Fatalf("unexpected error %v for binToSlice in test %d", err, i)
		}

		got, err := NewPredWeightTable(bits.NewBitReader(bytes.NewReader(inBytes)),
			&test.sliceHeader, test.chromaArrayType)
		if err != nil {
			t.Fatalf("unexpected error %v for NewPredWeightTable in test %d", err, i)
		}

		if !reflect.DeepEqual(*got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

func TestDecRefPicMarking(t *testing.T) {
	tests := []struct {
		in     string
		idrPic bool
		want   DecRefPicMarking
	}{
		{
			in: "0" + // u(1) no_output_of_prior_pics_flag = false
				"1", // u(1) long_term_reference_flag = true
			idrPic: true,
			want: DecRefPicMarking{
				NoOutputOfPriorPicsFlag: false,
				LongTermReferenceFlag:   true,
			},
		},
		{
			in: "1" + // u(1) adaptive_ref_pic_marking_mode_flag = true

				"010" + // ue(v) memory_management_control_operation = 1
				"011" + // ue(v) difference_of_pic_nums_minus1 = 2

				"00100" + // ue(v) memory_management_control_operation = 3
				"1" + // ue(v) difference_of_pic_nums_minus1 = 0
				"011" + // ue(v) long_term_frame_idx = 2

				"011" + // ue(v) memory_management_control_operation = 2
				"00100" + // ue(v) long_term_pic_num = 3

				"00101" + // ue(v) memory_management_control_operation = 4
				"010" + // ue(v) max_long_term_frame_idx_plus1 = 1

				"1", // ue(v) memory_management_control_operation = 0

			idrPic: false,

			want: DecRefPicMarking{
				AdaptiveRefPicMarkingModeFlag: true,
				elements: []drpmElement{
					{
						MemoryManagementControlOperation: 1,
						DifferenceOfPicNumsMinus1:        2,
					},
					{
						MemoryManagementControlOperation: 3,
						DifferenceOfPicNumsMinus1:        0,
						LongTermFrameIdx:                 2,
					},
					{
						MemoryManagementControlOperation: 2,
						LongTermPicNum:                   3,
					},
					{
						MemoryManagementControlOperation: 4,
						MaxLongTermFrameIdxPlus1:         1,
					},
					{
						MemoryManagementControlOperation: 0,
					},
				},
			},
		},
	}

	for i, test := range tests {
		inBytes, err := binToSlice(test.in)
		if err != nil {
			t.Fatalf("unexpected error %v for binToSlice in test %d", err, i)
		}

		got, err := NewDecRefPicMarking(bits.NewBitReader(bytes.NewReader(inBytes)), test.idrPic)
		if err != nil {
			t.Fatalf("unexpected error %v for NewPredWeightTable in test %d", err, i)
		}

		if !reflect.DeepEqual(*got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

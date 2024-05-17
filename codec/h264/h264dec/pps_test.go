/*
DESCRIPTION
  pps_test.go provides testing for parsing functionality found in pps.go.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>, The Australian Ocean Laboratory (AusOcean)
*/

package h264dec

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
)

func TestNewPPS(t *testing.T) {
	// TODO: add test with scaling list once we have a test for scalingList func.
	tests := []struct {
		in           string
		chromaFormat int
		want         PPS
	}{
		{
			in: "1" + // ue(v) pic_parameter_set_id = 0
				"1" + // ue(v) seq_parameter_set_id = 0
				"1" + // u(1) entropy_coding_mode_flag = 1
				"0" + // u(1) pic_order_present_flag = 0
				"1" + // ue(v) num_slice_groups_minus1 = 0
				"1" + // ue(v) num_ref_idx_L0_active_minus1 = 0
				"1" + // ue(v) num_ref_idx_L1_active_minus1 = 0
				"1" + // u(1) weighted_pred_flag = 1
				"00" + // u(2) weighted_bipred_idc = 0
				"1" + // se(v) pic_init_qp_minus26 =  0
				"1" + // se(v) pic_init_qs_minus26 =  0
				"1" + // se(v) chroma_qp_index_offset =  0
				"1" + // u(1) deblocking_filter_control_present_flag =  1
				"0" + // u(1) constrained_intra_pred_flag =  0
				"0" + // u(1) redundant_pic_cnt_present_flag =  0
				"10000000", // rbspTrailingBits
			want: PPS{
				ID:                                0,
				SPSID:                             0,
				EntropyCodingMode:                 1,
				BottomFieldPicOrderInFramePresent: false,
				NumSliceGroupsMinus1:              0,
				NumRefIdxL0DefaultActiveMinus1:    0,
				NumRefIdxL1DefaultActiveMinus1:    0,
				WeightedPred:                      true,
				WeightedBipred:                    0,
				PicInitQpMinus26:                  0,
				PicInitQsMinus26:                  0,
				ChromaQpIndexOffset:               0,
				DeblockingFilterControlPresent:    true,
				ConstrainedIntraPred:              false,
				RedundantPicCntPresent:            false,
			},
		},
		{
			in: "1" + // ue(v) pic_parameter_set_id = 0
				"1" + // ue(v) seq_parameter_set_id = 0
				"1" + // u(1) entropy_coding_mode_flag = 1
				"1" + // u(1) bottom_field_pic_order_in_frame_present_flag = 1
				"010" + // ue(v) num_slice_groups_minus1 = 1
				"1" + // ue(v) slice_group_map_type = 0
				"1" + // ue(v) run_length_minus1[0] = 0
				"1" + // ue(v) run_length_minus1[1] = 0
				"1" + // ue(v) num_ref_idx_L0_active_minus1 = 0
				"1" + // ue(v) num_ref_idx_L1_active_minus1 = 0
				"1" + // u(1) weighted_pred_flag = 0
				"00" + // u(2) weighted_bipred_idc = 0
				"011" + // se(v) pic_init_qp_minus26 = -1
				"010" + // se(v) pic_init_qs_minus26 = 1
				"00100" + // se(v) chroma_qp_index_offset = 2
				"0" + // u(1) deblocking_filter_control_present_flag =0
				"0" + // u(1) constrained_intra_pred_flag=0
				"0" + // u(1) redundant_pic_cnt_present_flag=0
				"0" + // u(1) transform_8x8_mode_flag=0
				"0" + // u(1) pic_scaling_matrix_present_flag=0
				"00100" + // se(v) second_chroma_qp_index_offset=2
				"10000", // stop bit and trailing bits
			want: PPS{
				ID:                                0,
				SPSID:                             0,
				EntropyCodingMode:                 1,
				BottomFieldPicOrderInFramePresent: true,
				NumSliceGroupsMinus1:              1,
				RunLengthMinus1:                   []int{0, 0},
				NumRefIdxL0DefaultActiveMinus1:    0,
				NumRefIdxL1DefaultActiveMinus1:    0,
				WeightedPred:                      true,
				WeightedBipred:                    0,
				PicInitQpMinus26:                  -1,
				PicInitQsMinus26:                  1,
				ChromaQpIndexOffset:               2,
				DeblockingFilterControlPresent:    false,
				ConstrainedIntraPred:              false,
				RedundantPicCntPresent:            false,
				Transform8x8Mode:                  0,
				PicScalingMatrixPresent:           false,
				SecondChromaQpIndexOffset:         2,
			},
		},
	}

	for i, test := range tests {
		bin, err := binToSlice(test.in)
		if err != nil {
			t.Fatalf("error: %v converting binary string to slice for test: %d", err, i)
		}

		pps, err := NewPPS(bits.NewBitReader(bytes.NewReader(bin)), test.chromaFormat)
		if err != nil {
			t.Fatalf("did not expect error: %v for test: %d", err, i)
		}

		if !reflect.DeepEqual(test.want, *pps) {
			t.Errorf("did not get expected result for test: %d.\nGot: %+v\nWant: %+v\n", i, *pps, test.want)
		}
	}
}

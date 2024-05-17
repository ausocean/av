/*
DESCRIPTION
  nalunit_test.go provides testing for functionality in nalunit.go.

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
	"bytes"
	"reflect"
	"testing"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
)

func TestNewMVCExtension(t *testing.T) {
	tests := []struct {
		in   string
		want *MVCExtension
		err  error
	}{
		{
			in: "0" + // u(1) non_idr_flag = false
				"00 0010" + // u(6) priority_id = 2
				"00 0001 1000" + // u(10) view_id = 24
				"100" + // u(3) temporal_id = 4
				"1" + // u(1) anchor_pic_flag = true
				"0" + // u(1) inter_view_flag = false
				"1" + // u(1) reserved_one_bit = 1
				"0", // Some padding
			want: &MVCExtension{
				NonIdrFlag:     false,
				PriorityID:     2,
				ViewID:         24,
				TemporalID:     4,
				AnchorPicFlag:  true,
				InterViewFlag:  false,
				ReservedOneBit: 1,
			},
		},
	}

	for i, test := range tests {
		inBytes, err := binToSlice(test.in)
		if err != nil {
			t.Fatalf("did not expect error %v from binToSlice for test %d", err, i)
		}

		got, err := NewMVCExtension(bits.NewBitReader(bytes.NewReader(inBytes)))
		if err != test.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v\n", i, err, test.err)
		}

		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, *got, test.want)
		}
	}
}

func TestNewThreeDAVCExtension(t *testing.T) {
	tests := []struct {
		in   string
		want *ThreeDAVCExtension
		err  error
	}{
		{
			in: "0001 0000" + // u(8) view_idx = 16
				"1" + // u(1) depth_flag = true
				"0" + // u(1) non_idr_flag = false
				"010" + // u(1) temporal_id = 2
				"1" + // u(1) anchor_pic_flag = true
				"1" + // u(1) inter_view_flag = true
				"000", // Some padding
			want: &ThreeDAVCExtension{
				ViewIdx:       16,
				DepthFlag:     true,
				NonIdrFlag:    false,
				TemporalID:    2,
				AnchorPicFlag: true,
				InterViewFlag: true,
			},
		},
	}

	for i, test := range tests {
		inBytes, err := binToSlice(test.in)
		if err != nil {
			t.Fatalf("did not expect error %v from binToSlice for test %d", err, i)
		}

		got, err := NewThreeDAVCExtension(bits.NewBitReader(bytes.NewReader(inBytes)))
		if err != test.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v\n", i, err, test.err)
		}

		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, *got, test.want)
		}
	}
}

func TestSVCExtension(t *testing.T) {
	tests := []struct {
		in   string
		want *SVCExtension
		err  error
	}{
		{
			in: "0" + // u(1) idr_flag = false
				"10 0000" + // u(6) priority_id = 32
				"0" + // u(1) no_inter_layer_pred_flag = false
				"001" + // u(3) dependency_id = 1
				"1000" + // u(4) quality_id = 8
				"010" + // u(3) temporal_id = 2
				"1" + // u(1) use_ref_base_pic_flag = true
				"0" + // u(1) discardable_flag = false
				"0" + // u(1) output_flag = false
				"11" + // ReservedThree2Bits
				"0", // padding
			want: &SVCExtension{
				IdrFlag:              false,
				PriorityID:           32,
				NoInterLayerPredFlag: false,
				DependencyID:         1,
				QualityID:            8,
				TemporalID:           2,
				UseRefBasePicFlag:    true,
				DiscardableFlag:      false,
				OutputFlag:           false,
				ReservedThree2Bits:   3,
			},
		},
	}

	for i, test := range tests {
		inBytes, err := binToSlice(test.in)
		if err != nil {
			t.Fatalf("did not expect error %v from binToSlice for test %d", err, i)
		}

		got, err := NewSVCExtension(bits.NewBitReader(bytes.NewReader(inBytes)))
		if err != test.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v\n", i, err, test.err)
		}

		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, *got, test.want)
		}
	}
}

func TestNewNALUnit(t *testing.T) {
	tests := []struct {
		in   string
		want *NALUnit
		err  error
	}{
		{
			in: "0" + // f(1) forbidden_zero_bit = 0
				"01" + // u(2) nal_ref_idc = 1
				"0 1110" + // u(5) nal_unit_type = 14
				"1" + // u(1) svc_extension_flag = true

				// svc extension
				"0" + // u(1) idr_flag = false
				"10 0000" + // u(6) priority_id = 32
				"0" + // u(1) no_inter_layer_pred_flag = false
				"001" + // u(3) dependency_id = 1
				"1000" + // u(4) quality_id = 8
				"010" + // u(3) temporal_id = 2
				"1" + // u(1) use_ref_base_pic_flag = true
				"0" + // u(1) discardable_flag = false
				"0" + // u(1) output_flag = false
				"11" + // ReservedThree2Bits

				// rbsp bytes
				"0000 0001" +
				"0000 0010" +
				"0000 0100" +
				"0000 1000" +
				"1000 0000", // trailing bits

			want: &NALUnit{
				ForbiddenZeroBit: 0,
				RefIdc:           1,
				Type:             14,
				SVCExtensionFlag: true,
				SVCExtension: &SVCExtension{
					IdrFlag:              false,
					PriorityID:           32,
					NoInterLayerPredFlag: false,
					DependencyID:         1,
					QualityID:            8,
					TemporalID:           2,
					UseRefBasePicFlag:    true,
					DiscardableFlag:      false,
					OutputFlag:           false,
					ReservedThree2Bits:   3,
				},

				RBSP: []byte{
					0x01,
					0x02,
					0x04,
					0x08,
				},
			},
		},
	}

	for i, test := range tests {
		inBytes, err := binToSlice(test.in)
		if err != nil {
			t.Fatalf("did not expect error %v from binToSlice for test %d", err, i)
		}

		got, err := NewNALUnit(bits.NewBitReader(bytes.NewReader(inBytes)))
		if err != test.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v\n", i, err, test.err)
		}

		if !nalEqual(got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, *got, test.want)
		}
	}
}

// nalEqual returns true if two NALUnits are equal.
func nalEqual(a, b *NALUnit) bool {
	aCopy := nalWithoutExtensions(*a)
	bCopy := nalWithoutExtensions(*b)

	if !reflect.DeepEqual(aCopy, bCopy) {
		return false
	}

	if (a.SVCExtension == nil || b.SVCExtension == nil) &&
		(a.SVCExtension != b.SVCExtension) {
		return false
	}

	if (a.MVCExtension == nil || b.MVCExtension == nil) &&
		(a.MVCExtension != b.MVCExtension) {
		return false
	}

	if (a.ThreeDAVCExtension == nil || b.ThreeDAVCExtension == nil) &&
		(a.ThreeDAVCExtension != b.ThreeDAVCExtension) {
		return false
	}

	if !reflect.DeepEqual(a.SVCExtension, b.SVCExtension) ||
		!reflect.DeepEqual(a.MVCExtension, b.MVCExtension) ||
		!reflect.DeepEqual(a.ThreeDAVCExtension, b.ThreeDAVCExtension) {
		return false
	}
	return true
}

func nalWithoutExtensions(n NALUnit) NALUnit {
	n.SVCExtension = nil
	n.MVCExtension = nil
	n.ThreeDAVCExtension = nil
	return n
}

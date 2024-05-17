package h264dec

import (
	"bytes"
	"fmt"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
	"github.com/pkg/errors"
)

var (
	DefaultScalingMatrix4x4 = [][]int{
		{6, 13, 20, 28, 13, 20, 28, 32, 20, 28, 32, 37, 28, 32, 37, 42},
		{10, 14, 20, 24, 14, 20, 24, 27, 20, 24, 27, 30, 24, 27, 30, 34},
	}

	DefaultScalingMatrix8x8 = [][]int{
		{6, 10, 13, 16, 18, 23, 25, 27,
			10, 11, 16, 18, 23, 25, 27, 29,
			13, 16, 18, 23, 25, 27, 29, 31,
			16, 18, 23, 25, 27, 29, 31, 33,
			18, 23, 25, 27, 29, 31, 33, 36,
			23, 25, 27, 29, 31, 33, 36, 38,
			25, 27, 29, 31, 33, 36, 38, 40,
			27, 29, 31, 33, 36, 38, 40, 42},
		{9, 13, 15, 17, 19, 21, 22, 24,
			13, 13, 17, 19, 21, 22, 24, 25,
			15, 17, 19, 21, 22, 24, 25, 27,
			17, 19, 21, 22, 24, 25, 27, 28,
			19, 21, 22, 24, 25, 27, 28, 30,
			21, 22, 24, 25, 27, 28, 30, 32,
			22, 24, 25, 27, 28, 30, 32, 33,
			24, 25, 27, 28, 30, 32, 33, 35},
	}
	Default4x4IntraList = []int{6, 13, 13, 20, 20, 20, 38, 38, 38, 38, 32, 32, 32, 37, 37, 42}
	Default4x4InterList = []int{10, 14, 14, 20, 20, 20, 24, 24, 24, 24, 27, 27, 27, 30, 30, 34}
	Default8x8IntraList = []int{
		6, 10, 10, 13, 11, 13, 16, 16, 16, 16, 18, 18, 18, 18, 18, 23,
		23, 23, 23, 23, 23, 25, 25, 25, 25, 25, 25, 25, 27, 27, 27, 27,
		27, 27, 27, 27, 29, 29, 29, 29, 29, 29, 29, 31, 31, 31, 31, 31,
		31, 33, 33, 33, 33, 33, 36, 36, 36, 36, 38, 38, 38, 40, 40, 42}
	Default8x8InterList = []int{
		9, 13, 13, 15, 13, 15, 17, 17, 17, 17, 19, 19, 19, 19, 19, 21,
		21, 21, 21, 21, 21, 22, 22, 22, 22, 22, 22, 22, 24, 24, 24, 24,
		24, 24, 24, 24, 25, 25, 25, 25, 25, 25, 25, 27, 27, 27, 27, 27,
		27, 28, 28, 28, 28, 28, 30, 30, 30, 30, 32, 32, 32, 33, 33, 35}
	ScalingList4x4 = map[int][]int{
		0:  Default4x4IntraList,
		1:  Default4x4IntraList,
		2:  Default4x4IntraList,
		3:  Default4x4InterList,
		4:  Default4x4InterList,
		5:  Default4x4InterList,
		6:  Default8x8IntraList,
		7:  Default8x8InterList,
		8:  Default8x8IntraList,
		9:  Default8x8InterList,
		10: Default8x8IntraList,
		11: Default8x8InterList,
	}
	ScalingList8x8 = ScalingList4x4
)

// SPS describes a sequence parameter set as defined by section 7.3.2.1.1 in
// the Specifications.
// For semantics see section 7.4.2.1. Comments for fields are excerpts from
// section 7.4.2.1.
type SPS struct {
	// pofile_idx and level_idc indicate the profile and level to which the
	// coded video sequence conforms.
	Profile, LevelIDC uint8

	// The constraint_setx_flag flags specify the constraints defined in A.2 for
	// which this stream conforms.
	Constraint0 bool
	Constraint1 bool
	Constraint2 bool
	Constraint3 bool
	Constraint4 bool
	Constraint5 bool

	// seq_parameter_set_id identifies this sequence parameter set, and can then
	// be reference by the picture parameter set. The seq_parameter_set_id is
	// in the range of 0 to 30 inclusive.
	SPSID uint64

	// chroma_format_idc specifies the chroma sampling relative to the luma
	// sampling as specified in caluse 6.2. Range of chroma_format_idc is in
	// from 0 to 3 inclusive.
	ChromaFormatIDC uint64

	// separate_color_plane_flag if true specifies that the three components of
	// the 4:4:4 chroma formta are coded separately.
	SeparateColorPlaneFlag bool

	// bit_depth_luma_minus8 specifies the luma array sample bit depth and the
	// luma quantisation parameter range offset QpBdOffset_y (eq 7-3 and 7-4).
	BitDepthLumaMinus8 uint64

	// bit_depth_luma_minus8 specifies the chroma array sample bit depth and the
	// chroma quantisation parameter range offset QpBdOffset_c (eq 7-3 and 7-4).
	BitDepthChromaMinus8 uint64

	// qpprime_y_zero_transform_bypass_flag equal to 1 specifies that, when QP′ Y
	// is equal to 0, a transform bypass operation for the transform coefficient
	// decoding process and picture construction process prior to deblocking
	// filter process as specified in clause 8.5 shall be applied.
	QPPrimeYZeroTransformBypassFlag bool

	// seq_scaling_matrix_present_flag equal to 1 specifies that
	// seq_scaling_list_present_flag[ i ] are present. When 0 they are not present
	// and the sequence-level scaling lists specified by Flat_4x4_16 and
	// Flat_8x8_16 shall be inferred.
	SeqScalingMatrixPresentFlag bool

	// seq_scaling_lit_present_flag[i] specifics whether the syntax structure for
	// scaling list i is present. If 1 then present, otherwise not, and scaling
	// list for i is inferred as per rule set A in table 7-2.
	SeqScalingListPresentFlag []bool

	// The 4x4 sequence scaling lists for each i.
	ScalingList4x4 [][]uint64

	// Flag to indicate for a 4x4 scaling list, if we use the default.
	UseDefaultScalingMatrix4x4Flag []bool

	// The 8x8 sequence scaling lists for each i.
	ScalingList8x8 [][]uint64

	// Flag to indicate for a 8x8 scaling list, if we use the default.
	UseDefaultScalingMatrix8x8Flag []bool

	// log2_max_frame_num_minus4 allows for derivation of MaxFrameNum using eq 7-10.
	Log2MaxFrameNumMinus4 uint64

	// pic_order_cnt_type specifiess the method to decode picture order count.
	PicOrderCountType uint64

	// log2_max_pic_order_cnt_lsb_minus4 allows for the dreivation of
	// MaxPicOrderCntLsb using eq 7-11.
	Log2MaxPicOrderCntLSBMin4 uint64

	// delta_pic_order_always_zero_flag if true indicates delta_pic_order_cnt[0]
	// and delta_pic_order_cnt[1].
	DeltaPicOrderAlwaysZeroFlag bool

	// offset_for_non_ref_pic is used to calculate the picture order count of a
	// non-reference picture as specified in clause 8.2.1.
	OffsetForNonRefPic int64

	// offset_for_top_to_bottom_field is used to calculate the picture order count
	// of a bottom field as specified in clause 8.2.1.
	OffsetForTopToBottomField int64

	// num_ref_frames_in_pic_order_cnt_cycle is used in the decoding process for
	// picture order count as specified in clause 8.2.1.
	NumRefFramesInPicOrderCntCycle uint64

	// offset_for_ref_frame[ i ] is an element of a list of
	// num_ref_frames_in_pic_order_cnt_cycle values used in the decoding process
	// for picture order count as specified in clause 8.2.1.
	OffsetForRefFrameList []int

	// max_num_ref_frames specifies the max number of short-term and long-term
	// reference frames, complementary reference field pairs, and non-paired
	// reference fields that may be used by the decoding process for inter prediction.
	MaxNumRefFrames uint64

	// gaps_in_frame_num_value_allowed_flag specifies the allowed values of
	// frame_num as specified in clause 7.4.3 and the decoding process in case of
	// an inferred gap between values of frame_num as specified in clause 8.2.5.2.
	GapsInFrameNumValueAllowed bool

	// pic_width_in_mbs_minus1 plus 1 specifies the width of each decode picutre
	// in units of macroblocks. See eq 7-13.
	PicWidthInMBSMinus1 uint64

	// pic_height_in_map_units_minus1 plus 1 specifies the height in slice group
	// map units of a decoded frame or field. See eq 7-16.
	PicHeightInMapUnitsMinus1 uint64

	// frame_mbs_only_flag if 0 coded pictures of the coded video sequence may be
	// coded fields or coded frames. If 1 every coded picture of the coded video
	// sequence is a coded frame containing only frame macroblocks.
	FrameMBSOnlyFlag bool

	// mb_adaptive_frame_field_flag if 0 specifies no switching between
	// frame and field macroblocks within a picture. If 1 specifies the possible
	// use of switching between frame and field macroblocks within frames.
	MBAdaptiveFrameFieldFlag bool

	// direct_8x8_inference_flag specifies the method used in the derivation
	// process for luma motion vectors for B_Skip, B_Direct_16x16 and B_Direct_8x8
	// as specified in clause 8.4.1.2.
	Direct8x8InferenceFlag bool

	// frame_cropping_flag if 1 then frame cropping offset parameters are next in
	// the sequence parameter set. If 0 they are not.
	FrameCroppingFlag bool

	// frame_crop_left_offset, frame_crop_right_offset, frame_crop_top_offset,
	// frame_crop_bottom_offset specify the samples of the pictures in the coded
	// video sequence that are output from the decoding process, in terms of a
	// rectangular region specified in frame coordinates for output.
	FrameCropLeftOffset   uint64
	FrameCropRightOffset  uint64
	FrameCropTopOffset    uint64
	FrameCropBottomOffset uint64

	// vui_parameters_present_flag if 1 the vui_parameters() syntax structure is
	// present, otherwise it is not.
	VUIParametersPresentFlag bool

	// The vui_parameters() syntax structure specified in appendix E.
	VUIParameters *VUIParameters
}

// NewSPS parses a sequence parameter set raw byte sequence from br following
// the syntax structure specified in section 7.3.2.1.1, and returns as a new
// SPS.
func NewSPS(rbsp []byte, showPacket bool) (*SPS, error) {
	logger.Printf("debug: SPS RBSP %d bytes %d bits\n", len(rbsp), len(rbsp)*8)
	logger.Printf("debug: \t%#v\n", rbsp[0:8])
	sps := SPS{}
	br := bits.NewBitReader(bytes.NewReader(rbsp))
	r := newFieldReader(br)

	sps.Profile = uint8(r.readBits(8))
	sps.Constraint0 = r.readBits(1) == 1
	sps.Constraint1 = r.readBits(1) == 1
	sps.Constraint2 = r.readBits(1) == 1
	sps.Constraint3 = r.readBits(1) == 1
	sps.Constraint4 = r.readBits(1) == 1
	sps.Constraint5 = r.readBits(1) == 1
	r.readBits(2) // 2 reserved bits.
	sps.LevelIDC = uint8(r.readBits(8))
	sps.SPSID = r.readUe()
	sps.ChromaFormatIDC = r.readUe()

	// This should be done only for certain ProfileIDC:
	isProfileIDC := []int{100, 110, 122, 244, 44, 83, 86, 118, 128, 138, 139, 134, 135}
	// SpecialProfileCase1
	if isInList(isProfileIDC, int(sps.Profile)) {
		if sps.ChromaFormatIDC == chroma444 {
			// TODO: should probably deal with error here.
			sps.SeparateColorPlaneFlag = r.readBits(1) == 1
		}

		sps.BitDepthLumaMinus8 = r.readUe()
		sps.BitDepthChromaMinus8 = r.readUe()
		sps.QPPrimeYZeroTransformBypassFlag = r.readBits(1) == 1
		sps.SeqScalingMatrixPresentFlag = r.readBits(1) == 1

		if sps.SeqScalingMatrixPresentFlag {
			max := 12
			if sps.ChromaFormatIDC != chroma444 {
				max = 8
			}
			logger.Printf("debug: \tbuilding Scaling matrix for %d elements\n", max)
			for i := 0; i < max; i++ {
				sps.SeqScalingListPresentFlag = append(sps.SeqScalingListPresentFlag, r.readBits(1) == 1)

				if sps.SeqScalingListPresentFlag[i] {
					if i < 6 {
						scalingList(
							br,
							ScalingList4x4[i],
							16,
							DefaultScalingMatrix4x4[i])
						// 4x4: Page 75 bottom
					} else {
						// 8x8 Page 76 top
						scalingList(
							br,
							ScalingList8x8[i],
							64,
							DefaultScalingMatrix8x8[i-6])
					}
				}
			}
		}
	} // End SpecialProfileCase1

	// showSPS()
	// return sps
	// Possibly wrong due to no scaling list being built
	sps.Log2MaxFrameNumMinus4 = r.readUe()
	sps.PicOrderCountType = r.readUe()

	if sps.PicOrderCountType == 0 {
		sps.Log2MaxPicOrderCntLSBMin4 = r.readUe()
	} else if sps.PicOrderCountType == 1 {
		sps.DeltaPicOrderAlwaysZeroFlag = r.readBits(1) == 1
		sps.OffsetForNonRefPic = int64(r.readSe())
		sps.OffsetForTopToBottomField = int64(r.readSe())
		sps.NumRefFramesInPicOrderCntCycle = r.readUe()

		for i := 0; i < int(sps.NumRefFramesInPicOrderCntCycle); i++ {
			sps.OffsetForRefFrameList = append(sps.OffsetForRefFrameList, r.readSe())
		}

	}

	sps.MaxNumRefFrames = r.readUe()
	sps.GapsInFrameNumValueAllowed = r.readBits(1) == 1
	sps.PicWidthInMBSMinus1 = r.readUe()
	sps.PicHeightInMapUnitsMinus1 = r.readUe()
	sps.FrameMBSOnlyFlag = r.readBits(1) == 1

	if !sps.FrameMBSOnlyFlag {
		sps.MBAdaptiveFrameFieldFlag = r.readBits(1) == 1
	}

	sps.Direct8x8InferenceFlag = r.readBits(1) == 1
	sps.FrameCroppingFlag = r.readBits(1) == 1

	if sps.FrameCroppingFlag {
		sps.FrameCropLeftOffset = r.readUe()
		sps.FrameCropRightOffset = r.readUe()
		sps.FrameCropTopOffset = r.readUe()
		sps.FrameCropBottomOffset = r.readUe()
	}

	sps.VUIParametersPresentFlag = r.readBits(1) == 1

	if sps.VUIParametersPresentFlag {

	} // End VuiParameters Annex E.1.1

	return &sps, nil
}

// SPS describes a sequence parameter set as defined by section E.1.1 in the
// Specifications.
// Semantics for fields are define in section E.2.1. Comments on fields are
// excerpts from the this section.
type VUIParameters struct {
	// aspect_ratio_info_present_flag if 1 then aspect_ratio_idc is present,
	// otherwsise is not.
	AspectRatioInfoPresentFlag bool

	// aspect_ratio_idc specifies the value of sample aspect ratio of the luma samples.
	AspectRatioIDC uint8

	// sar_width indicates the horizontal size of the sample aspect ratio (in
	// arbitrary units).
	SARWidth uint32

	// sar_height indicates the vertical size of the sample aspect ratio (in the
	// same arbitrary units as sar_width).
	SARHeight uint32

	// overscan_info_present_flag if 1 then overscan_appropriate_flag is present,
	// otherwise if 0, then the display method for the video signal is unspecified.
	OverscanInfoPresentFlag bool

	// overscan_appropriate_flag if 1 then the cropped decoded pictures output
	// are suitable for display using overscan, othersise if 0, then the cropped
	// decoded pictures output should not be displayed using overscan.
	OverscanAppropriateFlag bool

	// video_signal_type_present_flag equal to 1 specifies that video_format,
	// video_full_range_flag and colour_description_present_flag are present,
	// otherwise if 0, then they are not present.
	VideoSignalTypePresentFlag bool

	// video_format indicates the representation of the pictures as specified in
	// Table E-2, before being coded in accordance with this Recommendation |
	// International Standard.
	VideoFormat uint8

	// video_full_range_flag indicates the black level and range of the luma and
	// chroma signals as derived from E′_Y, E′_PB, and E′_PR or E′_R, E′_G,
	// and E′_B real-valued component signals.
	VideoFullRangeFlag bool

	// colour_description_present_flag if 1 specifies that colour_primaries,
	// transfer_characteristics and matrix_coefficients are present, otherwise if
	// 0 then they are not present.
	ColorDescriptionPresentFlag bool

	// colour_primaries indicates the chromaticity coordinates of the source
	// primaries as specified in Table E-3 in terms of the CIE 1931 definition of
	// x and y as specified by ISO 11664-1.
	ColorPrimaries uint8

	// transfer_characteristics either indicates the reference opto-electronic
	// transfer characteristic function of the source picture, or indicates the
	// inverse of the reference electro-optical transfer characteristic function.
	TransferCharacteristics uint8

	// matrix_coefficients describes the matrix coefficients used in deriving luma
	// and chroma signals from the green, blue, and red, or Y, Z, and X primaries,
	// as specified in Table E-5.
	MatrixCoefficients uint8

	// chroma_loc_info_present_flag if 1 specifies that chroma_sample_loc_type_top_field
	// and chroma_sample_loc_type_bottom_field are present, otherwise if 0,
	// they are not present.
	ChromaLocInfoPresentFlag bool

	// chroma_sample_loc_type_top_field and chroma_sample_loc_type_bottom_field
	// specify the location of chroma samples.
	ChromaSampleLocTypeTopField, ChromaSampleLocTypeBottomField uint64

	// timing_info_present_flag if 1 specifies that num_units_in_tick, time_scale
	// and fixed_frame_rate_flag are present in the bitstream, otherwise if 0,
	// they are not present.
	TimingInfoPresentFlag bool

	// num_units_in_tick is the number of time units of a clock operating at the
	// frequency time_scale Hz that corresponds to one increment (called a clock
	// tick) of a clock tick counter.
	NumUnitsInTick uint32

	// time_scale is the number of time units that pass in one second.
	TimeScale uint32

	// fixed_frame_rate_flag if 1 indicates that the temporal distance
	// between the HRD output times of any two consecutive pictures in output
	// order is constrained as follows. fixed_frame_rate_flag equal to 0 indicates
	// that no such constraints apply to the temporal distance between the HRD
	// output times of any two consecutive pictures in output order.
	FixedFrameRateFlag bool

	// nal_hrd_parameters_present_flag if 1 then NAL HRD parameters (pertaining to
	// Type II bitstream conformance) are present, otherwise if 0, then they
	// are not present.
	NALHRDParametersPresentFlag bool

	// The nal_hrd_parameters() syntax structure as specified in section E.1.2.
	NALHRDParameters *HRDParameters

	// vcl_hrd_parameters_present_flag if 1 specifies that VCL HRD parameters
	// (pertaining to all bitstream conformance) are present, otherwise if 0, then
	// they are not present.
	VCLHRDParametersPresentFlag bool

	// The vcl_nal_hrd_parameters() syntax structure as specified in section E.1.2.
	VCLHRDParameters *HRDParameters

	// low_delay_hrd_flag specifies the HRD operational mode as specified in Annex C.
	LowDelayHRDFlag bool

	// pic_struct_present_flag if 1 then picture timing SEI messages (clause D.2.3)
	// are present that include the pic_struct syntax element, otherwise if 0, then
	// not present.
	PicStructPresentFlag bool

	// bitstream_restriction_flag if 1, then the following coded video sequence
	// bitstream restriction parameters are present, otherwise if 0, then they are
	// not present.
	BitstreamRestrictionFlag bool

	// motion_vectors_over_pic_boundaries_flag if 0 then no sample outside the
	// picture boundaries and no sample at a fractional sample position for which
	// the sample value is derived using one or more samples outside the picture
	// boundaries is used for inter prediction of any sample, otherwise if 1,
	// indicates that one or more samples outside picture boundaries may be used
	// in inter prediction.
	MotionVectorsOverPicBoundariesFlag bool

	// max_bytes_per_pic_denom indicates a number of bytes not exceeded by the sum
	// of the sizes of the VCL NAL units associated with any coded picture in the
	// coded video sequence.
	MaxBytesPerPicDenom uint64

	// max_bits_per_mb_denom indicates an upper bound for the number of coded bits
	// of macroblock_layer() data for any macroblock in any picture of the coded
	// video sequence.
	MaxBitsPerMBDenom uint64

	// log2_max_mv_length_horizontal and log2_max_mv_length_vertical indicate the
	// maximum absolute value of a decoded horizontal and vertical motion vector
	// component, respectively, in 1⁄4 luma sample units, for all pictures in the
	// coded video sequence.
	Log2MaxMVLengthHorizontal, Log2MaxMVLengthVertical uint64

	// max_num_reorder_frames indicates an upper bound for the number of frames
	// buffers, in the decoded picture buffer (DPB), that are required for storing
	// frames, complementary field pairs, and non-paired fields before output.
	MaxNumReorderFrames uint64

	// max_dec_frame_buffering specifies the required size of the HRD decoded
	// picture buffer (DPB) in units of frame buffers.
	MaxDecFrameBuffering uint64
}

// NewVUIParameters parses video usability information parameters from br
// following the syntax structure specified in section E.1.1, and returns as a
// new VUIParameters.
func NewVUIParameters(br *bits.BitReader) (*VUIParameters, error) {
	p := &VUIParameters{}
	r := newFieldReader(br)

	p.AspectRatioInfoPresentFlag = r.readBits(1) == 1

	if p.AspectRatioInfoPresentFlag {
		p.AspectRatioIDC = uint8(r.readBits(8))

		EXTENDED_SAR := 999
		if int(p.AspectRatioIDC) == EXTENDED_SAR {
			p.SARWidth = uint32(r.readBits(16))
			p.SARHeight = uint32(r.readBits(16))
		}
	}

	p.OverscanInfoPresentFlag = r.readBits(1) == 1

	if p.OverscanInfoPresentFlag {
		p.OverscanAppropriateFlag = r.readBits(1) == 1
	}

	p.VideoSignalTypePresentFlag = r.readBits(1) == 1

	if p.VideoSignalTypePresentFlag {
		p.VideoFormat = uint8(r.readBits(3))
	}

	if p.VideoSignalTypePresentFlag {
		p.VideoFullRangeFlag = r.readBits(1) == 1
		p.ColorDescriptionPresentFlag = r.readBits(1) == 1

		if p.ColorDescriptionPresentFlag {
			p.ColorPrimaries = uint8(r.readBits(8))
			p.TransferCharacteristics = uint8(r.readBits(8))
			p.MatrixCoefficients = uint8(r.readBits(8))
		}
	}
	p.ChromaLocInfoPresentFlag = r.readBits(1) == 1

	if p.ChromaLocInfoPresentFlag {
		p.ChromaSampleLocTypeTopField = uint64(r.readUe())
		p.ChromaSampleLocTypeBottomField = uint64(r.readUe())
	}

	p.TimingInfoPresentFlag = r.readBits(1) == 1

	if p.TimingInfoPresentFlag {
		p.NumUnitsInTick = uint32(r.readBits(32))
		p.TimeScale = uint32(r.readBits(32))
		p.FixedFrameRateFlag = r.readBits(1) == 1
	}

	p.NALHRDParametersPresentFlag = r.readBits(1) == 1

	var err error
	if p.NALHRDParametersPresentFlag {
		p.NALHRDParameters, err = NewHRDParameters(br)
		if err != nil {
			return nil, errors.Wrap(err, "could not get hrdParameters")
		}
	}

	p.VCLHRDParametersPresentFlag = r.readBits(1) == 1

	if p.VCLHRDParametersPresentFlag {
		p.VCLHRDParameters, err = NewHRDParameters(br)
		if err != nil {
			return nil, errors.Wrap(err, "could not get hrdParameters")
		}
	}
	if p.NALHRDParametersPresentFlag || p.VCLHRDParametersPresentFlag {
		p.LowDelayHRDFlag = r.readBits(1) == 1
	}

	p.PicStructPresentFlag = r.readBits(1) == 1
	p.BitstreamRestrictionFlag = r.readBits(1) == 1

	if p.BitstreamRestrictionFlag {
		p.MotionVectorsOverPicBoundariesFlag = r.readBits(1) == 1
		p.MaxBytesPerPicDenom = r.readUe()
		p.MaxBitsPerMBDenom = r.readUe()
		p.Log2MaxMVLengthHorizontal = r.readUe()
		p.Log2MaxMVLengthVertical = r.readUe()
		p.MaxNumReorderFrames = r.readUe()
		p.MaxDecFrameBuffering = r.readUe()
	}
	return p, nil
}

// HRDParameters describes hypothetical reference decoder parameters as defined
// by section E.1.2 in the specifications.
// Field semantics are defined in section E.2.2. Comments on fields are excerpts
// from section E.2.2.
type HRDParameters struct {
	// cpb_cnt_minus1 plus 1 specifies the number of alternative CPB specifications
	// in the bitstream.
	CPBCntMinus1 uint64

	// bit_rate_scale (together with bit_rate_value_minus1[ SchedSelIdx ])
	// specifies the maximum input bit rate of the SchedSelIdx-th CPB.
	BitRateScale uint8

	// cpb_size_scale (together with cpb_size_value_minus1[ SchedSelIdx ])
	// specifies the CPB size of the SchedSelIdx-th CPB.
	CPBSizeScale uint8

	// bit_rate_value_minus1[ SchedSelIdx ] (together with bit_rate_scale)
	//specifies the maximum input bit rate for the SchedSelIdx-th CPB.
	BitRateValueMinus1 []uint64

	// cpb_size_value_minus1[ SchedSelIdx ] is used together with cpb_size_scale
	// to specify the SchedSelIdx-th CPB size.
	CPBSizeValueMinus1 []uint64

	// cbr_flag[ SchedSelIdx ] equal to 0 specifies that to decode this bitstream
	// by the HRD using the SchedSelIdx-th CPB specification, the hypothetical
	// stream delivery scheduler (HSS) operates in an intermittent bit rate mode,
	// otherwise if 1 specifies that the HSS operates in a constant bit rate mode.
	CBRFlag []bool

	// initial_cpb_removal_delay_length_minus1 specifies the length in bits of the
	// initial_cpb_removal_delay[ SchedSelIdx ] and
	// initial_cpb_removal_delay_offset[ SchedSelIdx ] syntax elements of the
	// buffering period SEI message.
	InitialCPBRemovalDelayLenMinus1 uint8

	// cpb_removal_delay_length_minus1 specifies the length in bits of the
	// cpb_removal_delay syntax element.
	CPBRemovalDelayLenMinus1 uint8

	// dpb_output_delay_length_minus1 specifies the length in bits of the
	// dpb_output_delay syntax element.
	DPBOutputDelayLenMinus1 uint8

	// time_offset_length greater than 0 specifies the length in bits of the
	// time_offset syntax element.
	TimeOffsetLen uint8
}

// NewHRDParameters parses hypothetical reference decoder parameter from br
// following the syntax structure specified in section E.1.2, and returns as a
// new HRDParameters.
func NewHRDParameters(br *bits.BitReader) (*HRDParameters, error) {
	h := &HRDParameters{}
	r := newFieldReader(br)

	h.CPBCntMinus1 = r.readUe()
	h.BitRateScale = uint8(r.readBits(4))
	h.CPBSizeScale = uint8(r.readBits(4))

	// SchedSelIdx E1.2
	for sseli := 0; sseli <= int(h.CPBCntMinus1); sseli++ {
		h.BitRateValueMinus1 = append(h.BitRateValueMinus1, r.readUe())
		h.CPBSizeValueMinus1 = append(h.CPBSizeValueMinus1, r.readUe())

		if v, _ := br.ReadBits(1); v == 1 {
			h.CBRFlag = append(h.CBRFlag, true)
		} else {
			h.CBRFlag = append(h.CBRFlag, false)
		}

		h.InitialCPBRemovalDelayLenMinus1 = uint8(r.readBits(5))
		h.CPBRemovalDelayLenMinus1 = uint8(r.readBits(5))
		h.DPBOutputDelayLenMinus1 = uint8(r.readBits(5))
		h.TimeOffsetLen = uint8(r.readBits(5))
	}

	if r.err() != nil {
		return nil, fmt.Errorf("error from fieldReader: %v", r.err())
	}
	return h, nil
}

func isInList(l []int, term int) bool {
	for _, m := range l {
		if m == term {
			return true
		}
	}
	return false
}

func scalingList(br *bits.BitReader, scalingList []int, sizeOfScalingList int, defaultScalingMatrix []int) error {
	lastScale := 8
	nextScale := 8
	for i := 0; i < sizeOfScalingList; i++ {
		if nextScale != 0 {
			deltaScale, err := readSe(br)
			if err != nil {
				return errors.Wrap(err, "could not parse deltaScale")
			}
			nextScale = (lastScale + deltaScale + 256) % 256
			if i == 0 && nextScale == 0 {
				// Scaling list should use the default list for this point in the matrix
				_ = defaultScalingMatrix
			}
		}
		if nextScale == 0 {
			scalingList[i] = lastScale
		} else {
			scalingList[i] = nextScale
		}
		lastScale = scalingList[i]
	}
	return nil
}

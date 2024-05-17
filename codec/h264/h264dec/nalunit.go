/*
DESCRIPTION
  nalunit.go provides structures for a NAL unit as well as it's extensions.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>, The Australian Ocean Laboratory (AusOcean)
  mrmod <mcmoranbjr@gmail.com>
*/

package h264dec

import (
	"fmt"
	"io"

	"github.com/pkg/errors"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
)

// MVCExtension describes a NAL unit header multiview video coding extension, as
// defined in section H.7.3.1.1 of the specifications.
// Semantics of fields are specified in section H.7.4.1.1.
type MVCExtension struct {
	// non_idr_flag, if true indicates that access unit is not IDR.
	NonIdrFlag bool

	// priority_id, indicates priority of NAL unit. A lower value => higher priority.
	PriorityID uint8

	// view_id, specifies a view identifier for the unit. Units with identical
	// view_id are in the same view.
	ViewID uint32

	// temporal_id, temporal identifier for the unit.
	TemporalID uint8

	// anchor_pic_flag, if true access unit is an anchor access unit.
	AnchorPicFlag bool

	// inter_view_flag, if false current view component not used for inter-view
	// prediction elsewhere in access unit.
	InterViewFlag bool

	// reserved_one_bit, always 1 (ignored by decoders)
	ReservedOneBit uint8
}

// NewMVCExtension parses a NAL unit header multiview video coding extension
// from br following the syntax structure specified in section H.7.3.1.1, and
// returns as a new MVCExtension.
func NewMVCExtension(br *bits.BitReader) (*MVCExtension, error) {
	e := &MVCExtension{}
	r := newFieldReader(br)

	e.NonIdrFlag = r.readBits(1) == 1
	e.PriorityID = uint8(r.readBits(6))
	e.ViewID = uint32(r.readBits(10))
	e.TemporalID = uint8(r.readBits(3))
	e.AnchorPicFlag = r.readBits(1) == 1
	e.InterViewFlag = r.readBits(1) == 1
	e.ReservedOneBit = uint8(r.readBits(1))

	if r.err() != nil {
		return nil, fmt.Errorf("error from fieldReader: %v", r.err())
	}
	return e, nil
}

// ThreeDAVCExtension describes a NAL unit header 3D advanced video coding
// extension, as defined in section J.7.3.1.1 of the specifications.
// For field semantics see section J.7.4.1.1.
type ThreeDAVCExtension struct {
	// view_idx, specifies the order index for the NAL i.e. view_id = view_id[view_idx].
	ViewIdx uint8

	// dpeth_flag, if true indicates NAL part of a depth view component, otherwise
	// a texture view component.
	DepthFlag bool

	// non_idr_flag, if true indicates that access unit is not IDR.
	NonIdrFlag bool

	// temporal_id, temporal identifier for the unit.
	TemporalID uint8

	// anchor_pic_flag, if true access unit is an anchor access unit.
	AnchorPicFlag bool

	// inter_view_flag, if false current view component not used for inter-view
	// prediction elsewhere in access unit.
	InterViewFlag bool
}

// NewThreeDAVCExtension parses a NAL unit header 3D advanced video coding
// extension from br following the syntax structure specified in section
// J.7.3.1.1, and returns as a new ThreeDAVCExtension.
func NewThreeDAVCExtension(br *bits.BitReader) (*ThreeDAVCExtension, error) {
	e := &ThreeDAVCExtension{}
	r := newFieldReader(br)

	e.ViewIdx = uint8(r.readBits(8))
	e.DepthFlag = r.readBits(1) == 1
	e.NonIdrFlag = r.readBits(1) == 1
	e.TemporalID = uint8(r.readBits(3))
	e.AnchorPicFlag = r.readBits(1) == 1
	e.InterViewFlag = r.readBits(1) == 1

	if r.err() != nil {
		return nil, fmt.Errorf("error from fieldReader: %v", r.err())
	}

	return e, nil
}

// SVCExtension describes a NAL unit header scalable video coding extension, as
// defined in section G.7.3.1.1 of the specifications.
// For field semantics see section G.7.4.1.1.
type SVCExtension struct {
	// idr_flag, if true the current coded picture is an IDR picture when
	// dependency_id == max(dependency_id) in the coded picture.
	IdrFlag bool

	// priority_id, specifies priority identifier for unit.
	PriorityID uint8

	// no_inter_layer_pred_flag, if true inter-layer prediction can't be used for
	// decoding slice.
	NoInterLayerPredFlag bool

	// dependency_id, specifies a dependency identifier for the NAL.
	DependencyID uint8

	// quality_id, specifies a quality identifier for the NAL.
	QualityID uint8

	// temporal_id, specifiesa temporal identifier for the NAL.
	TemporalID uint8

	// use_ref_base_pic_flag, if true indicates reference base pictures and
	// decoded pictures are used as references for inter prediction.
	UseRefBasePicFlag bool

	// discardable_flag, if true, indicates current NAL is not used for decoding
	// dependency representations that are part of the current coded picture or
	// any subsequent coded picture in decoding order and have a greater
	// dependency_id value than current NAL.
	DiscardableFlag bool

	// output_flag, affects the decoded picture output and removal processes as
	// specified in Annex C.
	OutputFlag bool

	// reserved_three_2bits, equal to 3. Decoders ignore.
	ReservedThree2Bits uint8
}

// NewSVCExtension parses a NAL unit header scalable video coding extension from
// br following the syntax structure specified in section G.7.3.1.1, and returns
// as a new SVCExtension.
func NewSVCExtension(br *bits.BitReader) (*SVCExtension, error) {
	e := &SVCExtension{}
	r := newFieldReader(br)

	e.IdrFlag = r.readBits(1) == 1
	e.PriorityID = uint8(r.readBits(6))
	e.NoInterLayerPredFlag = r.readBits(1) == 1
	e.DependencyID = uint8(r.readBits(3))
	e.QualityID = uint8(r.readBits(4))
	e.TemporalID = uint8(r.readBits(3))
	e.UseRefBasePicFlag = r.readBits(1) == 1
	e.DiscardableFlag = r.readBits(1) == 1
	e.OutputFlag = r.readBits(1) == 1
	e.ReservedThree2Bits = uint8(r.readBits(2))

	if r.err() != nil {
		return nil, fmt.Errorf("error from fieldReader: %v", r.err())
	}
	return e, nil
}

// NALUnit describes a network abstraction layer unit, as defined in section
// 7.3.1 of the specifications.
// Field semantics are defined in section 7.4.1.
type NALUnit struct {
	// forbidden_zero_bit, always 0.
	ForbiddenZeroBit uint8

	// nal_ref_idc, if not 0 indicates content of NAL contains a sequence parameter
	// set, a sequence parameter set extension, a subset sequence parameter set,
	// a picture parameter set, a slice of a reference picture, a slice data
	// partition of a reference picture, or a prefix NAL preceding a slice of
	// a reference picture.
	RefIdc uint8

	// nal_unit_type, specifies the type of RBSP data contained in the NAL as
	// defined in Table 7-1.
	Type uint8

	// svc_extension_flag, indicates whether a nal_unit_header_svc_extension()
	// (G.7.3.1.1) or nal_unit_header_mvc_extension() (H.7.3.1.1) will follow next
	// in the syntax structure.
	SVCExtensionFlag bool

	// avc_3d_extension_flag, for nal_unit_type = 21, indicates that a
	// nal_unit_header_mvc_extension() (H.7.3.1.1) or
	// nal_unit_header_3davc_extension() (J.7.3.1.1) will follow next in syntax
	// structure.
	AVC3DExtensionFlag bool

	// nal_unit_header_svc_extension() as defined in section G.7.3.1.1.
	SVCExtension *SVCExtension

	// nal_unit_header_3davc_extension() as defined in section J.7.3.1.1
	ThreeDAVCExtension *ThreeDAVCExtension

	// nal_unit_header_mvc_extension() as defined in section H.7.3.1.1).
	MVCExtension *MVCExtension

	// emulation_prevention_three_byte, equal to 0x03 and is discarded by decoder.
	EmulationPreventionThreeByte byte

	// rbsp_byte, the raw byte sequence payload data for the NAL.
	RBSP []byte
}

// NewNALUnit parses a network abstraction layer unit from br following the
// syntax structure specified in section 7.3.1, and returns as a new NALUnit.
func NewNALUnit(br *bits.BitReader) (*NALUnit, error) {
	n := &NALUnit{}
	r := newFieldReader(br)

	n.ForbiddenZeroBit = uint8(r.readBits(1))
	n.RefIdc = uint8(r.readBits(2))
	n.Type = uint8(r.readBits(5))

	var err error
	if n.Type == naluTypePrefixNALU || n.Type == naluTypeSliceLayerExtRBSP || n.Type == naluTypeSliceLayerExtRBSP2 {
		if n.Type != naluTypeSliceLayerExtRBSP2 {
			n.SVCExtensionFlag = r.readBits(1) == 1
		} else {
			n.AVC3DExtensionFlag = r.readBits(1) == 1
		}
		if n.SVCExtensionFlag {
			n.SVCExtension, err = NewSVCExtension(br)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse SVCExtension")
			}
		} else if n.AVC3DExtensionFlag {
			n.ThreeDAVCExtension, err = NewThreeDAVCExtension(br)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse ThreeDAVCExtension")
			}
		} else {
			n.MVCExtension, err = NewMVCExtension(br)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse MVCExtension")
			}
		}
	}

	for moreRBSPData(br) {
		next3Bytes, err := br.PeekBits(24)

		// If PeekBits cannot get 3 bytes, but there still might be 2 bytes left in
		// the source, we will get an io.ErrUnexpectedEOF; we wish to ignore this
		// and continue. The call to moreRBSPData will determine when we have
		// reached the end of the NAL unit.
		if err != nil && errors.Cause(err) != io.ErrUnexpectedEOF {
			return nil, errors.Wrap(err, "could not Peek next 3 bytes")
		}

		if next3Bytes == 0x000003 {
			for j := 0; j < 2; j++ {
				rbspByte := byte(r.readBits(8))
				n.RBSP = append(n.RBSP, byte(rbspByte))
			}

			// Read Emulation prevention three byte.
			n.EmulationPreventionThreeByte = byte(r.readBits(8))
		} else {
			n.RBSP = append(n.RBSP, byte(r.readBits(8)))
		}
	}

	if r.err() != nil {
		return nil, fmt.Errorf("fieldReader error: %v", r.err())
	}

	return n, nil
}

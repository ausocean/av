package h264dec

import (
	"fmt"
	"io"
	"os"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
	"github.com/pkg/errors"
)

type H264Reader struct {
	IsStarted    bool
	Stream       io.Reader
	NalUnits     []*bits.BitReader
	VideoStreams []*VideoStream
	DebugFile    *os.File
	bytes        []byte
	byteOffset   int
	*bits.BitReader
}

func (h *H264Reader) BufferToReader(cntBytes int) error {
	buf := make([]byte, cntBytes)
	if _, err := h.Stream.Read(buf); err != nil {
		logger.Printf("error: while reading %d bytes: %v\n", cntBytes, err)
		return err
	}
	h.bytes = append(h.bytes, buf...)
	if h.DebugFile != nil {
		h.DebugFile.Write(buf)
	}
	h.byteOffset += cntBytes
	return nil
}

func (h *H264Reader) Discard(cntBytes int) error {
	buf := make([]byte, cntBytes)
	if _, err := h.Stream.Read(buf); err != nil {
		logger.Printf("error: while discarding %d bytes: %v\n", cntBytes, err)
		return err
	}
	h.byteOffset += cntBytes
	return nil
}

// TODO: what does this do ?
func bitVal(bits []int) int {
	t := 0
	for i, b := range bits {
		if b == 1 {
			t += 1 << uint((len(bits)-1)-i)
		}
	}
	// fmt.Printf("\t bitVal: %d\n", t)
	return t
}

func (h *H264Reader) Start() {
	for {
		// TODO: need to handle error from this.
		nalUnit, _, _ := h.readNalUnit()
		switch nalUnit.Type {
		case NALTypeSPS:
			// TODO: handle this error
			sps, _ := NewSPS(nalUnit.RBSP, false)
			h.VideoStreams = append(
				h.VideoStreams,
				&VideoStream{SPS: sps},
			)
		case NALTypePPS:
			videoStream := h.VideoStreams[len(h.VideoStreams)-1]
			// TODO: handle this error
			// TODO: fix chromaFormat
			videoStream.PPS, _ = NewPPS(nil, 0)
		case NALTypeIDR:
			fallthrough
		case NALTypeNonIDR:
			videoStream := h.VideoStreams[len(h.VideoStreams)-1]
			logger.Printf("info: frame number %d\n", len(videoStream.Slices))
			// TODO: handle this error
			sliceContext, _ := NewSliceContext(videoStream, nalUnit, nalUnit.RBSP, true)
			videoStream.Slices = append(videoStream.Slices, sliceContext)
		}
	}
}

func (r *H264Reader) readNalUnit() (*NALUnit, *bits.BitReader, error) {
	// Read to start of NAL
	logger.Printf("debug: Seeking NAL %d start\n", len(r.NalUnits))

	// TODO: Fix this.
	for !isStartSequence(nil) {
		if err := r.BufferToReader(1); err != nil {
			// TODO: should this return an error here.
			return nil, nil, nil
		}
	}
	/*
		if !r.IsStarted {
			logger.Printf("debug: skipping initial NAL zero byte spaces\n")
			r.LogStreamPosition()
			// Annex B.2 Step 1
			if err := r.Discard(1); err != nil {
				logger.Printf("error: while discarding empty byte (Annex B.2:1): %v\n", err)
				return nil
			}
			if err := r.Discard(2); err != nil {
				logger.Printf("error: while discarding start code prefix one 3bytes (Annex B.2:2): %v\n", err)
				return nil
			}
		}
	*/
	startOffset := r.BytesRead()
	logger.Printf("debug: Seeking next NAL start\n")
	// Read to start of next NAL
	so := r.BytesRead()
	for so == startOffset || !isStartSequence(nil) {
		so = r.BytesRead()
		if err := r.BufferToReader(1); err != nil {
			// TODO: should this return an error here?
			return nil, nil, nil
		}
	}
	// logger.Printf("debug: PreRewind %#v\n", r.Bytes())
	// Rewind back the length of the start sequence
	// r.RewindBytes(4)
	// logger.Printf("debug: PostRewind %#v\n", r.Bytes())
	endOffset := r.BytesRead()
	logger.Printf("debug: found NAL unit with %d bytes from %d to %d\n", endOffset-startOffset, startOffset, endOffset)
	nalUnitReader := bits.NewBitReader(nil)
	r.NalUnits = append(r.NalUnits, nalUnitReader)
	// TODO: this should really take an io.Reader rather than []byte. Need to fix nil
	// once this is fixed.
	nalUnit, err := NewNALUnit(nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create new nal unit")
	}
	return nalUnit, nalUnitReader, nil
}

func isStartSequence(packet []byte) bool {
	if len(packet) < len(InitialNALU) {
		return false
	}
	naluSegment := packet[len(packet)-4:]
	for i := range InitialNALU {
		if naluSegment[i] != InitialNALU[i] {
			return false
		}
	}
	return true
}

func isStartCodeOnePrefix(buf []byte) bool {
	for i, b := range buf {
		if i < 2 && b != byte(0) {
			return false
		}
		// byte 3 may be 0 or 1
		if i == 3 && b != byte(0) || b != byte(1) {
			return false
		}
	}
	logger.Printf("debug: found start code one prefix byte\n")
	return true
}

func isEmpty3Byte(buf []byte) bool {
	if len(buf) < 3 {
		return false
	}
	for _, i := range buf[len(buf)-3:] {
		if i != 0 {
			return false
		}
	}
	return true
}

func moreRBSPData(br *bits.BitReader) bool {
	// If we get an error then we must at end of NAL unit or end of stream, so
	// return false.
	b, err := br.PeekBits(1)
	if err != nil {
		return false
	}

	// If b is not 1, then we don't have a stop bit and therefore there is more
	// data so return true.
	if b == 0 {
		return true
	}

	// If we have a stop bit and trailing zeros then we're okay, otherwise return
	// now, we haven't found the end.
	b, err = br.PeekBits(8 - br.Off())
	if err != nil {
		return false
	}
	rem := 0x01 << uint(7-br.Off())
	if int(b) != rem {
		return true
	}

	// If we try to read another bit but get EOF then we must be at the end of the
	// NAL or stream.
	_, err = br.PeekBits(9 - br.Off())
	if err != nil {
		return false
	}

	// Do we have some trailing 0 bits, and then a 24-bit start code ? If so, it
	// there must not be any more RBSP data left.
	// If we get an error from the Peek, then there must not be another NAL, and
	// there must be some more RBSP, because trailing bits do not extend past the
	// byte in which the stop bit is found.
	b, err = br.PeekBits(8 - br.Off() + 24)
	if err != nil {
		return true
	}
	rem = (0x01 << uint((7-br.Off())+24)) | 0x01
	if int(b) == rem {
		return false
	}

	// Similar check to above, but this time checking for 32-bit start code.
	b, err = br.PeekBits(8 - br.Off() + 32)
	if err != nil {
		return true
	}
	rem = (0x01 << uint((7-br.Off())+32)) | 0x01
	if int(b) == rem {
		return false
	}

	return true
}

type field struct {
	loc  *int
	name string
	n    int
}

func readFields(br *bits.BitReader, fields []field) error {
	for _, f := range fields {
		b, err := br.ReadBits(f.n)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("could not read %s", f.name))
		}
		*f.loc = int(b)
	}
	return nil
}

type flag struct {
	loc  *bool
	name string
}

func readFlags(br *bits.BitReader, flags []flag) error {
	for _, f := range flags {
		b, err := br.ReadBits(1)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("could not read %s", f.name))
		}
		*f.loc = b == 1
	}
	return nil
}

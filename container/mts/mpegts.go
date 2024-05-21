/*
NAME
  mpegts.go - provides a data structure intended to encapsulate the properties
  of an MPEG-TS packet and also functions to allow manipulation of these packets.

DESCRIPTION
  See Readme.md

AUTHORS
  Saxon A. Nelson-Milton <saxon.milton@gmail.com>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

// Package mts provides MPEGT-TS (mts) encoding and related functions.
package mts

import (
	"fmt"

	"github.com/Comcast/gots/packet"
	gotspsi "github.com/Comcast/gots/psi"
	"github.com/pkg/errors"

	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/container/mts/psi"
)

const PacketSize = 188

// Standard program IDs for program specific information MPEG-TS packets.
const (
	SdtPid = 17
	PatPid = 0
	PmtPid = 4096
)

// HeadSize is the size of an MPEG-TS packet header.
const HeadSize = 4

// Consts relating to adaptation field.
const (
	AdaptationIdx              = 4                 // Index to the adaptation field (index of AFL).
	AdaptationControlIdx       = 3                 // Index to octet with adaptation field control.
	AdaptationFieldsIdx        = AdaptationIdx + 1 // Adaptation field index is the index of the adaptation fields.
	DefaultAdaptationSize      = 2                 // Default size of the adaptation field.
	AdaptationControlMask      = 0x30              // Mask for the adaptation field control in octet 3.
	DefaultAdaptationBodySize  = 1                 // Default size of the adaptation field body.
	DiscontinuityIndicatorMask = 0x80              // Mask for the discontinuity indicator at the discontinuity indicator idk.
	DiscontinuityIndicatorIdx  = AdaptationIdx + 1 // The index at which the discontinuity indicator is found in an MTS packet.
)

// TODO: make this better - currently doesn't make sense.
const (
	HasPayload         = 0x1
	HasAdaptationField = 0x2
)

/*
Packet encapsulates the fields of an MPEG-TS packet. Below is
the formatting of an MPEG-TS packet for reference!

============================================================================
| octet no | bit 0 | bit 1 | bit 2 | bit 3 | bit 4 | bit 5 | bit 6 | bit 7 |
============================================================================
| octet 0  | sync byte (0x47)                                              |
----------------------------------------------------------------------------
| octet 1  | TEI   | PUSI  | Prior | PID                                   |
----------------------------------------------------------------------------
| octet 2  | PID cont.                                                     |
----------------------------------------------------------------------------
| octet 3  | TSC           | AFC           | CC                            |
----------------------------------------------------------------------------
| octet 4  | AFL                                                           |
----------------------------------------------------------------------------
| octet 5  | DI    | RAI   | ESPI  | PCRF  | OPCRF | SPF   | TPDF  | AFEF  |
----------------------------------------------------------------------------
| optional | PCR (48 bits => 6 bytes)                                      |
----------------------------------------------------------------------------
| -        | PCR cont.                                                     |
----------------------------------------------------------------------------
| -        | PCR cont.                                                     |
----------------------------------------------------------------------------
| -        | PCR cont.                                                     |
----------------------------------------------------------------------------
| -        | PCR cont.                                                     |
----------------------------------------------------------------------------
| -        | PCR cont.                                                     |
----------------------------------------------------------------------------
| optional | OPCR (48 bits => 6 bytes)                                     |
----------------------------------------------------------------------------
| -        | OPCR cont.                                                    |
----------------------------------------------------------------------------
| -        | OPCR cont.                                                    |
----------------------------------------------------------------------------
| -        | OPCR cont.                                                    |
----------------------------------------------------------------------------
| -        | OPCR cont.                                                    |
----------------------------------------------------------------------------
| -        | OPCR cont.                                                    |
----------------------------------------------------------------------------
| optional | SC                                                            |
----------------------------------------------------------------------------
| optional | TPDL                                                          |
----------------------------------------------------------------------------
| optional | TPD (variable length)                                         |
----------------------------------------------------------------------------
| -        | ...                                                           |
----------------------------------------------------------------------------
| optional | Extension (variable length)                                   |
----------------------------------------------------------------------------
| -        | ...                                                           |
----------------------------------------------------------------------------
| optional | Stuffing (variable length)                                    |
----------------------------------------------------------------------------
| -        | ...                                                           |
----------------------------------------------------------------------------
| optional | Payload (variable length)                                     |
----------------------------------------------------------------------------
| -        | ...                                                           |
----------------------------------------------------------------------------
*/
type Packet struct {
	TEI      bool   // Transport Error Indicator
	PUSI     bool   // Payload Unit Start Indicator
	Priority bool   // Tranposrt priority indicator
	PID      uint16 // Packet identifier
	TSC      byte   // Transport Scrambling Control
	AFC      byte   // Adaption Field Control
	CC       byte   // Continuity Counter
	DI       bool   // Discontinouty indicator
	RAI      bool   // random access indicator
	ESPI     bool   // Elementary stream priority indicator
	PCRF     bool   // PCR flag
	OPCRF    bool   // OPCR flag
	SPF      bool   // Splicing point flag
	TPDF     bool   // Transport private data flag
	AFEF     bool   // Adaptation field extension flag
	PCR      uint64 // Program clock reference
	OPCR     uint64 // Original program clock reference
	SC       byte   // Splice countdown
	TPDL     byte   // Tranposrt private data length
	TPD      []byte // Private data
	Ext      []byte // Adaptation field extension
	Payload  []byte // Mpeg ts Payload
}

// FindPmt will take a clip of MPEG-TS and try to find a PMT table - if one
// is found, then it is returned along with its index, otherwise nil, -1 and an error is returned.
func FindPmt(d []byte) ([]byte, int, error) {
	return FindPid(d, PmtPid)
}

// FindPat will take a clip of MPEG-TS and try to find a PAT table - if one
// is found, then it is returned along with its index, otherwise nil, -1 and an error is returned.
func FindPat(d []byte) ([]byte, int, error) {
	return FindPid(d, PatPid)
}

// Errors used by FindPid.
var (
	ErrInvalidLen   = errors.New("MPEG-TS data not of valid length")
)

// FindPid will take a clip of MPEG-TS and try to find a packet with given PID - if one
// is found, then it is returned along with its index, otherwise nil, -1 and an error is returned.
func FindPid(d []byte, pid uint16) (pkt []byte, i int, err error) {
	if len(d) < PacketSize {
		return nil, -1, ErrInvalidLen
	}
	for i = 0; i < len(d); i += PacketSize {
		p := (uint16(d[i+1]&0x1f) << 8) | uint16(d[i+2])
		if p == pid {
			pkt = d[i : i+PacketSize]
			return
		}
	}
	return nil, -1, fmt.Errorf("could not find packet with PID %d", pid)
}

// LastPid will take a clip of MPEG-TS and try to find a packet
// with given PID searching in reverse from the end of the clip. If
// one is found, then it is returned along with its index, otherwise
// nil, -1 and an error is returned.
func LastPid(d []byte, pid uint16) (pkt []byte, i int, err error) {
	if len(d) < PacketSize {
		return nil, -1, ErrInvalidLen
	}

	for i = len(d) - PacketSize; i >= 0; i -= PacketSize {
		p := (uint16(d[i+1]&0x1f) << 8) | uint16(d[i+2])
		if p == pid {
			pkt = d[i : i+PacketSize]
			return
		}
	}
	return nil, -1, fmt.Errorf("could not find packet with PID %d", pid)
}

// Errors used by FindPSI.
var (
	ErrMultiplePrograms = errors.New("more than one program not supported")
	ErrNoPrograms       = errors.New("no programs in PAT")
	ErrNotConsecutive   = errors.New("could not find consecutive PIDs")
)

// FindPSI finds the index of a PAT in an a slice of MPEG-TS and returns, along
// with a map of meta from the PMT and the stream PIDs and their types.
func FindPSI(d []byte) (int, map[uint16]uint8, map[string]string, error) {
	if len(d) < PacketSize {
		return -1, nil, nil, ErrInvalidLen
	}

	// Find the PAT if it exists.
	pkt, i, err := FindPid(d, PatPid)
	if err != nil {
		return -1, nil, nil, errors.Wrap(err, "error finding PAT")
	}

	// Let's take this opportunity to check what programs are in this MPEG-TS
	// stream, and therefore the PID of the PMT, from which we can get metadata.
	// NB: currently we only support one program.
	progs, err := Programs(pkt)
	if err != nil {
		return i, nil, nil, errors.Wrap(err, "cannot get programs from PAT")
	}

	if len(progs) == 0 {
		return i, nil, nil, ErrNoPrograms
	}

	if len(progs) > 1 {
		return i, nil, nil, ErrMultiplePrograms
	}

	pmtPID := pmtPIDs(progs)[0]

	// Now we can look for the PMT. We want to adjust d so that we're not looking
	// at the same data twice.
	d = d[i+PacketSize:]
	pkt, pmtIdx, err := FindPid(d, pmtPID)
	if err != nil {
		return i, nil, nil, errors.Wrap(err, "error finding PMT")
	}

	// Check that the PMT comes straight after the PAT.
	if pmtIdx != 0 {
		return i, nil, nil, ErrNotConsecutive
	}

	// Now we can try to get meta from the PMT.
	meta, _ := metaFromPMT(pkt)

	// Now to get the elementary streams defined for this program.
	streams, err := Streams(pkt)
	if err != nil {
		return i, nil, meta, errors.Wrap(err, "could not get streams from PMT")
	}

	streamMap := make(map[uint16]uint8)
	for _, s := range streams {
		streamMap[(uint16)(s.ElementaryPid())] = s.StreamType()
	}

	return i, streamMap, meta, nil
}

var (
	ErrStreamMap = errors.New("stream map is empty")
)

// FirstMediaPID returns the first PID and it's type in the given streamMap.
func FirstMediaPID(streamMap map[uint16]uint8) (p uint16, t uint8, err error) {
	for p, t = range streamMap {
		return
	}
	err = ErrStreamMap
	return
}

// FillPayload takes a channel and fills the packets Payload field until the
// channel is empty or we've the packet reaches capacity
func (p *Packet) FillPayload(data []byte) int {
	currentPktLen := 6 + asInt(p.PCRF)*6
	if len(data) > PacketSize-currentPktLen {
		p.Payload = make([]byte, PacketSize-currentPktLen)
	} else {
		p.Payload = make([]byte, len(data))
	}
	return copy(p.Payload, data)
}

// Bytes interprets the fields of the ts packet instance and outputs a
// corresponding byte slice
func (p *Packet) Bytes(buf []byte) []byte {
	if buf == nil || cap(buf) < PacketSize {
		buf = make([]byte, PacketSize)
	}

	if p.OPCRF {
		panic("original program clock reference field unsupported")
	}
	if p.SPF {
		panic("splicing countdown unsupported")
	}
	if p.TPDF {
		panic("transport private data unsupported")
	}
	if p.AFEF {
		panic("adaptation field extension unsupported")
	}

	buf = buf[:6]
	buf[0] = 0x47
	buf[1] = (asByte(p.TEI)<<7 | asByte(p.PUSI)<<6 | asByte(p.Priority)<<5 | byte((p.PID&0xFF00)>>8))
	buf[2] = byte(p.PID & 0x00FF)
	buf[3] = (p.TSC<<6 | p.AFC<<4 | p.CC)

	var maxPayloadSize int
	if p.AFC&0x2 != 0 {
		maxPayloadSize = PacketSize - 6 - asInt(p.PCRF)*6
	} else {
		maxPayloadSize = PacketSize - 4
	}

	stuffingLen := maxPayloadSize - len(p.Payload)
	if p.AFC&0x2 != 0 {
		buf[4] = byte(1 + stuffingLen + asInt(p.PCRF)*6)
		buf[5] = (asByte(p.DI)<<7 | asByte(p.RAI)<<6 | asByte(p.ESPI)<<5 | asByte(p.PCRF)<<4 | asByte(p.OPCRF)<<3 | asByte(p.SPF)<<2 | asByte(p.TPDF)<<1 | asByte(p.AFEF))
	} else {
		buf = buf[:4]
	}

	for i := 40; p.PCRF && i >= 0; i -= 8 {
		buf = append(buf, byte((p.PCR<<15)>>uint(i)))
	}

	for i := 0; i < stuffingLen; i++ {
		buf = append(buf, 0xff)
	}
	curLen := len(buf)
	buf = buf[:PacketSize]
	copy(buf[curLen:], p.Payload)
	return buf
}

func asInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func asByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

type Option func(p *packet.Packet)

// addAdaptationField adds an adaptation field to p, and applys the passed options to this field.
// TODO: this will probably break if we already have adaptation field.
func addAdaptationField(p *packet.Packet, options ...Option) error {
	if packet.ContainsAdaptationField((*packet.Packet)(p)) {
		return errors.New("Adaptation field is already present in packet")
	}
	// Create space for adaptation field.
	copy(p[HeadSize+DefaultAdaptationSize:], p[HeadSize:len(p)-DefaultAdaptationSize])

	// TODO: seperate into own function
	// Update adaptation field control.
	p[AdaptationControlIdx] &= 0xff ^ AdaptationControlMask
	p[AdaptationControlIdx] |= AdaptationControlMask
	// Default the adaptationfield.
	resetAdaptation(p)

	// Apply and options that have bee passed.
	for _, option := range options {
		option(p)
	}
	return nil
}

// resetAdaptation sets fields in ps adaptation field to 0 if the adaptation field
// exists, otherwise an error is returned.
func resetAdaptation(p *packet.Packet) error {
	if !packet.ContainsAdaptationField((*packet.Packet)(p)) {
		return errors.New("No adaptation field in this packet")
	}
	p[AdaptationIdx] = DefaultAdaptationBodySize
	p[AdaptationIdx+1] = 0x00
	return nil
}

// DiscontinuityIndicator returns an Option that will set p's discontinuity
// indicator according to f.
func DiscontinuityIndicator(f bool) Option {
	return func(p *packet.Packet) {
		set := byte(DiscontinuityIndicatorMask)
		if !f {
			set = 0x00
		}
		p[DiscontinuityIndicatorIdx] &= 0xff ^ DiscontinuityIndicatorMask
		p[DiscontinuityIndicatorIdx] |= DiscontinuityIndicatorMask & set
	}
}

// Error used by GetPTSRange.
var errNoPTS = errors.New("could not find PTS")

// GetPTSRange retreives the first and last PTS of an MPEGTS clip.
// If there is only one PTS, it is included twice in the pts return value.
func GetPTSRange(clip []byte, pid uint16) (pts [2]uint64, err error) {
	var _pts int64
	// Get the first PTS for the given PID.
	var i int
	for {
		if i >= len(clip) {
			return pts, errNoPTS
		}
		pkt, _i, err := FindPid(clip[i:], pid)
		if err != nil {
			return pts, errors.Wrap(err, fmt.Sprintf("could not find packet of PID: %d", pid))
		}
		_pts, err = GetPTS(pkt)
		if err == nil {
			break
		}
		i += _i + PacketSize
	}

	pts[0] = uint64(_pts)
	pts[1] = pts[0] // Until we have find a second PTS.

	// Get the last PTS searching in reverse from end of the clip.
	first := i
	i = len(clip)
	for {
		pkt, _i, err := LastPid(clip[:i], pid)
		if err != nil || i <= first {
			return pts, nil
		}
		_pts, err = GetPTS(pkt)
		if err == nil {
			break
		}
		i = _i
	}

	pts[1] = uint64(_pts)

	return
}

var (
	errNoPesPayload      = errors.New("no PES payload")
	errNoPesPTS          = errors.New("no PES PTS")
	errInvalidPesHeader  = errors.New("invalid PES header")
	errInvalidPesPayload = errors.New("invalid PES payload")
)

// GetPTS returns a PTS from a packet that has PES payload, or an error otherwise.
func GetPTS(pkt []byte) (pts int64, err error) {
	// Check the Payload Unit Start Indicator.
	if pkt[1]&0x040 == 0 {
		err = errNoPesPayload
		return
	}

	// Compute start of PES payload and check its length.
	start := HeadSize
	if pkt[3]&0x20 != 0 {
		// Adaptation field is present, so adjust start of payload accordingly.
		start += 1 + int(pkt[4])
	}
	pes := pkt[start:]

	if len(pes) < 14 {
		err = errInvalidPesHeader
		return
	}

	// Check the PTS DTS indicator.
	if pes[7]&0xc0 == 0 {
		err = errNoPesPTS
		return
	}

	pts = extractPTS(pes[9:14])
	return
}

// extractTime extracts a PTS from the given data.
func extractPTS(d []byte) int64 {
	return (int64((d[0]>>1)&0x07) << 30) | (int64(d[1]) << 22) | (int64((d[2]>>1)&0x7f) << 15) | (int64(d[3]) << 7) | int64((d[4]>>1)&0x7f)
}

var errNoMeta = errors.New("PMT does not contain meta")

// ExtractMeta returns a map of metadata from the first PMT's metaData
// descriptor, that is found in the MPEG-TS clip d. d must contain a series of
// complete MPEG-TS packets.
func ExtractMeta(d []byte) (map[string]string, error) {
	pmt, _, err := FindPid(d, PmtPid)
	if err != nil {
		return nil, err
	}
	return metaFromPMT(pmt)
}

// metaFromPMT returns metadata, if any, from a PMT.
func metaFromPMT(d []byte) (m map[string]string, err error) {
	// Get as PSI type, skipping the MTS header.
	pmt := psi.PSIBytes(d[HeadSize:])

	// Get the metadata descriptor.
	_, desc := pmt.HasDescriptor(psi.MetadataTag)
	if desc == nil {
		return m, errNoMeta
	}
	// Get the metadata as a map, skipping the descriptor head.
	return meta.GetAllAsMap(desc[2:])
}

// TrimToMetaRange trims a slice of MPEG-TS to a segment between two points of
// meta data described by key, from and to.
func TrimToMetaRange(d []byte, key, from, to string) ([]byte, error) {
	if len(d)%PacketSize != 0 {
		return nil, errors.New("MTS clip is not of valid size")
	}

	if from == to {
		return nil, errors.New("'from' and 'to' cannot be identical")
	}

	var (
		start = -1 // Index of the start of the segment in d.
		end   = -1 // Index of the end of segment in d.
		off   int  // Index of remaining slice of d to check after each PMT found.
	)

	for {
		// Find the next PMT.
		pmt, idx, err := FindPid(d[off:], PmtPid)
		if err != nil {
			switch -1 {
			case start:
				return nil, errMetaLowerBound
			case end:
				return nil, errMetaUpperBound
			default:
				panic("should not have got error from FindPid")
			}
		}
		off += idx + PacketSize

		meta, err := ExtractMeta(pmt)
		switch err {
		case nil: // do nothing
		case errNoMeta:
			continue
		default:
			return nil, err
		}

		if start == -1 {
			if meta[key] == from {
				start = off - PacketSize
			}
		} else if meta[key] == to {
			end = off
			return d[start:end], nil
		}
	}
}

// SegmentForMeta returns segments of MTS slice d that correspond to a value of
// meta for key and val. Therefore, any sequence of packets corresponding to
// key and val will be appended to the returned [][]byte.
func SegmentForMeta(d []byte, key, val string) ([][]byte, error) {
	var (
		pkt        packet.Packet // We copy data to this so that we can use comcast gots stuff.
		segmenting bool          // If true we are currently in a segment corresponsing to given meta.
		res        [][]byte      // The resultant [][]byte holding the segments.
		start      int           // The start index of the current segment.
	)

	// Go through packets.
	for i := 0; i < len(d); i += PacketSize {
		copy(pkt[:], d[i:i+PacketSize])
		if pkt.PID() == PmtPid {
			_meta, err := ExtractMeta(pkt[:])
			switch err {
			// If there's no meta or a problem with meta, we consider this the end
			// of the segment.
			case errNoMeta, meta.ErrUnexpectedMetaFormat:
				if segmenting {
					res = append(res, d[start:i])
					segmenting = false
				}
				continue
			case nil: // do nothing.
			default:
				return nil, err
			}

			// If we've got the meta of interest in the PMT and we're not segmenting
			// then start segmenting. If we don't have the meta of interest in the PMT
			// and we are segmenting then we want to stop and append the segment to result.
			if _meta[key] == val && !segmenting {
				start = i
				segmenting = true
			} else if _meta[key] != val && segmenting {
				res = append(res, d[start:i])
				segmenting = false
			}
		}
	}

	// We've reached the end of the entire MTS clip so if we're segmenting we need
	// to append current segment to res.
	if segmenting {
		res = append(res, d[start:])
	}

	return res, nil
}

// PID returns the packet identifier for the given packet.
func PID(p []byte) (uint16, error) {
	if len(p) < PacketSize {
		return 0, errors.New("packet length less than 188")
	}
	return uint16(p[1]&0x1f)<<8 | uint16(p[2]), nil
}

// Programs returns a map of program numbers and corresponding PMT PIDs for a
// given MPEG-TS PAT packet.
func Programs(p []byte) (map[uint16]uint16, error) {
	pat, err := gotspsi.NewPAT(p)
	if err != nil {
		return nil, err
	}
	// Convert to map[uint16]uint16.
	m := make(map[uint16]uint16)
	for k, v := range pat.ProgramMap() {
		m[uint16(k)] = uint16(v)
	}
	return m, nil
}

// Streams returns elementary streams defined in a given MPEG-TS PMT packet.
func Streams(p []byte) ([]gotspsi.PmtElementaryStream, error) {
	payload, err := Payload(p)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get packet payload")
	}
	pmt, err := gotspsi.NewPMT(payload)
	if err != nil {
		return nil, err
	}
	return pmt.ElementaryStreams(), nil
}

// MediaStreams retrieves the PmtElementaryStreams from the given PSI. This
// function currently assumes that PSI contain a PAT followed by a PMT directly
// after. We also assume that this MPEG-TS stream contains just one program,
// but this program may contain different streams, i.e. a video stream + audio
// stream.
func MediaStreams(p []byte) ([]gotspsi.PmtElementaryStream, error) {
	if len(p) < 2*PacketSize {
		return nil, errors.New("PSI is not two packets or more long")
	}
	pat := p[:PacketSize]
	pmt := p[PacketSize : 2*PacketSize]

	pid, _ := PID(pat)
	if pid != PatPid {
		return nil, errors.New("first packet is not a PAT")
	}

	m, err := Programs(pat)
	if err != nil {
		return nil, errors.Wrap(err, "could not get programs from PAT")
	}

	if len(m) == 0 {
		return nil, ErrNoPrograms
	}

	if len(m) > 1 {
		return nil, ErrMultiplePrograms
	}

	pid, _ = PID(pmt)
	if pid != pmtPIDs(m)[0] {
		return nil, errors.New("second packet is not desired PMT")
	}

	s, err := Streams(pmt)
	if err != nil {
		return nil, errors.Wrap(err, "could not get streams from PMT")
	}
	return s, nil
}

// pmtPIDs returns PMT PIDS from a map containing program number as keys and
// corresponding PMT PIDs as values.
func pmtPIDs(m map[uint16]uint16) []uint16 {
	r := make([]uint16, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}

// Errors used by Payload.
var ErrNoPayload = errors.New("no payload")

// Payload returns the payload of an MPEG-TS packet p.
// NB: this is not a copy of the payload in the interests of performance.
// TODO: offer function that will do copy if we have interests in safety.
func Payload(p []byte) ([]byte, error) {
	c := byte((p[3] & 0x30) >> 4)
	if c == 2 {
		return nil, ErrNoPayload
	}

	// Check if there is an adaptation field.
	off := 4
	if p[3]&0x20 == 1 {
		off = int(5 + p[4])
	}
	return p[off:], nil
}

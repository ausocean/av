/*
NAME
  encoder.go

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package mts

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/ausocean/av/codec/h264"
	"github.com/ausocean/av/codec/h264/h264dec"
	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/container/mts/pes"
	"github.com/ausocean/av/container/mts/psi"
	"github.com/ausocean/utils/logging"
	"github.com/ausocean/utils/realtime"
)

// These three constants are used to select between the three different
// methods of when the PSI is sent.
const (
	psiMethodPacket = iota // PSI is inserted after a certain number of packets.
	psiMethodTime          // PSI is inserted after a certain amount of time.
	psiMethodNAL           // PSI is inserted before each "key frame" of media.
)

// Constants used to communicate which media codec will be packetized.
const (
	EncodeH264 = iota
	EncodeH265
	EncodeJPEG
	EncodeMJPEG
	EncodePCM
	EncodeADPCM
)

// The program IDs we assign to different types of media.
const (
	PIDVideo = 256
	PIDAudio = 210
)

// Time-related constants.
const (
	// ptsOffset is the offset added to the clock to determine
	// the current presentation timestamp.
	ptsOffset = 700 * time.Millisecond

	// PCRFrequency is the base Program Clock Reference frequency in Hz.
	PCRFrequency = 90000

	// PTSFrequency is the presentation timestamp frequency in Hz.
	PTSFrequency = 90000

	// MaxPTS is the largest PTS value (i.e., for a 33-bit unsigned integer).
	MaxPTS = (1 << 33) - 1
)

// If we are not using NAL based PSI intervals then we will send PSI every 7 packets.
const psiSendCount = 7

const (
	hasPayload         = 0x1
	hasAdaptationField = 0x2
)

const (
	hasDTS = 0x1
	hasPTS = 0x2
)

// Default encoder configuration parameters.
const (
	defaultRate      = 25 // FPS
	defaultPSIMethod = psiMethodNAL
	defaultStreamID  = pes.H264SID
	defaultMediaPID  = PIDVideo
)

// Used to consistently read and write MTS metadata entries.
const (
	WriteRateKey = "writeRate"
	TimestampKey = "ts"
	LocationKey  = "loc"
)

// Meta allows addition of metadata to encoded mts from outside of this pkg.
// See meta pkg for usage.
//
// TODO: make this not global.
var Meta *meta.Data

// RealTime will help us obtain a realtime for timestamp meta encoding.
var RealTime = realtime.NewRealTime()

// Encoder encapsulates properties of an MPEG-TS generator.
type Encoder struct {
	dst io.WriteCloser

	clock       time.Duration
	lastTime    time.Time
	writePeriod time.Duration
	ptsOffset   time.Duration
	tsSpace     [PacketSize]byte
	pesSpace    [pes.MaxPesSize]byte

	continuity map[uint16]byte

	psiMethod    int
	pktCount     int
	psiSendCount int
	psiTime      time.Duration
	psiSetTime   time.Duration
	startTime    time.Time
	mediaPID     uint16
	streamID     byte

	pmt                *psi.PSI
	patBytes, pmtBytes []byte

	// log is a function that will be used through the encoder code for logging.
	log logging.Logger
}

// NewEncoder returns an Encoder with the specified media type and rate eg. if a video stream
// calls write for every frame, the rate will be the frame rate of the video.
func NewEncoder(dst io.WriteCloser, log logging.Logger, options ...func(*Encoder) error) (*Encoder, error) {
	e := &Encoder{
		dst:         dst,
		writePeriod: time.Duration(float64(time.Second) / defaultRate),
		ptsOffset:   ptsOffset,
		psiMethod:   defaultPSIMethod,
		pktCount:    8,
		mediaPID:    defaultMediaPID,
		streamID:    defaultStreamID,
		continuity:  map[uint16]byte{PatPid: 0, PmtPid: 0, defaultMediaPID: 0},
		log:         log,
		patBytes:    psi.NewPATPSI().Bytes(),
		pmt:         psi.NewPMTPSI(),
	}

	for _, option := range options {
		err := option(e)
		if err != nil {
			return nil, fmt.Errorf("option failed with error: %w", err)
		}
	}
	log.Debug("encoder options applied")

	Meta.Add(WriteRateKey, fmt.Sprintf("%f", 1/float64(e.writePeriod.Seconds())))

	e.pmt.SyntaxSection.SpecificData.(*psi.PMT).StreamSpecificData.StreamType = e.streamID
	e.pmt.SyntaxSection.SpecificData.(*psi.PMT).StreamSpecificData.PID = e.mediaPID
	e.pmtBytes = e.pmt.Bytes()

	return e, nil
}

// Write implements io.Writer. Write takes raw video or audio data and encodes into MPEG-TS,
// then sending it to the encoder's io.Writer destination.
func (e *Encoder) Write(data []byte) (int, error) {
	e.log.Debug("writing data", "len(data)", len(data))
	switch e.psiMethod {
	case psiMethodPacket:
		e.log.Debug("checking packet no. conditions for PSI write", "count", e.pktCount, "PSI count", e.psiSendCount)
		if e.pktCount >= e.psiSendCount {
			e.pktCount = 0
			err := e.writePSI()
			if err != nil {
				return 0, fmt.Errorf("could not write psi (psiMethodPacket): %w", err)
			}
		}
	case psiMethodNAL:
		nalType, err := h264.NALType(data)
		if err != nil {
			return 0, fmt.Errorf("could not get type from NAL unit, failed with error: %w", err)
		}
		e.log.Debug("checking conditions for PSI write", "AU type", nalType, "needed type", h264dec.NALTypeSPS)
		if nalType == h264dec.NALTypeSPS {
			err := e.writePSI()
			if err != nil {
				return 0, fmt.Errorf("could not write psi (psiMethodNAL): %w", err)
			}
		}
	case psiMethodTime:
		dur := time.Now().Sub(e.startTime)
		e.log.Debug("checking time conditions for PSI write")
		if dur >= e.psiTime {
			e.psiTime = e.psiSetTime
			e.startTime = time.Now()
			err := e.writePSI()
			if err != nil {
				return 0, fmt.Errorf("could not write psi (psiMethodTime): %w", err)
			}
		}
	default:
		panic("undefined PSI method")
	}

	// Prepare PES data.
	pts := e.pts()
	pesPkt := pes.Packet{
		StreamID:     e.streamID,
		PDI:          hasPTS,
		PTS:          pts,
		Data:         data,
		HeaderLength: 5,
	}

	buf := pesPkt.Bytes(e.pesSpace[:pes.MaxPesSize])

	pusi := true
	for len(buf) != 0 {
		pkt := Packet{
			PUSI: pusi,
			PID:  uint16(e.mediaPID),
			RAI:  pusi,
			CC:   e.ccFor(e.mediaPID),
			AFC:  hasAdaptationField | hasPayload,
			PCRF: pusi,
		}
		n := pkt.FillPayload(buf)
		buf = buf[n:]

		if pusi {
			// If the packet has a Payload Unit Start Indicator
			// flag set then we need to write a PCR.
			pcr := e.pcr()
			e.log.Debug("new access unit", "PCR", pcr, "PTS", pts)
			pkt.PCR = pcr
			pusi = false
		}

		b := pkt.Bytes(e.tsSpace[:PacketSize])
		e.log.Debug("writing MTS packet to destination", "size", len(b), "pusi", pusi, "PID", pkt.PID, "PTS", pts, "PCR", pkt.PCR)
		_, err := e.dst.Write(b)
		if err != nil {
			return len(data), fmt.Errorf("could not write MTS packet to destination: %w", err)
		}
		e.pktCount++
	}

	e.tick()

	return len(data), nil
}

// writePSI creates MPEG-TS with pat and pmt tables - with pmt table having updated
// location and time data.
func (e *Encoder) writePSI() error {
	// Write PAT.
	patPkt := Packet{
		PUSI:    true,
		PID:     PatPid,
		CC:      e.ccFor(PatPid),
		AFC:     hasPayload,
		Payload: psi.AddPadding(e.patBytes),
	}
	_, err := e.dst.Write(patPkt.Bytes(e.tsSpace[:PacketSize]))
	if err != nil {
		return fmt.Errorf("could not write pat packet: %w", err)
	}
	e.pktCount++

	e.pmtBytes, err = updateMeta(e.pmtBytes, e.log)
	if err != nil {
		return fmt.Errorf("could not update pmt metadata: %w", err)
	}

	// Create mts packet from pmt table.
	pmtPkt := Packet{
		PUSI:    true,
		PID:     PmtPid,
		CC:      e.ccFor(PmtPid),
		AFC:     hasPayload,
		Payload: psi.AddPadding(e.pmtBytes),
	}
	_, err = e.dst.Write(pmtPkt.Bytes(e.tsSpace[:PacketSize]))
	if err != nil {
		return fmt.Errorf("could not write pmt packet: %w", err)
	}
	e.pktCount++

	e.log.Debug("PSI written", "PAT CC", patPkt.CC, "PMT CC", pmtPkt.CC)
	return nil
}

// tick advances the clock one frame interval.
func (e *Encoder) tick() {
	e.clock += e.writePeriod
}

// pts retuns the current presentation timestamp.
func (e *Encoder) pts() uint64 {
	return uint64((e.clock + e.ptsOffset).Seconds() * PTSFrequency)
}

// pcr returns the current program clock reference.
func (e *Encoder) pcr() uint64 {
	return uint64(e.clock.Seconds() * PCRFrequency)
}

// ccFor returns the next continuity counter for pid.
func (e *Encoder) ccFor(pid uint16) byte {
	cc := e.continuity[pid]
	const continuityCounterMask = 0xf
	e.continuity[pid] = (cc + 1) & continuityCounterMask
	return cc
}

// updateMeta adds/updates a metaData descriptor in the given psi bytes using data
// contained in the global Meta struct.
func updateMeta(b []byte, log logging.Logger) ([]byte, error) {
	p := psi.PSIBytes(b)
	if RealTime.IsSet() {
		t := strconv.Itoa(int(RealTime.Get().Unix()))
		Meta.Add(TimestampKey, t)
		log.Debug("latest time added to meta", "time", t)
	}
	err := p.AddDescriptor(psi.MetadataTag, Meta.Encode())
	return []byte(p), err
}

func (e *Encoder) Close() error {
	e.log.Debug("closing encoder")
	return e.dst.Close()
}

/*
DESCRIPTION
  options.go provides option functions that can be provided to the MTS encoders
  constructor NewEncoder for encoder configuration. These options include media
  type, PSI insertion strategy and intended access unit rate.

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package mts

import (
	"errors"
	"time"

	"github.com/ausocean/av/container/mts/pes"
)

var (
	ErrUnsupportedMedia = errors.New("unsupported media type")
	ErrInvalidRate      = errors.New("invalid access unit rate")
)

// PacketBasedPSI is an option that can be passed to NewEncoder to select
// packet based PSI writing, i.e. PSI are written to the destination every
// sendCount packets.
func PacketBasedPSI(sendCount int) func(*Encoder) error {
	return func(e *Encoder) error {
		e.psiMethod = psiMethodPacket
		e.psiSendCount = sendCount
		e.pktCount = e.psiSendCount
		e.log.Debug("configured for packet based PSI insertion", "count", sendCount)
		return nil
	}
}

// TimeBasedPSI is another option that can be passed to NewEncoder to select
// time based PSI writing, i.e. PSI are written to the destination every dur
// (duration). The defualt is 2 seconds.
func TimeBasedPSI(dur time.Duration) func(*Encoder) error {
	return func(e *Encoder) error {
		e.psiMethod = psiMethodTime
		e.psiTime = 0
		e.psiSetTime = dur
		e.startTime = time.Now()
		e.log.Debug("configured for time based PSI insertion")
		return nil
	}
}

// MediaType is an option that can be passed to NewEncoder. It is used to
// specifiy the media type/codec of the data we are packetising using the
// encoder. Currently supported options are EncodeH264, EncodeH265, EncodeMJPEG,
// EncodeJPEG, EncodePCM and EncodeADPCM.
func MediaType(mt int) func(*Encoder) error {
	return func(e *Encoder) error {
		switch mt {
		case EncodePCM:
			e.mediaPID = PIDAudio
			e.streamID = pes.PCMSID
			e.log.Debug("configured for PCM packetisation")
		case EncodeADPCM:
			e.mediaPID = PIDAudio
			e.streamID = pes.ADPCMSID
			e.log.Debug("configured for ADPCM packetisation")
		case EncodeH265:
			e.mediaPID = PIDVideo
			e.streamID = pes.H265SID
			e.log.Debug("configured for h.265 packetisation")
		case EncodeH264:
			e.mediaPID = PIDVideo
			e.streamID = pes.H264SID
			e.log.Debug("configured for h.264 packetisation")
		case EncodeMJPEG:
			e.mediaPID = PIDVideo
			e.streamID = pes.MJPEGSID
			e.log.Debug("configured for MJPEG packetisation")
		case EncodeJPEG:
			e.mediaPID = PIDVideo
			e.streamID = pes.JPEGSID
			e.log.Debug("configure for JPEG packetisation")
		default:
			return ErrUnsupportedMedia
		}
		e.continuity = map[uint16]byte{PatPid: 0, PmtPid: 0, e.mediaPID: 0}
		return nil
	}
}

// Rate is an option that can be passed to NewEncoder. It is used to specifiy
// the rate at which the access units should be played in playback. This will
// be used to create timestamps and counts such as PTS and PCR.
func Rate(r float64) func(*Encoder) error {
	return func(e *Encoder) error {
		if r < 1 || r > 60 {
			return ErrInvalidRate
		}
		e.writePeriod = time.Duration(float64(time.Second) / r)
		return nil
	}
}

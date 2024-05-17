/*
NAME
  payload.go

DESCRIPTION
  payload.go provides functionality for extracting and manipulating the payload
  data from MPEG-TS.

AUTHOR
  Saxon A. Nelson-Milton <saxon@ausocean.org>

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
	"fmt"
	"sort"

	"github.com/Comcast/gots/packet"
	"github.com/Comcast/gots/pes"
)

// Extract extracts the media, PTS, stream ID and meta for an MPEG-TS clip given
// by p, and returns as a Clip. The MPEG-TS must contain only complete packets.
// The resultant data is a copy of the original.
func Extract(p []byte) (*Clip, error) {
	l := len(p)
	// Check that clip is divisible by 188, i.e. contains a series of full MPEG-TS clips.
	if l%PacketSize != 0 {
		return nil, errors.New("MTS clip is not of valid size")
	}

	var (
		frameStart  int               // Index used to indicate the start of current frame in backing slice.
		clip        = &Clip{}         // The data that will be returned.
		meta        map[string]string // Holds the most recently extracted meta.
		lenOfFrame  int               // Len of current frame.
		dataLen     int               // Len of data from MPEG-TS packet.
		curPTS      uint64            // Holds the current PTS.
		curStreamID uint8             // Holds current StreamID (shouldn't change)
		firstPUSI   = true            // Indicates that we have not yet received a PUSI.
		err         error
	)

	// This will hold a copy of all the media in the MPEG-TS clip.
	clip.backing = make([]byte, 0, l/PacketSize)

	// Go through the MPEGT-TS packets.
	var pkt packet.Packet
	for i := 0; i < l; i += PacketSize {
		// We will use comcast/gots Packet type, so copy in.
		copy(pkt[:], p[i:i+PacketSize])

		switch pkt.PID() {
		case PatPid: // Do nothing.
		case PmtPid:
			meta, err = ExtractMeta(pkt[:])
			if err != nil {
				return nil, fmt.Errorf("could not extract meta data: %w", err)
			}
		default: // Must be media.
			// Get the MPEG-TS payload.
			payload, err := pkt.Payload()
			if err != nil {
				return nil, fmt.Errorf("could not extract payload: %w", err)
			}

			// If PUSI is true then we know it's the start of a new frame, and we have
			// a PES header in the MTS payload.
			if pkt.PayloadUnitStartIndicator() {
				_pes, err := pes.NewPESHeader(payload)
				if err != nil {
					return nil, fmt.Errorf("could not parse PES: %w", err)
				}

				// Extract the PTS and ID, then add a new frame to the clip.
				curPTS = _pes.PTS()
				curStreamID = _pes.StreamId()
				clip.frames = append(clip.frames, Frame{
					PTS:  curPTS,
					ID:   curStreamID,
					Meta: meta,
				})

				// Append the data to the underlying buffer and get appended length.
				clip.backing = append(clip.backing, _pes.Data()...)
				dataLen = len(_pes.Data())

				// If we haven't hit the first PUSI, then we know we have a full frame
				// and can add this data to the frame pertaining to the finish frame.
				if !firstPUSI {
					clip.frames[len(clip.frames)-2].Media = clip.backing[frameStart:lenOfFrame]
					clip.frames[len(clip.frames)-2].idx = frameStart
					frameStart = lenOfFrame
				}
				firstPUSI = false
			} else {
				// We're not at the start of the frame, so we don't have a PES header.
				// We can append the MPEG-TS data directly to the underlying buf.
				dataLen = len(payload)
				clip.backing = append(clip.backing, payload...)
			}
			lenOfFrame += dataLen
		}
	}
	// We're finished up with media frames, so give the final Frame it's data.
	clip.frames[len(clip.frames)-1].Media = clip.backing[frameStart:lenOfFrame]
	clip.frames[len(clip.frames)-1].idx = frameStart
	return clip, nil
}

// Clip represents a clip of media, i.e. a sequence of media frames.
type Clip struct {
	frames  []Frame
	backing []byte
}

// Frame describes a media frame that may be extracted from a PES packet.
type Frame struct {
	Media []byte            // Contains the media from the frame.
	PTS   uint64            // PTS from PES packet (this gives time relative from start of stream).
	ID    uint8             // StreamID from the PES packet, identifying media codec.
	Meta  map[string]string // Contains metadata from PMT relevant to this frame.
	idx   int               // Index in the backing slice.
}

// Frames returns the frames of a h264 clip.
func (c *Clip) Frames() []Frame {
	return c.frames
}

// Bytes returns the concatentated media bytes from each frame in the Clip c.
func (c *Clip) Bytes() []byte {
	if c.backing == nil {
		panic("the clip backing array cannot be nil")
	}
	return c.backing
}

// Errors used in TrimToPTSRange.
var (
	errPTSLowerBound = errors.New("PTS 'from' cannot be found")
	errPTSUpperBound = errors.New("PTS 'to' cannot be found")
	errPTSRange      = errors.New("PTS interval invalid")
)

// TrimToPTSRange returns the sub Clip in a PTS range defined by from and to.
// The first Frame in the new Clip will be the Frame for which from corresponds
// exactly with Frame.PTS, or the Frame in which from lies within. The final
// Frame in the Clip will be the previous of that for which to coincides with,
// or the Frame that to lies within.
func (c *Clip) TrimToPTSRange(from, to uint64) (*Clip, error) {
	// First check that the interval makes sense.
	if from >= to {
		return nil, errPTSRange
	}

	// Use binary search to find 'from'.
	n := len(c.frames) - 1
	startFrameIdx := sort.Search(
		n,
		func(i int) bool {
			if from < c.frames[i+1].PTS {
				return true
			}
			return false
		},
	)
	if startFrameIdx == n {
		return nil, errPTSLowerBound
	}

	// Now get the start index for the backing slice from this Frame.
	startBackingIdx := c.frames[startFrameIdx].idx

	// Now use binary search again to find 'to'.
	off := startFrameIdx + 1
	n = n - (off)
	endFrameIdx := sort.Search(
		n,
		func(i int) bool {
			if to <= c.frames[i+off].PTS {
				return true
			}
			return false
		},
	)
	if endFrameIdx == n {
		return nil, errPTSUpperBound
	}

	// Now get the end index for the backing slice from this Frame.
	endBackingIdx := c.frames[endFrameIdx+off-1].idx

	// Now return a new clip. NB: data is not copied.
	return &Clip{
		frames:  c.frames[startFrameIdx : endFrameIdx+1],
		backing: c.backing[startBackingIdx : endBackingIdx+len(c.frames[endFrameIdx+off].Media)],
	}, nil
}

// Errors that maybe returned from TrimToMetaRange.
var (
	errMetaRange      = errors.New("invalid meta range")
	errMetaLowerBound = errors.New("meta 'from' cannot be found")
	errMetaUpperBound = errors.New("meta 'to' cannot be found")
)

// TrimToMetaRange returns a sub Clip with meta range described by from and to
// with key 'key'. The meta values must not be equivalent.
func (c *Clip) TrimToMetaRange(key, from, to string) (*Clip, error) {
	// First check that the interval makes sense.
	if from == to {
		return nil, errMetaRange
	}

	var start, end int

	// Try and find from.
	for i := 0; i < len(c.frames); i++ {
		f := c.frames[i]
		startFrameIdx := i
		if f.Meta[key] == from {
			start = f.idx

			// Now try and find to.
			for ; i < len(c.frames); i++ {
				f = c.frames[i]
				if f.Meta[key] == to {
					end = f.idx
					endFrameIdx := i
					return &Clip{
						frames:  c.frames[startFrameIdx : endFrameIdx+1],
						backing: c.backing[start : end+len(f.Media)],
					}, nil
				}
			}
			return nil, errMetaUpperBound
		}
	}
	return nil, errMetaLowerBound
}

// SegmentForMeta segments sequences of frames within c possesing meta described
// by key and val and are appended to a []Clip which is subsequently returned.
func (c *Clip) SegmentForMeta(key, val string) []Clip {
	var (
		segmenting bool   // If true we are currently in a segment corresponsing to given meta.
		res        []Clip // The resultant [][]Clip holding the segments.
		start      int    // The start index of the current segment.
	)

	// Go through frames of clip.
	for i, frame := range c.frames {
		// If there is no meta (meta = nil) and we are segmenting, then append the
		// current segment to res.
		if frame.Meta == nil {
			if segmenting {
				res = appendSegment(res, c, start, i)
				segmenting = false
			}
			continue
		}

		// If we've got the meta of interest in current frame and we're not
		// segmenting, set this i as start and set segmenting true. If we don't
		// have the meta of interest and we are segmenting then we
		// want to stop and append the segment to res.
		if frame.Meta[key] == val && !segmenting {
			start = i
			segmenting = true
		} else if frame.Meta[key] != val && segmenting {
			res = appendSegment(res, c, start, i)
			segmenting = false
		}
	}

	// We've reached the end of the entire clip so if we're segmenting we need
	// to append current segment to res.
	if segmenting {
		res = appendSegment(res, c, start, len(c.frames))
	}

	return res
}

// appendSegment is a helper function used by Clip.SegmentForMeta to append a
// clip to a []Clip.
func appendSegment(segs []Clip, c *Clip, start, end int) []Clip {
	return append(segs, Clip{
		frames:  c.frames[start:end],
		backing: c.backing[c.frames[start].idx : c.frames[end-1].idx+len(c.frames[end-1].Media)],
	},
	)
}

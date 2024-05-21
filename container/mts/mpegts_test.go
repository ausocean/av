/*
NAME
  mpegts_test.go

DESCRIPTION
  mpegts_test.go contains testing for functionality found in mpegts.go.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package mts

import (
	"bytes"
	"math/rand"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/Comcast/gots/v2/packet"
	gotspsi "github.com/Comcast/gots/v2/psi"
	"github.com/pkg/errors"

	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/container/mts/pes"
	"github.com/ausocean/av/container/mts/psi"
	"github.com/ausocean/utils/logging"
)

// TestGetPTSRange checks that GetPTSRange can correctly get the first and last
// PTS in an MPEGTS clip for a general case.
func TestGetPTSRange1(t *testing.T) {
	const (
		numOfFrames  = 20
		maxFrameSize = 1000
		minFrameSize = 100
		rate         = 25                // fps
		interval     = float64(1) / rate // s
		ptsFreq      = 90000             // Hz
	)

	// Generate randomly sized data for each frame.
	rand.Seed(time.Now().UnixNano())
	frames := make([][]byte, numOfFrames)
	for i := range frames {
		size := rand.Intn(maxFrameSize-minFrameSize) + minFrameSize
		frames[i] = make([]byte, size)
	}

	var clip bytes.Buffer

	// Write the PSI first.
	err := writePSI(&clip)
	if err != nil {
		t.Fatalf("did not expect error writing psi: %v", err)
	}

	// Now write frames.
	var curTime float64
	for _, frame := range frames {
		nextPTS := curTime * ptsFreq

		err = writeFrame(&clip, frame, uint64(nextPTS))
		if err != nil {
			t.Fatalf("did not expect error writing frame: %v", err)
		}

		curTime += interval
	}

	got, err := GetPTSRange(clip.Bytes(), PIDVideo)
	if err != nil {
		t.Fatalf("did not expect error getting PTS range: %v", err)
	}

	want := [2]uint64{0, uint64((numOfFrames - 1) * interval * ptsFreq)}
	if got != want {
		t.Errorf("did not get expected result.\n Got: %v\n Want: %v\n", got, want)
	}
}

// writePSI is a helper function write the PSI found at the start of a clip.
func writePSI(b *bytes.Buffer) error {
	patBytes := psi.NewPATPSI().Bytes()
	pmtBytes := psi.NewPMTPSI().Bytes()
	// Write PAT.
	pat := Packet{
		PUSI:    true,
		PID:     PatPid,
		CC:      0,
		AFC:     HasPayload,
		Payload: psi.AddPadding(patBytes),
	}
	_, err := b.Write(pat.Bytes(nil))
	if err != nil {
		return err
	}

	// Write PMT.
	pmt := Packet{
		PUSI:    true,
		PID:     PmtPid,
		CC:      0,
		AFC:     HasPayload,
		Payload: psi.AddPadding(pmtBytes),
	}
	_, err = b.Write(pmt.Bytes(nil))
	if err != nil {
		return err
	}
	return nil
}

// writeFrame is a helper function used to form a PES packet from a frame, and
// then fragment this across MPEGTS packets where they are then written to the
// given buffer.
func writeFrame(b *bytes.Buffer, frame []byte, pts uint64) error {
	// Prepare PES data.
	pesPkt := pes.Packet{
		StreamID:     pes.H264SID,
		PDI:          hasPTS,
		PTS:          pts,
		Data:         frame,
		HeaderLength: 5,
	}
	buf := pesPkt.Bytes(nil)

	// Write PES data acroos MPEGTS packets.
	pusi := true
	for len(buf) != 0 {
		pkt := Packet{
			PUSI: pusi,
			PID:  PIDVideo,
			RAI:  pusi,
			CC:   0,
			AFC:  hasAdaptationField | hasPayload,
			PCRF: pusi,
		}
		n := pkt.FillPayload(buf)
		buf = buf[n:]

		pusi = false
		_, err := b.Write(pkt.Bytes(nil))
		if err != nil {
			return err
		}
	}
	return nil
}

// TestGetPTSRange2 checks that GetPTSRange behaves correctly with cases where
// the first instance of a PID is not a payload start, and also where there
// are no payload starts.
func TestGetPTSRange2(t *testing.T) {
	const (
		nPackets = 8 // The number of MTS packets we will generate.
		wantPID  = 1 // The PID we want.
	)
	tests := []struct {
		pusi []bool    // The value of PUSI for each packet.
		pid  []uint16  // The PIDs for each packet.
		pts  []uint64  // The PTS for each packet.
		want [2]uint64 // The wanted PTS from GetPTSRange.
		err  error     // The error we expect from GetPTSRange.
	}{
		{
			[]bool{false, false, false, true, false, false, true, false},
			[]uint16{0, 0, 1, 1, 1, 1, 1, 1},
			[]uint64{0, 0, 0, 1, 0, 0, 2, 0},
			[2]uint64{1, 2},
			nil,
		},
		{
			[]bool{false, false, false, true, false, false, false, false},
			[]uint16{0, 0, 1, 1, 1, 1, 1, 1},
			[]uint64{0, 0, 0, 1, 0, 0, 0, 0},
			[2]uint64{1, 1},
			nil,
		},
		{
			[]bool{false, false, false, false, false, false, false, false},
			[]uint16{0, 0, 1, 1, 1, 1, 1, 1},
			[]uint64{0, 0, 0, 0, 0, 0, 0, 0},
			[2]uint64{0, 0},
			errNoPTS,
		},
	}

	var clip bytes.Buffer

	for i, test := range tests {
		// Generate MTS packets for this test.
		clip.Reset()
		for j := 0; j < nPackets; j++ {
			pesPkt := pes.Packet{
				StreamID:     pes.H264SID,
				PDI:          hasPTS,
				PTS:          test.pts[j],
				Data:         []byte{},
				HeaderLength: 5,
			}
			buf := pesPkt.Bytes(nil)

			pkt := Packet{
				PUSI: test.pusi[j],
				PID:  test.pid[j],
				RAI:  true,
				CC:   0,
				AFC:  hasAdaptationField | hasPayload,
				PCRF: true,
			}
			pkt.FillPayload(buf)

			_, err := clip.Write(pkt.Bytes(nil))
			if err != nil {
				t.Fatalf("did not expect clip write error: %v", err)
			}
		}

		pts, err := GetPTSRange(clip.Bytes(), wantPID)
		if err != test.err {
			t.Errorf("did not get expected error for test: %v\nGot: %v\nWant: %v\n", i, err, test.err)
		}

		if pts != test.want {
			t.Errorf("did not get expected result for test: %v\nGot: %v\nWant: %v\n", i, pts, test.want)
		}
	}
}

// TestBytes checks that Packet.Bytes() correctly produces a []byte
// representation of a Packet.
func TestBytes(t *testing.T) {
	const payloadLen, payloadChar, stuffingChar = 120, 0x11, 0xff
	const stuffingLen = PacketSize - payloadLen - 12

	tests := []struct {
		packet         Packet
		expectedHeader []byte
	}{
		{
			packet: Packet{
				PUSI: true,
				PID:  1,
				RAI:  true,
				CC:   4,
				AFC:  HasPayload | HasAdaptationField,
				PCRF: true,
				PCR:  1,
			},
			expectedHeader: []byte{
				0x47,                               // Sync byte.
				0x40,                               // TEI=0, PUSI=1, TP=0, PID=00000.
				0x01,                               // PID(Cont)=00000001.
				0x34,                               // TSC=00, AFC=11(adaptation followed by payload), CC=0100(4).
				byte(7 + stuffingLen),              // AFL=.
				0x50,                               // DI=0,RAI=1,ESPI=0,PCRF=1,OPCRF=0,SPF=0,TPDF=0, AFEF=0.
				0x00, 0x00, 0x00, 0x00, 0x80, 0x00, // PCR.
			},
		},
	}

	for testNum, test := range tests {
		// Construct payload.
		payload := make([]byte, 0, payloadLen)
		for i := 0; i < payloadLen; i++ {
			payload = append(payload, payloadChar)
		}

		// Fill the packet payload.
		test.packet.FillPayload(payload)

		// Create expected packet data and copy in expected header.
		expected := make([]byte, len(test.expectedHeader), PacketSize)
		copy(expected, test.expectedHeader)

		// Append stuffing.
		for i := 0; i < stuffingLen; i++ {
			expected = append(expected, stuffingChar)
		}

		// Append payload to expected bytes.
		expected = append(expected, payload...)

		// Compare got with expected.
		got := test.packet.Bytes(nil)
		if !bytes.Equal(got, expected) {
			t.Errorf("did not get expected result for test: %v.\n Got: %v\n Want: %v\n", testNum, got, expected)
		}
	}
}

// TestFindPid checks that FindPid can correctly extract the first instance
// of a PID from an MPEG-TS stream.
func TestFindPid(t *testing.T) {
	const targetPacketNum, numOfPackets, targetPid, stdPid = 6, 15, 1, 0

	// Prepare the stream of packets.
	var stream []byte
	for i := 0; i < numOfPackets; i++ {
		pid := uint16(stdPid)
		if i == targetPacketNum {
			pid = targetPid
		}

		p := Packet{
			PID: pid,
			AFC: hasPayload | hasAdaptationField,
		}
		p.FillPayload([]byte{byte(i)})
		stream = append(stream, p.Bytes(nil)...)
	}

	// Try to find the targetPid in the stream.
	p, i, err := FindPid(stream, targetPid)
	if err != nil {
		t.Fatalf("unexpected error finding PID: %v\n", err)
	}

	// Check the payload.
	var _p packet.Packet
	copy(_p[:], p)
	payload, err := packet.Payload(&_p)
	if err != nil {
		t.Fatalf("unexpected error getting packet payload: %v\n", err)
	}
	got := payload[0]
	if got != targetPacketNum {
		t.Errorf("payload of found packet is not correct.\nGot: %v, Want: %v\n", got, targetPacketNum)
	}

	// Check the index.
	_got := i / PacketSize
	if _got != targetPacketNum {
		t.Errorf("index of found packet is not correct.\nGot: %v, want: %v\n", _got, targetPacketNum)
	}
}

// TestTrimToMetaRange checks that TrimToMetaRange can correctly return a segment
// of MPEG-TS corresponding to a meta interval in a slice of MPEG-TS.
func TestTrimToMetaRange(t *testing.T) {
	Meta = meta.New()

	const (
		nPSI = 10
		key  = "n"
	)

	var clip bytes.Buffer

	for i := 0; i < nPSI; i++ {
		Meta.Add(key, strconv.Itoa((i*2)+1))
		err := writePSIWithMeta(&clip, t)
		if err != nil {
			t.Fatalf("did not expect to get error writing PSI, error: %v", err)
		}
	}

	tests := []struct {
		from   string
		to     string
		expect []byte
		err    error
	}{
		{
			from:   "3",
			to:     "9",
			expect: clip.Bytes()[3*PacketSize : 10*PacketSize],
			err:    nil,
		},
		{
			from:   "30",
			to:     "8",
			expect: nil,
			err:    errMetaLowerBound,
		},
		{
			from:   "3",
			to:     "30",
			expect: nil,
			err:    errMetaUpperBound,
		},
	}

	// Run tests.
	for i, test := range tests {
		got, err := TrimToMetaRange(clip.Bytes(), key, test.from, test.to)

		// First check the error.
		if err != nil && err != test.err {
			t.Errorf("unexpected error: %v for test: %v", err, i)
			continue
		} else if err != test.err {
			t.Errorf("expected to get error: %v for test: %v", test.err, i)
			continue
		}

		// Now check data.
		if test.err == nil && !bytes.Equal(test.expect, got) {
			t.Errorf("did not get expected data for test: %v\n Got: %v\n, Want: %v\n", i, got, test.expect)
		}
	}
}

// TestSegmentForMeta checks that SegmentForMeta can correctly segment some MTS
// data based on a given meta key and value.
func TestSegmentForMeta(t *testing.T) {
	// Copyright information prefixed to all metadata.
	const (
		metaPreambleKey  = "copyright"
		metaPreambleData = "ausocean.org/license/content2019"
	)

	Meta = meta.NewWith([][2]string{{metaPreambleKey, metaPreambleData}})

	const (
		nPSI = 10  // The number of PSI pairs to write.
		key  = "n" // The meta key we will work with.
		val  = "*" // This is the meta value we will look for.
	)

	tests := []struct {
		metaVals   [nPSI]string // This represents the meta value for meta pairs (PAT and PMT)
		expectIdxs []rng        // This gives the expected index ranges for the segments.
	}{
		{
			metaVals: [nPSI]string{"1", "2", val, val, val, "3", val, val, "4", "4"},
			expectIdxs: []rng{
				scale(2, 5),
				scale(6, 8),
			},
		},
		{
			metaVals: [nPSI]string{"1", "2", val, val, val, "", "3", val, val, "4"},
			expectIdxs: []rng{
				scale(2, 5),
				scale(7, 9),
			},
		},
		{
			metaVals: [nPSI]string{"1", "2", val, val, val, "", "3", val, val, val},
			expectIdxs: []rng{
				scale(2, 5),
				{((7 * 2) + 1) * PacketSize, (nPSI * 2) * PacketSize},
			},
		},
		{
			metaVals:   [nPSI]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
			expectIdxs: nil,
		},
	}

	var clip bytes.Buffer

	for testn, test := range tests {
		// We want a clean buffer for each new test, so reset.
		clip.Reset()

		// Add meta and write PSI to clip.
		for i := 0; i < nPSI; i++ {
			if test.metaVals[i] != "" {
				Meta.Add(key, test.metaVals[i])
			} else {
				Meta.Delete(key)
			}
			err := writePSIWithMeta(&clip, t)
			if err != nil {
				t.Fatalf("did not expect to get error writing PSI, error: %v", err)
			}
		}

		// Now we get the expected segments using the index ranges from the test.
		var want [][]byte
		for _, idxs := range test.expectIdxs {
			want = append(want, clip.Bytes()[idxs.start:idxs.end])
		}

		// Now use the function we're testing to get the segments.
		got, err := SegmentForMeta(clip.Bytes(), key, val)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check that segments are OK.
		if !reflect.DeepEqual(want, got) {
			t.Errorf("did not get expected result for test %v\nGot: %v\nWant: %v\n", testn, got, want)
		}

		// Now test IndexPid.
		i, _, m, err := FindPSI(clip.Bytes())
		if err != nil {
			t.Fatalf("IndexPid failed with error: %v", err)
		}
		if i != 0 {
			t.Fatalf("IndexPid unexpected index; got %d, expected 0", i)
		}
		if m["n"] != "1" {
			t.Fatalf("IndexPid unexpected metadata; got %s, expected 1", m["n"])
		}
	}

	// Finally, test IndexPid error handling.
	for _, d := range [][]byte{[]byte{}, make([]byte, PacketSize/2), make([]byte, PacketSize)} {
		_, _, _, err := FindPSI(d)
		if err == nil {
			t.Fatalf("IndexPid expected error")
		}
	}
}

// rng describes an index range and is used by TestSegmentForMeta.
type rng struct {
	start int
	end   int
}

// scale takes a PSI index (i.e. first PSI is 0, next is 1) and modifies to be
// the index of the first byte of the PSI pair (PAT and PMT) in the byte stream.
// This assumes there are only PSI written consequitively, and is used by
// TestSegmentForMeta.
func scale(x, y int) rng {
	return rng{
		((x * 2) + 1) * PacketSize,
		((y * 2) + 1) * PacketSize,
	}
}

func TestFindPSI(t *testing.T) {
	const (
		pat = iota
		pmt
		media
	)

	const (
		metaKey   = "key"
		mediaType = gotspsi.PmtStreamTypeMpeg4Video
		pmtPID    = 3
		streamPID = 4
	)

	type want struct {
		idx        int
		streamType uint8
		streamPID  uint16
		meta       map[string]string
		err        error
	}

	tests := []struct {
		pkts []int
		meta string
		want want
	}{
		{
			pkts: []int{pat, pmt, media, media},
			meta: "1",
			want: want{
				idx:        0,
				streamType: gotspsi.PmtStreamTypeMpeg4Video,
				streamPID:  4,
				meta: map[string]string{
					"key": "1",
				},
				err: nil,
			},
		},
		{
			pkts: []int{media, pat, pmt, media, media},
			meta: "1",
			want: want{
				idx:        188,
				streamType: gotspsi.PmtStreamTypeMpeg4Video,
				streamPID:  4,
				meta: map[string]string{
					"key": "1",
				},
				err: nil,
			},
		},
		{
			pkts: []int{pat, media, pmt, media, media},
			meta: "1",
			want: want{
				idx:        0,
				streamType: gotspsi.PmtStreamTypeMpeg4Video,
				streamPID:  4,
				meta: map[string]string{
					"key": "1",
				},
				err: ErrNotConsecutive,
			},
		},
	}

	var clip bytes.Buffer
	var err error
	Meta = meta.New()

	for i, test := range tests {
		// Generate MTS packets for this test.
		clip.Reset()

		for _, pkt := range test.pkts {
			switch pkt {
			case pat:
				patTable := (&psi.PSI{
					PointerField:    0x00,
					TableID:         0x00,
					SyntaxIndicator: true,
					PrivateBit:      false,
					SectionLen:      0x0d,
					SyntaxSection: &psi.SyntaxSection{
						TableIDExt:  0x01,
						Version:     0,
						CurrentNext: true,
						Section:     0,
						LastSection: 0,
						SpecificData: &psi.PAT{
							Program:       0x01,
							ProgramMapPID: pmtPID,
						},
					},
				}).Bytes()

				pat := Packet{
					PUSI:    true,
					PID:     PatPid,
					CC:      0,
					AFC:     HasPayload,
					Payload: psi.AddPadding(patTable),
				}
				_, err := clip.Write(pat.Bytes(nil))
				if err != nil {
					t.Fatalf("could not write PAT to clip for test %d", i)
				}
			case pmt:
				pmtTable := (&psi.PSI{
					PointerField:    0x00,
					TableID:         0x02,
					SyntaxIndicator: true,
					SectionLen:      0x12,
					SyntaxSection: &psi.SyntaxSection{
						TableIDExt:  0x01,
						Version:     0,
						CurrentNext: true,
						Section:     0,
						LastSection: 0,
						SpecificData: &psi.PMT{
							ProgramClockPID: 0x0100,
							ProgramInfoLen:  0,
							StreamSpecificData: &psi.StreamSpecificData{
								StreamType:    mediaType,
								PID:           streamPID,
								StreamInfoLen: 0x00,
							},
						},
					},
				}).Bytes()

				Meta.Add(metaKey, test.meta)
				pmtTable, err = updateMeta(pmtTable, (*logging.TestLogger)(t))
				if err != nil {
					t.Fatalf("could not update meta for test %d", i)
				}

				pmt := Packet{
					PUSI:    true,
					PID:     pmtPID,
					CC:      0,
					AFC:     HasPayload,
					Payload: psi.AddPadding(pmtTable),
				}
				_, err = clip.Write(pmt.Bytes(nil))
				if err != nil {
					t.Fatalf("could not write PMT to clip for test %d", i)
				}
			case media:
				pesPkt := pes.Packet{
					StreamID:     mediaType,
					PDI:          hasPTS,
					Data:         []byte{},
					HeaderLength: 5,
				}
				buf := pesPkt.Bytes(nil)

				pkt := Packet{
					PUSI: true,
					PID:  uint16(streamPID),
					RAI:  true,
					CC:   0,
					AFC:  hasAdaptationField | hasPayload,
					PCRF: true,
				}
				pkt.FillPayload(buf)

				_, err := clip.Write(pkt.Bytes(nil))
				if err != nil {
					t.Fatalf("did not expect clip write error: %v", err)
				}
			default:
				t.Fatalf("undefined pkt type %d in test %d", pkt, i)
			}
		}

		gotIdx, gotStreams, gotMeta, gotErr := FindPSI(clip.Bytes())

		// Check error.
		if errors.Cause(gotErr) != test.want.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v\n", i, gotErr, test.want.err)
		}

		if gotErr == nil {
			// Check idx
			if gotIdx != test.want.idx {
				t.Errorf("did not get expected idx for test %d\nGot: %v\nWant: %v\n", i, gotIdx, test.want.idx)
			}

			// Check stream type and PID
			if gotStreams == nil {
				t.Fatalf("gotStreams should not be nil")
			}

			if len(gotStreams) == 0 {
				t.Fatalf("gotStreams should not be 0 length")
			}

			var (
				gotStreamPID  uint16
				gotStreamType uint8
			)

			for k, v := range gotStreams {
				gotStreamPID = k
				gotStreamType = v
			}

			if gotStreamType != test.want.streamType {
				t.Errorf("did not get expected stream type for test %d\nGot: %v\nWant: %v\n", i, gotStreamType, test.want.streamType)
			}

			if gotStreamPID != test.want.streamPID {
				t.Errorf("did not get expected stream PID for test %d\nGot: %v\nWant: %v\n", i, gotStreamPID, test.want.streamPID)
			}

			// Check meta
			if !reflect.DeepEqual(gotMeta, test.want.meta) {
				t.Errorf("did not get expected meta for test %d\nGot: %v\nWant: %v\n", i, gotMeta, test.want.meta)
			}
		}
	}
}

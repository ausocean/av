/*
NAME
  payload_test.go

DESCRIPTION
  payload_test.go provides testing to validate utilities found in payload.go.

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
	"bytes"
	"math/rand"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/container/mts/pes"
	"github.com/ausocean/av/container/mts/psi"
	"github.com/ausocean/utils/logging"
)

// TestExtract checks that we can coorectly extract media, pts, id and meta from
// an MPEGTS stream using Extract.
func TestExtract(t *testing.T) {
	Meta = meta.New()

	const (
		psiInterval  = 5                 // Write PSI at start and after every 5 frames.
		numOfFrames  = 30                // Total number of frames to write.
		maxFrameSize = 1000              // Max frame size to randomly generate.
		minFrameSize = 100               // Min frame size to randomly generate.
		rate         = 25                // Framerate (fps)
		interval     = float64(1) / rate // Time interval between frames.
		ptsFreq      = 90000             // Standard PTS frequency base.
	)

	frames := genFrames(numOfFrames, minFrameSize, maxFrameSize)

	var (
		clip bytes.Buffer // This will hold the MPEG-TS data.
		want Clip         // This is the Clip that we should get.
		err  error
	)

	// Now write frames.
	var curTime float64
	for i, frame := range frames {
		// Check to see if it's time to write another lot of PSI.
		if i%psiInterval == 0 && i != len(frames)-1 {
			// We'll add the frame number as meta.
			Meta.Add("frameNum", strconv.Itoa(i))

			err = writePSIWithMeta(&clip, t)
			if err != nil {
				t.Fatalf("did not expect error writing psi: %v", err)
			}
		}
		nextPTS := uint64(curTime * ptsFreq)

		err = writeFrame(&clip, frame, uint64(nextPTS))
		if err != nil {
			t.Fatalf("did not expect error writing frame: %v", err)
		}

		curTime += interval

		// Need the meta map for the new expected Frame.
		metaMap, err := meta.GetAllAsMap(Meta.Encode())
		if err != nil {
			t.Fatalf("did not expect error getting meta map: %v", err)
		}

		// Create an equivalent Frame and append to our Clip want.
		want.frames = append(want.frames, Frame{
			Media: frame,
			PTS:   nextPTS,
			ID:    pes.H264SID,
			Meta:  metaMap,
		})
	}

	// Now use Extract to get frames from clip.
	got, err := Extract(clip.Bytes())
	if err != nil {
		t.Fatalf("did not expect error using Extract. Err: %v", err)
	}

	// Check length of got and want.
	if len(want.frames) != len(got.frames) {
		t.Fatalf("did not get expected length for got.\nGot: %v\n, Want: %v\n", len(got.frames), len(want.frames))
	}

	// Check frames individually.
	for i, frame := range want.frames {
		// Check media data.
		wantMedia := frame.Media
		gotMedia := got.frames[i].Media
		if !bytes.Equal(wantMedia, gotMedia) {
			t.Fatalf("did not get expected data for frame: %v\nGot: %v\nWant: %v\n", i, gotMedia, wantMedia)
		}

		// Check stream ID.
		wantID := frame.ID
		gotID := got.frames[i].ID
		if wantID != gotID {
			t.Fatalf("did not get expected ID for frame: %v\nGot: %v\nWant: %v\n", i, gotID, wantID)
		}

		// Check meta.
		wantMeta := frame.Meta
		gotMeta := got.frames[i].Meta
		if !reflect.DeepEqual(wantMeta, gotMeta) {
			t.Fatalf("did not get expected meta for frame: %v\nGot: %v\nwant: %v\n", i, gotMeta, wantMeta)
		}
	}
}

// writePSIWithMeta writes PSI to b with updated metadata.
func writePSIWithMeta(b *bytes.Buffer, t *testing.T) error {
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

	// Update the meta in the pmt table.
	pmtBytes, err = updateMeta(pmtBytes, (*logging.TestLogger)(t))
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

// TestClipBytes checks that Clip.Bytes correctly returns the concatendated media
// data from the Clip's frames slice.
func TestClipBytes(t *testing.T) {
	Meta = meta.New()

	const (
		psiInterval  = 5                 // Write PSI at start and after every 5 frames.
		numOfFrames  = 30                // Total number of frames to write.
		maxFrameSize = 1000              // Max frame size to randomly generate.
		minFrameSize = 100               // Min frame size to randomly generate.
		rate         = 25                // Framerate (fps)
		interval     = float64(1) / rate // Time interval between frames.
		ptsFreq      = 90000             // Standard PTS frequency base.
	)

	frames := genFrames(numOfFrames, minFrameSize, maxFrameSize)

	var (
		clip bytes.Buffer // This will hold the MPEG-TS data.
		want []byte       // This is the Clip that we should get.
		err  error
	)

	// Now write frames.
	var curTime float64
	for i, frame := range frames {
		// Check to see if it's time to write another lot of PSI.
		if i%psiInterval == 0 && i != len(frames)-1 {
			// We'll add the frame number as meta.
			Meta.Add("frameNum", strconv.Itoa(i))

			err = writePSIWithMeta(&clip, t)
			if err != nil {
				t.Fatalf("did not expect error writing psi: %v", err)
			}
		}
		nextPTS := uint64(curTime * ptsFreq)

		err = writeFrame(&clip, frame, uint64(nextPTS))
		if err != nil {
			t.Fatalf("did not expect error writing frame: %v", err)
		}

		curTime += interval

		// Append the frame straight to the expected pure media slice.
		want = append(want, frame...)
	}

	// Now use Extract to get Clip and then use Bytes to get the slice of straight media.
	gotClip, err := Extract(clip.Bytes())
	if err != nil {
		t.Fatalf("did not expect error using Extract. Err: %v", err)
	}
	got := gotClip.Bytes()

	// Check length and equality of got and want.
	if len(want) != len(got) {
		t.Fatalf("did not get expected length for got.\nGot: %v\n, Want: %v\n", len(got), len(want))
	}
	if !bytes.Equal(want, got) {
		t.Error("did not get expected result")
	}
}

// genFrames is a helper function to generate a series of dummy media frames
// with randomized size. n is the number of frames to generate, min is the min
// size is min size of random frame and max is max size of random frames.
func genFrames(n, min, max int) [][]byte {
	// Generate randomly sized data for each frame and fill.
	rand.Seed(time.Now().UnixNano())
	frames := make([][]byte, n)
	for i := range frames {
		frames[i] = make([]byte, rand.Intn(max-min)+min)
		for j := 0; j < len(frames[i]); j++ {
			frames[i][j] = byte(j)
		}
	}
	return frames
}

// TestTrimToPTSRange checks that Clip.TrimToPTSRange will correctly return a
// sub Clip of the given PTS range.
func TestTrimToPTSRange(t *testing.T) {
	const (
		numOfTestFrames = 10
		ptsInterval     = 4
		frameSize       = 3
	)

	clip := &Clip{}

	// Generate test frames.
	for i := 0; i < numOfTestFrames; i++ {
		clip.backing = append(clip.backing, []byte{byte(i), byte(i), byte(i)}...)
		clip.frames = append(
			clip.frames,
			Frame{
				Media: clip.backing[i*frameSize : (i+1)*frameSize],
				PTS:   uint64(i * ptsInterval),
				idx:   i * frameSize,
			},
		)
	}

	// We test each of these scenarios.
	tests := []struct {
		from   uint64
		to     uint64
		expect []byte
		err    error
	}{
		{
			from: 6,
			to:   15,
			expect: []byte{
				0x01, 0x01, 0x01,
				0x02, 0x02, 0x02,
				0x03, 0x03, 0x03,
			},
			err: nil,
		},
		{
			from: 4,
			to:   16,
			expect: []byte{
				0x01, 0x01, 0x01,
				0x02, 0x02, 0x02,
				0x03, 0x03, 0x03,
			},
			err: nil,
		},
		{
			from:   10,
			to:     5,
			expect: nil,
			err:    errPTSRange,
		},
		{
			from:   50,
			to:     70,
			expect: nil,
			err:    errPTSLowerBound,
		},
		{
			from:   5,
			to:     70,
			expect: nil,
			err:    errPTSUpperBound,
		},
	}

	// Run tests.
	for i, test := range tests {
		got, err := clip.TrimToPTSRange(test.from, test.to)

		// First check the error.
		if err != nil && err != test.err {
			t.Errorf("unexpected error: %v for test: %v", err, i)
			continue
		} else if err != test.err {
			t.Errorf("expected to get error: %v for test: %v", test.err, i)
			continue
		}

		// Now check data.
		if test.err == nil && !bytes.Equal(test.expect, got.Bytes()) {
			t.Errorf("did not get expected data for test: %v\n Got: %v\n, Want: %v\n", i, got, test.expect)
		}
	}
}

// TestTrimToMetaRange checks that Clip.TrimToMetaRange correctly provides a
// sub Clip for a given meta range.
func TestClipTrimToMetaRange(t *testing.T) {
	const (
		numOfTestFrames = 10
		ptsInterval     = 4
		frameSize       = 3
		key             = "n"
	)

	clip := &Clip{}

	// Generate test frames.
	for i := 0; i < numOfTestFrames; i++ {
		clip.backing = append(clip.backing, []byte{byte(i), byte(i), byte(i)}...)
		clip.frames = append(
			clip.frames,
			Frame{
				Media: clip.backing[i*frameSize : (i+1)*frameSize],
				idx:   i * frameSize,
				Meta: map[string]string{
					key: strconv.Itoa(i),
				},
			},
		)
	}

	// We test each of these scenarios.
	tests := []struct {
		from   string
		to     string
		expect []byte
		err    error
	}{
		{
			from: "1",
			to:   "3",
			expect: []byte{
				0x01, 0x01, 0x01,
				0x02, 0x02, 0x02,
				0x03, 0x03, 0x03,
			},
			err: nil,
		},
		{
			from:   "1",
			to:     "1",
			expect: nil,
			err:    errMetaRange,
		},
		{
			from:   "20",
			to:     "1",
			expect: nil,
			err:    errMetaLowerBound,
		},
		{
			from:   "1",
			to:     "20",
			expect: nil,
			err:    errMetaUpperBound,
		},
	}

	// Run tests.
	for i, test := range tests {
		got, err := clip.TrimToMetaRange(key, test.from, test.to)

		// First check the error.
		if err != nil && err != test.err {
			t.Errorf("unexpected error: %v for test: %v", err, i)
			continue
		} else if err != test.err {
			t.Errorf("expected to get error: %v for test: %v", test.err, i)
			continue
		}

		// Now check data.
		if test.err == nil && !bytes.Equal(test.expect, got.Bytes()) {
			t.Errorf("did not get expected data for test: %v\n Got: %v\n, Want: %v\n", i, got, test.expect)
		}
	}
}

// TestClipSegmentForMeta checks that Clip.SegmentForMeta correctly returns
// segments from a clip with consistent meta defined by a key and value.
func TestClipSegmentForMeta(t *testing.T) {
	const (
		nFrames = 10  // The number of test frames we want to create.
		fSize   = 3   // The size of the frame media.
		key     = "n" // Meta key we will use.
		val     = "*" // The meta val of interest.
	)

	tests := []struct {
		metaVals []string // These will be the meta vals each frame has.
		fIndices []rng    // These are the indices of the segments of interest.
	}{
		{
			metaVals: []string{"1", "2", "*", "*", "*", "3", "*", "*", "4", "5"},
			fIndices: []rng{{2, 5}, {6, 8}},
		},
		{
			metaVals: []string{"1", "2", "*", "*", "*", "", "*", "*", "4", "5"},
			fIndices: []rng{{2, 5}, {6, 8}},
		},
		{
			metaVals: []string{"1", "2", "*", "*", "*", "3", "4", "5", "*", "*"},
			fIndices: []rng{{2, 5}, {8, nFrames}},
		},
		{
			metaVals: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
			fIndices: nil,
		},
	}

	// Run the tests.
	for testn, test := range tests {
		clip := &Clip{}

		// Generate test frames.
		for i := 0; i < nFrames; i++ {
			clip.backing = append(clip.backing, []byte{byte(i), byte(i), byte(i)}...)
			clip.frames = append(
				clip.frames,
				Frame{
					Media: clip.backing[i*fSize : (i+1)*fSize],
					idx:   i * fSize,
					Meta: map[string]string{
						key: test.metaVals[i],
					},
				},
			)
		}

		// Use function we're testing to get segments.
		got := clip.SegmentForMeta(key, val)

		// Now get expected segments using indices defined in the test.
		var want []Clip
		for _, indices := range test.fIndices {
			// Calculate the indices for the backing array from the original clip.
			backStart := clip.frames[indices.start].idx
			backEnd := clip.frames[indices.end-1].idx + len(clip.frames[indices.end-1].Media)

			// Use calculated indices to create Clip for current expected segment.
			want = append(want, Clip{
				frames:  clip.frames[indices.start:indices.end],
				backing: clip.backing[backStart:backEnd],
			})
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("did not get expected result for test %v\nGot: %v\nWant: %v\n", testn, got, want)
		}
	}
}

/*
NAME
  mtsSender_test.go

DESCRIPTION
  mtsSender_test.go contains tests that validate the functionalilty of the
  mtsSender under senders.go. Tests include checks that the mtsSender is
  segmenting sends correctly, and also that it can correct discontinuities.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package revid

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Comcast/gots/v2/packet"
	"github.com/Comcast/gots/v2/pes"

	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/utils/pool"
)

var (
	errSendFailed = errors.New("send failed")
)

// destination simulates a destination for the mtsSender. It allows for the
// emulation of failed and delayed sends.
type destination struct {
	// Holds the clips written to this destination using Write.
	buf [][]byte

	// testFails is set to true if we would like a write to fail at a particular
	// clip as determined by failAt.
	testFails bool
	failAt    int

	// Holds the current clip number.
	currentClip int

	// Pointer to the testing.T of a test where this struct is being used. This
	// is used so that logging can be done through the testing log utilities.
	t *testing.T

	// sendDelay is the amount of time we would like a Write to be delayed when
	// we hit the clip number indicated by delayAt.
	sendDelay time.Duration
	delayAt   int

	// done will be used to send a signal to the main routine to indicate that
	// the destination has received all clips. doneAt indicates the final clip
	// number.
	done   chan struct{}
	doneAt int
}

func (ts *destination) Write(d []byte) (int, error) {
	ts.t.Log("writing clip to destination")
	if ts.delayAt != 0 && ts.currentClip == ts.delayAt {
		time.Sleep(ts.sendDelay)
	}
	if ts.testFails && ts.currentClip == ts.failAt {
		ts.t.Log("failed send")
		ts.currentClip++
		return 0, errSendFailed
	}
	cpy := make([]byte, len(d))
	copy(cpy, d)
	ts.buf = append(ts.buf, cpy)
	if ts.currentClip == ts.doneAt {
		close(ts.done)
	}
	ts.currentClip++
	return len(d), nil
}

func (ts *destination) Close() error { return nil }

// TestSegment ensures that the mtsSender correctly segments data into clips
// based on positioning of PSI in the mtsEncoder's output stream.
func TestMTSSenderSegment(t *testing.T) {
	// Skip this tests in Circle CI.
	// TODO: Rewrite tests/determine testing failure.
	if _, CI := os.LookupEnv("CI"); CI {
		t.Skip("Skipped faulty test in CI")
	}
	mts.Meta = meta.New()

	// Create ringBuffer, sender, sender and the MPEGTS encoder.
	const numberOfClips = 11
	dst := &destination{t: t, done: make(chan struct{}), doneAt: numberOfClips}
	const testPoolCapacity = 50000000
	nElements := testPoolCapacity / poolStartingElementSize
	sender := newMTSSender(dst, (*testLogger)(t), pool.NewBuffer(nElements, poolStartingElementSize, 0), 0)

	const psiSendCount = 10
	encoder, err := mts.NewEncoder(sender, (*testLogger)(t), mts.PacketBasedPSI(psiSendCount), mts.Rate(25), mts.MediaType(mts.EncodeH264))
	if err != nil {
		t.Fatalf("could not create MTS encoder, failed with error: %v", err)
	}

	// Write the packets to the encoder, which will in turn write to the mtsSender.
	// Payload will just be packet number.
	t.Log("writing packets")
	const noOfPacketsToWrite = 100
	for i := 0; i < noOfPacketsToWrite; i++ {
		encoder.Write([]byte{byte(i)})
	}

	// Wait until the destination has all the data, then close the sender.
	<-dst.done
	sender.Close()

	// Check the data.
	result := dst.buf
	expectData := 0
	for clipNo, clip := range result {
		t.Logf("Checking clip: %v\n", clipNo)

		// Check that the clip is of expected length.
		clipLen := len(clip)
		if clipLen != psiSendCount*mts.PacketSize {
			t.Fatalf("Clip %v is not correct length. Got: %v Want: %v\n Clip: %v\n", clipNo, clipLen, psiSendCount*mts.PacketSize, clip)
		}

		// Also check that the first packet is a PAT.
		firstPkt := clip[:mts.PacketSize]
		var pkt packet.Packet
		copy(pkt[:], firstPkt)
		pid := pkt.PID()
		if pid != mts.PatPid {
			t.Fatalf("First packet of clip %v is not pat, but rather: %v\n", clipNo, pid)
		}

		// Check that the clip data is okay.
		t.Log("checking clip data")
		for i := 0; i < len(clip); i += mts.PacketSize {
			copy(pkt[:], clip[i:i+mts.PacketSize])
			if pkt.PID() == mts.PIDVideo {
				t.Log("got video PID")
				payload, err := pkt.Payload()
				if err != nil {
					t.Fatalf("Unexpected err: %v\n", err)
				}

				// Parse PES from the MTS payload.
				pes, err := pes.NewPESHeader(payload)
				if err != nil {
					t.Fatalf("Unexpected err: %v\n", err)
				}

				// Get the data from the PES packet and convert to an int.
				data := int8(pes.Data()[0])

				// Calc expected data in the PES and then check.
				if data != int8(expectData) {
					t.Errorf("Did not get expected pkt data. ClipNo: %v, pktNoInClip: %v, Got: %v, want: %v\n", clipNo, i/mts.PacketSize, data, expectData)
				}
				expectData++
			}
		}
	}
}

// TestMtsSenderFailedSend checks that a failed send is correctly handled by
// the mtsSender. The mtsSender should try to send the same clip again.
func TestMtsSenderFailedSend(t *testing.T) {
	t.Skip("Skipped TestMtsSenderFailedSend") // TODO: Fix this test.
	// Skip this tests in Circle CI.
	// TODO: Rewrite tests/determine testing failure.
	if _, CI := os.LookupEnv("CI"); CI {
		t.Skip("Skipped faulty test in CI")
	}
	mts.Meta = meta.New()

	// Create destination, the mtsSender and the mtsEncoder
	const clipToFailAt = 3
	dst := &destination{t: t, testFails: true, failAt: clipToFailAt, done: make(chan struct{})}
	const testPoolCapacity = 50000000 // 50MB
	nElements := testPoolCapacity / poolStartingElementSize
	sender := newMTSSender(dst, (*testLogger)(t), pool.NewBuffer(nElements, poolStartingElementSize, 0), 0)

	const psiSendCount = 10
	encoder, err := mts.NewEncoder(sender, (*testLogger)(t), mts.PacketBasedPSI(psiSendCount), mts.Rate(25), mts.MediaType(mts.EncodeH264))
	if err != nil {
		t.Fatalf("could not create MTS encoder, failed with error: %v", err)
	}

	// Write the packets to the encoder, which will in turn write to the mtsSender.
	// Payload will just be packet number.
	t.Log("writing packets")
	const noOfPacketsToWrite = 100
	for i := 0; i < noOfPacketsToWrite; i++ {
		_, err := encoder.Write([]byte{byte(i)})
		if err != nil {
			t.Errorf("did not expect error from encoder write: %v", err)
		}
	}

	// Wait until the destination has all the data, then close the sender.
	<-dst.done
	sender.Close()

	// Check that we have data as expected.
	result := dst.buf
	expectData := 0
	for clipNo, clip := range result {
		t.Logf("Checking clip: %v\n", clipNo)

		// Check that the clip is of expected length.
		clipLen := len(clip)
		if clipLen != psiSendCount*mts.PacketSize {
			t.Fatalf("Clip %v is not correct length. Got: %v Want: %v\n Clip: %v\n", clipNo, clipLen, psiSendCount*mts.PacketSize, clip)
		}

		// Also check that the first packet is a PAT.
		firstPkt := clip[:mts.PacketSize]
		var pkt packet.Packet
		copy(pkt[:], firstPkt)
		pid := pkt.PID()
		if pid != mts.PatPid {
			t.Fatalf("First packet of clip %v is not pat, but rather: %v\n", clipNo, pid)
		}

		// Check that the clip data is okay.
		t.Log("checking clip data")
		for i := 0; i < len(clip); i += mts.PacketSize {
			copy(pkt[:], clip[i:i+mts.PacketSize])
			if pkt.PID() == mts.PIDVideo {
				t.Log("got video PID")
				payload, err := pkt.Payload()
				if err != nil {
					t.Fatalf("Unexpected err: %v\n", err)
				}

				// Parse PES from the MTS payload.
				pes, err := pes.NewPESHeader(payload)
				if err != nil {
					t.Fatalf("Unexpected err: %v\n", err)
				}

				// Get the data from the PES packet and convert to an int.
				data := int8(pes.Data()[0])

				// Calc expected data in the PES and then check.
				if data != int8(expectData) {
					t.Errorf("Did not get expected pkt data. ClipNo: %v, pktNoInClip: %v, Got: %v, want: %v\n", clipNo, i/mts.PacketSize, data, expectData)
				}
				expectData++
			}
		}
	}
}

// TestMtsSenderDiscontinuity checks that a discontinuity in a stream is
// correctly handled by the mtsSender. A discontinuity is caused by overflowing
// the mtsSender's ringBuffer. It is expected that the next clip seen has the
// disconinuity indicator applied.
func TestMtsSenderDiscontinuity(t *testing.T) {
	// Skip this tests in Circle CI.
	// TODO: Rewrite tests/determine testing failure.
	if _, CI := os.LookupEnv("CI"); CI {
		t.Skip("Skipped faulty test in CI")
	}
	mts.Meta = meta.New()

	// Create destination, the mtsSender and the mtsEncoder.
	const clipToDelay = 3
	dst := &destination{t: t, sendDelay: 10 * time.Millisecond, delayAt: clipToDelay, done: make(chan struct{})}
	sender := newMTSSender(dst, (*testLogger)(t), pool.NewBuffer(1, poolStartingElementSize, 0), 0)

	const psiSendCount = 10
	encoder, err := mts.NewEncoder(sender, (*testLogger)(t), mts.PacketBasedPSI(psiSendCount), mts.Rate(25), mts.MediaType(mts.EncodeH264))
	if err != nil {
		t.Fatalf("could not create MTS encoder, failed with error: %v", err)
	}

	// Write the packets to the encoder, which will in turn write to the mtsSender.
	// Payload will just be packet number.
	const noOfPacketsToWrite = 100
	for i := 0; i < noOfPacketsToWrite; i++ {
		_, err := encoder.Write([]byte{byte(i)})
		if err != nil {
			t.Errorf("did not expect error from encoder write: %v", err)
		}
	}

	// Wait until the destination has all the data, then close the sender.
	<-dst.done
	sender.Close()

	// Check the data.
	result := dst.buf
	expectedCC := 0
	for clipNo, clip := range result {
		t.Logf("Checking clip: %v\n", clipNo)

		// Check that the clip is of expected length.
		clipLen := len(clip)
		if clipLen != psiSendCount*mts.PacketSize {
			t.Fatalf("Clip %v is not correct length. Got: %v Want: %v\n Clip: %v\n", clipNo, clipLen, psiSendCount*mts.PacketSize, clip)
		}

		// Also check that the first packet is a PAT.
		firstPkt := clip[:mts.PacketSize]
		var pkt packet.Packet
		copy(pkt[:], firstPkt)
		pid := pkt.PID()
		if pid != mts.PatPid {
			t.Fatalf("First packet of clip %v is not pat, but rather: %v\n", clipNo, pid)
		}

		// Get the discontinuity indicator
		discon, _ := (*packet.AdaptationField)(&pkt).Discontinuity()

		// Check the continuity counter.
		cc := pkt.ContinuityCounter()
		if cc != expectedCC {
			t.Log("discontinuity found")
			expectedCC = cc
			if !discon {
				t.Errorf("discontinuity indicator not set where expected for clip: %v", clipNo)
			}
		} else {
			if discon && clipNo != 0 {
				t.Errorf("did not expect discontinuity indicator to be set for clip: %v", clipNo)
			}
		}
		expectedCC = (expectedCC + 1) & 0xf
	}
}

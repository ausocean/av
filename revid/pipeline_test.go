/*
NAME
  revid_test.go

DESCRIPTION
  See Readme.md

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package revid

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/client/pi/netsender"
)

const raspividPath = "/usr/local/bin/raspivid"

// Suppress all test logging, except for t.Errorf output.
var silent = true

// TestRaspivid tests that raspivid starts correctly.
// It is intended to be run on a Raspberry Pi.
func TestRaspivid(t *testing.T) {
	if _, err := os.Stat(raspividPath); os.IsNotExist(err) {
		t.Skip("Skipping TestRaspivid since no raspivid found.")
	}

	var logger testLogger
	ns, err := netsender.New(&logger, nil, nil, nil)
	if err != nil {
		t.Errorf("netsender.New failed with error %v", err)
	}

	var c config.Config
	c.Logger = &logger
	c.Input = config.InputRaspivid

	rv, err := New(c, ns)
	if err != nil {
		t.Errorf("revid.New failed with error %v", err)
	}

	err = rv.Start()
	if err != nil {
		t.Errorf("revid.Start failed with error %v", err)
	}
}

// tstMtsEncoder emulates the mts.Encoder to the extent of the dst field.
// This will allow access to the dst to check that it has been set corrctly.
type tstMtsEncoder struct {
	// dst is here solely to detect the type stored in the encoder.
	// No data is written to dst.
	dst io.WriteCloser
}

func (e *tstMtsEncoder) Write(d []byte) (int, error) { return len(d), nil }
func (e *tstMtsEncoder) Close() error                { return nil }

// tstFlvEncoder emulates the flv.Encoder to the extent of the dst field.
// This will allow access to the dst to check that it has been set corrctly.
type tstFlvEncoder struct {
	// dst is here solely to detect the type stored in the encoder.
	// No data is written to dst.
	dst io.WriteCloser
}

func (e *tstFlvEncoder) Write(d []byte) (int, error) { return len(d), nil }
func (e *tstFlvEncoder) Close() error                { return nil }

// dummyMultiWriter emulates the MultiWriter provided by std lib, so that we
// can access the destinations.
type dummyMultiWriter struct {
	// dst is here solely to detect the types stored in the multiWriter.
	// No data is written to dst.
	dst []io.WriteCloser
}

func (w *dummyMultiWriter) Write(d []byte) (int, error) { return len(d), nil }
func (w *dummyMultiWriter) Close() error                { return nil }

// TestResetEncoderSenderSetup checks that revid.reset() correctly sets up the
// revid.encoder slice and the senders the encoders write to.
func TestResetEncoderSenderSetup(t *testing.T) {
	// We will use these to indicate types after assertion.
	const (
		mtsSenderStr  = "*revid.mtsSender"
		rtpSenderStr  = "*revid.rtpSender"
		rtmpSenderStr = "*revid.rtmpSender"
		mtsEncoderStr = "*revid.tstMtsEncoder"
		flvEncoderStr = "*revid.tstFlvEncoder"
	)

	// Struct that will be used to format test cases nicely below.
	type encoder struct {
		encoderType  string
		destinations []string
	}

	tests := []struct {
		outputs  []uint8
		encoders []encoder
	}{
		{
			outputs: []uint8{config.OutputHTTP},
			encoders: []encoder{
				{
					encoderType:  mtsEncoderStr,
					destinations: []string{mtsSenderStr},
				},
			},
		},
		{
			outputs: []uint8{config.OutputRTMP},
			encoders: []encoder{
				{
					encoderType:  flvEncoderStr,
					destinations: []string{rtmpSenderStr},
				},
			},
		},
		{
			outputs: []uint8{config.OutputRTP},
			encoders: []encoder{
				{
					encoderType:  mtsEncoderStr,
					destinations: []string{rtpSenderStr},
				},
			},
		},
		{
			outputs: []uint8{config.OutputHTTP, config.OutputRTMP},
			encoders: []encoder{
				{
					encoderType:  mtsEncoderStr,
					destinations: []string{mtsSenderStr},
				},
				{
					encoderType:  flvEncoderStr,
					destinations: []string{rtmpSenderStr},
				},
			},
		},
		{
			outputs: []uint8{config.OutputHTTP, config.OutputRTP, config.OutputRTMP},
			encoders: []encoder{
				{
					encoderType:  mtsEncoderStr,
					destinations: []string{mtsSenderStr, rtpSenderStr},
				},
				{
					encoderType:  flvEncoderStr,
					destinations: []string{rtmpSenderStr},
				},
			},
		},
		{
			outputs: []uint8{config.OutputRTP, config.OutputRTMP},
			encoders: []encoder{
				{
					encoderType:  mtsEncoderStr,
					destinations: []string{rtpSenderStr},
				},
				{
					encoderType:  flvEncoderStr,
					destinations: []string{rtmpSenderStr},
				},
			},
		},
	}

	rv, err := New(config.Config{Logger: &testLogger{}}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Go through our test cases.
	for testNum, test := range tests {
		// Create a new config and reset revid with it.
		const dummyURL = "rtmp://dummy"
		c := config.Config{Logger: &testLogger{}, Outputs: test.outputs, RTMPURL: []string{dummyURL}}
		err := rv.setConfig(c)
		if err != nil {
			t.Fatalf("unexpected error: %v for test %v", err, testNum)
		}

		// This logic is what we want to check.
		err = rv.setupPipeline(
			func(dst io.WriteCloser, rate float64) (io.WriteCloser, error) {
				return &tstMtsEncoder{dst: dst}, nil
			},
			func(dst io.WriteCloser, rate int) (io.WriteCloser, error) {
				return &tstFlvEncoder{dst: dst}, nil
			},
			func(writers ...io.WriteCloser) io.WriteCloser {
				return &dummyMultiWriter{dst: writers}
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v for test %v", err, testNum)
		}

		// First check that we have the correct number of encoders.
		got := len(rv.encoders.(*dummyMultiWriter).dst)
		want := len(test.encoders)
		if got != want {
			t.Errorf("incorrect number of encoders in revid for test: %v. \nGot: %v\nWant: %v\n", testNum, got, want)
		}

		// Now check the correctness of encoders and their destinations.
		for _, e := range rv.encoders.(*dummyMultiWriter).dst {
			// Get e's type.
			encoderType := fmt.Sprintf("%T", e)

			// Check that we expect this encoder to be here.
			idx := -1
			for i, expect := range test.encoders {
				if expect.encoderType == encoderType {
					idx = i
				}
			}
			if idx == -1 {
				t.Fatalf("encoder %v isn't expected in test %v", encoderType, testNum)
			}

			// Now check that this encoder has correct number of destinations (senders).
			var ms io.WriteCloser
			switch encoderType {
			case mtsEncoderStr:
				ms = e.(*tstMtsEncoder).dst
			case flvEncoderStr:
				ms = e.(*tstFlvEncoder).dst
			}

			senders := ms.(*dummyMultiWriter).dst
			got = len(senders)
			want = len(test.encoders[idx].destinations)
			if got != want {
				t.Errorf("did not get expected number of senders in test %v. \nGot: %v\nWant: %v\n", testNum, got, want)
			}

			// Check that destinations are as expected.
			for _, expectDst := range test.encoders[idx].destinations {
				ok := false
				for _, dst := range senders {
					// Get type of sender.
					senderType := fmt.Sprintf("%T", dst)

					// If it's one we want, indicate.
					if senderType == expectDst {
						ok = true
					}
				}

				// If not okay then we couldn't find expected sender.
				if !ok {
					t.Errorf("could not find expected destination %v, for test %v", expectDst, testNum)
				}
			}
		}
	}
}

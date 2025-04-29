/*
NAME
  filters_test.go

DESCRIPTION
  filter_test.go contains functions for testing functions in filters.go.

AUTHOR
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package pcm

import (
	"math"
	"math/cmplx"
	"os"
	"testing"

	"github.com/mjibson/go-dsp/fft"
)

// Set constant values for testing.
const (
	sampleRate   = 44100
	filterLength = 500
	freqTest     = 1000
)

// TestLowPass is used to test the lowpass constructor and application. Testing is done by ensuring frequency response as well as
// comparing against an expected audio file.
func TestLowPass(t *testing.T) {
	// Generate an audio buffer to run test on.
	genAudio, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	var buf = Buffer{Data: genAudio, Format: BufferFormat{SFormat: S16_LE, Rate: sampleRate, Channels: 1}}

	// Create a lowpass filter to test.
	const fc = 4500.0
	lp, err := NewLowPass(fc, buf.Format, filterLength)
	if err != nil {
		t.Fatal(err)
	}

	// Filter the audio.
	filteredAudio, err := lp.Apply(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Take the FFT of the signal.
	filteredFloats, err := bytesToFloats(filteredAudio)
	if err != nil {
		t.Fatal(err)
	}
	filteredFFT := fft.FFTReal(filteredFloats)

	// Check if the lowpass filter worked (any high values in filteredFFT above cutoff freq result in fail).
	for i := int(fc); i < sampleRate/2; i++ {
		mag := math.Pow(cmplx.Abs(filteredFFT[i]), 2)
		if mag > freqTest {
			t.Error("Lowpass filter failed to meet spec.")
			break
		}
	}

	// Read audio from the test location.
	const fileName = "../../../test/av/input/lp_4500.pcm"
	expectedAudio, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("File for comparison not read.\n\t%s", err)
	}

	// Compare the filtered audio against the expected signal.
	compare(filteredAudio, expectedAudio, t)
}

// TestHighPass is used to test the highpass constructor and application. Testing is done by ensuring frequency response as well as
// comparing against an expected audio file.
func TestHighPass(t *testing.T) {
	// Generate an audio buffer to run test on.
	genAudio, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	var buf = Buffer{Data: genAudio, Format: BufferFormat{SFormat: S16_LE, Rate: sampleRate, Channels: 1}}

	// Create a highpass filter to test.
	const fc = 4500.0
	hp, err := NewHighPass(fc, buf.Format, filterLength)
	if err != nil {
		t.Fatal(err)
	}

	// Filter the audio.
	filteredAudio, err := hp.Apply(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Take the FFT of signal.
	filteredFloats, err := bytesToFloats(filteredAudio)
	if err != nil {
		t.Fatal(err)
	}
	filteredFFT := fft.FFTReal(filteredFloats)

	// Check if the highpass filter worked (any high values in filteredFFT below cutoff freq result in fail).
	for i := 0; i < int(fc); i++ {
		mag := math.Pow(cmplx.Abs(filteredFFT[i]), 2)
		if mag > freqTest {
			t.Error("Highpass Filter doesn't meet Spec", i)
		}
	}

	// Read audio from the test location.
	const fileName = "../../../test/av/input/hp_4500.pcm"
	expectedAudio, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("File for comparison not read.\n\t%s", err)
	}

	// Compare against expected results.
	compare(expectedAudio, filteredAudio, t)
}

// TestBandPass is used to test the bandpass constructor and application. Testing is done by ensuring frequency response as well as
// comparing against an expected audio file.
func TestBandPass(t *testing.T) {
	// Generate an audio buffer to run test on.
	genAudio, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	var buf = Buffer{Data: genAudio, Format: BufferFormat{SFormat: S16_LE, Rate: sampleRate, Channels: 1}}

	// Create a bandpass filter to test.
	const (
		fc_l = 4500.0
		fc_u = 9500.0
	)
	hp, err := NewBandPass(fc_l, fc_u, buf.Format, filterLength)
	if err != nil {
		t.Fatal(err)
	}

	// Filter audio with filter.
	filteredAudio, err := hp.Apply(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Take FFT of signal.
	filteredFloats, err := bytesToFloats(filteredAudio)
	if err != nil {
		t.Fatal(err)
	}
	filteredFFT := fft.FFTReal(filteredFloats)

	// Check if the bandpass filter worked (any high values in filteredFFT above cutoff or below cutoff freq result in fail).
	for i := 0; i < int(fc_l); i++ {
		mag := math.Pow(cmplx.Abs(filteredFFT[i]), 2)
		if mag > freqTest {
			t.Error("Bandpass Filter doesn't meet Spec", i)
		}
	}

	for i := int(fc_u); i < sampleRate/2; i++ {
		mag := math.Pow(cmplx.Abs(filteredFFT[i]), 2)
		if mag > freqTest {
			t.Error("Bandpass Filter doesn't meet Spec", i)
		}
	}

	// Read audio from test location.
	const fileName = "../../../test/av/input/bp_4500-9500.pcm"
	expectedAudio, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("File for comparison not read.\n\t%s", err)
	}

	// Compare against the expected audio.
	compare(expectedAudio, filteredAudio, t)
}

// TestBandPass is used to test the bandpass constructor and application. Testing is done by ensuring frequency response as well as
// comparing against an expected audio file.
func TestBandStop(t *testing.T) {
	// Generate an audio buffer to run test on.
	genAudio, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	var buf = Buffer{Data: genAudio, Format: BufferFormat{SFormat: S16_LE, Rate: sampleRate, Channels: 1}}

	// Create a bandpass filter to test.
	const (
		fc_l = 4500.0
		fc_u = 9500.0
	)
	bs, err := NewBandStop(fc_l, fc_u, buf.Format, filterLength)
	if err != nil {
		t.Fatal(err)
	}

	// Filter audio with filter.
	filteredAudio, err := bs.Apply(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Take FFT of signal.
	filteredFloats, err := bytesToFloats(filteredAudio)
	if err != nil {
		t.Fatal(err)
	}
	filteredFFT := fft.FFTReal(filteredFloats)

	// Check if the bandpass filter worked (any high values in filteredFFT above cutoff or below cutoff freq result in fail).
	for i := int(fc_l); i < int(fc_u); i++ {
		mag := math.Pow(cmplx.Abs(filteredFFT[i]), 2)
		if mag > freqTest {
			t.Error("BandStop Filter doesn't meet Spec", i)
		}
	}

	// Read audio from test location.
	const fileName = "../../../test/av/input/bs_4500-9500.pcm"
	expectedAudio, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatalf("File for comparison not read.\n\t%s", err)
	}

	// Compare against the expected audio.
	compare(expectedAudio, filteredAudio, t)
}

// TestAmplifier is used to test the amplifier constructor and application. Testing is done by checking the maximum value before and
// after application, as well as comparing against an expected audio file.
func TestAmplifier(t *testing.T) {
	// Load a simple sine wave with amplitude of 0.1 and load into buffer.
	const audioFileName = "../../../test/av/input/sine.pcm"
	lowSine, err := os.ReadFile(audioFileName)
	if err != nil {
		t.Errorf("File for filtering not read.\n\t%s", err)
		t.FailNow()
	}
	var buf = Buffer{Data: lowSine, Format: BufferFormat{SFormat: S16_LE, Rate: sampleRate, Channels: 1}}

	// Create an amplifier filter.
	const factor = 5.0
	amp := NewAmplifier(factor)

	// Apply the amplifier to the audio.
	filteredAudio, err := amp.Apply(buf)
	if err != nil {
		t.Fatal(err)
	}

	// Find the maximum sample before and after amplification.
	dataFloats, err := bytesToFloats(buf.Data)
	if err != nil {
		t.Fatal(err)
	}
	preMax := max(dataFloats)
	filteredFloats, err := bytesToFloats(filteredAudio)
	if err != nil {
		t.Fatal(err)
	}
	postMax := max(filteredFloats)

	// Compare the values.
	if preMax*factor > 1 && postMax > 0.99 {
	} else if postMax/preMax > 1.01*factor || postMax/preMax < 0.99*factor {
		t.Error("Amplifier failed to meet spec, expected:", factor, " got:", postMax/preMax)
	}

	// Load expected audio file.
	const compFileName = "../../../test/av/input/amp_5.pcm"
	expectedAudio, err := os.ReadFile(compFileName)
	if err != nil {
		t.Fatalf("File for comparison not read.\n\t%s", err)
	}

	// Compare against the expected audio file.
	compare(filteredAudio, expectedAudio, t)
}

// generate returns a byte slice in the same format that would be read from a PCM file.
// The function generates a sound with a range of frequencies for testing against,
// with a length of 1 second.
func generate() ([]byte, error) {
	// Create an slice to generate values across.
	t := make([]float64, sampleRate)
	s := make([]float64, sampleRate)
	// Define spacing of generated frequencies.
	const (
		deltaFreq = 1000
		maxFreq   = 21000
		amplitude = float64(deltaFreq) / float64((maxFreq - deltaFreq))
	)
	for n := 0; n < sampleRate; n++ {
		t[n] = float64(n) / float64(sampleRate)
		// Generate sinewaves of different frequencies.
		s[n] = 0
		for f := deltaFreq; f < maxFreq; f += deltaFreq {
			s[n] += amplitude * math.Sin(float64(f)*2*math.Pi*t[n])
		}
	}
	// Return the spectrum as bytes (PCM).
	bytesOut, err := floatsToBytes(s)
	if err != nil {
		return nil, err
	}
	return bytesOut, nil
}

// compare takes in two audio files (S16_LE), compares them, and returns an error if the
// signals are more than 10% different at any individual sample.
func compare(a, b []byte, t *testing.T) {
	// Convert to floats to compare.
	aFloats, err := bytesToFloats(a)
	if err != nil {
		t.Fatal(err)
	}
	bFloats, err := bytesToFloats(b)
	if err != nil {
		t.Fatal(err)
	}

	// Compare against filtered audio.
	for i := range aFloats {
		diff := (bFloats[i] - aFloats[i])
		if math.Abs(diff) > 0.1 {
			t.Error("Filtered audio is too different to database")
			return
		}
	}
}

// max takes a float slice and returns the absolute largest value in the slice.
func max(a []float64) float64 {
	var runMax float64 = -1
	for i := range a {
		if math.Abs(a[i]) > runMax {
			runMax = math.Abs(a[i])
		}
	}
	return runMax
}

/*
NAME
  pcm_test.go

DESCRIPTION
  pcm_test.go contains functions for testing the pcm package.

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package pcm

import (
	"bytes"
	"io/ioutil"
	"log"
	"testing"
)

// TestResample tests the Resample function using a pcm file that contains audio of a freq. sweep.
// The output of the Resample function is compared with a file containing the expected result.
func TestResample(t *testing.T) {
	inPath := "../../../test/av/input/sweep_400Hz_20000Hz_-3dBFS_5s_48khz.pcm"
	expPath := "../../../test/av/output/sweep_400Hz_20000Hz_resampled_48to8kHz.pcm"

	// Read input pcm.
	inPcm, err := ioutil.ReadFile(inPath)
	if err != nil {
		log.Fatal(err)
	}

	format := BufferFormat{
		Channels: 1,
		Rate:     48000,
		SFormat:  S16_LE,
	}

	buf := Buffer{
		Format: format,
		Data:   inPcm,
	}

	// Resample pcm.
	resampled, err := Resample(buf, 8000)
	if err != nil {
		log.Fatal(err)
	}

	// Read expected resampled pcm.
	exp, err := ioutil.ReadFile(expPath)
	if err != nil {
		log.Fatal(err)
	}

	// Compare result with expected.
	if !bytes.Equal(resampled.Data, exp) {
		t.Error("Resampled data does not match expected result.")
	}
}

// TestStereoToMono tests the StereoToMono function using a pcm file that contains stereo audio.
// The output of the StereoToMono function is compared with a file containing the expected mono audio.
func TestStereoToMono(t *testing.T) {
	inPath := "../../../test/av/input/stereo_DTMF_tones.pcm"
	expPath := "../../../test/av/output/mono_DTMF_tones.pcm"

	// Read input pcm.
	inPcm, err := ioutil.ReadFile(inPath)
	if err != nil {
		log.Fatal(err)
	}

	format := BufferFormat{
		Channels: 2,
		Rate:     44100,
		SFormat:  S16_LE,
	}

	buf := Buffer{
		Format: format,
		Data:   inPcm,
	}

	// Convert audio.
	mono, err := StereoToMono(buf)
	if err != nil {
		log.Fatal(err)
	}

	// Read expected mono pcm.
	exp, err := ioutil.ReadFile(expPath)
	if err != nil {
		log.Fatal(err)
	}

	// Compare result with expected.
	if !bytes.Equal(mono.Data, exp) {
		t.Error("Converted data does not match expected result.")
	}
}

/*
NAME
  adpcm_test.go

DESCRIPTION
  adpcm_test.go contains tests for the adpcm package.

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package adpcm

import (
	"bytes"
	"io/ioutil"
	"testing"
)

// TestEncodeBlock will read PCM data, encode it in blocks and generate ADPCM
// then compare the result with expected ADPCM.
func TestEncodeBlock(t *testing.T) {
	// Read input pcm.
	pcm, err := ioutil.ReadFile("../../../test/av/input/original_8kHz_adpcm_test.pcm")
	if err != nil {
		t.Errorf("Unable to read input PCM file: %v", err)
	}

	// Encode adpcm.
	comp := bytes.NewBuffer(make([]byte, 0, EncBytes(len(pcm))))
	enc := NewEncoder(comp)
	_, err = enc.Write(pcm)
	if err != nil {
		t.Errorf("Unable to write to encoder: %v", err)
	}

	// Read expected adpcm file.
	exp, err := ioutil.ReadFile("../../../test/av/output/encoded_8kHz_adpcm_test2.adpcm")
	if err != nil {
		t.Errorf("Unable to read expected ADPCM file: %v", err)
	}

	if !bytes.Equal(comp.Bytes(), exp) {
		t.Error("ADPCM generated does not match expected ADPCM")
	}
}

// TestDecodeBlock will read encoded ADPCM, decode it in blocks and then compare the
// resulting PCM with the expected decoded PCM.
func TestDecodeBlock(t *testing.T) {
	// Read adpcm.
	comp, err := ioutil.ReadFile("../../../test/av/input/encoded_8kHz_adpcm_test2.adpcm")
	if err != nil {
		t.Errorf("Unable to read input ADPCM file: %v", err)
	}

	// Decode adpcm.
	decoded := bytes.NewBuffer(make([]byte, 0, len(comp)*4))
	dec := NewDecoder(decoded)
	_, err = dec.Write(comp)
	if err != nil {
		t.Errorf("Unable to write to decoder: %v", err)
	}

	// Read expected pcm file.
	exp, err := ioutil.ReadFile("../../../test/av/output/decoded_8kHz_adpcm_test2.pcm")
	if err != nil {
		t.Errorf("Unable to read expected PCM file: %v", err)
	}

	if !bytes.Equal(decoded.Bytes(), exp) {
		t.Error("PCM generated does not match expected PCM")
	}
}

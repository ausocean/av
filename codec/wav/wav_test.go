/*
NAME
  wav.go

DESCRIPTION
  wav.go contains functions for processing wav.

AUTHOR
  David Sutton <davidsutton@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package wav provides functions for converting wav audio.
package wav

import (
	"testing"
)

func TestWavWriter(t *testing.T) {
	tests := []struct {
		name    string
		md Metadata
		input []byte
		wantN   int
		wantErr error
	}{
		{name: "Header Only", md: Metadata{AudioFormat: PCMFormat, Channels: 1, SampleRate: 48000, BitDepth: 16}, input: nil, wantN: 44, wantErr: nil},
		{name: "4 bytes", md: Metadata{AudioFormat: PCMFormat, Channels: 1, SampleRate: 48000, BitDepth: 16}, input: []byte{0,0,0,0}, wantN: 48, wantErr: nil},
		{name: "No format", md: Metadata{Channels: 1, SampleRate: 48000, BitDepth: 16}, input: []byte{0,0,0,0}, wantN: 0, wantErr: errInvalidFormat},
		{name: "Invalid format", md: Metadata{AudioFormat: 2, Channels: 1, SampleRate: 48000, BitDepth: 16}, input: []byte{0,0,0,0}, wantN: 0, wantErr: errInvalidFormat},
		{name: "No channels", md: Metadata{AudioFormat: PCMFormat, SampleRate: 48000, BitDepth: 16}, input: []byte{0,0,0,0}, wantN: 0, wantErr: errInvalidChannels},
		{name: "Invalid channels", md: Metadata{AudioFormat: PCMFormat, Channels: 0, SampleRate: 48000, BitDepth: 16}, input: []byte{0,0,0,0}, wantN: 0, wantErr: errInvalidChannels},
		{name: "No sample rate", md: Metadata{AudioFormat: PCMFormat, Channels: 1, BitDepth: 16}, input: []byte{0,0,0,0}, wantN: 0, wantErr: errInvalidRate},
		{name: "Invalid sample rate", md: Metadata{AudioFormat: PCMFormat, Channels: 1, SampleRate: 0, BitDepth: 16}, input: []byte{0,0,0,0}, wantN: 0, wantErr: errInvalidRate},
		{name: "No bit depth", md: Metadata{AudioFormat: PCMFormat, Channels: 1, SampleRate: 48000}, input: []byte{0,0,0,0}, wantN: 0, wantErr: errInvalidBitDepth},
		{name: "Invalid bit depth", md: Metadata{AudioFormat: PCMFormat, Channels: 1, SampleRate: 48000, BitDepth: 0}, input: []byte{0,0,0,0}, wantN: 0, wantErr: errInvalidBitDepth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WAV{
				Metadata: tt.md,
			}

			gotN, err := w.Write(tt.input)
			if err != tt.wantErr {
				t.Errorf("WAV.Write() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotN != tt.wantN {
				t.Errorf("WAV.Write() = %v, want %v", gotN, tt.wantN)
			}

		})
	}
}

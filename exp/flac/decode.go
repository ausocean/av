/*
NAME
	decode.go

DESCRIPTION
  decode.go provides functionality for the decoding of FLAC compressed audio

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package flac provides functionality for the decoding of FLAC compressed audio.
package flac

import (
	"bytes"
	"errors"
	"io"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/mewkiz/flac"
)

const wavFormat = 1

// writeSeeker implements a memory based io.WriteSeeker.
type writeSeeker struct {
	buf []byte
	pos int
}

// Bytes returns the bytes contained in the writeSeekers buffer.
func (ws *writeSeeker) Bytes() []byte {
	return ws.buf
}

// Write writes len(p) bytes from p to the writeSeeker's buf and returns the number
// of bytes written. If less than len(p) bytes are written, an error is returned.
func (ws *writeSeeker) Write(p []byte) (n int, err error) {
	minCap := ws.pos + len(p)
	if minCap > cap(ws.buf) { // Make sure buf has enough capacity:
		buf2 := make([]byte, len(ws.buf), minCap+len(p)) // add some extra
		copy(buf2, ws.buf)
		ws.buf = buf2
	}
	if minCap > len(ws.buf) {
		ws.buf = ws.buf[:minCap]
	}
	copy(ws.buf[ws.pos:], p)
	ws.pos += len(p)
	return len(p), nil
}

// Seek sets the offset for the next Read or Write to offset, interpreted according
// to whence: SeekStart means relative to the start of the file, SeekCurrent means
// relative to the current offset, and SeekEnd means relative to the end. Seek returns
// the new offset relative to the start of the file and an error, if any.
func (ws *writeSeeker) Seek(offset int64, whence int) (int64, error) {
	newPos, offs := 0, int(offset)
	switch whence {
	case io.SeekStart:
		newPos = offs
	case io.SeekCurrent:
		newPos = ws.pos + offs
	case io.SeekEnd:
		newPos = len(ws.buf) + offs
	}
	if newPos < 0 {
		return 0, errors.New("negative result pos")
	}
	ws.pos = newPos
	return int64(newPos), nil
}

// Decode takes buf, a slice of FLAC, and decodes to WAV. If complete decoding
// fails, an error is returned.
func Decode(buf []byte) ([]byte, error) {

	// Lex the FLAC into a stream to hold audio and it's properties.
	r := bytes.NewReader(buf)
	stream, err := flac.Parse(r)
	if err != nil {
		return nil, errors.New("Could not parse FLAC")
	}

	// Create WAV encoder and pass writeSeeker that will store output WAV.
	ws := &writeSeeker{}
	sr := int(stream.Info.SampleRate)
	bps := int(stream.Info.BitsPerSample)
	nc := int(stream.Info.NChannels)
	enc := wav.NewEncoder(ws, sr, bps, nc, wavFormat)
	defer enc.Close()

	// Decode FLAC into frames of samples
	intBuf := &audio.IntBuffer{
		Format:         &audio.Format{NumChannels: nc, SampleRate: sr},
		SourceBitDepth: bps,
	}
	return decodeFrames(stream, intBuf, enc, ws)
}

// decodeFrames parses frames from the stream and encodes them into WAV until
// the end of the stream is reached. The bytes from writeSeeker buffer are then
// returned. If any errors occur during encodeing, nil bytes and the error is returned.
func decodeFrames(s *flac.Stream, intBuf *audio.IntBuffer, e *wav.Encoder, ws *writeSeeker) ([]byte, error) {
	var data []int
	for {
		frame, err := s.ParseNext()

		// If we've reached the end of the stream then we can output the writeSeeker's buffer.
		if err == io.EOF {
			return ws.Bytes(), nil
		} else if err != nil {
			return nil, err
		}

		// Encode WAV audio samples.
		data = data[:0]
		for i := 0; i < frame.Subframes[0].NSamples; i++ {
			for _, subframe := range frame.Subframes {
				data = append(data, int(subframe.Samples[i]))
			}
		}
		intBuf.Data = data
		if err := e.Write(intBuf); err != nil {
			return nil, err
		}
	}
}

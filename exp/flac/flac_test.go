/*
NAME
	flac_test.go

DESCRIPTION
  flac_test.go provides utilities to test FLAC audio decoding

AUTHOR
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package flac

import (
	"io"
	"io/ioutil"
	"os"
	"testing"
)

const (
	testFile = "../../../test/test-data/av/input/robot.flac"
	outFile  = "testOut.wav"
)

// TestWriteSeekerWrite checks that basic writing to the ws works as expected.
func TestWriteSeekerWrite(t *testing.T) {
	ws := &writeSeeker{}

	const tstStr1 = "hello"
	ws.Write([]byte(tstStr1))
	got := string(ws.buf)
	if got != tstStr1 {
		t.Errorf("Write failed, got: %v, want: %v", got, tstStr1)
	}

	const tstStr2 = " world"
	const want = "hello world"
	ws.Write([]byte(tstStr2))
	got = string(ws.buf)
	if got != want {
		t.Errorf("Second write failed, got: %v, want: %v", got, want)
	}
}

// TestWriteSeekerSeek checks that writing and seeking works as expected, i.e. we
// can write, then seek to a knew place in the buf, and write again, either replacing
// bytes, or appending bytes.
func TestWriteSeekerSeek(t *testing.T) {
	ws := &writeSeeker{}

	const tstStr1 = "hello"
	want1 := tstStr1
	ws.Write([]byte(tstStr1))
	got := string(ws.buf)
	if got != tstStr1 {
		t.Errorf("Unexpected output, got: %v, want: %v", got, want1)
	}

	const tstStr2 = " world"
	const want2 = tstStr1 + tstStr2
	ws.Write([]byte(tstStr2))
	got = string(ws.buf)
	if got != want2 {
		t.Errorf("Unexpected output, got: %v, want: %v", got, want2)
	}

	const tstStr3 = "k!"
	const want3 = "hello work!"
	ws.Seek(-2, io.SeekEnd)
	ws.Write([]byte(tstStr3))
	got = string(ws.buf)
	if got != want3 {
		t.Errorf("Unexpected output, got: %v, want: %v", got, want3)
	}

	const tstStr4 = "gopher"
	const want4 = "hello gopher"
	ws.Seek(6, io.SeekStart)
	ws.Write([]byte(tstStr4))
	got = string(ws.buf)
	if got != want4 {
		t.Errorf("Unexpected output, got: %v, want: %v", got, want4)
	}
}

// TestDecodeFlac checks that we can load a flac file and decode to wav, writing
// to a wav file.
func TestDecodeFlac(t *testing.T) {
	b, err := ioutil.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Could not read test file, failed with err: %v", err.Error())
	}
	out, err := Decode(b)
	if err != nil {
		t.Errorf("Could not decode, failed with err: %v", err.Error())
	}
	f, err := os.Create(outFile)
	if err != nil {
		t.Fatalf("Could not create output file, failed with err: %v", err.Error())
	}
	_, err = f.Write(out)
	if err != nil {
		t.Fatalf("Could not write to output file, failed with err: %v", err.Error())
	}
}

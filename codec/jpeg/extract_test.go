/*
DESCRIPTION
  extract_test.go provides testing for extract.go.

AUTHOR
  Scott Barnard <scott@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package jpeg

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
)

type testReader struct {
	i int
}

func (r *testReader) Read(b []byte) (int, error) {
	if r.i >= len(testPackets) {
		return 0, io.EOF
	}
	copy(b, testPackets[r.i])
	r.i++
	return len(testPackets[r.i-1]), nil
}

func TestExtract(t *testing.T) {
	got := &bytes.Buffer{}
	err := NewExtractor().Extract(got, &testReader{}, 0)
	if err != nil {
		t.Fatalf("could not extract: %v", err)
	}

	want, err := ioutil.ReadFile("testdata/expect.mjpeg")
	if err != nil {
		t.Fatalf("could not read file for wanted JPEG data: %v", err)
	}

	if !bytes.Equal(got.Bytes(), want) {
		t.Error("did not get expected result")
	}
}

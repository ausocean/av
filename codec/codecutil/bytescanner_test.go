/*
NAME
  bytescanner_test.go

DESCRIPTION
  See Readme.md

AUTHOR
  Dan Kortschak <dan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package codecutil

import (
	"bytes"
	"reflect"
	"testing"
)

type chunkEncoder [][]byte

func (e *chunkEncoder) Encode(b []byte) error {
	*e = append(*e, b)
	return nil
}

func (*chunkEncoder) Stream() <-chan []byte { panic("INVALID USE") }

func TestScannerReadByte(t *testing.T) {
	data := []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.")

	for _, size := range []int{1, 2, 8, 1 << 10} {
		r := NewByteScanner(bytes.NewReader(data), make([]byte, size))
		var got []byte
		for {
			b, err := r.ReadByte()
			if err != nil {
				break
			}
			got = append(got, b)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("unexpected result for buffer size %d:\ngot :%q\nwant:%q", size, got, data)
		}
	}
}

func TestScannerScanUntilZero(t *testing.T) {
	data := []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit,\x00 sed do eiusmod tempor incididunt ut \x00labore et dolore magna aliqua.")

	for _, size := range []int{1, 2, 8, 1 << 10} {
		r := NewByteScanner(bytes.NewReader(data), make([]byte, size))
		var got [][]byte
		for {
			buf, _, err := r.ScanUntil(nil, 0x0)
			got = append(got, buf)
			if err != nil {
				break
			}
		}
		want := bytes.SplitAfter(data, []byte{0})
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected result for buffer zie %d:\ngot :%q\nwant:%q", size, got, want)
		}
	}
}

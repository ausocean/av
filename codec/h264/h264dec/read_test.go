/*
DESCRIPTION
  read_test.go provides testing for utilities in read.go.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>, The Australian Ocean Laboratory (AusOcean)
*/
package h264dec

import (
	"bytes"
	"testing"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
)

func TestMoreRBSPData(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{
			in:   "00000100",
			want: true,
		},
		{
			in:   "10000100",
			want: true,
		},
		{
			in:   "10000000",
			want: false,
		},
		{
			in:   "10000000 00000000 00000000 00000001",
			want: false,
		},
		{
			in:   "10000000 00000000 00000000 00000000 00000001",
			want: false,
		},
		{
			in:   "10000000 00000000",
			want: true,
		},
	}

	for i, test := range tests {
		b, err := binToSlice(test.in)
		if err != nil {
			t.Fatalf("unexpected binToSlice error: %v for test: %d", err, i)
		}

		got := moreRBSPData(bits.NewBitReader(bytes.NewReader(b)))
		if got != test.want {
			t.Errorf("unexpected result for test: %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

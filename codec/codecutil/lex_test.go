/*
NAME
  lex_test.go

AUTHOR
  Trek Hopton <trek@ausocean.org>

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
	"io"
	"strconv"
	"testing"
	"time"
)

var lexTests = []struct {
	data    []byte
	t       time.Duration
	n       int
	isValid bool // Whether or not this test should fail.
}{
	{[]byte{0x10, 0x00, 0xf3, 0x45, 0xfe, 0xd2, 0xaa, 0x4e}, time.Millisecond, 4, true},
	{[]byte{0x10, 0x00, 0xf3, 0x45, 0xfe, 0xd2, 0xaa, 0x4e}, time.Millisecond, 3, true},
	{[]byte{0x10, 0x00, 0xf3, 0x45, 0xfe, 0xd2, 0xaa, 0x4e}, 0, 2, true},
	{[]byte{0x10, 0x00, 0xf3, 0x45, 0xfe, 0xd2, 0xaa, 0x4e}, 0, 1, true},
	{[]byte{0x10, 0x00, 0xf3, 0x45, 0xfe, 0xd2, 0xaa, 0x4e}, time.Nanosecond, 0, false},
	{[]byte{0x10, 0x00, 0xf3, 0x45, 0xfe, 0xd2, 0xaa, 0x4e}, time.Millisecond, -1, false},
	{[]byte{0x10, 0x00, 0xf3, 0x45, 0xfe, 0xd2, 0xaa, 0x4e}, time.Millisecond, 15, true},
}

func TestByteLexer(t *testing.T) {
	for i, tt := range lexTests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dst := bytes.NewBuffer([]byte{})
			l, err := NewByteLexer(tt.n)
			if err != nil {
				if tt.isValid {
					t.Errorf("unexpected error: %v", err)
				} else {
					t.Skip()
				}
			}
			err = l.Lex(dst, bytes.NewReader(tt.data), tt.t)
			if err != nil && err != io.EOF {
				if tt.isValid {
					t.Errorf("unexpected error: %v", err)
				}
			} else if !bytes.Equal(dst.Bytes(), tt.data) {
				t.Errorf("data before and after lex are not equal: want %v, got %v", tt.data, dst.Bytes())
			}
		})
	}
}

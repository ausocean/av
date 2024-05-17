/*
NAME
  lex_test.go

DESCRIPTION
  lex_test.go provides testing for the lexer in lex.go.

AUTHOR
  Dan Kortschak <dan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// lex_test.go provides testing for the lexer in lex.go.

package jpeg

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/ausocean/utils/logging"
)

var jpegTests = []struct {
	name  string
	input []byte
	delay time.Duration
	want  [][]byte
	err   error
}{
	{
		name: "empty",
		err:  io.ErrUnexpectedEOF,
	},
	{
		name:  "null",
		input: []byte{0xff, 0xd8, 0xff, 0xd9},
		delay: 0,
		want:  [][]byte{{0xff, 0xd8, 0xff, 0xd9}},
		err:   io.ErrUnexpectedEOF,
	},
	{
		name:  "null delayed",
		input: []byte{0xff, 0xd8, 0xff, 0xd9},
		delay: time.Millisecond,
		want:  [][]byte{{0xff, 0xd8, 0xff, 0xd9}},
		err:   io.ErrUnexpectedEOF,
	},
	{
		name: "full",
		input: []byte{
			0xff, 0xd8, 'f', 'u', 'l', 'l', 0xff, 0xd9,
			0xff, 0xd8, 'f', 'r', 'a', 'm', 'e', 0xff, 0xd9,
			0xff, 0xd8, 'w', 'i', 't', 'h', 0xff, 0xd9,
			0xff, 0xd8, 'l', 'e', 'n', 'g', 't', 'h', 0xff, 0xd9,
			0xff, 0xd8, 's', 'p', 'r', 'e', 'a', 'd', 0xff, 0xd9,
		},
		delay: 0,
		want: [][]byte{
			{0xff, 0xd8, 'f', 'u', 'l', 'l', 0xff, 0xd9},
			{0xff, 0xd8, 'f', 'r', 'a', 'm', 'e', 0xff, 0xd9},
			{0xff, 0xd8, 'w', 'i', 't', 'h', 0xff, 0xd9},
			{0xff, 0xd8, 'l', 'e', 'n', 'g', 't', 'h', 0xff, 0xd9},
			{0xff, 0xd8, 's', 'p', 'r', 'e', 'a', 'd', 0xff, 0xd9},
		},
		err: io.ErrUnexpectedEOF,
	},
	{
		name: "full delayed",
		input: []byte{
			0xff, 0xd8, 'f', 'u', 'l', 'l', 0xff, 0xd9,
			0xff, 0xd8, 'f', 'r', 'a', 'm', 'e', 0xff, 0xd9,
			0xff, 0xd8, 'w', 'i', 't', 'h', 0xff, 0xd9,
			0xff, 0xd8, 'l', 'e', 'n', 'g', 't', 'h', 0xff, 0xd9,
			0xff, 0xd8, 's', 'p', 'r', 'e', 'a', 'd', 0xff, 0xd9,
		},
		delay: time.Millisecond,
		want: [][]byte{
			{0xff, 0xd8, 'f', 'u', 'l', 'l', 0xff, 0xd9},
			{0xff, 0xd8, 'f', 'r', 'a', 'm', 'e', 0xff, 0xd9},
			{0xff, 0xd8, 'w', 'i', 't', 'h', 0xff, 0xd9},
			{0xff, 0xd8, 'l', 'e', 'n', 'g', 't', 'h', 0xff, 0xd9},
			{0xff, 0xd8, 's', 'p', 'r', 'e', 'a', 'd', 0xff, 0xd9},
		},
		err: io.ErrUnexpectedEOF,
	},
}

func TestLex(t *testing.T) {
	Log = (*logging.TestLogger)(t)
	for _, test := range jpegTests {
		var buf chunkEncoder
		err := Lex(&buf, bytes.NewReader(test.input), test.delay)
		if fmt.Sprint(err) != fmt.Sprint(test.err) {
			t.Errorf("unexpected error for %q: got:%v want:%v", test.name, err, test.err)
		}
		if err != nil {
			continue
		}
		got := [][]byte(buf)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected result for %q:\ngot :%#v\nwant:%#v", test.name, got, test.want)
		}
	}
}

type chunkEncoder [][]byte

func (e *chunkEncoder) Write(b []byte) (int, error) {
	*e = append(*e, b)
	return len(b), nil
}

func (*chunkEncoder) Stream() <-chan []byte { panic("INVALID USE") }

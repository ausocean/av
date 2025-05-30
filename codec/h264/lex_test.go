/*
NAME
  lex_test.go

DESCRIPTION
  lex_test.go provides tests for the lexer in lex.go.

AUTHOR
  Dan Kortschak <dan@ausocean.org>
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h264

import (
	"time"
)

var h264Tests = []struct {
	name  string
	input []byte
	delay time.Duration
	want  [][]byte
	err   error
}{
	{
		name: "empty",
	},
	{
		name:  "null short",
		input: []byte{0x00, 0x00, 0x01, 0x0},
		delay: 0,
		want:  [][]byte{{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x0}},
	},
	{
		name:  "null short delayed",
		input: []byte{0x00, 0x00, 0x01, 0x0},
		delay: time.Millisecond,
		want:  [][]byte{{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x0}},
	},
	{
		name: "full short type 1",
		input: []byte{
			0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
			0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
			0x00, 0x00, 0x01, 0x41, 'w', 'i', 't', 'h',
			0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
			0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd',
		},
		delay: 0,
		want: [][]byte{
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
				0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
				0x00, 0x00, 0x01, 0x41, 'w', 'i', 't', 'h'},
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
				0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd'},
		},
	},
	{
		name: "full short type 5",
		input: []byte{
			0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
			0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
			0x00, 0x00, 0x01, 0x45, 'w', 'i', 't', 'h',
			0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
			0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd',
		},
		delay: 0,
		want: [][]byte{
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
				0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
				0x00, 0x00, 0x01, 0x45, 'w', 'i', 't', 'h'},
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
				0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd'},
		},
	},
	{
		name: "full short type 8",
		input: []byte{
			0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
			0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
			0x00, 0x00, 0x01, 0x48, 'w', 'i', 't', 'h',
			0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
			0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd',
		},
		delay: 0,
		want: [][]byte{
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
				0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
				0x00, 0x00, 0x01, 0x48, 'w', 'i', 't', 'h'},
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
				0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd'},
		},
	},
	{
		name: "full short delayed",
		input: []byte{
			0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
			0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
			0x00, 0x00, 0x01, 0x41, 'w', 'i', 't', 'h',
			0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
			0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd',
		},
		delay: time.Millisecond,
		want: [][]byte{
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
				0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
				0x00, 0x00, 0x01, 0x41, 'w', 'i', 't', 'h'},
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
				0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd'},
		},
	},
	{
		name: "full long type 1",
		input: []byte{
			0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
			0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
			0x00, 0x00, 0x00, 0x01, 0x41, 'w', 'i', 't', 'h',
			0x00, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
			0x00, 0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd',
		},
		delay: 0,
		want: [][]byte{
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
				0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
				0x00, 0x00, 0x00, 0x01, 0x41, 'w', 'i', 't', 'h'},
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
				0x00, 0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd'},
		},
	},
	{
		name: "full long type 5",
		input: []byte{
			0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
			0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
			0x00, 0x00, 0x00, 0x01, 0x45, 'w', 'i', 't', 'h',
			0x00, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
			0x00, 0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd',
		},
		delay: 0,
		want: [][]byte{
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
				0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
				0x00, 0x00, 0x00, 0x01, 0x45, 'w', 'i', 't', 'h'},
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
				0x00, 0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd'},
		},
	},
	{
		name: "full long type 8",
		input: []byte{
			0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
			0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
			0x00, 0x00, 0x00, 0x01, 0x48, 'w', 'i', 't', 'h',
			0x00, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
			0x00, 0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd',
		},
		delay: 0,
		want: [][]byte{
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
				0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
				0x00, 0x00, 0x00, 0x01, 0x48, 'w', 'i', 't', 'h'},
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
				0x00, 0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd'},
		},
	},
	{
		name: "full long delayed",
		input: []byte{
			0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
			0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
			0x00, 0x00, 0x00, 0x01, 0x41, 'w', 'i', 't', 'h',
			0x00, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
			0x00, 0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd',
		},
		delay: time.Millisecond,
		want: [][]byte{
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'u', 'l', 'l',
				0x00, 0x00, 0x00, 0x01, 0x00, 'f', 'r', 'a', 'm', 'e',
				0x00, 0x00, 0x00, 0x01, 0x41, 'w', 'i', 't', 'h'},
			{0x0, 0x0, 0x1, 0x9, 0xf0, 0x00, 0x00, 0x00, 0x01, 0x00, 'l', 'e', 'n', 'g', 't', 'h',
				0x00, 0x00, 0x00, 0x01, 0x00, 's', 'p', 'r', 'e', 'a', 'd'},
		},
	},
}

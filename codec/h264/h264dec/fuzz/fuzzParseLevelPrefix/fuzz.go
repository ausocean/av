// +build gofuzz

/*
DESCRIPTION
  fuzz.go provides a function with the required signature such that it may be
  accessed by go-fuzz. The function will compare the output of C level_prefix
  parser with a go version.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

import "C"
import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/ausocean/av/codec/h264/h264dec"
	"github.com/ausocean/av/codec/h264/h264dec/bits"
)

func Fuzz(data []byte) int {
	// Create C based BitReader based on data.
	cbr := C.new_BitReader((*C.char)(unsafe.Pointer(&data[0])), C.int(len(data)))
	if cbr == nil {
		panic("new_BitReader returned NULL pointer")
	}

	// Get the level_prefix from the C code. If got is < 0, then the C code
	// doesn't like it and we shouldn't have that input in the corpus, so return -1.
	want := C.read_levelprefix(cbr)
	if want < 0 {
		return -1
	}

	// Get the level_prefix from the go code. If the C code was okay with this
	// input, but the go code isn't then that's bad, so panic - want crasher info.
	got, err := h264dec.FuzzParseLevelPrefix(bits.NewBitReader(bytes.NewReader(data)))
	if err != nil {
		panic(fmt.Sprintf("error from go parseLevelPrefix: %v", err))
	}

	// Compare the result of the C and go code. If they are not the same then
	// panic - our go code is not doing what it should.
	if int(got) != int(want) {
		panic(fmt.Sprintf("did not get expected results for input: %v\nGot: %v\nWant: %v\n", data, got, want))
	}
	return 1
}

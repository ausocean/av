/*
TODO: this file should really be in a 'h264enc' package.

DESCRIPTION
  cabacenc_test.go provides testing for functionality found in cabacenc.go.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h264dec

import (
	"reflect"
	"testing"
)

func TestMbTypeBinString(t *testing.T) {
	tests := []struct {
		v, slice int
		want     []int
		err      error
	}{
		{v: 6, slice: sliceTypeI, want: []int{1, 0, 0, 1, 0, 0, 1}},
		{v: 26, slice: sliceTypeI, err: errBadMbType},
		{v: -1, slice: sliceTypeI, err: errBadMbType},
		{v: 4, slice: sliceTypeSI, want: []int{0}},
		{v: 6, slice: sliceTypeSI, want: []int{1, 1, 0, 0, 1, 0, 0, 0}},
		{v: 0, slice: sliceTypeSI, err: errBadMbType},
		{v: 27, slice: sliceTypeSI, err: errBadMbType},
		{v: 2, slice: sliceTypeP, want: []int{0, 1, 0}},
		{v: 3, slice: sliceTypeSP, want: []int{0, 0, 1}},
		{v: 7, slice: sliceTypeP, want: []int{1, 1, 0, 0, 0, 0, 1}},
		{v: 7, slice: sliceTypeSP, want: []int{1, 1, 0, 0, 0, 0, 1}},
		{v: -1, slice: sliceTypeP, err: errBadMbType},
		{v: 31, slice: sliceTypeP, err: errBadMbType},
		{v: 8, slice: sliceTypeB, want: []int{1, 1, 0, 1, 0, 1}},
		{v: 30, slice: sliceTypeB, want: []int{1, 1, 1, 1, 0, 1, 1, 0, 0, 1, 0, 1, 0}},
		{v: -1, slice: sliceTypeB, err: errBadMbType},
		{v: 49, slice: sliceTypeB, err: errBadMbType},
		{v: 6, slice: 20, err: errBadMbSliceType},
	}

	for i, test := range tests {
		got, err := mbTypeBinString(test.v, test.slice)
		if err != test.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v", i, err, test.err)
		}

		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v", i, got, test.want)
		}
	}
}

func TestSubMbTypeBinString(t *testing.T) {
	tests := []struct {
		v, slice int
		want     []int
		err      error
	}{
		{v: 2, slice: sliceTypeP, want: []int{0, 1, 1}},
		{v: 2, slice: sliceTypeSP, want: []int{0, 1, 1}},
		{v: -1, slice: sliceTypeSP, err: errBadMbType},
		{v: 4, slice: sliceTypeSP, err: errBadMbType},
		{v: 9, slice: sliceTypeB, want: []int{1, 1, 1, 0, 1, 0}},
		{v: 9, slice: 40, err: errBadSubMbSliceType},
	}

	for i, test := range tests {
		got, err := subMbTypeBinString(test.v, test.slice)
		if err != test.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v", i, err, test.err)
		}

		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v", i, got, test.want)
		}
	}
}

func TestUnaryBinString(t *testing.T) {
	// Test data has been extracted from table 9-35 of the specifications.
	tests := []struct {
		in   int
		want []int
		err  error
	}{
		{in: 0, want: []int{0}, err: nil},
		{in: 1, want: []int{1, 0}, err: nil},
		{in: 2, want: []int{1, 1, 0}, err: nil},
		{in: 3, want: []int{1, 1, 1, 0}, err: nil},
		{in: 4, want: []int{1, 1, 1, 1, 0}, err: nil},
		{in: 5, want: []int{1, 1, 1, 1, 1, 0}, err: nil},
		{in: -3, want: nil, err: errNegativeSyntaxVal},
	}

	for i, test := range tests {
		got, err := unaryBinString(test.in)
		if err != test.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v", i, err, test.err)
		}

		if !reflect.DeepEqual(test.want, got) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

func TestFixedLengthBinString(t *testing.T) {
	tests := []struct {
		v    int
		cMax int
		want []int
		err  error
	}{
		{v: 0, cMax: 7, want: []int{0, 0, 0}},
		{v: 1, cMax: 7, want: []int{0, 0, 1}},
		{v: 2, cMax: 7, want: []int{0, 1, 0}},
		{v: 3, cMax: 7, want: []int{0, 1, 1}},
		{v: 4, cMax: 7, want: []int{1, 0, 0}},
		{v: 5, cMax: 7, want: []int{1, 0, 1}},
		{v: 6, cMax: 7, want: []int{1, 1, 0}},
		{v: 7, cMax: 7, want: []int{1, 1, 1}},
		{v: -1, cMax: 7, want: nil, err: errNegativeValue},
	}

	for i, test := range tests {
		got, err := fixedLenBinString(test.v, test.cMax)
		if err != test.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v\n", i, err, test.err)
		}

		if !reflect.DeepEqual(test.want, got) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

func TestTruncUnaryBinString(t *testing.T) {
	tests := []struct {
		v    int
		cMax int
		want []int
		err  error
	}{
		{v: 0, cMax: 10, want: []int{0}, err: nil},
		{v: 1, cMax: 10, want: []int{1, 0}, err: nil},
		{v: 2, cMax: 10, want: []int{1, 1, 0}, err: nil},
		{v: 0, cMax: 0, want: []int{}, err: nil},
		{v: 4, cMax: 4, want: []int{1, 1, 1, 1}, err: nil},
		{v: 1, cMax: 10, want: []int{1, 0}, err: nil},
		{v: 2, cMax: 10, want: []int{1, 1, 0}, err: nil},
		{v: -3, cMax: 10, want: nil, err: errNegativeSyntaxVal},
		{v: 5, cMax: 4, want: nil, err: errInvalidSyntaxVal},
	}

	for i, test := range tests {
		got, err := truncUnaryBinString(test.v, test.cMax)
		if err != test.err {
			t.Errorf("did not get expected error for test %d\nGot: %v\nWant: %v", i, err, test.err)
		}

		if !reflect.DeepEqual(test.want, got) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

func TestUEGkSuffix(t *testing.T) {
	// Data from https://patents.google.com/patent/US20070092150
	tests := []struct {
		v, uCoff, k   int
		signedValFlag bool
		want          []int
	}{
		0:  {v: 14, uCoff: 14, want: []int{0}},
		1:  {v: 15, uCoff: 14, want: []int{1, 0, 0}},
		2:  {v: 16, uCoff: 14, want: []int{1, 0, 1}},
		3:  {v: 17, uCoff: 14, want: []int{1, 1, 0, 0, 0}},
		4:  {v: 18, uCoff: 14, want: []int{1, 1, 0, 0, 1}},
		5:  {v: 19, uCoff: 14, want: []int{1, 1, 0, 1, 0}},
		6:  {v: 20, uCoff: 14, want: []int{1, 1, 0, 1, 1}},
		7:  {v: 21, uCoff: 14, want: []int{1, 1, 1, 0, 0, 0, 0}},
		8:  {v: 22, uCoff: 14, want: []int{1, 1, 1, 0, 0, 0, 1}},
		9:  {v: 23, uCoff: 14, want: []int{1, 1, 1, 0, 0, 1, 0}},
		10: {v: 24, uCoff: 14, want: []int{1, 1, 1, 0, 0, 1, 1}},
		11: {v: 25, uCoff: 14, want: []int{1, 1, 1, 0, 1, 0, 0}},
	}

	for i, test := range tests {
		got := suffix(test.v, test.uCoff, test.k, test.signedValFlag)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

func TestUnaryExpGolombBinString(t *testing.T) {
	tests := []struct {
		v, uCoff, k   int
		signedValFlag bool
		want          []int
	}{
		0: {v: 7, uCoff: 14, want: []int{1, 1, 1, 1, 1, 1, 1, 0}},
		1: {v: 17, uCoff: 14, want: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0}},
		2: {v: 15, uCoff: 14, signedValFlag: true, want: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0}},
		3: {v: -15, uCoff: 14, signedValFlag: true, want: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 1}},
	}

	for i, test := range tests {
		got, err := unaryExpGolombBinString(test.v, test.uCoff, test.k, test.signedValFlag)
		if err != nil {
			t.Errorf("did not expect error %v for test %d", err, i)
		}

		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

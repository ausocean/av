/*
DESCRIPTION
  cavlc_test.go provides testing for functionality in cavlc.go.

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
	"bytes"
	"reflect"
	"testing"

	"github.com/ausocean/av/codec/h264/h264dec/bits"
)

func TestFormCoeffTokenMap(t *testing.T) {
	tests := []struct {
		in   [][]string
		want [nColumns]tokenMap
	}{
		{
			in: [][]string{
				{"0", "0", "1", "11", "1111", "0000 11", "01", "1"},
				{"0", "1", "0001 01", "0010 11", "0011 11", "0000 00", "0001 11", "0001 111"},
			},
			want: [nColumns]tokenMap{
				0: {
					0: {1: {0, 0}},
					3: {5: {0, 1}},
				},
				1: {
					0: {3: {0, 0}},
					2: {11: {0, 1}},
				},
				2: {
					0: {15: {0, 0}},
					2: {15: {0, 1}},
				},
				3: {
					4: {3: {0, 0}},
					6: {0: {0, 1}},
				},
				4: {
					1: {1: {0, 0}},
					3: {7: {0, 1}},
				},
				5: {
					0: {1: {0, 0}},
					3: {15: {0, 1}},
				},
			},
		},
		{
			in: [][]string{
				{"0", "0", "1", "11", "1111", "0000 11", "01", "1"},
				{"0", "1", "0001 01", "0010 11", "0011 11", "-", "0001 11", "0001 111"},
			},
			want: [nColumns]tokenMap{
				0: {
					0: {1: {0, 0}},
					3: {5: {0, 1}},
				},
				1: {
					0: {3: {0, 0}},
					2: {11: {0, 1}},
				},
				2: {
					0: {15: {0, 0}},
					2: {15: {0, 1}},
				},
				3: {
					4: {3: {0, 0}},
				},
				4: {
					1: {1: {0, 0}},
					3: {7: {0, 1}},
				},
				5: {
					0: {1: {0, 0}},
					3: {15: {0, 1}},
				},
			},
		},
	}

	for i, test := range tests {
		m, err := formCoeffTokenMap(test.in)
		if err != nil {
			t.Errorf("did not expect error: %v for test: %d", err, i)
		}

		if !reflect.DeepEqual(m, test.want) {
			t.Errorf("did not get expected result for test: %d\nGot: %v\nWant: %v\n", i, m, test.want)
		}
	}
}

func TestParseLevelPrefix(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{in: "00001", want: 4},
		{in: "0000001", want: 6},
		{in: "1", want: 0},
	}

	for i, test := range tests {
		s, _ := binToSlice(test.in)
		l, err := parseLevelPrefix(bits.NewBitReader(bytes.NewReader(s)))
		if err != nil {
			t.Errorf("did not expect error: %v, for test %d", err, i)
		}

		if l != test.want {
			t.Errorf("did not get expected result for test %d\nGot: %d\nWant: %d\n", i, l, test.want)
		}
	}
}

func TestReadCoeffToken(t *testing.T) {
	tests := []struct {
		// Input.
		nC        int
		tokenBits string

		// Expected.
		trailingOnes int
		totalCoeff   int
		err          error
	}{
		{
			nC:           5,
			tokenBits:    "0001001",
			trailingOnes: 0,
			totalCoeff:   6,
		},
		{
			nC:        -1,
			tokenBits: "0000000000111111111",
			err:       errBadToken,
		},
		{
			nC:        -3,
			tokenBits: "0001001",
			err:       errInvalidNC,
		},
	}

	for i, test := range tests {
		b, err := binToSlice(test.tokenBits)
		if err != nil {
			t.Errorf("converting bin string to slice failed with error: %v for test", err)
			continue
		}

		gotTrailingOnes, gotTotalCoeff, _, gotErr := readCoeffToken(bits.NewBitReader(bytes.NewReader(b)), test.nC)
		if gotErr != test.err {
			t.Errorf("did not get expected error for test: %d\nGot: %v\nWant: %v\n", i, gotErr, test.err)
			continue
		}

		if gotTrailingOnes != test.trailingOnes {
			t.Errorf("did not get expected TrailingOnes(coeff_token) for test %d\nGot: %v\nWant: %v\n", i, gotTrailingOnes, test.trailingOnes)
		}

		if gotTotalCoeff != test.totalCoeff {
			t.Errorf("did not get expected TotalCoeff(coeff_token) for test %d\nGot: %v\nWant: %v\n", i, gotTotalCoeff, test.totalCoeff)
		}
	}
}

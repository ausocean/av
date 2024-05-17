package h264dec

import "testing"

func TestBinToInt(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{in: "101", want: 5},
		{in: "1", want: 1},
		{in: "00000", want: 0},
		{in: "", want: 0},
		{in: "1111", want: 15},
		{in: "1 111", want: 15},
	}

	for i, test := range tests {
		n, err := binToInt(test.in)
		if err != nil {
			t.Errorf("did not expect error: %v from binToInt", err)
		}

		if n != test.want {
			t.Errorf("did not get expected result for test %d\nGot: %v\nWant: %v\n", i, n, test.want)
		}
	}
}

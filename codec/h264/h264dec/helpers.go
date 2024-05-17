/*
DESCRIPTION
  helpers.go provides general helper utilities.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>, The Australian Ocean Laboratory (AusOcean)

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h264dec

import (
	"errors"
	"math"
)

// binToSlice is a helper function to convert a string of binary into a
// corresponding byte slice, e.g. "0100 0001 1000 1100" => {0x41,0x8c}.
// Spaces in the string are ignored.
func binToSlice(s string) ([]byte, error) {
	var (
		a     byte = 0x80
		cur   byte
		bytes []byte
	)

	for i, c := range s {
		switch c {
		case ' ':
			continue
		case '1':
			cur |= a
		case '0':
		default:
			return nil, errors.New("invalid binary string")
		}

		a >>= 1
		if a == 0 || i == (len(s)-1) {
			bytes = append(bytes, cur)
			cur = 0
			a = 0x80
		}
	}
	return bytes, nil
}

// binToInt converts a binary string provided as a string and returns as an int.
// White spaces are ignored.
func binToInt(s string) (int, error) {
	var sum int
	var nSpace int
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ' ' {
			nSpace++
			continue
		}
		sum += int(math.Pow(2, float64(len(s)-1-i-nSpace))) * int(s[i]-'0')
	}
	return sum, nil
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mini(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absi(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

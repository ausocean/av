/*
DESCRIPTION
  cabac_test.go provides testing for functionality found in cabac.go.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Shawn Smith <shawnpsmith@gmail.com>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package h264dec

import (
	"testing"
)

var ctxIdxTests = []struct {
	binIdx       int
	maxBinIdxCtx int
	ctxIdxOffset int
	want         int
}{
	{0, 0, 0, 10000},
	{999, 0, 0, 10000},

	{0, 0, 3, 10000},
	{1, 0, 3, 276},
	{2, 0, 3, 3},
	{3, 0, 3, 4},
	{4, 0, 3, 10000},
	{5, 0, 3, 10000},
	{999, 0, 3, 7},

	{0, 0, 11, 10000},
	{999, 0, 11, 10000},

	{0, 0, 14, 0},
	{1, 0, 14, 1},
	{2, 0, 14, 10000},
	{999, 0, 14, 10000},

	{0, 0, 17, 0},
	{1, 0, 17, 276},
	{2, 0, 17, 1},
	{3, 0, 17, 2},
	{4, 0, 17, 10000},
	{999, 0, 17, 3},

	{0, 0, 21, 0},
	{1, 0, 21, 1},
	{2, 0, 21, 2},
	{999, 0, 21, 10000},

	{0, 0, 24, 10000},
	{999, 0, 24, 10000},

	{0, 0, 27, 10000},
	{1, 0, 27, 3},
	{2, 0, 27, 10000},
	{999, 0, 27, 5},

	{0, 0, 32, 0},
	{1, 0, 32, 276},
	{2, 0, 32, 1},
	{3, 0, 32, 2},
	{4, 0, 32, 10000},
	{999, 0, 32, 3},

	{0, 0, 36, 0},
	{1, 0, 36, 1},
	{2, 0, 36, 10000},
	{3, 0, 36, 3},
	{4, 0, 36, 3},
	{5, 0, 36, 3},

	{0, 0, 40, 10000},

	{0, 0, 47, 10000},
	{1, 0, 47, 3},
	{2, 0, 47, 4},
	{3, 0, 47, 5},
	{999, 0, 47, 6},

	{0, 0, 54, 10000},
	{1, 0, 54, 4},
	{999, 0, 54, 5},

	{0, 0, 60, 10000},
	{1, 0, 60, 2},
	{999, 0, 60, 3},

	{0, 0, 64, 10000},
	{1, 0, 64, 3},
	{2, 0, 64, 3},
	{999, 0, 64, 10000},

	{0, 0, 68, 0},
	{999, 0, 68, 10000},

	{0, 0, 69, 0},
	{1, 0, 69, 0},
	{2, 0, 69, 0},

	{0, 0, 70, 10000},
	{999, 0, 70, 10000},

	{0, 0, 73, 10000},
	{1, 0, 73, 10000},
	{2, 0, 73, 10000},
	{3, 0, 73, 10000},
	{4, 0, 73, 10000},
	{999, 0, 73, 10000},

	{0, 0, 77, 10000},
	{1, 0, 77, 10000},
	{999, 0, 77, 10000},

	{0, 0, 276, 0},
	{999, 0, 276, 10000},

	{0, 0, 399, 10000},
	{999, 0, 399, 10000},
}

// TestCtxIdx tests that the CtxIdx function returns the correct
// value given binIdx and ctxIdxOffset.
func TestCtxIdx(t *testing.T) {
	for _, tt := range ctxIdxTests {
		if got := CtxIdx(tt.binIdx, tt.maxBinIdxCtx, tt.ctxIdxOffset); got != tt.want {
			t.Errorf("CtxIdx(%d, %d, %d) = %d, want %d", tt.binIdx, tt.maxBinIdxCtx, tt.ctxIdxOffset, got, tt.want)
		}
	}
}

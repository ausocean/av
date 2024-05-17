/*
DESCRIPTION
  cavlc_fuzz.go provides exported wrappers for unexported CAVLC functions such
  that the fuzz package can access these functions for testing.

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

import "github.com/ausocean/av/codec/h264/h264dec/bits"

func FuzzParseLevelPrefix(br *bits.BitReader) (int, error) {
	return parseLevelPrefix(br)
}

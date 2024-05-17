/*
NAME
  crc.go
DESCRIPTION
  See Readme.md

AUTHOR
	Dan Kortschak <dan@ausocean.org>
  Saxon Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package psi

import (
	"encoding/binary"
	"hash/crc32"
	"math/bits"
)

// addCrc appends a crc table to a given psi table in bytes
func AddCRC(out []byte) []byte {
	t := make([]byte, len(out)+4)
	copy(t, out)
	UpdateCrc(t[1:])
	return t
}

// updateCrc updates the crc of bytes slice, writing the checksum into the last four bytes.
func UpdateCrc(b []byte) {
	crc32 := crc32_Update(0xffffffff, crc32_MakeTable(bits.Reverse32(crc32.IEEE)), b[:len(b)-4])
	binary.BigEndian.PutUint32(b[len(b)-4:], crc32)
}

func crc32_MakeTable(poly uint32) *crc32.Table {
	var t crc32.Table
	for i := range t {
		crc := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ poly
			} else {
				crc <<= 1
			}
		}
		t[i] = crc
	}
	return &t
}

func crc32_Update(crc uint32, tab *crc32.Table, p []byte) uint32 {
	for _, v := range p {
		crc = tab[byte(crc>>24)^v] ^ (crc << 8)
	}
	return crc
}

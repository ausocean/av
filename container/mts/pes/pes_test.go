/*
NAME
  pes_test.go

DESCRIPTION
  See Readme.md

AUTHOR
  Dan Kortschak <dan@ausocean.org>
  Saxon Nelson-Milton <saxon.milton@gmail.com>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package pes

import (
	"reflect"
	"testing"
)

const (
	dataLength = 3 // bytes
)

func TestPesToByteSlice(t *testing.T) {
	pkt := Packet{
		StreamID:     0xE0, // StreamID
		PDI:          byte(2),
		PTS:          100000,
		HeaderLength: byte(10),
		Stuff:        []byte{0xFF, 0xFF},
		Data:         []byte{0xEA, 0x4B, 0x12},
	}
	got := pkt.Bytes(nil)
	want := []byte{
		0x00, // packet start code prefix byte 1
		0x00, // packet start code prefix byte 2
		0x01, // packet start code prefix byte 3
		0xE0, // stream ID
		0x00, // PES Packet length byte 1
		0x00, // PES packet length byte 2
		0x80, // Marker bits,ScramblingControl, Priority, DAI, Copyright, Original
		0x80, // PDI, ESCR, ESRate, DSMTrickMode, ACI, CRC, Ext
		10,   // header length
		0x21, // PCR byte 1
		0x00, // pcr byte 2
		0x07, // pcr byte 3
		0x0D, // pcr byte 4
		0x41, // pcr byte 5
		0xFF, // Stuffing byte 1
		0xFF, // stuffing byte 3
		0xEA, // data byte 1
		0x4B, // data byte 2
		0x12, // data byte 3
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected packet encoding:\ngot: %#v\nwant:%#v", got, want)
	}
}

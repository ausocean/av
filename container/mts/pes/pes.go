/*
NAME
  pes.go -

DESCRIPTION
  See Readme.md

AUTHOR
  Saxon A. Nelson-Milton <saxon.milton@gmail.com>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

// TODO: add DSMTM, ACI, CRC, Ext fields
type Packet struct {
	StreamID     byte   // Type of stream
	Length       uint16 // Pes packet length in bytes after this field
	SC           byte   // Scrambling control
	Priority     bool   // Priority Indicator
	DAI          bool   // Data alginment indicator
	Copyright    bool   // Copyright indicator
	Original     bool   // Original data indicator
	PDI          byte   // PTS DTS indicator
	ESCRF        bool   // Elementary stream clock reference flag
	ESRF         bool   // Elementary stream rate reference flag
	DSMTMF       bool   // Dsm trick mode flag
	ACIF         bool   // Additional copy info flag
	CRCF         bool   // Not sure
	EF           bool   // Extension flag
	HeaderLength byte   // Pes header length
	PTS          uint64 // Presentation time stamp
	DTS          uint64 // Decoding timestamp
	ESCR         uint64 // Elementary stream clock reference
	ESR          uint32 // Elementary stream rate reference
	Stuff        []byte // Stuffing bytes
	Data         []byte // Pes packet data
}

func (p *Packet) Bytes(buf []byte) []byte {
	if buf == nil || cap(buf) != MaxPesSize {
		buf = make([]byte, 0, MaxPesSize)
	}
	buf = buf[:0]
	buf = append(buf, []byte{
		0x00, 0x00, 0x01,
		p.StreamID,
		byte((p.Length & 0xFF00) >> 8),
		byte(p.Length & 0x00FF),
		(0x2<<6 | p.SC<<4 | boolByte(p.Priority)<<3 | boolByte(p.DAI)<<2 |
			boolByte(p.Copyright)<<1 | boolByte(p.Original)),
		(p.PDI<<6 | boolByte(p.ESCRF)<<5 | boolByte(p.ESRF)<<4 | boolByte(p.DSMTMF)<<3 |
			boolByte(p.ACIF)<<2 | boolByte(p.CRCF)<<1 | boolByte(p.EF)),
		p.HeaderLength,
	}...)

	if p.PDI == byte(2) {
		ptsIdx := len(buf)
		buf = buf[:ptsIdx+5]
		gots.InsertPTS(buf[ptsIdx:], p.PTS)
	}
	buf = append(buf, append(p.Stuff, p.Data...)...)
	return buf
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

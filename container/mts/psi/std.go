/*
NAME
  std.go

DESCRIPTION
  See Readme.md

AUTHOR
  Saxon Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package psi

const (
	PmtTimeLocationPil = 44
)

// Std PSI in bytes form
var (
	StandardPatBytes = []byte{
		0x00, // pointer

		// ---- section included in data sent to CRC32 during check
		// table header
		0x00, // table id
		0xb0, // section syntax indicator:1|private bit:1|reserved:2|section length:2|more bytes...:2
		0x0d, // more bytes...

		// syntax section
		0x00, 0x01, // table id extension
		0xc1, // reserved bits:2|version:5|use now:1 1100 0001
		0x00, // section number
		0x00, // last section number
		// table data
		0x00, 0x01, // Program number
		0xf0, 0x00, // reserved:3|program map PID:13

		// 0x2a, 0xb1, 0x04, 0xb2, // CRC
		// ----
	}
	StandardPmtBytes = []byte{
		0x00, // pointer

		// ---- section included in data sent to CRC32 during check
		// table header
		0x02, // table id
		0xb0, // section syntax indicator:1|private bit:1|reserved:2|section length:2|more bytes...:2
		0x12, // more bytes...

		// syntax section
		0x00, 0x01, // table id extension
		0xc1, // reserved bits:3|version:5|use now:1
		0x00, // section number
		0x00, // last section number
		// table data
		0xe1, 0x00, // reserved:3|PCR PID:13
		0xf0, 0x00, // reserved:4|unused:2|program info length:10
		// No program descriptors since program info length is 0.
		// elementary stream info data
		0x1b,       // stream type
		0xe1, 0x00, // reserved:3|elementary PID:13
		0xf0, 0x00, // reserved:4|unused:2|ES info length:10
		// No elementary stream descriptors since ES info length is 0.

		// 0x15, 0xbd, 0x4d, 0x56, // CRC
		// ----
	}
)

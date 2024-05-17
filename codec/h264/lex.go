/*
NAME
  lex.go

DESCRIPTION
  lex.go provides a lexer to lex h264 bytestream into access units.

AUTHOR
  Dan Kortschak <dan@ausocean.org>
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package h264 provides a h264 bytestream lexer and RTP H264 access unit
// extracter.
package h264

import (
	"io"
	"time"

	"github.com/ausocean/av/codec/codecutil"
)

var noDelay = make(chan time.Time)

func init() {
	close(noDelay)
}

var h264Prefix = [...]byte{0x00, 0x00, 0x01, 0x09, 0xf0}

// Lex lexes H.264 NAL units read from src into separate writes
// to dst with successive writes being performed not earlier than the specified
// delay. NAL units are split after type 1 (Coded slice of a non-IDR picture), 5
// (Coded slice of a IDR picture) and 8 (Picture parameter set).
func Lex(dst io.Writer, src io.Reader, delay time.Duration) error {
	var tick <-chan time.Time
	if delay == 0 {
		tick = noDelay
	} else {
		ticker := time.NewTicker(delay)
		defer ticker.Stop()
		tick = ticker.C
	}

	const bufSize = 8 << 10

	c := codecutil.NewByteScanner(src, make([]byte, 4<<10)) // Standard file buffer size.

	buf := make([]byte, len(h264Prefix), bufSize)
	copy(buf, h264Prefix[:])
	writeOut := false

	for {
		var b byte
		var err error
		buf, b, err = c.ScanUntil(buf, 0x00)
		if err != nil {
			if err != io.EOF {
				return err
			}
			if len(buf) != 0 {
				return io.ErrUnexpectedEOF
			}
			return io.EOF
		}

		for n := 1; b == 0x0 && n < 4; n++ {
			b, err = c.ReadByte()
			if err != nil {
				if err != io.EOF {
					return err
				}
				return io.ErrUnexpectedEOF
			}
			buf = append(buf, b)

			if b != 0x1 || (n != 2 && n != 3) {
				continue
			}

			if writeOut {
				<-tick
				_, err := dst.Write(buf[:len(buf)-(n+1)])
				if err != nil {
					return err
				}
				buf = make([]byte, len(h264Prefix)+n, bufSize)
				copy(buf, h264Prefix[:])
				buf = append(buf, 1)
				writeOut = false
			}

			b, err = c.ReadByte()
			if err != nil {
				if err != io.EOF {
					return err
				}
				return io.ErrUnexpectedEOF
			}
			buf = append(buf, b)

			// http://www.itu.int/rec/dologin_pub.asp?lang=e&id=T-REC-H.264-200305-S!!PDF-E&type=items
			// Table 7-1 NAL unit type codes
			const (
				nonIdrPic   = 1
				idrPic      = 5
				suppEnhInfo = 6
				paramSet    = 8
			)
			switch nalTyp := b & 0x1f; nalTyp {
			case nonIdrPic, idrPic, paramSet, suppEnhInfo:
				writeOut = true
			}
		}
	}
}

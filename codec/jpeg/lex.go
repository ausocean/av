/*
NAME
  lex.go

DESCRIPTION
  lex.go provides a lexer to extract separate JPEG images from a JPEG stream.
  This could either be a series of descrete JPEG images, or an MJPEG stream.

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


package jpeg

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/ausocean/utils/logging"
)

var Log logging.Logger

var noDelay = make(chan time.Time)

func init() {
	close(noDelay)
}

// Lex parses JPEG frames read from src into separate writes to dst with
// successive writes being performed not earlier than the specified delay.
func Lex(dst io.Writer, src io.Reader, delay time.Duration) error {
	var tick <-chan time.Time
	if delay == 0 {
		tick = noDelay
	} else {
		ticker := time.NewTicker(delay)
		defer ticker.Stop()
		tick = ticker.C
	}

	r := bufio.NewReader(src)
	for {
		buf := make([]byte, 2, 4<<10)
		n, err := r.Read(buf)
		if n < 2 {
			return io.ErrUnexpectedEOF
		}
		if err != nil {
			return err
		}

		if !bytes.Equal(buf, []byte{0xff, 0xd8}) {
			return fmt.Errorf("parser: not JPEG frame start: %#v", buf)
		}

		nImg := 1

		var last byte
		for {
			b, err := r.ReadByte()
			if err != nil {
				if err == io.EOF {
					return io.ErrUnexpectedEOF
				}
				return err
			}

			buf = append(buf, b)

			if last == 0xff && b == 0xd8 {
				nImg++
			}

			if last == 0xff && b == 0xd9 {
				nImg--
			}

			if nImg == 0 {
				<-tick
				Log.Debug("writing buf", "len(buf)", len(buf))
				_, err = dst.Write(buf)
				if err != nil {
					return err
				}
				break
			}

			last = b
		}
	}
}

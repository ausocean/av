/*
NAME
  bytescanner.go

AUTHOR
  Dan Kortschak <dan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package bytescan implements a byte-level scanner.
package codecutil

import "io"

// ByteScanner is a byte scanner.
type ByteScanner struct {
	buf []byte
	off int

	// r is the source of data for the scanner.
	r io.Reader
}

// NewByteScanner returns a scanner initialised with an io.Reader and a read buffer.
func NewByteScanner(r io.Reader, buf []byte) *ByteScanner {
	return &ByteScanner{r: r, buf: buf[:0]}
}

// ScanUntil scans the scanner's underlying io.Reader until a delim byte
// has been read, appending all read bytes to dst. The resulting appended data,
// the last read byte and whether the last read byte was the delimiter.
func (c *ByteScanner) ScanUntil(dst []byte, delim byte) (res []byte, b byte, err error) {
outer:
	for {
		var i int
		for i, b = range c.buf[c.off:] {
			if b != delim {
				continue
			}
			dst = append(dst, c.buf[c.off:c.off+i+1]...)
			c.off += i + 1
			break outer
		}
		dst = append(dst, c.buf[c.off:]...)
		err = c.reload()
		if err != nil {
			break
		}
	}
	return dst, b, err
}

// ReadByte is an unexported ReadByte.
func (c *ByteScanner) ReadByte() (byte, error) {
	if c.off >= len(c.buf) {
		err := c.reload()
		if err != nil {
			return 0, err
		}
	}
	b := c.buf[c.off]
	c.off++
	return b, nil
}

// reload re-fills the scanner's buffer.
func (c *ByteScanner) reload() error {
	n, err := c.r.Read(c.buf[:cap(c.buf)])
	c.buf = c.buf[:n]
	if err != nil {
		if err != io.EOF {
			return err
		}
		if n == 0 {
			return io.EOF
		}
	}
	c.off = 0
	return nil
}

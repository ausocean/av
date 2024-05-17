/*
DESCRIPTION
  extract.go provides an Extractor to get JPEG images from an RTP/JPEG stream
  defined by RFC 2435 (see https://tools.ietf.org/html/rfc2435).

AUTHOR
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
	"fmt"
	"io"
	"time"

	"github.com/ausocean/av/protocol/rtp"
)

const maxRTPSize = 1500 // Max ethernet transmission unit in bytes.

// Extractor is an Extractor for extracting JPEG from an RTP stream.
type Extractor struct {
	dst io.Writer // The destination we'll be writing extracted JPEGs to.
}

// NewExtractor returns a new Extractor.
func NewExtractor() *Extractor { return &Extractor{} }

// Extract will continously read RTP packets from src containing JPEG (in RTP
// payload format) and extract the JPEG images, sending them to dst. This
// function expects that each read from src will provide a single RTP packet.
func (e *Extractor) Extract(dst io.Writer, src io.Reader, delay time.Duration) error {
	buf := make([]byte, maxRTPSize)
	ctx := NewContext(dst)

	for {
		n, err := src.Read(buf)
		switch err {
		case nil: // Do nothing.
		case io.EOF:
			return nil
		default:
			return fmt.Errorf("source read error: %v\n", err)
		}

		// Get payload from RTP packet.
		p, err := rtp.Payload(buf[:n])
		if err != nil {
			return fmt.Errorf("could not get RTP payload: %w\n", err)
		}

		// Also grab the marker so that we know when the JPEG is finished.
		m, err := rtp.Marker(buf[:n])
		if err != nil {
			return fmt.Errorf("could not read RTP marker: %w", err)
		}

		err = ctx.ParsePayload(p, m)
		switch err {
		case nil: // Do nothing.
		case ErrNoFrameStart: // If no frame start then we continue until we get one.
		default:
			return fmt.Errorf("could not parse JPEG scan: %w", err)
		}
	}
}

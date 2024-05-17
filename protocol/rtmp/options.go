/*
DESCRIPTION
  options.go provides RTMP connection option functions used to change
  configuration parameters such as timeouts and bandwidths.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package rtmp

import "errors"

// Option parameter errors.
var (
	ErrClientBandwidth = errors.New("bad client bandwidth")
	ErrServerBandwidth = errors.New("bad server bandwidth")
	ErrLinkTimeout     = errors.New("bad link timeout")
)

// ClientBandwidth changes the Conn's clientBW parameter to the given value.
// See default value under conn.go.
func ClientBandwidth(b int) func(*Conn) error {
	return func(c *Conn) error {
		if b <= 0 {
			return ErrClientBandwidth
		}
		c.clientBW = uint32(b)
		return nil
	}
}

// ServerBandwidth changes the Conn's serverBW parameter to the given value.
// See default value under conn.go.
func ServerBandwidth(b int) func(*Conn) error {
	return func(c *Conn) error {
		if b <= 0 {
			return ErrServerBandwidth
		}
		c.serverBW = uint32(b)
		return nil
	}
}

// LinkTimeout changes the Conn.link's timeout parameter to the given value.
// See default value under conn.go.
func LinkTimeout(t uint) func(*Conn) error {
	return func(c *Conn) error {
		if t <= 0 {
			return ErrLinkTimeout
		}
		c.link.timeout = t
		return nil
	}
}

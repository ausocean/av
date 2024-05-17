/*
NAME
  client.go

DESCRIPTION
  client.go provides an RTP client.

AUTHOR
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package rtp

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// Client describes an RTP client that can receive an RTP stream and implements
// io.Reader.
type Client struct {
	r        *PacketReader
	ssrc     uint32
	mu       sync.Mutex
	sequence uint16
	cycles   uint16
}

// NewClient returns a pointer to a new Client.
//
// addr is the address of form <ip>:<port> that we expect to receive
// RTP at.
func NewClient(addr string) (*Client, error) {
	c := &Client{r: &PacketReader{}}

	a, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	c.r.PacketConn, err = net.ListenUDP("udp", a)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// SSRC returns the identified for the source from which the RTP packets being
// received are coming from.
func (c *Client) SSRC() uint32 {
	return c.ssrc
}

// Read implements io.Reader.
func (c *Client) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if err != nil {
		return n, err
	}
	if c.ssrc == 0 {
		c.ssrc, _ = SSRC(p[:n])
	}
	s, _ := Sequence(p[:n])
	c.setSequence(s)
	return n, err
}

// Close will close the RTP client's connection.
func (c *Client) Close() error {
	return c.r.PacketConn.Close()
}

// setSequence sets the most recently received sequence number, and updates the
// cycles count if the sequence number has rolled over.
func (c *Client) setSequence(s uint16) {
	c.mu.Lock()
	if s < c.sequence {
		c.cycles++
	}
	c.sequence = s
	c.mu.Unlock()
}

// Sequence returns the most recent RTP packet sequence number received.
func (c *Client) Sequence() uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sequence
}

// Cycles returns the number of RTP sequence number cycles that have been received.
func (c *Client) Cycles() uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cycles
}

// PacketReader provides an io.Reader interface to an underlying UDP PacketConn.
type PacketReader struct {
	net.PacketConn
}

// Read implements io.Reader.
func (r PacketReader) Read(b []byte) (int, error) {
	const readTimeout = 5 * time.Second
	err := r.PacketConn.SetReadDeadline(time.Now().Add(readTimeout))
	if err != nil {
		return 0, fmt.Errorf("could not set read deadline for PacketConn: %w", err)
	}
	n, _, err := r.PacketConn.ReadFrom(b)
	return n, err
}

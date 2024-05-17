/*
NAME
  client.go

DESCRIPTION
  client.go provides a Client type providing functionality to send RTSP requests
  of methods DESCRIBE, OPTIONS, SETUP and PLAY to an RTSP server.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package rtsp

import (
	"net"
	"net/url"
	"strconv"
)

// Client describes an RTSP Client.
type Client struct {
	cSeq      int
	addr      string
	url       *url.URL
	conn      net.Conn
	sessionID string
}

// NewClient returns a pointer to a new Client and the local address of the
// RTSP connection. The address addr will be parsed and a connection to the
// RTSP server will be made.
func NewClient(addr string) (c *Client, local, remote *net.TCPAddr, err error) {
	c = &Client{addr: addr}
	c.url, err = url.Parse(addr)
	if err != nil {
		return nil, nil, nil, err
	}
	c.conn, err = net.Dial("tcp", c.url.Host)
	if err != nil {
		return nil, nil, nil, err
	}
	local = c.conn.LocalAddr().(*net.TCPAddr)
	remote = c.conn.RemoteAddr().(*net.TCPAddr)
	return
}

// Close closes the RTSP connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Describe forms and sends an RTSP request of method DESCRIBE to the RTSP server.
func (c *Client) Describe() (*Response, error) {
	req, err := NewRequest("DESCRIBE", c.nextCSeq(), c.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/sdp")
	return c.Do(req)
}

// Options forms and sends an RTSP request of method OPTIONS to the RTSP server.
func (c *Client) Options() (*Response, error) {
	req, err := NewRequest("OPTIONS", c.nextCSeq(), c.url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Setup forms and sends an RTSP request of method SETUP to the RTSP server.
func (c *Client) Setup(track, transport string) (*Response, error) {
	u, err := url.Parse(c.addr + "/" + track)
	if err != nil {
		return nil, err
	}

	req, err := NewRequest("SETUP", c.nextCSeq(), u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Transport", transport)

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	c.sessionID = resp.Header.Get("Session")

	return resp, err
}

// Play forms and sends an RTSP request of method PLAY to the RTSP server
func (c *Client) Play() (*Response, error) {
	req, err := NewRequest("PLAY", c.nextCSeq(), c.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Session", c.sessionID)

	return c.Do(req)
}

// Do sends the given RTSP request req, reads any responses and returns the response
// and any errors.
func (c *Client) Do(req *Request) (*Response, error) {
	err := req.Write(c.conn)
	if err != nil {
		return nil, err
	}

	resp, err := ReadResponse(c.conn)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// nextCSeq provides the next CSeq number for the next RTSP request.
func (c *Client) nextCSeq() string {
	c.cSeq++
	return strconv.Itoa(c.cSeq)
}

/*
NAME
  conn.go

DESCRIPTION
  RTMP connection functionality.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package rtmp provides an RTMP client implementation.
// The package currently supports live streaming to YouTube.
package rtmp

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/ausocean/av/protocol/rtmp/amf"
)

// Log levels used by Log.
const (
	DebugLevel int8 = -1
	InfoLevel  int8 = 0
	WarnLevel  int8 = 1
	ErrorLevel int8 = 2
	FatalLevel int8 = 5
)

// Configuration defaults.
const (
	defaultTimeout         = 10
	defaultClientBandwidth = 2500000
	defaultServerBandwidth = 2500000
)

// Conn represents an RTMP connection.
type Conn struct {
	inChunkSize          uint32
	outChunkSize         uint32
	nBytesIn             uint32
	nBytesInSent         uint32
	streamID             uint32
	serverBW             uint32
	clientBW             uint32
	clientBW2            uint8
	isPlaying            bool
	numInvokes           int32
	methodCalls          []method
	channelsAllocatedIn  int32
	channelsAllocatedOut int32
	channelsIn           []*packet
	channelsOut          []*packet
	channelTimestamp     []int32
	deferred             []byte
	link                 link
	log                  Log
}

// link represents RTMP URL and connection information.
type link struct {
	host     string
	playpath string
	url      string
	app      string
	auth     string
	flags    int32
	protocol int32
	timeout  uint
	port     uint16
	conn     net.Conn
}

// method represents an RTMP method.
type method struct {
	name string
	num  int32
}

// Log defines the RTMP logging function.
type Log func(level int8, message string, params ...interface{})

// flvTagheaderSize is the FLV header size we expect.
// NB: We don't accept extended headers.
const flvTagheaderSize = 11

// Dial connects to RTMP server specified by the given URL and returns the connection.
func Dial(url string, log Log, options ...func(*Conn) error) (*Conn, error) {
	log(DebugLevel, pkg+"rtmp.Dial")
	c := Conn{
		inChunkSize:  128,
		outChunkSize: 128,
		clientBW:     defaultClientBandwidth,
		clientBW2:    2,
		serverBW:     defaultServerBandwidth,
		log:          log,
		link: link{
			timeout: defaultTimeout,
		},
	}

	// Apply any options that have been provided.
	for _, option := range options {
		err := option(&c)
		if err != nil {
			return nil, fmt.Errorf("error from option: %w", err)
		}
	}

	var err error
	c.link.protocol, c.link.host, c.link.port, c.link.app, c.link.playpath, err = parseURL(url)
	if err != nil {
		return nil, fmt.Errorf("could not parse url: %w", err)
	}
	c.link.url = rtmpProtocolStrings[c.link.protocol] + "://" + c.link.host + ":" + strconv.Itoa(int(c.link.port)) + "/" + c.link.app
	c.link.protocol |= featureWrite

	err = connect(&c)
	if err != nil {
		return nil, fmt.Errorf("could not connect: %w", err)
	}
	return &c, nil
}

// Close terminates the RTMP connection.
// NB: Close is idempotent and the connection value is cleared completely.
func (c *Conn) Close() error {
	if !c.isConnected() {
		return errNotConnected
	}
	c.log(DebugLevel, pkg+"Conn.Close")
	if c.streamID > 0 {
		if c.link.protocol&featureWrite != 0 {
			err := sendFCUnpublish(c)
			if err != nil {
				return fmt.Errorf("could not send fc unpublish: %w", err)
			}
		}
		err := sendDeleteStream(c, float64(c.streamID))
		if err != nil {
			return fmt.Errorf("could not send delete stream: %w", err)
		}
	}
	err := c.link.conn.Close()
	if err != nil {
		return fmt.Errorf("could not close link conn: %w", err)
	}
	*c = Conn{}
	return nil
}

// Write writes a frame (flv tag) to the rtmp connection.
func (c *Conn) Write(data []byte) (int, error) {
	if !c.isConnected() {
		return 0, errNotConnected
	}
	if len(data) < flvTagheaderSize {
		return 0, ErrInvalidFlvTag
	}
	if data[0] == packetTypeInfo || (data[0] == 'F' && data[1] == 'L' && data[2] == 'V') {
		return 0, errUnimplemented
	}

	pkt := packet{
		packetType: data[0],
		bodySize:   amf.DecodeInt24(data[1:4]),
		timestamp:  amf.DecodeInt24(data[4:7]) | uint32(data[7])<<24,
		channel:    chanSource,
		streamID:   c.streamID,
	}

	pkt.resize(pkt.bodySize, headerSizeAuto)
	copy(pkt.body, data[flvTagheaderSize:flvTagheaderSize+pkt.bodySize])
	err := pkt.writeTo(c, false)
	if err != nil {
		return 0, fmt.Errorf("could not write packet to connection: %w", err)
	}
	return len(data), nil
}

// I/O functions

// read from an RTMP connection. Sends a bytes received message if the
// number of bytes received (nBytesIn) is greater than the number sent
// (nBytesInSent) by 10% of the bandwidth.
func (c *Conn) read(buf []byte) (int, error) {
	err := c.link.conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(c.link.timeout)))
	if err != nil {
		return 0, fmt.Errorf("could not set read deadline: %w", err)
	}
	n, err := io.ReadFull(c.link.conn, buf)
	if err != nil {
		c.log(DebugLevel, pkg+"read failed", "error", err.Error())
		return 0, fmt.Errorf("could not read conn: %w", err)
	}
	c.nBytesIn += uint32(n)
	if c.nBytesIn > (c.nBytesInSent + c.clientBW/10) {
		err := sendBytesReceived(c)
		if err != nil {
			return n, fmt.Errorf("could not send bytes received: %w", err) // NB: we still read n bytes, even though send bytes failed
		}
	}
	return n, nil
}

// write to an RTMP connection.
func (c *Conn) write(buf []byte) (int, error) {
	//ToDo: consider using a different timeout for writes than for reads
	err := c.link.conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(c.link.timeout)))
	if err != nil {
		return 0, fmt.Errorf("could not set write deadline: %w", err)
	}
	n, err := c.link.conn.Write(buf)
	if err != nil {
		c.log(WarnLevel, pkg+"write failed", "error", err.Error())
		return 0, fmt.Errorf("could not write to conn: %w", err)
	}
	return n, nil
}

// isConnected returns true if the RTMP connection is up.
func (c *Conn) isConnected() bool {
	return c.link.conn != nil
}

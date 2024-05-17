/*
NAME
  client.go

DESCRIPTION
  Client.go provides an implemntation of a basic RTCP Client that will send
  receiver reports, and receive sender reports to parse relevant statistics.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/ausocean/av/protocol/rtp"
	"github.com/ausocean/utils/logging"
)

const (
	clientSSRC          = 1 // Any non-zero value will do.
	defaultClientName   = "Client"
	defaultSendInterval = 2 * time.Second
	delayUnit           = 1.0 / 65536.0
	pkg                 = "rtcp: "
	rtcpVer             = 2
	receiverBufSize     = 200
)

// Log describes a function signature required by the RTCP for the purpose of
// logging.
type Log func(lvl int8, msg string, args ...interface{})

// Client is an RTCP Client that will handle receiving SenderReports from a server
// and sending out ReceiverReports.
type Client struct {
	cAddr       *net.UDPAddr          // Address of client.
	sAddr       *net.UDPAddr          // Address of RTSP server.
	name        string                // Name of the client for source description purposes.
	sourceSSRC  uint32                // Source identifier of this client.
	mu          sync.Mutex            // Will be used to change parameters during operation safely.
	seq         uint32                // Last RTP sequence number.
	senderTs    [8]byte               // The timestamp of the last sender report.
	interval    time.Duration         // Interval between sender report and receiver report.
	receiveTime time.Time             // Time last sender report was received.
	buf         [receiverBufSize]byte // Buf used to store the receiver report and source descriptions.
	conn        *net.UDPConn          // The UDP connection used for receiving and sending RTSP packets.
	wg          sync.WaitGroup        // This is used to wait for send and recv routines to stop when Client is stopped.
	quit        chan struct{}         // Channel used to communicate quit signal to send and recv routines.
	log         Log                   // Used to log any messages.
	rtpClt      *rtp.Client
	err         chan error // Client will send any errors through this chan. Can be accessed by Err().
}

// NewClient returns a pointer to a new Client.
func NewClient(clientAddress, serverAddress string, rtpClt *rtp.Client, l Log) (*Client, error) {
	c := &Client{
		name:     defaultClientName,
		quit:     make(chan struct{}),
		err:      make(chan error),
		interval: defaultSendInterval,
		rtpClt:   rtpClt,
		log:      l,
	}

	var err error
	c.cAddr, err = net.ResolveUDPAddr("udp", clientAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("can't resolve Client address, failed with error: %v\n", err))
	}

	c.sAddr, err = net.ResolveUDPAddr("udp", serverAddress)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("can't resolve server address, failed with error: %v\n", err))
	}

	c.conn, err = net.DialUDP("udp", c.cAddr, c.sAddr)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("can't dial, failed with error: %v\n", err))
	}
	return c, nil
}

// SetSendInterval sets a custom receiver report send interval (default is 5 seconds.)
func (c *Client) SetSendInterval(d time.Duration) {
	c.interval = d
}

// SetName sets a custom client name for use in receiver report source description.
// Default is "Client".
func (c *Client) SetName(name string) {
	c.name = name
}

// Start starts the listen and send routines. This will start the process of
// receiving and parsing sender reports, and the process of sending receiver
// reports to the server.
func (c *Client) Start() {
	c.log(logging.Debug, pkg+"Client is starting")
	c.wg.Add(2)
	go c.recv()
	go c.send()
}

// Stop sends a quit signal to the send and receive routines and closes the
// UDP connection. It will wait until both routines have returned.
func (c *Client) Stop() {
	c.log(logging.Debug, pkg+"Client is stopping")
	close(c.quit)
	c.conn.Close()
	c.wg.Wait()
	close(c.err)
}

// Err provides read access to the Client err channel. This must be checked
// otherwise the client will block if an error encountered.
func (c *Client) Err() <-chan error {
	return c.err
}

// recv reads from the UDP connection and parses SenderReports.
func (c *Client) recv() {
	defer c.wg.Done()
	c.log(logging.Debug, pkg+"Client is receiving")
	buf := make([]byte, 4096)
	for {
		select {
		case <-c.quit:
			return
		default:
			n, _, err := c.conn.ReadFromUDP(buf)
			if err != nil {
				c.err <- err
				continue
			}
			c.log(logging.Debug, pkg+"sender report received", "report", buf[:n])
			c.parse(buf[:n])
		}
	}
}

// send writes receiver reports to the server.
func (c *Client) send() {
	defer c.wg.Done()
	c.log(logging.Debug, pkg+"Client is sending")
	for {
		select {
		case <-c.quit:
			return
		default:
			time.Sleep(c.interval)

			report := ReceiverReport{
				Header: Header{
					Version:     rtcpVer,
					Padding:     false,
					ReportCount: 1,
					Type:        typeReceiverReport,
				},
				SenderSSRC: clientSSRC,
				Blocks: []ReportBlock{
					ReportBlock{
						SourceIdentifier:  c.rtpClt.SSRC(),
						FractionLost:      0,
						PacketsLost:       math.MaxUint32,
						HighestSequence:   uint32((c.rtpClt.Cycles() << 16) | c.rtpClt.Sequence()),
						Jitter:            c.jitter(),
						SenderReportTs:    c.lastSenderTs(),
						SenderReportDelay: c.delay(),
					},
				},
				Extensions: nil,
			}

			description := Description{
				Header: Header{
					Version:     rtcpVer,
					Padding:     false,
					ReportCount: 1,
					Type:        typeDescription,
				},
				Chunks: []Chunk{
					Chunk{
						SSRC: clientSSRC,
						Items: []SDESItem{
							SDESItem{
								Type: typeCName,
								Text: []byte(c.name),
							},
						},
					},
				},
			}

			c.log(logging.Debug, pkg+"sending receiver report")
			_, err := c.conn.Write(c.formPayload(&report, &description))
			if err != nil {
				c.err <- err
			}
		}
	}
}

// formPayload takes a pointer to a ReceiverReport and a pointer to a
// Source Description and calls Bytes on both, writing to the underlying Client
// buf. A slice to the combined writtem memory is returned.
func (c *Client) formPayload(r *ReceiverReport, d *Description) []byte {
	rl := len(r.Bytes(c.buf[:]))
	dl := len(d.Bytes(c.buf[rl:]))
	t := rl + dl
	if t > cap(c.buf) {
		panic("Client buf not big enough")
	}
	return c.buf[:t]
}

// parse will read important statistics from sender reports.
func (c *Client) parse(buf []byte) {
	c.markReceivedTime()
	t, err := ParseTimestamp(buf)
	if err != nil {
		c.err <- fmt.Errorf("could not get timestamp from sender report, failed with error: %w", err)
	}
	c.setSenderTs(t)
}

// jitter returns the interarrival jitter as described by RTCP specifications:
// https://tools.ietf.org/html/rfc3550
// TODO(saxon): complete this.
func (c *Client) jitter() uint32 {
	return 0
}

// setSenderTs allows us to safely set the current sender report timestamp.
func (c *Client) setSenderTs(t Timestamp) {
	c.mu.Lock()
	binary.BigEndian.PutUint32(c.senderTs[:], t.Seconds)
	binary.BigEndian.PutUint32(c.senderTs[4:], t.Fraction)
	c.mu.Unlock()
}

// lastSenderTs returns the timestamp of the most recent sender report.
func (c *Client) lastSenderTs() uint32 {
	c.mu.Lock()
	t := binary.BigEndian.Uint32(c.senderTs[2:])
	c.mu.Unlock()
	return t
}

// delay returns the duration between the receive time of the last sender report
// and now. This is called when forming a receiver report.
func (c *Client) delay() uint32 {
	c.mu.Lock()
	t := c.receiveTime
	c.mu.Unlock()
	return uint32(time.Now().Sub(t).Seconds() / delayUnit)
}

// markReceivedTime is called when a sender report is received to mark the receive time.
func (c *Client) markReceivedTime() {
	c.mu.Lock()
	c.receiveTime = time.Now()
	c.mu.Unlock()
}

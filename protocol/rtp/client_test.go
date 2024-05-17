/*
NAME
  client_test.go

DESCRIPTION
  client_test.go provides testing utilities to check RTP client functionality
  provided in client.go.

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
	"bytes"
	"fmt"
	"io"
	"net"
	"testing"
)

// TestReceive checks that the Client can correctly receive RTP packets and
// perform a specificed operation on the packets before storing in the ringBuffer.
func TestReceive(t *testing.T) {
	const (
		clientAddr    = "localhost:8000"
		packetsToSend = 20
	)

	testErr := make(chan error)
	serverErr := make(chan error)
	done := make(chan struct{})
	clientReady := make(chan struct{})
	var c *Client

	// Start routine to read from client.
	go func() {
		// Create and start the client.
		var err error
		c, err = NewClient(clientAddr)
		if err != nil {
			testErr <- fmt.Errorf("could not create client, failed with error: %w\n", err)
		}
		close(clientReady)

		// Read packets using the client and check them with expected.
		var packetsReceived int
		buf := make([]byte, 4096)
		for packetsReceived != packetsToSend {
			n, err := c.Read(buf)
			switch err {
			case nil:
			case io.EOF:
				continue
			default:
				testErr <- fmt.Errorf("unexpected error from c.Read: %w\n", err)
			}

			// Create expected data and apply operation if there is one.
			expect := (&Packet{Version: rtpVer, Payload: []byte{byte(packetsReceived)}}).Bytes(nil)

			// Compare.
			got := buf[:n]
			if !bytes.Equal(got, expect) {
				testErr <- fmt.Errorf("did not get expected result. \nGot: %v\n Want: %v\n", got, expect)
			}
			packetsReceived++
		}
		close(done)
	}()

	// Start the RTP server.
	go func() {
		<-clientReady
		cAddr, err := net.ResolveUDPAddr("udp", clientAddr)
		if err != nil {
			serverErr <- fmt.Errorf("could not resolve server address, failed with err: %w\n", err)
		}

		conn, err := net.DialUDP("udp", nil, cAddr)
		if err != nil {
			serverErr <- fmt.Errorf("could not dial udp, failed with err: %w\n", err)
		}

		// Send packets to the client.
		for i := 0; i < packetsToSend; i++ {
			p := (&Packet{Version: rtpVer, Payload: []byte{byte(i)}}).Bytes(nil)
			_, err := conn.Write(p)
			if err != nil {
				serverErr <- fmt.Errorf("could not write packet to conn, failed with err: %w\n", err)
			}
		}
	}()

	<-clientReady
loop:
	for {
		select {
		case err := <-testErr:
			t.Fatal(err)
		case err := <-serverErr:
			t.Fatal(err)
		case <-done:
			break loop
		default:
		}
	}
}

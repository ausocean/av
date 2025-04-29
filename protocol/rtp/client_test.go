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
	"time"
)

// TestReceive checks that the Client can correctly receive RTP packets
// and perform a specified operation on the packets before storing in the ringBuffer.
func TestReceive(t *testing.T) {
	const packetsToSend = 20

	testErr := make(chan error, 1)
	serverErr := make(chan error, 1)
	done := make(chan struct{})

	// Dynamically allocate a free UDP port.
	l, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("could not find free port: %v", err)
	}
	port := l.LocalAddr().(*net.UDPAddr).Port
	l.Close()

	clientAddr := fmt.Sprintf("127.0.0.1:%d", port)

	// Create the client.
	c, err := NewClient(clientAddr)
	if err != nil {
		t.Fatalf("could not create client: %v", err)
	}
	defer c.Close()

	// Start reading from the client.
	go func() {
		defer close(done)

		var packetsReceived int
		buf := make([]byte, 4096)

		for packetsReceived != packetsToSend {
			n, err := c.Read(buf)
			if err != nil {
				if err == io.EOF {
					continue
				}
				select {
				case testErr <- fmt.Errorf("unexpected error from c.Read: %w", err):
				default:
				}
				return
			}

			expect := (&Packet{Version: rtpVer, Payload: []byte{byte(packetsReceived)}}).Bytes(nil)
			got := buf[:n]

			if !bytes.Equal(got, expect) {
				select {
				case testErr <- fmt.Errorf("did not get expected result. Got: %v, Want: %v", got, expect):
				default:
				}
				return
			}

			packetsReceived++
		}
	}()

	// Start the RTP "server" to send packets.
	go func() {
		cAddr, err := net.ResolveUDPAddr("udp", clientAddr)
		if err != nil {
			select {
			case serverErr <- fmt.Errorf("could not resolve server address: %w", err):
			default:
			}
			return
		}

		conn, err := net.DialUDP("udp", nil, cAddr)
		if err != nil {
			select {
			case serverErr <- fmt.Errorf("could not dial udp: %w", err):
			default:
			}
			return
		}
		defer conn.Close()

		for i := 0; i < packetsToSend; i++ {
			p := (&Packet{Version: rtpVer, Payload: []byte{byte(i)}}).Bytes(nil)
			if _, err := conn.Write(p); err != nil {
				select {
				case serverErr <- fmt.Errorf("could not write packet: %w", err):
				default:
				}
				return
			}
		}
	}()

	// Wait for result.
	select {
	case err := <-testErr:
		t.Fatal(err)
	case err := <-serverErr:
		t.Fatal(err)
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out")
	}
}

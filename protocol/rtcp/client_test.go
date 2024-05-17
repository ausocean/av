/*
NAME
  client_test.go

DESCRIPTION
  client_test.go contains testing utilities for functionality provided in client.go.

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
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ausocean/av/protocol/rtp"
	"github.com/ausocean/utils/logging"
)

// TestFromPayload checks that formPayload is working as expected.
func TestFormPayload(t *testing.T) {
	// Expected data from a valid RTCP packet.
	expect := []byte{
		0x81, 0xc9, 0x00, 0x07,
		0xd6, 0xe0, 0x98, 0xda,
		0x6f, 0xad, 0x40, 0xc6,
		0x00, 0xff, 0xff, 0xff,
		0x00, 0x01, 0x83, 0x08,
		0x00, 0x00, 0x00, 0x20,
		0xb9, 0xe1, 0x25, 0x2a,
		0x00, 0x00, 0x2b, 0xf9,
		0x81, 0xca, 0x00, 0x04,
		0xd6, 0xe0, 0x98, 0xda,
		0x01, 0x08, 0x73, 0x61,
		0x78, 0x6f, 0x6e, 0x2d,
		0x70, 0x63, 0x00, 0x00,
	}

	report := ReceiverReport{
		Header: Header{
			Version:     2,
			Padding:     false,
			ReportCount: 1,
			Type:        typeReceiverReport,
		},
		SenderSSRC: 3605043418,
		Blocks: []ReportBlock{
			ReportBlock{
				SourceIdentifier:  1873625286,
				FractionLost:      0,
				PacketsLost:       math.MaxUint32,
				HighestSequence:   99080,
				Jitter:            32,
				SenderReportTs:    3118540074,
				SenderReportDelay: 11257,
			},
		},
		Extensions: nil,
	}

	description := Description{
		Header: Header{
			Version:     2,
			Padding:     false,
			ReportCount: 1,
			Type:        typeDescription,
		},
		Chunks: []Chunk{
			Chunk{
				SSRC: 3605043418,
				Items: []SDESItem{
					SDESItem{
						Type: typeCName,
						Text: []byte("saxon-pc"),
					},
				},
			},
		},
	}

	c := &Client{}
	p := c.formPayload(&report, &description)

	if !bytes.Equal(p, expect) {
		t.Fatalf("unexpected result.\nGot: %v\n Want: %v\n", p, expect)
	}

	bufAddr := fmt.Sprintf("%p", c.buf[:])
	pAddr := fmt.Sprintf("%p", p)
	if bufAddr != pAddr {
		t.Errorf("unexpected result.\nGot: %v\n want: %v\n", pAddr, bufAddr)
	}
}

// dummyLogger will allow logging to be done by the testing pkg.
type dummyLogger testing.T

func (dl *dummyLogger) log(lvl int8, msg string, args ...interface{}) {
	var l string
	switch lvl {
	case logging.Warning:
		l = "warning"
	case logging.Debug:
		l = "debug"
	case logging.Info:
		l = "info"
	case logging.Error:
		l = "error"
	case logging.Fatal:
		l = "fatal"
	}
	msg = l + ": " + msg
	for i := 0; i < len(args); i++ {
		msg += " %v"
	}
	if len(args) == 0 {
		dl.Log(msg + "\n")
		return
	}
	dl.Logf(msg+"%v\n", args)
}

// TestReceiveAndSend tests basic RTCP client behaviour with a basic RTCP server.
// The RTCP client will send through receiver reports, and the RTCP server will
// respond with sender reports.
func TestReceiveAndSend(t *testing.T) {
	const clientAddr, serverAddr = "localhost:8000", "localhost:8001"
	rtpClt, err := rtp.NewClient("localhost:8002")
	if err != nil {
		t.Fatalf("unexpected error when creating RTP client: %v", err)
	}

	c, err := NewClient(
		clientAddr,
		serverAddr,
		rtpClt,
		(*dummyLogger)(t).log,
	)
	if err != nil {
		t.Fatalf("unexpected error when creating client: %v\n", err)
	}

	go func() {
		for {
			err, ok := <-c.Err()
			if ok {
				const errConnClosed = "use of closed network connection"
				if !strings.Contains(err.Error(), errConnClosed) {
					t.Fatalf("error received from client error chan: %v\n", err)
				}
			} else {
				return
			}
		}
	}()

	c.Start()

	sAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		t.Fatalf("could not resolve test server address, failed with error: %v", err)
	}

	cAddr, err := net.ResolveUDPAddr("udp", clientAddr)
	if err != nil {
		t.Fatalf("could not resolve client address, failed with error: %v", err)
	}

	conn, err := net.DialUDP("udp", sAddr, cAddr)
	if err != nil {
		t.Fatalf("could not dial, failed with error: %v\n", err)
	}

	buf := make([]byte, 4096)
	for i := 0; i < 5; i++ {
		t.Log("SERVER: waiting for receiver report\n")
		n, _, _ := conn.ReadFromUDP(buf)
		t.Logf("SERVER: receiver report received: \n%v\n", buf[:n])

		now := time.Now().Second()
		var time [8]byte
		binary.BigEndian.PutUint64(time[:], uint64(now))
		msw := binary.BigEndian.Uint32(time[:4])
		lsw := binary.BigEndian.Uint32(time[4:])

		report := SenderReport{
			Header: Header{
				Version:     rtcpVer,
				Padding:     false,
				ReportCount: 0,
				Type:        typeSenderReport,
			},
			SSRC:         1234567,
			TimestampMSW: msw,
			TimestampLSW: lsw,
			RTPTimestamp: 0,
			PacketCount:  0,
			OctetCount:   0,
		}
		r := report.Bytes()
		t.Logf("SERVER: sending sender report: \n%v\n", r)
		_, err := conn.Write(r)
		if err != nil {
			t.Errorf("did not expect error: %v\n", err)
		}
	}
	c.Stop()
}

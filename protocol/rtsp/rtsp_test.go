/*
NAME
 0x r,tsp_test.go

DESCRIPTION
  rtsp_test.go provides a test to check functionality provided in rtsp.go.

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
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode"
)

// The max request size we should get in bytes.
const maxRequest = 1024

// TestMethods checks that we can correctly form requests for each of the RTSP
// methods supported in the rtsp pkg. This test also checks that communication
// over a TCP connection is performed correctly.
func TestMethods(t *testing.T) {
	const dummyURL = "rtsp://admin:admin@192.168.0.50:8554/CH001.sdp"
	url, err := url.Parse(dummyURL)
	if err != nil {
		t.Fatalf("could not parse dummy address, failed with err: %v", err)
	}

	// tests holds tests which consist of a function used to create and write a
	// request of a particular method, and also the expected request bytes
	// to be received on the server side. The bytes in these tests have been
	// obtained from a valid RTSP communication cltion..
	tests := []struct {
		method   func(c *Client) (*Response, error)
		expected []byte
	}{
		{
			method: func(c *Client) (*Response, error) {
				req, err := NewRequest("DESCRIBE", c.nextCSeq(), url, nil)
				if err != nil {
					return nil, err
				}
				req.Header.Add("Accept", "application/sdp")
				return c.Do(req)
			},
			expected: []byte{
				0x44, 0x45, 0x53, 0x43, 0x52, 0x49, 0x42, 0x45, 0x20, 0x72, 0x74, 0x73, 0x70, 0x3a, 0x2f, 0x2f, // |DESCRIBE rtsp://|
				0x61, 0x64, 0x6d, 0x69, 0x6e, 0x3a, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x40, 0x31, 0x39, 0x32, 0x2e, // |admin:admin@192.|
				0x31, 0x36, 0x38, 0x2e, 0x30, 0x2e, 0x35, 0x30, 0x3a, 0x38, 0x35, 0x35, 0x34, 0x2f, 0x43, 0x48, // |168.0.50:8554/CH|
				0x30, 0x30, 0x31, 0x2e, 0x73, 0x64, 0x70, 0x20, 0x52, 0x54, 0x53, 0x50, 0x2f, 0x31, 0x2e, 0x30, // |001.sdp RTSP/1.0|
				0x0d, 0x0a, 0x43, 0x53, 0x65, 0x71, 0x3a, 0x20, 0x32, 0x0d, 0x0a, 0x41, 0x63, 0x63, 0x65, 0x70, // |..CSeq: 2..Accep|
				0x74, 0x3a, 0x20, 0x61, 0x70, 0x70, 0x6c, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x73, // |t: application/s|
				0x64, 0x70, 0x0d, 0x0a, 0x0d, 0x0a, /*                                                       */ // |dp....|
			},
		},
		{
			method: func(c *Client) (*Response, error) {
				req, err := NewRequest("OPTIONS", c.nextCSeq(), url, nil)
				if err != nil {
					return nil, err
				}
				return c.Do(req)
			},
			expected: []byte{
				0x4f, 0x50, 0x54, 0x49, 0x4f, 0x4e, 0x53, 0x20, 0x72, 0x74, 0x73, 0x70, 0x3a, 0x2f, 0x2f, 0x61, // |OPTIONS rtsp://a|
				0x64, 0x6d, 0x69, 0x6e, 0x3a, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x40, 0x31, 0x39, 0x32, 0x2e, 0x31, // |dmin:admin@192.1|
				0x36, 0x38, 0x2e, 0x30, 0x2e, 0x35, 0x30, 0x3a, 0x38, 0x35, 0x35, 0x34, 0x2f, 0x43, 0x48, 0x30, // |68.0.50:8554/CH0|
				0x30, 0x31, 0x2e, 0x73, 0x64, 0x70, 0x20, 0x52, 0x54, 0x53, 0x50, 0x2f, 0x31, 0x2e, 0x30, 0x0d, // |01.sdp RTSP/1.0.|
				0x0a, 0x43, 0x53, 0x65, 0x71, 0x3a, 0x20, 0x31, 0x0d, 0x0a, 0x0d, 0x0a, /*                   */ // |.CSeq: 1....|
			},
		},
		{
			method: func(c *Client) (*Response, error) {
				u, err := url.Parse(dummyURL + "/track1")
				if err != nil {
					return nil, err
				}

				req, err := NewRequest("SETUP", c.nextCSeq(), u, nil)
				if err != nil {
					return nil, err
				}
				req.Header.Add("Transport", fmt.Sprintf("RTP/AVP;unicast;client_port=%d-%d", 6870, 6871))

				return c.Do(req)
			},
			expected: []byte{
				0x53, 0x45, 0x54, 0x55, 0x50, 0x20, 0x72, 0x74, 0x73, 0x70, 0x3a, 0x2f, 0x2f, 0x61, 0x64, 0x6d, // |SETUP rtsp://adm|
				0x69, 0x6e, 0x3a, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x40, 0x31, 0x39, 0x32, 0x2e, 0x31, 0x36, 0x38, // |in:admin@192.168|
				0x2e, 0x30, 0x2e, 0x35, 0x30, 0x3a, 0x38, 0x35, 0x35, 0x34, 0x2f, 0x43, 0x48, 0x30, 0x30, 0x31, // |.0.50:8554/CH001|
				0x2e, 0x73, 0x64, 0x70, 0x2f, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x31, 0x20, 0x52, 0x54, 0x53, 0x50, // |.sdp/track1 RTSP|
				0x2f, 0x31, 0x2e, 0x30, 0x0d, 0x0a, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x70, 0x6f, 0x72, 0x74, 0x3a, // |/1.0..Transport:|
				0x20, 0x52, 0x54, 0x50, 0x2f, 0x41, 0x56, 0x50, 0x3b, 0x75, 0x6e, 0x69, 0x63, 0x61, 0x73, 0x74, // | RTP/AVP;unicast|
				0x3b, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x5f, 0x70, 0x6f, 0x72, 0x74, 0x3d, 0x36, 0x38, 0x37, // |;client_port=687|
				0x30, 0x2d, 0x36, 0x38, 0x37, 0x31, 0x0d, 0x0a, 0x43, 0x53, 0x65, 0x71, 0x3a, 0x20, 0x33, 0x0d, // |0-6871..CSeq: 3.|
				0x0a, 0x0d, 0x0a, /*                                                                         */ // |...|
			},
		},
		{
			method: func(c *Client) (*Response, error) {
				req, err := NewRequest("PLAY", c.nextCSeq(), url, nil)
				if err != nil {
					return nil, err
				}
				req.Header.Add("Session", "00000021")

				return c.Do(req)
			},
			expected: []byte{
				0x50, 0x4c, 0x41, 0x59, 0x20, 0x72, 0x74, 0x73, 0x70, 0x3a, 0x2f, 0x2f, 0x61, 0x64, 0x6d, 0x69, // |PLAY rtsp://admi|
				0x6e, 0x3a, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x40, 0x31, 0x39, 0x32, 0x2e, 0x31, 0x36, 0x38, 0x2e, // |n:admin@192.168.|
				0x30, 0x2e, 0x35, 0x30, 0x3a, 0x38, 0x35, 0x35, 0x34, 0x2f, 0x43, 0x48, 0x30, 0x30, 0x31, 0x2e, // |0.50:8554/CH001.|
				0x73, 0x64, 0x70, 0x20, 0x52, 0x54, 0x53, 0x50, 0x2f, 0x31, 0x2e, 0x30, 0x0d, 0x0a, 0x43, 0x53, // |sdp RTSP/1.0..CS|
				0x65, 0x71, 0x3a, 0x20, 0x34, 0x0d, 0x0a, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x3a, 0x20, // |eq: 4..Session: |
				0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x32, 0x31, 0x0d, 0x0a, 0x0d, 0x0a, /*                   */ // |00000021....|
			},
		},
	}

	const serverAddr = "rtsp://localhost:8005"
	const retries = 10

	clientErr := make(chan error)
	serverErr := make(chan error)
	done := make(chan struct{})

	// This routine acts as the server.
	go func() {
		l, err := net.Listen("tcp", strings.TrimLeft(serverAddr, "rtsp://"))
		if err != nil {
			serverErr <- errors.New(fmt.Sprintf("server could not listen, error: %v", err))
		}

		conn, err := l.Accept()
		if err != nil {
			serverErr <- errors.New(fmt.Sprintf("server could not accept connection, error: %v", err))
		}

		buf := make([]byte, maxRequest)
		var n int
		for i, test := range tests {
		loop:
			for {
				n, err = conn.Read(buf)
				err, ok := err.(net.Error)

				switch {
				case err == nil:
					break loop
				case err == io.EOF:
				case ok && err.Timeout():
				default:
					serverErr <- errors.New(fmt.Sprintf("server could not read conn, error: %v", err))
					return
				}
			}

			// Write a dummy response, client won't care.
			conn.Write([]byte{'\n'})

			want := test.expected
			got := buf[:n]
			if !equal(got, want) {
				serverErr <- errors.New(fmt.Sprintf("unexpected result for test: %v. \nGot: %v\n Want: %v\n", i, got, want))
			}
		}
		close(done)
	}()

	// This routine acts as the client.
	go func() {
		var clt *Client
		var err error

		// Keep trying to connect to server.
		// TODO: use generalised retry utility when available.
		for retry := 0; ; retry++ {
			clt, _, _, err = NewClient(serverAddr)
			if err == nil {
				break
			}

			if retry > retries {
				clientErr <- errors.New(fmt.Sprintf("client could not connect to server, error: %v", err))
			}
			time.Sleep(10 * time.Millisecond)
		}

		for i, test := range tests {
			_, err = test.method(clt)
			if err != nil && err != io.EOF && err != errInvalidResponse {
				clientErr <- errors.New(fmt.Sprintf("error request for: %v err: %v", i, err))
			}
		}
	}()

	// We check for errors or a done signal from the server and client routines.
	for {
		select {
		case err := <-clientErr:
			t.Fatalf("client error: %v", err)
		case err := <-serverErr:
			t.Fatalf("server error: %v", err)
		case <-done:
			return
		default:
		}
	}
}

// equal checks that the got slice is considered equivalent to the want slice,
// neglecting unimportant differences such as order of items in header and the
// CSeq number.
func equal(got, want []byte) bool {
	const eol = "\r\n"
	gotParts := strings.Split(strings.TrimRight(string(got), eol), eol)
	wantParts := strings.Split(strings.TrimRight(string(want), eol), eol)
	gotParts, ok := rmSeqNum(gotParts)
	if !ok {
		return false
	}
	wantParts, ok = rmSeqNum(wantParts)
	if !ok {
		return false
	}
	for _, gotStr := range gotParts {
		for i, wantStr := range wantParts {
			if gotStr == wantStr {
				wantParts = append(wantParts[:i], wantParts[i+1:]...)
			}
		}
	}
	return len(wantParts) == 0
}

// rmSeqNum removes the CSeq number from a string in []string that contains it.
// If a CSeq field is not found nil and false is returned.
func rmSeqNum(s []string) ([]string, bool) {
	for i, _s := range s {
		if strings.Contains(_s, "CSeq") {
			s[i] = strings.TrimFunc(s[i], func(r rune) bool { return unicode.IsNumber(r) })
			return s, true
		}
	}
	return nil, false
}

// TestReadResponse checks that ReadResponse behaves as expected.
func TestReadResponse(t *testing.T) {
	// input has been obtained from a valid RTSP response.
	input := []byte{
		0x52, 0x54, 0x53, 0x50, 0x2f, 0x31, 0x2e, 0x30, 0x20, 0x32, 0x30, 0x30, 0x20, 0x4f, 0x4b, 0x0d, // |RTSP/1.0 200 OK.|
		0x0a, 0x43, 0x53, 0x65, 0x71, 0x3a, 0x20, 0x32, 0x0d, 0x0a, 0x44, 0x61, 0x74, 0x65, 0x3a, 0x20, // |.CSeq: 2..Date: |
		0x57, 0x65, 0x64, 0x2c, 0x20, 0x4a, 0x61, 0x6e, 0x20, 0x32, 0x31, 0x20, 0x31, 0x39, 0x37, 0x30, // |Wed, Jan 21 1970|
		0x20, 0x30, 0x32, 0x3a, 0x33, 0x37, 0x3a, 0x31, 0x34, 0x20, 0x47, 0x4d, 0x54, 0x0d, 0x0a, 0x50, // | 02:37:14 GMT..P|
		0x75, 0x62, 0x6c, 0x69, 0x63, 0x3a, 0x20, 0x4f, 0x50, 0x54, 0x49, 0x4f, 0x4e, 0x53, 0x2c, 0x20, // |ublic: OPTIONS, |
		0x44, 0x45, 0x53, 0x43, 0x52, 0x49, 0x42, 0x45, 0x2c, 0x20, 0x53, 0x45, 0x54, 0x55, 0x50, 0x2c, // |DESCRIBE, SETUP,|
		0x20, 0x54, 0x45, 0x41, 0x52, 0x44, 0x4f, 0x57, 0x4e, 0x2c, 0x20, 0x50, 0x4c, 0x41, 0x59, 0x2c, // | TEARDOWN, PLAY,|
		0x20, 0x47, 0x45, 0x54, 0x5f, 0x50, 0x41, 0x52, 0x41, 0x4d, 0x45, 0x54, 0x45, 0x52, 0x2c, 0x20, // | GET_PARAMETER, |
		0x53, 0x45, 0x54, 0x5f, 0x50, 0x41, 0x52, 0x41, 0x4d, 0x45, 0x54, 0x45, 0x52, 0x0d, 0x0a, 0x0d, // |SET_PARAMETER...|
		0x0a,
	}

	expect := Response{
		Proto:         "RTSP",
		ProtoMajor:    1,
		ProtoMinor:    0,
		StatusCode:    200,
		ContentLength: 0,
		Header: map[string][]string{
			"Cseq":   []string{"2"},
			"Date":   []string{"Wed, Jan 21 1970 02:37:14 GMT"},
			"Public": []string{"OPTIONS, DESCRIBE, SETUP, TEARDOWN, PLAY, GET_PARAMETER, SET_PARAMETER"},
		},
	}

	got, err := ReadResponse(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("should not have got error: %v", err)
	}

	if !respEqual(*got, expect) {
		t.Errorf("did not get expected result.\nGot: %+v\n Want: %+v\n", got, expect)
	}
}

// respEqual checks the equality of two Responses.
func respEqual(got, want Response) bool {
	for _, f := range [][2]interface{}{
		{got.Proto, want.Proto},
		{got.ProtoMajor, want.ProtoMajor},
		{got.ProtoMinor, want.ProtoMinor},
		{got.StatusCode, want.StatusCode},
		{got.ContentLength, want.ContentLength},
	} {
		if f[0] != f[1] {
			return false
		}
	}

	if len(got.Header) != len(want.Header) {
		return false
	}

	for k, v := range got.Header {
		if len(v) != len(want.Header[k]) {
			return false
		}

		for i, _v := range v {
			if _v != want.Header[k][i] {
				return false
			}
		}
	}
	return true
}

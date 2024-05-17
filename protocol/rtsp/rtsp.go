/*
NAME
  rtsp.go

DESCRIPTION
  rtsp.go provides functionality for forming and sending RTSP requests for
  methods, DESCRIBE, OPTIONS, SETUP and PLAY, as described by
  the RTSP standards, see https://tools.ietf.org/html/rfc7826

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package rtsp provides an RTSP client implementation and methods for
// communication with an RTSP server to request video.
package rtsp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Minimum response size to be considered valid in bytes.
const minResponse = 12

var errInvalidResponse = errors.New("invalid response")

// Request describes an RTSP request.
type Request struct {
	Method        string
	URL           *url.URL
	Proto         string
	ProtoMajor    int
	ProtoMinor    int
	Header        http.Header
	ContentLength int
	Body          io.ReadCloser
}

// NewRequest returns a pointer to a new Request.
func NewRequest(method, cSeq string, u *url.URL, body io.ReadCloser) (*Request, error) {
	req := &Request{
		Method:     method,
		URL:        u,
		Proto:      "RTSP",
		ProtoMajor: 1,
		ProtoMinor: 0,
		Header:     map[string][]string{"CSeq": []string{cSeq}},
		Body:       body,
	}
	return req, nil
}

// Write writes the request r to the given io.Writer w.
func (r *Request) Write(w io.Writer) error {
	_, err := w.Write([]byte(r.String()))
	return err
}

// String returns a formatted string of the Request.
func (r Request) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s/%d.%d\r\n", r.Method, r.URL.String(), r.Proto, r.ProtoMajor, r.ProtoMinor)
	for k, v := range r.Header {
		for _, v := range v {
			fmt.Fprintf(&b, "%s: %s\r\n", k, v)
		}
	}
	b.WriteString("\r\n")
	if r.Body != nil {
		s, _ := ioutil.ReadAll(r.Body)
		b.WriteString(string(s))
	}
	return b.String()
}

// Response describes an RTSP response.
type Response struct {
	Proto         string
	ProtoMajor    int
	ProtoMinor    int
	StatusCode    int
	ContentLength int
	Header        http.Header
	Body          io.ReadCloser
}

// String returns a formatted string of the Response.
func (r Response) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s/%d.%d %d\n", r.Proto, r.ProtoMajor, r.ProtoMinor, r.StatusCode)
	for k, v := range r.Header {
		for _, v := range v {
			fmt.Fprintf(&b, "%s: %s", k, v)
		}
	}
	return b.String()
}

// ReadResponse will read the response of the RTSP request from the connection,
// and return a pointer to a new Response.
func ReadResponse(r io.Reader) (*Response, error) {
	resp := &Response{Header: make(map[string][]string)}

	scanner := bufio.NewScanner(r)

	// Read the first line.
	scanner.Scan()
	err := scanner.Err()
	if err != nil {
		return nil, err
	}
	s := scanner.Text()

	if len(s) < minResponse || !strings.HasPrefix(s, "RTSP/") {
		return nil, errInvalidResponse
	}
	resp.Proto = "RTSP"

	n, err := fmt.Sscanf(s[5:], "%d.%d %d", &resp.ProtoMajor, &resp.ProtoMinor, &resp.StatusCode)
	if err != nil || n != 3 {
		return nil, fmt.Errorf("could not Sscanf response, error: %w", err)
	}

	// Read headers.
	for scanner.Scan() {
		err = scanner.Err()
		if err != nil {
			return nil, err
		}
		parts := strings.SplitN(scanner.Text(), ":", 2)
		if len(parts) < 2 {
			break
		}
		resp.Header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}
	// Get the content length from the header.
	resp.ContentLength, _ = strconv.Atoi(resp.Header.Get("Content-Length"))

	resp.Body = closer{r}
	return resp, nil
}

type closer struct {
	io.Reader
}

func (c closer) Close() error {
	if c.Reader == nil {
		return nil
	}
	defer func() {
		c.Reader = nil
	}()
	if r, ok := c.Reader.(io.ReadCloser); ok {
		return r.Close()
	}
	return nil
}

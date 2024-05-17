/*
NAME
  parseurl.go

DESCRIPTION
  See Readme.md

AUTHOR
  Dan Kortschak <dan@ausocean.org>
  Saxon Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package rtmp

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Errors.
var (
	errInvalidPath     = errors.New("invalid url path")
	errInvalidElements = errors.New("invalid url elements")
)

// parseURL parses an RTMP URL (ok, technically it is lexing).
func parseURL(addr string) (protocol int32, host string, port uint16, app, playpath string, err error) {
	u, err := url.Parse(addr)
	if err != nil {
		return protocol, host, port, app, playpath, fmt.Errorf("could not parse to url value: %w", err)
	}

	switch u.Scheme {
	case "rtmp":
		protocol = protoRTMP
	case "rtmpt":
		protocol = protoRTMPT
	case "rtmps":
		protocol = protoRTMPS
	case "rtmpe":
		protocol = protoRTMPE
	case "rtmfp":
		protocol = protoRTMFP
	case "rtmpte":
		protocol = protoRTMPTE
	case "rtmpts":
		protocol = protoRTMPTS
	default:
		return protocol, host, port, app, playpath, fmt.Errorf("unknown scheme: %s", u.Scheme)
	}

	host = u.Host
	if p := u.Port(); p != "" {
		pi, err := strconv.Atoi(p)
		if err != nil {
			return protocol, host, port, app, playpath, fmt.Errorf("could convert port to integer: %w", err)
		}
		port = uint16(pi)
	}

	if len(u.Path) < 1 || !path.IsAbs(u.Path) {
		return protocol, host, port, app, playpath, errInvalidPath
	}
	elems := strings.SplitN(u.Path[1:], "/", 3)
	if len(elems) < 2 || elems[0] == "" || elems[1] == "" {
		return protocol, host, port, app, playpath, errInvalidElements
	}
	app = elems[0]
	playpath = path.Join(elems[1:]...)

	switch ext := path.Ext(playpath); ext {
	case ".f4v", ".mp4":
		playpath = playpath[:len(playpath)-len(ext)]
		if !strings.HasPrefix(playpath, "mp4:") {
			playpath = "mp4:" + playpath
		}
	case ".mp3":
		playpath = playpath[:len(playpath)-len(ext)]
		if !strings.HasPrefix(playpath, "mp3:") {
			playpath = "mp3:" + playpath
		}
	case ".flv":
		playpath = playpath[:len(playpath)-len(ext)]
	}
	if u.RawQuery != "" {
		playpath += "?" + u.RawQuery
	}

	switch {
	case port != 0:
	case (protocol & featureSSL) != 0:
		return protocol, host, port, app, playpath, errors.New("ssl not implemented")
	case (protocol & featureHTTP) != 0:
		port = 80
	default:
		port = 1935
	}

	return protocol, host, port, app, playpath, nil
}

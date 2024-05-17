/*
NAME
  parseurl_test.go

DESCRIPTION
  See Readme.md

AUTHOR
  Dan Kortschak <dan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package rtmp

import (
	"testing"
)

var parseURLTests = []struct {
	url          string
	wantProtocol int32
	wantHost     string
	wantPort     uint16
	wantApp      string
	wantPlaypath string
	wantErr      error
}{
	{
		url:     "rtmp://addr",
		wantErr: errInvalidPath,
	},
	{
		url:     "rtmp://addr/",
		wantErr: errInvalidElements,
	},
	{
		url:     "rtmp://addr/live2",
		wantErr: errInvalidElements,
	},
	{
		url:     "rtmp://addr/live2/",
		wantErr: errInvalidElements,
	},
	{
		url:          "rtmp://addr/appname/key",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "key",
	},
	{
		url:          "rtmp://addr/appname/instancename",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "instancename",
	},
	{
		url:          "rtmp://addr/appname/instancename/",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "instancename",
	},
	{
		url:          "rtmp://addr/appname/mp4:path.f4v",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "mp4:path",
	},
	{
		url:          "rtmp://addr/appname/mp4:path.f4v?param1=value1&param2=value2",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "mp4:path?param1=value1&param2=value2",
	},
	{
		url:          "rtmp://addr/appname/mp4:path/to/file.f4v",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "mp4:path/to/file",
	},
	{
		url:          "rtmp://addr/appname/mp4:path/to/file.f4v?param1=value1&param2=value2",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "mp4:path/to/file?param1=value1&param2=value2",
	},
	{
		url:          "rtmp://addr/appname/path.mp4",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "mp4:path",
	},
	{
		url:          "rtmp://addr/appname/path.mp4?param1=value1&param2=value2",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "mp4:path?param1=value1&param2=value2",
	},
	{
		url:          "rtmp://addr/appname/path/to/file.mp4",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "mp4:path/to/file",
	},
	{
		url:          "rtmp://addr/appname/path/to/file.mp4?param1=value1&param2=value2",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "mp4:path/to/file?param1=value1&param2=value2",
	},
	{
		url:          "rtmp://addr/appname/path.flv",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "path",
	},
	{
		url:          "rtmp://addr/appname/path.flv?param1=value1&param2=value2",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "path?param1=value1&param2=value2",
	},
	{
		url:          "rtmp://addr/appname/path/to/file.flv",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "path/to/file",
	},
	{
		url:          "rtmp://addr/appname/path/to/file.flv?param1=value1&param2=value2",
		wantHost:     "addr",
		wantPort:     1935,
		wantApp:      "appname",
		wantPlaypath: "path/to/file?param1=value1&param2=value2",
	},
}

func TestParseURL(t *testing.T) {
	for _, test := range parseURLTests {
		func() {
			defer func() {
				p := recover()
				if p != nil {
					t.Errorf("unexpected panic for %q: %v", test.url, p)
				}
			}()

			protocol, host, port, app, playpath, err := parseURL(test.url)
			if err != test.wantErr {
				t.Errorf("unexpected error for %q: got:%v want:%v", test.url, err, test.wantErr)
				return
			}
			if err != nil {
				return
			}
			if protocol != test.wantProtocol {
				t.Errorf("unexpected protocol for %q: got:%v want:%v", test.url, protocol, test.wantProtocol)
			}
			if host != test.wantHost {
				t.Errorf("unexpected host for %q: got:%v want:%v", test.url, host, test.wantHost)
			}
			if port != test.wantPort {
				t.Errorf("unexpected port for %q: got:%v want:%v", test.url, port, test.wantPort)
			}
			if app != test.wantApp {
				t.Errorf("unexpected app for %q: got:%v want:%v", test.url, app, test.wantApp)
			}
			if playpath != test.wantPlaypath {
				t.Errorf("unexpected playpath for %q: got:%v want:%v", test.url, playpath, test.wantPlaypath)
			}
		}()
	}
}

/*
DESCRIPTION
  config.go provides exported functionality of a basic API to allow programmatic
  control over the GeoVision camera (namely the GV-BX4700) through the HTTP
  server used for settings control. See package documentation for further
  information on the API.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package config provides a basic API for programmatic control of the
// web based interface provided by GeoVision cameras. This API has been
// developed and tested only with the GV-BX4700, and therefore may not work
// with other models without further evolution.
//
// Settings on a GeoVision camera are updated using the Set function. One or
// more option functions may be provided to control camera function.
package config

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"
)

// Option describes a function that will apply an option to the passed s.
type Option func(s settings) (settings, error)

// Set will log-in to the camera at host and submit a form of settings. The
// settings form is populated with values influenced by the optional options
// passed. Available options are defined below this function.
//
// The following defaults are applied to each configurable parameter if not
// influenced by the passed options:
// codec: H264
// resolution: 640x360
// framerate: 25
// variable bitrate: off
// variable bitrate quality: good
// vbr bitrate: 250 kbps
// cbr bitrate: 512 kbps
// refresh: 2 seconds
// channel: 2
func Set(host string, options ...Option) error {
	// Randomly generate an ID our client will use.
	const (
		minID = 10000
		maxID = 99999
	)
	rand.Seed(time.Now().UTC().UnixNano())
	id := strconv.Itoa(maxID + rand.Intn(maxID-minID))

	// Create a client with a cookie jar.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("could not create cookie jar, failed with error: %w", err)
	}

	client := &http.Client{
		Timeout: time.Duration(5 * time.Second),
		Jar:     jar,
	}

	// Get the request body required for log-in.
	body, err := getLogin(client, id, host)
	if err != nil {
		return fmt.Errorf("could not generate log-in request data: %w", err)
	}

	// Log in using generated log-in request body.
	err = login(client, id, host, body)
	if err != nil {
		return fmt.Errorf("could not login: %w", err)
	}

	// Apply the options to the settings specified by the user.
	s := newSettings()
	for _, op := range options {
		s, err = op(s)
		if err != nil {
			return fmt.Errorf("could not action Option: %w", err)
		}
	}

	// Submit the settings to the server.
	err = submitSettings(client, id, host, s)
	if err != nil {
		return fmt.Errorf("could not submit settings: %w", err)
	}
	return nil
}

// Channel will set the GeoVision channel we will be using.
func Channel(c uint8) Option {
	return func(s settings) (settings, error) {
		if c != 1 && c != 2 {
			return s, errors.New("invalid channel")
		}
		s.ch = c
		return s, nil
	}
}

// Codec is a video codec.
type Codec string

// The avilable codecs that may be selected using CodecOut below.
const (
	CodecH265  Codec = "28"
	CodecH264  Codec = "10"
	CodecMJPEG Codec = "4"
)

// CodecOut will set the video codec outputted by the camera. The available
// codec options are listed above as consts.
func CodecOut(c Codec) Option {
	return func(s settings) (settings, error) {
		if (c != CodecH265 && c != CodecH264) && (s.ch == 1 || (s.ch == 2 && c != CodecMJPEG)) {
			return s, fmt.Errorf("unknown Codec: %v", c)
		}
		s.codec = c
		return s, nil
	}
}

// Height will set the height component of the video resolution. Available
// heights are 256, 360 and 720.
func Height(h uint) Option {
	return func(s settings) (settings, error) {
		var m map[uint]string
		switch s.ch {
		case 1:
			// TODO: add other resolutions supported by channel 1.
			m = map[uint]string{1080: res1080}
		case 2:
			m = map[uint]string{256: res256, 360: res360, 720: res720}
		}
		v, ok := m[h]
		if !ok {
			return s, fmt.Errorf("invalid display height: %d", h)
		}
		s.res = v
		return s, nil
	}
}

// FrameRate will set the frame rate of the video. This value is defined in
// units of frames per second, and must be between 1 and 30 inclusive.
func FrameRate(f uint) Option {
	return func(s settings) (settings, error) {
		if 1 > f || f > 30 {
			return s, fmt.Errorf("invalid frame rate: %d", f)
		}
		s.frameRate = strconv.Itoa(int(f * 1000))
		return s, nil
	}
}

// VariableBitrate with b set true will turn on variable bitrate video and
// with b set false will turn off variable bitrate (resulting in constant bitrate).
func VariableBitrate(b bool) Option {
	return func(s settings) (settings, error) {
		s.vbr = "0"
		if b {
			s.vbr = "1"
		}
		return s, nil
	}
}

// Quality defines an average quality of video from the camera.
type Quality string

// The available video qualities under variable bitrate. NB: it is not known
// what bitrates these correspond to.
const (
	QualityStandard  Quality = "4"
	QualityFair      Quality = "3"
	QualityGood      Quality = "2"
	QualityGreat     Quality = "1"
	QualityExcellent Quality = "0"
)

// VBRQuality will set the average quality of video under variable bitrate.
// The quality may be chosen from standard to excellent, as defined above.
func VBRQuality(q Quality) Option {
	return func(s settings) (settings, error) {
		switch q {
		case QualityStandard, QualityFair, QualityGood, QualityGreat, QualityExcellent:
			s.quality = q
			return s, nil
		default:
			return s, fmt.Errorf("invalid Quality: %v", q)
		}
	}
}

// VBRBitrate will set the maximal bitrate when the camera is set to variable
// bitrate. The possible values of maximal bitrate in kbps are predefined (by
// the camera) as: 250, 500, 750, 1000, 1250, 1500, 1750, 2000, 2250 and 2500.
// If the passed rate does not match one of these values, the closest value is
// selected.
func VBRBitrate(r uint) Option {
	return func(s settings) (settings, error) {
		var vbrRates = []uint{250, 500, 750, 1000, 1250, 1500, 1750, 2000, 2250, 2500}
		if s.vbr == "1" {
			s.vbrBitrate = convRate(r, vbrRates)
			return s, nil
		}
		return s, nil
	}
}

// CBRBitrate will select the bitrate when the camera is set to constant bitrate.
// The possible values of bitrate are predefined for each resolution as follows:
// 256p: 128, 256, 512, 1024
// 360p: 512, 1024, 2048, 3072
// 720p: 1024, 2048, 4096, 6144
// If the passed rate does not align with one of these values, the closest
// value is selected.
func CBRBitrate(r uint) Option {
	return func(s settings) (settings, error) {
		v, ok := map[string]string{
			res1080: convRate(r, []uint{2048, 4096, 6144, 8192}),
			res720:  convRate(r, []uint{1024, 2048, 4096, 6144}),
			res360:  convRate(r, []uint{512, 1024, 2048, 3072}),
			res256:  convRate(r, []uint{128, 256, 512, 1024}),
		}[s.res]
		if !ok {
			panic("bad resolution")
		}
		s.cbrBitrate = v
		return s, nil
	}
}

// Refresh will set the intra refresh period. The passed value is in seconds and
// must be between .25 and 5 inclusive. The value will be rounded to the nearest
// value divisible by .25 seconds.
func Refresh(r float64) Option {
	return func(s settings) (settings, error) {
		const (
			maxRefreshPeriod = 5
			minRefreshPeriod = .25
		)

		if minRefreshPeriod > r || r > maxRefreshPeriod {
			return s, fmt.Errorf("invalid refresh period: %g", r)
		}

		refOptions := []uint{250, 500, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000}
		s.refresh = strconv.Itoa(int(refOptions[closestValIdx(uint(r*1000), refOptions)]))
		return s, nil
	}
}

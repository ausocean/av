/*
DESCRIPTION
  utils.go provides general constants, structs and helper functions for use in
  this package.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package config

import (
	"crypto/md5"
	"encoding/hex"
	"math"
	"net/url"
	"strconv"
	"strings"
)

// The strings used in encoding the settings form to indicate resultion.
const (
	res256  = "4480256"  // 480x256
	res360  = "6400360"  // 640x360
	res720  = "12800720" // 1280x720
	res1080 = "19201080" // 1920x1080
)

// Default values for fields in the settings struct when the newSettings
// constructor is used.
const (
	defaultCodec      = CodecH264
	defaultRes        = "6400360" // 360p
	defaultFrameRate  = "25000"   // 25 fps
	defaultVBR        = "0"       // Variable bitrate off
	defaultQuality    = QualityGood
	defaultVBRBitrate = "250000" // 512 kbps (lowest with 360p)
	defaultCBRBitrate = "512000"
	defaultRefresh    = "2000" // 2 seconds
	defaultChan       = 2
)

// settings holds string representations required by the settings form for each
// of the parameters configurable through this API.
type settings struct {
	codec      Codec
	res        string
	frameRate  string
	vbr        string
	quality    Quality
	vbrBitrate string
	cbrBitrate string
	refresh    string
	ch         uint8
}

// newSetting will return a settings with default values.
func newSettings() settings {
	return settings{
		codec:      defaultCodec,
		res:        defaultRes,
		frameRate:  defaultFrameRate,
		vbr:        defaultVBR,
		quality:    defaultQuality,
		vbrBitrate: defaultVBRBitrate,
		cbrBitrate: defaultCBRBitrate,
		refresh:    defaultRefresh,
		ch:         defaultChan,
	}
}

// md5Hex returns the md5 hex string of s with alphanumerics as upper case.
func md5Hex(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

// closestValIdx will return the index of the value in l that is closest to the
// value v.
func closestValIdx(v uint, l []uint) uint {
	var idx int
	for i := range l {
		if math.Abs(float64(int(l[i])-int(v))) < math.Abs(float64(int(l[idx])-int(v))) {
			idx = i
		}
	}
	return uint(idx)
}

// convRate is used to firstly find a value in l closest to the bitrate v (in
// kbps), convert from kbps to bps, and the convert to string.
func convRate(v uint, l []uint) string {
	return strconv.Itoa(int(l[closestValIdx(v, l)] * 1000))
}

// populateForm will populate the settings form using the passed settings struct
// s and return as a url.Values.
func populateForm(s settings) url.Values {
	f := url.Values{}
	f.Set("dwConnType", "5")
	f.Set("mpeg_type", string(s.codec))
	f.Set("dwflicker_hz", "0")
	f.Set("szResolution", s.res)
	f.Set("dwFrameRate", s.frameRate)
	f.Set("custom_qp_init", "25")

	if s.ch == 1 {
		f.Set("dwflicker_less", "1")
		f.Set("bSliceMode", "4")
		f.Set("dwCameraId", "0")
		f.Set("szCamName", "Camera")
		f.Set("bAudioCodec", "7")
		f.Set("bTVoutFormat", "2")
		f.Set("bReadyLed", "0")
		f.Set("bLedLan", "0")
		f.Set("bLedWan", "0")
		f.Set("bLedMonitor", "0")
		f.Set("bAlarmLedAutoLevel", "5")
		f.Set("bAlarmLedAutoDuration", "60")
		f.Set("bAlarmLed", "1")
		f.Set("face_detect_level", "1")
		f.Set("bDayNight", "0")
		f.Set("bDayNightAutoLevel", "3")
		f.Set("bIRout", "0")
		f.Set("bAutoIris", "0")
		f.Set("IrisType", "1")
		f.Set("bBLC", "0")
		f.Set("bIR", "1")
		f.Set("bNSR", "0")
		f.Set("ReplaceHomePreset1", "0")
		f.Set("webpageEncoding", "windows-1252")
	} else if s.ch == 2 {
		f.Set("dwCameraId", "1")
	} else {
		panic("invalid channel")
	}

	if s.codec == CodecMJPEG {
		f.Set("vbr_enable", "1")
		f.Set("dwVbrQuality", string(s.quality))

		switch s.res {
		case res256:
			f.Set("vbrmaxbitrate", "250000")
		case res360:
			f.Set("vbrmaxbitrate", "500000")
		case res720:
			f.Set("vbrmaxbitrate", "750000")
		default:
			panic("invalid resolution")
		}
	} else {
		switch s.vbr {
		case "0":
			f.Set("vbr_enable", "0")
			f.Set("max_bit_rate", s.cbrBitrate)
		case "1":
			f.Set("vbr_enable", "1")
			f.Set("dwVbrQuality", string(s.quality))
			f.Set("vbrmaxbitrate", s.vbrBitrate)
		default:
			panic("invalid vbrEnable parameter")
		}

		f.Set("custom_rate_control_type", "0")
		f.Set("custom_bitrate", "0")
		f.Set("custom_qp_min", "10")
		f.Set("custom_qp_max", "40")
	}

	f.Set("gop_N", s.refresh)
	if s.codec == CodecMJPEG {
		f.Set("gop_N", "1500")
	}

	if s.codec == CodecH264 || s.codec == CodecH265 {
		if s.ch == 1 {
			f.Set("dwEncProfile", "3")
		} else {
			f.Set("dwEncProfile", "1")
		}

		f.Set("dwEncLevel", "31")
		f.Set("dwEntropy", "0")
	}

	f.Set("u8PreAlarmBuf", "1")
	f.Set("u32PostAlarmBuf2Disk", "1")
	f.Set("u8SplitInterval", "5")
	f.Set("bEbIoIn", "1")
	f.Set("bEbIoIn1", "1")
	f.Set("szOsdCamName", "Camera")
	f.Set("bOSDFontSize", "0")
	f.Set("bCamNamePos", "2")
	f.Set("bDatePos", "2")
	f.Set("bTimePos", "2")
	f.Set("u16PostAlarmBuf", "1")
	f.Set("LangCode", "undefined")
	f.Set("Recflag", "0")
	f.Set("submit", "Apply")

	return f
}

/*
DESCRIPTION
  config_test.go provides tests of functionality in the config package.

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
	"errors"
	"net/url"
	"reflect"
	"testing"
)

func TestClosestValIdx(t *testing.T) {
	tests := []struct {
		l    []uint
		v    uint
		want uint
	}{
		{
			l:    []uint{2, 5, 8, 11, 14},
			v:    6,
			want: 1,
		},
		{
			l:    []uint{2, 5, 8, 11, 14},
			v:    12,
			want: 3,
		},
		{
			l:    []uint{2, 5, 8, 11, 14},
			v:    13,
			want: 4,
		},
		{
			l:    []uint{2, 5, 8, 11, 14},
			v:    0,
			want: 0,
		},
		{
			l:    []uint{2, 5, 8, 11, 14},
			v:    17,
			want: 4,
		},
		{
			l:    []uint{2, 5, 8, 11, 15},
			v:    13,
			want: 3,
		},
		{
			l:    []uint{},
			v:    17,
			want: 0,
		},
	}

	for i, test := range tests {
		got := closestValIdx(test.v, test.l)
		if got != test.want {
			t.Errorf("did not get expected result for test: %d\nGot: %v\nWant: %v", i, got, test.want)
		}
	}
}

func TestConvRate(t *testing.T) {
	tests := []struct {
		l    []uint
		v    uint
		want string
	}{
		{
			l:    []uint{512, 1024, 2048, 3072},
			v:    1400,
			want: "1024000",
		},
		{
			l:    []uint{512, 1024, 2048, 3072},
			v:    1900,
			want: "2048000",
		},
		{
			l:    []uint{512, 1024, 2048, 3072},
			v:    4000,
			want: "3072000",
		},
	}

	for i, test := range tests {
		got := convRate(test.v, test.l)
		if got != test.want {
			t.Errorf("did not get expected result for test: %d\nGot: %v\nWant: %v", i, got, test.want)
		}
	}
}

func TestCodecOut(t *testing.T) {
	tests := []struct {
		s    settings
		c    Codec
		want settings
		err  bool
	}{
		{
			s:    settings{ch: 1},
			c:    CodecH264,
			want: settings{ch: 1, codec: CodecH264},
		},
		{
			s:    settings{ch: 1},
			c:    CodecH265,
			want: settings{ch: 1, codec: CodecH265},
		},
		{
			s:    settings{ch: 1},
			c:    CodecMJPEG,
			want: settings{ch: 1},
			err:  true,
		},
		{
			s:    settings{ch: 2},
			c:    CodecH264,
			want: settings{ch: 2, codec: CodecH264},
		},
		{
			s:    settings{ch: 2},
			c:    CodecH265,
			want: settings{ch: 2, codec: CodecH265},
		},
		{
			s:    settings{ch: 2},
			c:    CodecMJPEG,
			want: settings{ch: 2, codec: CodecMJPEG},
		},
		{
			s:    settings{ch: 1},
			c:    Codec("500"),
			want: settings{ch: 1},
			err:  true,
		},
		{
			s:    settings{ch: 2},
			c:    Codec("500"),
			want: settings{ch: 2},
			err:  true,
		},
	}

	for i, test := range tests {
		got, err := CodecOut(test.c)(test.s)
		if err != nil && test.err != true {
			t.Errorf("did not expect error: %v for test: %d", err, i)
		}

		if got != test.want {
			t.Errorf("did not get expected result for test: %d\nGot: %v\nWant: %v\n", i, got, test.want)
		}
	}
}

func TestHeight(t *testing.T) {
	tests := []struct {
		s    settings
		h    uint
		want settings
		err  error
	}{
		{
			s:    settings{ch: 2},
			h:    256,
			want: settings{ch: 2, res: "4480256"},
		},
		{
			s:    settings{ch: 2},
			h:    360,
			want: settings{ch: 2, res: "6400360"},
		},
		{
			s:    settings{ch: 2},
			h:    720,
			want: settings{ch: 2, res: "12800720"},
		},
		{
			s:    settings{ch: 2},
			h:    500,
			want: settings{ch: 2},
			err:  errors.New(""),
		},
		{
			s:    settings{ch: 1},
			h:    1080,
			want: settings{ch: 1, res: "19201080"},
		},
		{
			s:    settings{ch: 1},
			h:    1000,
			want: settings{ch: 1},
			err:  errors.New(""),
		},
	}

	for i, test := range tests {
		got, err := Height(test.h)(test.s)
		if test.err == nil && err != nil || test.err != nil && err == nil {
			t.Errorf("did not get expected error: %v for test: %d", err, i)
		}

		if got != test.want {
			t.Errorf("did not get expected result for test: %d\nGot: %v\nWant: %v", i, got, test.want)
		}
	}
}

func TestVBRBitrate(t *testing.T) {
	tests := []struct {
		r    uint
		in   settings
		want settings
	}{
		{
			r:  300,
			in: settings{vbr: "1"},
			want: settings{
				vbr:        "1",
				vbrBitrate: "250000",
			},
		},
		{
			r:  400,
			in: settings{vbr: "1"},
			want: settings{
				vbr:        "1",
				vbrBitrate: "500000",
			},
		},
	}

	for i, test := range tests {
		got, _ := VBRBitrate(test.r)(test.in)
		if got != test.want {
			t.Errorf("did not get expected result for test: %d\nGot: %v\nWant: %v", i, got, test.want)
		}
	}
}

func TestCBRBitrate(t *testing.T) {
	tests := []struct {
		r    uint
		in   settings
		want settings
	}{
		{
			r: 600,
			in: settings{
				vbr: "0",
				res: res256,
			},
			want: settings{
				vbr:        "0",
				res:        res256,
				cbrBitrate: "512000",
			},
		},
		{
			r: 100,
			in: settings{
				vbr: "0",
				res: res256,
			},
			want: settings{
				vbr:        "0",
				res:        res256,
				cbrBitrate: "128000",
			},
		},
		{
			r: 2048,
			in: settings{
				vbr: "0",
				res: res360,
			},
			want: settings{
				vbr:        "0",
				res:        res360,
				cbrBitrate: "2048000",
			},
		},
		{
			r: 500,
			in: settings{
				vbr: "0",
				res: res720,
			},
			want: settings{
				vbr:        "0",
				res:        res720,
				cbrBitrate: "1024000",
			},
		},
	}

	for i, test := range tests {
		got, _ := CBRBitrate(test.r)(test.in)
		if got != test.want {
			t.Errorf("did not get expected result for test: %d\nGot: %v\nWant: %v", i, got, test.want)
		}
	}
}

func TestRefresh(t *testing.T) {
	tests := []struct {
		r    float64
		want settings
		err  error
	}{
		{
			r:    1.6,
			want: settings{refresh: "1500"},
		},
		{
			r:    2.4,
			want: settings{refresh: "2500"},
		},
		{
			r:    0,
			want: settings{},
			err:  errors.New(""),
		},
		{
			r:    6,
			want: settings{},
			err:  errors.New(""),
		},
	}

	for i, test := range tests {
		s := settings{}
		got, err := Refresh(test.r)(s)
		if test.err == nil && err != nil || test.err != nil && err == nil {
			t.Errorf("did not get expected error: %v", test.err)
		}

		if got != test.want {
			t.Errorf("did not get expected result for test: %d\nGot: %v\nWant: %v", i, got, test.want)
		}
	}
}

func TestPopulateForm(t *testing.T) {
	tests := []struct {
		in   settings
		want string
	}{
		{
			in:   newSettings(),
			want: "dwConnType=5&mpeg_type=10&dwflicker_hz=0&szResolution=6400360&dwFrameRate=25000&vbr_enable=0&max_bit_rate=512000&custom_rate_control_type=0&custom_bitrate=0&custom_qp_init=25&custom_qp_min=10&custom_qp_max=40&gop_N=2000&dwEncProfile=1&dwEncLevel=31&dwEntropy=0&u8PreAlarmBuf=1&u32PostAlarmBuf2Disk=1&u8SplitInterval=5&bEbIoIn=1&bEbIoIn1=1&bOSDFontSize=0&bCamNamePos=2&bDatePos=2&bTimePos=2&szOsdCamName=Camera&u16PostAlarmBuf=1&dwCameraId=1&LangCode=undefined&Recflag=0&submit=Apply",
		},
		{
			in: settings{
				codec:      CodecH265,
				res:        defaultRes,
				frameRate:  defaultFrameRate,
				vbr:        defaultVBR,
				quality:    defaultQuality,
				vbrBitrate: defaultVBRBitrate,
				cbrBitrate: defaultCBRBitrate,
				refresh:    defaultRefresh,
				ch:         2,
			},
			want: "dwConnType=5&mpeg_type=28&dwflicker_hz=0&szResolution=6400360&dwFrameRate=25000&vbr_enable=0&max_bit_rate=512000&custom_rate_control_type=0&custom_bitrate=0&custom_qp_init=25&custom_qp_min=10&custom_qp_max=40&gop_N=2000&dwEncProfile=1&dwEncLevel=31&dwEntropy=0&u8PreAlarmBuf=1&u32PostAlarmBuf2Disk=1&u8SplitInterval=5&bEbIoIn=1&bEbIoIn1=1&bOSDFontSize=0&bCamNamePos=2&bDatePos=2&bTimePos=2&szOsdCamName=Camera&u16PostAlarmBuf=1&dwCameraId=1&LangCode=undefined&Recflag=0&submit=Apply",
		},
		{
			in: settings{
				codec:      CodecMJPEG,
				res:        defaultRes,
				frameRate:  defaultFrameRate,
				vbr:        defaultVBR,
				quality:    defaultQuality,
				vbrBitrate: defaultVBRBitrate,
				cbrBitrate: defaultCBRBitrate,
				refresh:    defaultRefresh,
				ch:         2,
			},
			want: "dwConnType=5&mpeg_type=4&dwflicker_hz=0&szResolution=6400360&dwFrameRate=25000&vbr_enable=1&dwVbrQuality=2&vbrmaxbitrate=500000&custom_qp_init=25&gop_N=1500&u8PreAlarmBuf=1&u32PostAlarmBuf2Disk=1&u8SplitInterval=5&bEbIoIn=1&bEbIoIn1=1&bOSDFontSize=0&bCamNamePos=2&bDatePos=2&bTimePos=2&szOsdCamName=Camera&u16PostAlarmBuf=1&dwCameraId=1&LangCode=undefined&Recflag=0&submit=Apply",
		},
		{
			in: settings{
				codec:      CodecH264,
				res:        defaultRes,
				frameRate:  defaultFrameRate,
				vbr:        "1",
				quality:    defaultQuality,
				vbrBitrate: defaultVBRBitrate,
				cbrBitrate: defaultCBRBitrate,
				refresh:    defaultRefresh,
				ch:         2,
			},
			want: "dwConnType=5&mpeg_type=10&dwflicker_hz=0&szResolution=6400360&dwFrameRate=25000&vbr_enable=1&dwVbrQuality=2&vbrmaxbitrate=250000&custom_rate_control_type=0&custom_bitrate=0&custom_qp_init=25&custom_qp_min=10&custom_qp_max=40&gop_N=2000&dwEncProfile=1&dwEncLevel=31&dwEntropy=0&u8PreAlarmBuf=1&u32PostAlarmBuf2Disk=1&u8SplitInterval=5&bEbIoIn=1&bEbIoIn1=1&bOSDFontSize=0&bCamNamePos=2&bDatePos=2&bTimePos=2&szOsdCamName=Camera&u16PostAlarmBuf=1&dwCameraId=1&LangCode=undefined&Recflag=0&submit=Apply",
		},
		{
			in: settings{
				codec:      CodecH264,
				res:        res1080,
				frameRate:  defaultFrameRate,
				vbr:        "0",
				quality:    defaultQuality,
				vbrBitrate: defaultVBRBitrate,
				cbrBitrate: "2048000",
				refresh:    defaultRefresh,
				ch:         1,
			},
			want: "dwConnType=5&mpeg_type=10&dwflicker_less=1&dwflicker_hz=0&szResolution=19201080&dwFrameRate=25000&vbr_enable=0&max_bit_rate=2048000&custom_rate_control_type=0&custom_bitrate=0&custom_qp_init=25&custom_qp_min=10&custom_qp_max=40&gop_N=2000&bSliceMode=4&dwEncProfile=3&dwEncLevel=31&dwEntropy=0&u8PreAlarmBuf=1&u32PostAlarmBuf2Disk=1&u8SplitInterval=5&szCamName=Camera&bEbIoIn=1&bEbIoIn1=1&szOsdCamName=Camera&bOSDFontSize=0&bCamNamePos=2&bDatePos=2&bTimePos=2&bAudioCodec=7&bTVoutFormat=2&bReadyLed=0&bLedLan=0&bLedWan=0&bLedMonitor=0&bAlarmLedAutoLevel=5&bAlarmLedAutoDuration=60&bAlarmLed=1&face_detect_level=1&bDayNight=0&bDayNightAutoLevel=3&bIRout=0&bAutoIris=0&IrisType=1&bBLC=0&bIR=1&bNSR=0&ReplaceHomePreset1=0&u16PostAlarmBuf=1&dwCameraId=0&LangCode=undefined&Recflag=0&webpageEncoding=windows-1252&submit=Apply",
		},
	}

	for i, test := range tests {
		got := populateForm(test.in)
		want, err := url.ParseQuery(test.want)
		if err != nil {
			t.Fatalf("should not have got error: %v parsing want string for test: %d", err, i)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("did not get expected result for test: %d\nGot: %v\nWant: %v", i, got, want)
		}
	}
}

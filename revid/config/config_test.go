/*
DESCRIPTION
  config_test.go provides testing for the Config struct methods (Validate and Update).

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
	"testing"
	"time"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/utils/logging"
	"github.com/google/go-cmp/cmp"
)

type dumbLogger struct{}

func (dl *dumbLogger) Log(l int8, m string, a ...interface{})  {}
func (dl *dumbLogger) SetLevel(l int8)                         {}
func (dl *dumbLogger) Debug(msg string, args ...interface{})   {}
func (dl *dumbLogger) Info(msg string, args ...interface{})    {}
func (dl *dumbLogger) Warning(msg string, args ...interface{}) {}
func (dl *dumbLogger) Error(msg string, args ...interface{})   {}
func (dl *dumbLogger) Fatal(msg string, args ...interface{})   {}

func TestValidate(t *testing.T) {
	dl := &dumbLogger{}

	want := Config{
		Logger:               dl,
		Input:                defaultInput,
		Outputs:              []uint8{defaultOutput},
		InputCodec:           defaultInputCodec,
		RTPAddress:           defaultRTPAddr,
		CameraIP:             defaultCameraIP,
		BurstPeriod:          defaultBurstPeriod,
		MinFrames:            defaultMinFrames,
		FrameRate:            defaultFrameRate,
		ClipDuration:         defaultClipDuration,
		PSITime:              defaultPSITime,
		FileFPS:              defaultFileFPS,
		PoolCapacity:         defaultPoolCapacity,
		PoolStartElementSize: defaultPoolStartElementSize,
		PoolWriteTimeout:     defaultPoolWriteTimeout,
		MinFPS:               defaultMinFPS,
	}

	got := Config{Logger: dl}
	err := (&got).Validate()
	if err != nil {
		t.Fatalf("did not expect error: %v", err)
	}

	if !cmp.Equal(got, want) {
		t.Errorf("configs not equal\nwant: %v\ngot: %v", want, got)
	}
}

func TestUpdate(t *testing.T) {
	updateMap := map[string]string{
		"AutoWhiteBalance":  "sun",
		"BitDepth":          "3",
		"Bitrate":           "200000",
		"Brightness":        "30",
		"BurstPeriod":       "10",
		"CameraChan":        "2",
		"CameraIP":          "192.168.1.5",
		"CBR":               "true",
		"ClipDuration":      "5",
		"Exposure":          "night",
		"FileFPS":           "30",
		"Filters":           "MOG",
		"FrameRate":         "30",
		"Height":            "300",
		"HorizontalFlip":    "true",
		"HTTPAddress":       "http://address",
		"Input":             "rtsp",
		"InputCodec":        "mjpeg",
		"InputPath":         "/inputpath",
		"logging":           "Error",
		"Loop":              "true",
		"MinFPS":            "30",
		"MinFrames":         "30",
		"MotionDownscaling": "3",
		"MotionHistory":     "4",
		"MotionInterval":    "6",
		"MotionKernel":      "2",
		"MotionMinArea":     "9",
		"MotionPadding":     "8",
		"MotionPixels":      "100",
		"MotionThreshold":   "34",
		"OutputPath":        "/outputpath",
		"Outputs":           "Rtmp,Rtp",
		"Quantization":      "30",
		"PoolCapacity":      "100000",
		"PoolWriteTimeout":  "50",
		"Rotation":          "180",
		"RTMPURL":           "rtmp://url",
		"RTPAddress":        "ip:port",
		"Saturation":        "-10",
		"VBRBitrate":        "300000",
		"VBRQuality":        "excellent",
		"VerticalFlip":      "true",
		"Width":             "300",
		"TransformMatrix":   "0.1,  0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8,0.9",
	}

	dl := &dumbLogger{}

	want := Config{
		Logger:            dl,
		AutoWhiteBalance:  "sun",
		BitDepth:          3,
		Bitrate:           200000,
		Brightness:        30,
		BurstPeriod:       10,
		CameraChan:        2,
		CameraIP:          "192.168.1.5",
		CBR:               true,
		ClipDuration:      5 * time.Second,
		Exposure:          "night",
		FileFPS:           30,
		Filters:           []uint{FilterMOG},
		FrameRate:         30,
		Height:            300,
		HorizontalFlip:    true,
		HTTPAddress:       "http://address",
		Input:             InputRTSP,
		InputCodec:        codecutil.MJPEG,
		InputPath:         "/inputpath",
		LogLevel:          logging.Error,
		Loop:              true,
		MinFPS:            30,
		MinFrames:         30,
		MotionDownscaling: 3,
		MotionHistory:     4,
		MotionInterval:    6,
		MotionKernel:      2,
		MotionMinArea:     9,
		MotionPadding:     8,
		MotionPixels:      100,
		MotionThreshold:   34,
		OutputPath:        "/outputpath",
		Outputs:           []uint8{OutputRTMP, OutputRTP},
		Quantization:      30,
		PoolCapacity:      100000,
		PoolWriteTimeout:  50,
		Rotation:          180,
		RTMPURL:           []string{"rtmp://url"},
		RTPAddress:        "ip:port",
		Saturation:        -10,
		VBRBitrate:        300000,
		VBRQuality:        QualityExcellent,
		VerticalFlip:      true,
		Width:             300,
		TransformMatrix:   []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9},
	}

	got := Config{Logger: dl}
	got.Update(updateMap)
	if !cmp.Equal(want, got) {
		t.Errorf("configs not equal\nwant: %v\ngot: %v", want, got)
	}
}

/*
DESCRIPTION
  See package description.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package config-cli is a command-line program for configuring the GeoVision camera.
package main

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/ausocean/av/device/geovision/config"
)

func main() {
	var (
		hostPtr       = flag.String("host", "192.168.1.50", "IP of GeoVision camera.")
		codecPtr      = flag.String("codec", "", "h264, h265 or mjpeg")
		heightPtr     = flag.Uint("height", 0, "256, 360 or 720")
		fpsPtr        = flag.Uint("fps", 0, "Frame rate in frames per second.")
		vbrPtr        = flag.Bool("vbr", false, "If true, variable bitrate.")
		vbrQualityPtr = flag.Int("quality", -1, "General quality under variable bitrate, 0 to 4 inclusive.")
		vbrRatePtr    = flag.Uint("vbr-rate", 0, "Variable bitrate maximal bitrate in kbps.")
		cbrRatePtr    = flag.Uint("cbr-rate", 0, "Constant bitrate, bitrate in kbps.")
		refreshPtr    = flag.Float64("refresh", 0, "Inter refresh period in seconds.")
	)
	flag.Parse()

	var options []config.Option

	if *codecPtr != "" {
		var c config.Codec
		switch *codecPtr {
		case "h264":
			c = config.CodecH264
		case "h265":
			c = config.CodecH265
		case "mjpeg":
			c = config.CodecMJPEG
		default:
			panic(fmt.Sprintf("invalid codec: %s", *codecPtr))
		}
		options = append(options, config.CodecOut(c))
	}

	if *heightPtr != 0 {
		options = append(options, config.Height(*heightPtr))
	}

	if *fpsPtr != 0 {
		options = append(options, config.FrameRate(*fpsPtr))
	}

	options = append(options, config.VariableBitrate(*vbrPtr))

	if *vbrQualityPtr != -1 {
		options = append(options, config.VBRQuality(config.Quality(strconv.Itoa(*vbrQualityPtr))))
	}

	if *vbrRatePtr != 0 {
		options = append(options, config.VBRBitrate(*vbrRatePtr))
	}

	if *cbrRatePtr != 0 {
		options = append(options, config.CBRBitrate(*cbrRatePtr))
	}

	if *refreshPtr != 0 {
		options = append(options, config.Refresh(*refreshPtr))
	}

	err := config.Set(*hostPtr, options...)
	if err != nil {
		panic(fmt.Sprintf("error from config.Set: %v", err))
	}
}

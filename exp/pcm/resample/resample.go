/*
NAME
  resample.go

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package resample is a command-line program for resampling a pcm file.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/ausocean/av/codec/pcm"
)

// This program accepts an input pcm file and outputs a resampled pcm file.
// Input and output file names, to and from sample rates, channels and sample format can be specified as arguments.
func main() {
	var inPath = *flag.String("in", "data.pcm", "file path of input data")
	var outPath = *flag.String("out", "resampled.pcm", "file path of output")
	var from = *flag.Uint("from", 48000, "sample rate of input file")
	var to = *flag.Uint("to", 8000, "sample rate of output file")
	var channels = *flag.Uint("ch", 1, "number of channels in input file")
	var SFString = *flag.String("sf", "S16_LE", "sample format of input audio, eg. S16_LE")
	flag.Parse()

	// Read pcm.
	inPcm, err := ioutil.ReadFile(inPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Read", len(inPcm), "bytes from file", inPath)

	var sf pcm.SampleFormat
	switch SFString {
	case "S32_LE":
		sf = pcm.S32_LE
	case "S16_LE":
		sf = pcm.S16_LE
	default:
		log.Fatalf("Unhandled ALSA format: %v", SFString)
	}

	format := pcm.BufferFormat{
		Channels: channels,
		Rate:     from,
		SFormat:  sf,
	}

	buf := pcm.Buffer{
		Format: format,
		Data:   inPcm,
	}

	// Resample audio.
	resampled, err := pcm.Resample(buf, to)
	if err != nil {
		log.Fatal(err)
	}

	// Save resampled to file.
	err = ioutil.WriteFile(outPath, resampled.Data, 0644)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Encoded and wrote", len(resampled.Data), "bytes to file", outPath)
}

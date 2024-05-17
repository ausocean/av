/*
NAME
  stereo-to-mono.go

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package stereo-to-mono is a command-line program for converting a mono pcm file to a stereo pcm file.
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
	var outPath = *flag.String("out", "mono.pcm", "file path of output")
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
		log.Fatalf("Unhandled sample format: %v", SFString)
	}

	format := pcm.BufferFormat{
		Channels: 2,
		SFormat:  sf,
	}

	buf := pcm.Buffer{
		Format: format,
		Data:   inPcm,
	}

	// Convert audio.
	mono, err := pcm.StereoToMono(buf)
	if err != nil {
		log.Fatal(err)
	}

	// Save mono to file.
	err = ioutil.WriteFile(outPath, mono.Data, 0644)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Encoded and wrote", len(mono.Data), "bytes to file", outPath)
}

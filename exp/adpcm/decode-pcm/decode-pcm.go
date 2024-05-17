/*
NAME
  decode-pcm.go

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package decode-pcm is a command-line program for decoding/decompressing an adpcm file to a pcm file.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/ausocean/av/codec/adpcm"
)

// This program accepts an input file encoded in adpcm and outputs a decoded pcm file.
// Input and output file names can be specified as arguments.
func main() {
	var inPath string
	var outPath string
	flag.StringVar(&inPath, "in", "encoded.adpcm", "file path of input")
	flag.StringVar(&outPath, "out", "decoded.pcm", "file path of output data")
	flag.Parse()

	// Read adpcm.
	comp, err := ioutil.ReadFile(inPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Read", len(comp), "bytes from file", inPath)

	// Decode adpcm.
	decoded := bytes.NewBuffer(make([]byte, 0, len(comp)*4))
	dec := adpcm.NewDecoder(decoded)
	_, err = dec.Write(comp)
	if err != nil {
		log.Fatal(err)
	}

	// Save pcm to file.
	err = ioutil.WriteFile(outPath, decoded.Bytes(), 0644)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Decoded and wrote", len(decoded.Bytes()), "bytes to file", outPath)
}

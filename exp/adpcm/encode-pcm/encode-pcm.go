/*
NAME
  encode-pcm.go

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package encode-pcm is a command-line program for encoding/compressing a pcm file to an adpcm file.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/ausocean/av/codec/adpcm"
)

// This program accepts an input pcm file and outputs an encoded adpcm file.
// Input and output file names can be specified as arguments.
func main() {
	var inPath string
	var adpcmPath string
	flag.StringVar(&inPath, "in", "data.pcm", "file path of input data")
	flag.StringVar(&adpcmPath, "out", "encoded.adpcm", "file path of output")
	flag.Parse()

	// Read pcm.
	pcm, err := ioutil.ReadFile(inPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Read", len(pcm), "bytes from file", inPath)

	// Encode adpcm.
	comp := bytes.NewBuffer(make([]byte, 0, adpcm.EncBytes(len(pcm))))
	enc := adpcm.NewEncoder(comp)
	_, err = enc.Write(pcm)
	if err != nil {
		log.Fatal(err)
	}

	// Save adpcm to file.
	err = ioutil.WriteFile(adpcmPath, comp.Bytes(), 0644)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Encoded and wrote", len(comp.Bytes()), "bytes to file", adpcmPath)
}

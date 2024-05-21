/*
AUTHORS
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

// mts-unwrapper will unwrap an MPEG-TS encoded file and output the data contained in the PES to a specified file.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/container/mts/meta"
	"github.com/ausocean/av/container/mts/psi"

	"github.com/Comcast/gots/v2/packet"
	"github.com/Comcast/gots/v2/pes"
)

func main() {
	const chunkSize = 16000
	var (
		inPath, outPath string
		pid             int
	)
	flag.StringVar(&inPath, "in", "media.ts", "file path of input")
	flag.StringVar(&outPath, "out", "media.raw", "file path of output data")
	flag.IntVar(&pid, "pid", 210, "PID of encoded media; 210 is audio, 256 is h264 video")
	flag.Parse()

	clip, err := os.ReadFile(inPath)
	if err != nil {
		log.Fatal(err)
	}

	pmt, _, err := mts.FindPid(clip, mts.PmtPid)
	if err != nil {
		log.Fatal(err)
	}
	var metadata []byte
	p := psi.PSIBytes(pmt)
	p = p[4:]
	_, metadata = p.HasDescriptor(psi.MetadataTag)

	if metadata != nil {
		v, err := meta.Get("codec", metadata[2:])
		if err != nil {
			log.Fatal(fmt.Errorf("unable to get MTS metadata: %v", err))
		}
		fmt.Println("codec:" + v)
	} else {
		fmt.Println("metadata nil")
	}

	// Get the first MTS packet to check.
	var pkt packet.Packet
	pesPacket := make([]byte, 0, chunkSize)
	got := make([]byte, 0)
	i := 0
	if i+mts.PacketSize <= len(clip) {
		copy(pkt[:], clip[i:i+mts.PacketSize])
	}

	// Loop through MTS packets until all the media data from PES packets has been retrieved.
	for i+mts.PacketSize <= len(clip) {
		// Check MTS packet.
		if !(pkt.PID() == pid) {
			i += mts.PacketSize
			if i+mts.PacketSize <= len(clip) {
				copy(pkt[:], clip[i:i+mts.PacketSize])
			}
			continue
		}
		if !pkt.PayloadUnitStartIndicator() {
			i += mts.PacketSize
			if i+mts.PacketSize <= len(clip) {
				copy(pkt[:], clip[i:i+mts.PacketSize])
			}
		} else {
			// Copy the first MTS payload.
			payload, err := pkt.Payload()
			if err != nil {
				log.Fatal(fmt.Errorf("unable to get MTS payload: %v", err))
			}
			pesPacket = append(pesPacket, payload...)

			i += mts.PacketSize
			if i+mts.PacketSize <= len(clip) {
				copy(pkt[:], clip[i:i+mts.PacketSize])
			}

			// Copy the rest of the MTS payloads that are part of the same PES packet.
			for (!pkt.PayloadUnitStartIndicator()) && i+mts.PacketSize <= len(clip) {
				payload, err = pkt.Payload()
				if err != nil {
					log.Fatal(fmt.Errorf("unable to get MTS payload: %v", err))
				}
				pesPacket = append(pesPacket, payload...)

				i += mts.PacketSize
				if i+mts.PacketSize <= len(clip) {
					copy(pkt[:], clip[i:i+mts.PacketSize])
				}
			}
		}
		// Get the media data from the current PES packet.
		pesHeader, err := pes.NewPESHeader(pesPacket)
		if err != nil {
			fmt.Println(fmt.Errorf("unable to read PES packet: %v", err))
			continue
		}
		fmt.Printf("PTS: %v\n", pesHeader.PTS())
		got = append(got, pesHeader.Data()...)
		pesPacket = pesPacket[:0]
	}

	// Save media to file.
	err = os.WriteFile(outPath, got, 0644)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Decoded and wrote", len(got), "bytes to file", outPath)
}

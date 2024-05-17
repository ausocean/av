/*
NAME
	ts-repair/main.go

DESCRIPTION
  This program attempts to repair mpegts discontinuities using one of two methods
	as selected by the mode flag. Setting the mode flag to 0 will result in repair
	by shifting all CCs such that they are continuous. Setting the mode flag to 1
	will result in repair through setting the discontinuity indicator to true at
	packets where a discontinuity exists.

	Specify the input file with the in flag, and the output file with out flag.

AUTHOR
  Saxon A. Nelson-Milton <saxon.milton@gmail.com>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ausocean/av/container/mts"
	"github.com/Comcast/gots/packet"
)

const (
	PatPid                     = 0
	PmtPid                     = 4096
	VideoPid                   = 256
	HeadSize                   = 4
	DefaultAdaptationSize      = 2
	AdaptationIdx              = 4
	AdaptationControlIdx       = 3
	AdaptationBodyIdx          = AdaptationIdx + 1
	AdaptationControlMask      = 0x30
	DefaultAdaptationBodySize  = 1
	DiscontinuityIndicatorMask = 0x80
	DiscontinuityIndicatorIdx  = AdaptationIdx + 1
)

// Various errors that we can encounter.
const (
	errBadInPath         = "No file path provided, or file does not exist"
	errCantCreateOut     = "Can't create output file"
	errCantGetPid        = "Can't get pid from packet"
	errReadFail          = "Read failed"
	errWriteFail         = "Write to file failed"
	errBadMode           = "Bad fix mode"
	errAdaptationPresent = "Adaptation field is already present in packet"
	errNoAdaptationField = "No adaptation field in this packet"
)

// Consts describing flag usage.
const (
	inUsage   = "The path to the file to be repaired"
	outUsage  = "Output file path"
	modeUsage = "Fix mode: 0 = cc-shift, 1 = di-update"
)

// Repair modes.
const (
	ccShift = iota
	diUpdate
)

var ccMap = map[int]byte{
	PatPid:   16,
	PmtPid:   16,
	VideoPid: 16,
}

// packetNo will keep track of the ts packet number for reference.
var packetNo int

// Option defines a func that performs an action on p in order to change a ts option.
type Option func(p *Packet)

// Packet is a byte array of size PacketSize i.e. 188 bytes. We define this
// to allow us to write receiver funcs for the [PacketSize]byte type.
type Packet [mts.PacketSize]byte

// CC returns the CC of p.
func (p *Packet) CC() byte {
	return (*p)[3] & 0x0f
}

// setCC sets the CC of p.
func (p *Packet) setCC(cc byte) {
	(*p)[3] |= cc & 0xf
}

// setDI sets the discontinuity counter of p.
func (p *Packet) setDI(di bool) {
	if di {
		p[5] |= 0x80
	} else {
		p[5] &= 0x7f
	}
}

// addAdaptationField adds an adaptation field to p, and applys the passed options to this field.
// TODO: this will probably break if we already have adaptation field.
func (p *Packet) addAdaptationField(options ...Option) error {
	if p.hasAdaptation() {
		return errors.New(errAdaptationPresent)
	}
	// Create space for adaptation field.
	copy(p[HeadSize+DefaultAdaptationSize:], p[HeadSize:len(p)-DefaultAdaptationSize])

	// TODO: seperate into own function
	// Update adaptation field control.
	p[AdaptationControlIdx] &= 0xff ^ AdaptationControlMask
	p[AdaptationControlIdx] |= AdaptationControlMask
	// Default the adaptationfield.
	p.resetAdaptation()

	// Apply and options that have bee passed.
	for _, option := range options {
		option(p)
	}
	return nil
}

// resetAdaptation sets fields in ps adaptation field to 0 if the adaptation field
// exists, otherwise an error is returned.
func (p *Packet) resetAdaptation() error {
	if !p.hasAdaptation() {
		return errors.New(errNoAdaptationField)
	}
	p[AdaptationIdx] = DefaultAdaptationBodySize
	p[AdaptationBodyIdx] = 0x00
	return nil
}

// hasAdaptation returns true if p has an adaptation field and false otherwise.
func (p *Packet) hasAdaptation() bool {
	afc := p[AdaptationControlIdx] & AdaptationControlMask
	if afc == 0x20 || afc == 0x30 {
		return true
	} else {
		return false
	}
}

// DiscontinuityIndicator returns and Option that will set p's discontinuity
// indicator according to f.
func DiscontinuityIndicator(f bool) Option {
	return func(p *Packet) {
		set := byte(DiscontinuityIndicatorMask)
		if !f {
			set = 0x00
		}
		p[DiscontinuityIndicatorIdx] &= 0xff ^ DiscontinuityIndicatorMask
		p[DiscontinuityIndicatorIdx] |= DiscontinuityIndicatorMask & set
	}
}

func main() {
	// Deal with input flags
	inPtr := flag.String("in", "", inUsage)
	outPtr := flag.String("out", "out.ts", outUsage)
	modePtr := flag.Int("mode", diUpdate, modeUsage)
	flag.Parse()

	// Try and open the given input file, otherwise panic - we can't do anything
	inFile, err := os.Open(*inPtr)
	defer inFile.Close()
	if err != nil {
		panic(errBadInPath)
	}

	// Try and create output file, otherwise panic - we can't do anything
	outFile, err := os.Create(*outPtr)
	defer outFile.Close()
	if err != nil {
		panic(errCantCreateOut)
	}

	// Read each packet from the input file reader
	var p Packet
	for {
		// If we get an end of file then return, otherwise we panic - can't do anything else
		if _, err := inFile.Read(p[:mts.PacketSize]); err == io.EOF {
			return
		} else if err != nil {
			panic(errReadFail + ": " + err.Error())
		}
		packetNo++

		// Get the pid from the packet
		pid := packet.Pid((*packet.Packet)(&p))

		// Get the cc from the packet and also the expected cc (if exists)
		cc := p.CC()
		expect, exists := expectedCC(int(pid))
		if !exists {
			updateCCMap(int(pid), cc)
		} else {
			switch *modePtr {
			// ccShift mode shifts all CC regardless of presence of Discontinuities or not
			case ccShift:
				p.setCC(expect)
				// diUpdate mode finds discontinuities and sets the discontinuity indicator to true.
				// If we have a pat or pmt then we need to add an adaptation field and then set the DI.
			case diUpdate:
				if cc != expect {
					fmt.Printf("***** Discontinuity found (packetNo: %v pid: %v, cc: %v, expect: %v)\n", packetNo, pid, cc, expect)
					if p.hasAdaptation() {
						p.setDI(true)
					} else {
						p.addAdaptationField(DiscontinuityIndicator(true))
					}
					updateCCMap(int(pid), p.CC())
				}
			default:
				panic(errBadMode)
			}
		}

		// Write this packet to the output file.
		if _, err := outFile.Write(p[:]); err != nil {
			panic(errWriteFail + ": " + err.Error())
		}
	}
}

// expectedCC returns the expected cc for the given pid. If the cc hasn't been
// used yet, then 16 and false is returned.
func expectedCC(pid int) (byte, bool) {
	cc := ccMap[pid]
	if cc == 16 {
		return 16, false
	}
	ccMap[pid] = (cc + 1) & 0xf
	return cc, true
}

// updateCCMap updates the cc for the passed pid.
func updateCCMap(pid int, cc byte) {
	ccMap[pid] = (cc + 1) & 0xf
}

/*
NAME
  discontinuity.go

DESCRIPTION
  discontinuity.go provides functionality for detecting discontinuities in
  MPEG-TS and accounting for using the discontinuity indicator in the adaptation
  field.

AUTHOR
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package mts

import (
	"github.com/Comcast/gots/v2/packet"
)

// discontinuityRepairer provides function to detect discontinuities in MPEG-TS
// and set the discontinuity indicator as appropriate.
type DiscontinuityRepairer struct {
	expCC map[int]int
}

// NewDiscontinuityRepairer returns a pointer to a new discontinuityRepairer.
func NewDiscontinuityRepairer() *DiscontinuityRepairer {
	return &DiscontinuityRepairer{
		expCC: map[int]int{
			PatPid:   16,
			PmtPid:   16,
			PIDVideo: 16,
		},
	}
}

// Failed is to be called in the case of a failed send. This will decrement the
// expectedCC so that it aligns with the failed chunks cc.
func (dr *DiscontinuityRepairer) Failed() {
	dr.decExpectedCC(PatPid)
}

// Repair takes a clip of MPEG-TS and checks that the first packet, which should
// be a PAT, contains a cc that is expected, otherwise the discontinuity indicator
// is set to true.
func (dr *DiscontinuityRepairer) Repair(d []byte) error {
	var pkt packet.Packet
	copy(pkt[:], d[:PacketSize])
	pid := pkt.PID()
	if pid != PatPid {
		panic("Clip to repair must have PAT first")
	}
	cc := pkt.ContinuityCounter()
	expect, _ := dr.ExpectedCC(pid)
	if cc != int(expect) {
		if packet.ContainsAdaptationField(&pkt) {
			(*packet.AdaptationField)(&pkt).SetDiscontinuity(true)
		} else {
			err := addAdaptationField(&pkt, DiscontinuityIndicator(true))
			if err != nil {
				return err
			}
		}
		dr.SetExpectedCC(pid, cc)
		copy(d[:PacketSize], pkt[:])
	}
	dr.IncExpectedCC(pid)
	return nil
}

// expectedCC returns the expected cc. If the cc hasn't been used yet, then 16
// and false is returned.
func (dr *DiscontinuityRepairer) ExpectedCC(pid int) (int, bool) {
	if dr.expCC[pid] == 16 {
		return 16, false
	}
	return dr.expCC[pid], true
}

// incExpectedCC increments the expected cc.
func (dr *DiscontinuityRepairer) IncExpectedCC(pid int) {
	dr.expCC[pid] = (dr.expCC[pid] + 1) & 0xf
}

// decExpectedCC decrements the expected cc.
func (dr *DiscontinuityRepairer) decExpectedCC(pid int) {
	dr.expCC[pid] = (dr.expCC[pid] - 1) & 0xf
}

// setExpectedCC sets the expected cc.
func (dr *DiscontinuityRepairer) SetExpectedCC(pid, cc int) {
	dr.expCC[pid] = cc
}

/*
NAME
  audio_linux.go

AUTHORS
  Alan Noble <alan@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package revid

import (
	"fmt"
	"strconv"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/device"
	"github.com/ausocean/av/device/alsa"
)

// Used to reliably read, write, and test audio metadata entry keys.
const (
	SampleRateKey = "sampleRate"
	ChannelsKey   = "channels"
	PeriodKey     = "period"
	BitDepthKey   = "bitDepth"
	CodecKey      = "codec"
)

func (r *Revid) setupAudio() error {
	// Create new ALSA device.
	d := alsa.New(r.cfg.Logger)
	r.input = d

	// Configure ALSA device.
	r.cfg.Logger.Debug("configuring input device")
	err := d.Setup(r.cfg)
	switch err := err.(type) {
	case nil:
		// Do nothing.
	case device.MultiError:
		r.cfg.Logger.Warning("errors from configuring input device", "errors", err)
	default:
		return err
	}
	r.cfg.Logger.Info("input device configured")

	// Set revid's lexer.
	l, err := codecutil.NewByteLexer(d.DataSize())
	if err != nil {
		return err
	}
	r.lexTo = l.Lex

	// Add metadata.
	mts.Meta.Add(SampleRateKey, strconv.Itoa(int(r.cfg.SampleRate)))
	mts.Meta.Add(ChannelsKey, strconv.Itoa(int(r.cfg.Channels)))
	mts.Meta.Add(PeriodKey, fmt.Sprintf("%.6f", r.cfg.RecPeriod))
	mts.Meta.Add(BitDepthKey, strconv.Itoa(int(r.cfg.BitDepth)))
	mts.Meta.Add(CodecKey, r.cfg.InputCodec)

	return nil
}

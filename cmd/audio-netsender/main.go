/*
NAME
  audio-netsender - netsender client for sending audio to the cloud.

AUTHORS
  Alan Noble <alan@ausocean.org>
  Trek Hopton <trek@ausocean.org>

ACKNOWLEDGEMENTS
  A special thanks to Joel Jensen for his Go ALSA package.

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package audio-netsender is a NetSender client for sending audio to
// the cloud. Audio is captured by means of an ALSA recording device,
// specified by the "source" cloud variable. It sent via HTTP to the
// cloud in raw audio form, i.e., as PCM data, where it is stored as
// BinaryData objects. Other variables are "rate", "period",
// "channels" and "bits", for specifiying the frame rate (Hz), audio
// period (seconds), number of channels and sample bit size
// respectively.
package main

import (
	"errors"
	"flag"
	"io"
	"strconv"
	"sync"
	"time"

	yalsa "github.com/yobert/alsa"

	"github.com/ausocean/av/codec/pcm"
	"github.com/ausocean/client/pi/netsender"
	"github.com/ausocean/client/pi/sds"
	"github.com/ausocean/client/pi/smartlogger"
	"github.com/ausocean/utils/logging"
	"github.com/ausocean/utils/pool"
)

const (
	progName         = "audio-netsender"
	logPath          = "/var/log/netsender"
	retryPeriod      = 5 * time.Second
	defaultFrameRate = 48000
	defaultPeriod    = 5 // seconds
	defaultChannels  = 2
	defaultBits      = 16
	rbDuration       = 300 // seconds
	rbTimeout        = 100 * time.Millisecond
	rbNextTimeout    = 100 * time.Millisecond
)

// audioClient holds everything we need to know about the client.
// NB: At 44100 Hz frame rate, 2 channels and 16-bit samples, a period of 5 seconds
// results in PCM data chunks of 882000 bytes! A longer period exceeds datastore's 1MB blob limit.
type audioClient struct {
	mu sync.Mutex // mu protects the audioClient.

	parameters

	// internals
	dev *yalsa.Device     // audio input device
	pb  pcm.Buffer        // Buffer to contain the direct audio from ALSA.
	buf *pool.Buffer      // Ring buffer to contain processed audio ready to be read.
	ns  *netsender.Sender // our NetSender
	vs  int               // our "var sum" to track var changes
}

type parameters struct {
	mode     string // operating mode, either "Normal" or "Paused"
	source   string // name of audio source, or empty for the default source
	rate     int    // frame rate in Hz, 44100Hz by default
	period   int    // audio period in seconds, 5s by default
	channels int    // number of audio channels, 1 for mono, 2 for stereo
	bits     int    // sample bit size, 16 by default
}

var log logging.Logger

func main() {
	var logLevel int
	flag.IntVar(&logLevel, "LogLevel", int(logging.Debug), "Specifies log level")
	flag.Parse()

	validLogLevel := true
	if logLevel < int(logging.Debug) || logLevel > int(logging.Fatal) {
		logLevel = int(logging.Info)
		validLogLevel = false
	}

	logSender := smartlogger.New(logPath)
	log = logging.New(int8(logLevel), &logSender.LogRoller, true)
	log.Info("log-netsender: Logger Initialized")
	if !validLogLevel {
		log.Error("invalid log level was defaulted to Info")
	}

	var ac audioClient
	var err error
	ac.ns, err = netsender.New(log, nil, sds.ReadSystem, nil)
	if err != nil {
		log.Fatal("netsender.Init failed", "error", err.Error())
	}

	// Get audio params and store the current var sum.
	vars, err := ac.ns.Vars()
	if err != nil {
		log.Warning("netsender.Vars failed; using defaults", "error", err.Error())
	}
	ac.params(vars)
	ac.vs = ac.ns.VarSum()

	// Open the requested audio device.
	err = ac.open()
	if err != nil {
		log.Fatal("yalsa.open failed", "error", err.Error())
	}

	// Capture audio in periods of ac.period seconds, and buffer rbDuration seconds in total.
	ab := ac.dev.NewBufferDuration(time.Second * time.Duration(ac.period))
	sf, err := pcm.SFFromString(ab.Format.SampleFormat.String())
	if err != nil {
		log.Error(err.Error())
	}
	cf := pcm.BufferFormat{
		SFormat:  sf,
		Channels: uint(ab.Format.Channels),
		Rate:     uint(ab.Format.Rate),
	}
	ac.pb = pcm.Buffer{
		Format: cf,
		Data:   ab.Data,
	}

	cs := pcm.DataSize(
		uint(ac.parameters.rate),
		uint(ac.parameters.channels),
		uint(ac.parameters.bits),
		float64(ac.parameters.period),
	)
	rbLen := rbDuration / ac.period
	ac.buf = pool.NewBuffer(int(rbLen), cs, rbTimeout)

	go ac.input()

	ac.output()
}

// params extracts audio params from corresponding cloud vars and returns true if anything has changed.
// See audioClient for a description of the params and their limits.
func (ac *audioClient) params(vars map[string]string) bool {
	// We are the only writers to this field
	// so we don't need to lock here.
	p := ac.parameters
	changed := false

	mode := vars["mode"]
	if p.mode != mode {
		p.mode = mode
		changed = true
	}
	source := vars["source"]
	if p.source != source {
		p.source = source
		changed = true
	}
	val, err := strconv.Atoi(vars["rate"])
	if err != nil {
		val = defaultFrameRate
	}
	if p.rate != val {
		p.rate = val
		changed = true
	}
	val, err = strconv.Atoi(vars["period"])
	if err != nil || val < 1 || 5 < val {
		val = defaultPeriod
	}
	if p.period != val {
		p.period = val
		changed = true
	}
	val, err = strconv.Atoi(vars["channels"])
	if err != nil || (val != 1 && val != 2) {
		val = defaultChannels
	}
	if p.channels != val {
		p.channels = val
		changed = true
	}
	val, err = strconv.Atoi(vars["bits"])
	if err != nil || (val != 16 && val != 32) {
		val = defaultBits
	}
	if p.bits != val {
		p.bits = val
		changed = true
	}

	if changed {
		ac.mu.Lock()
		ac.parameters = p
		ac.mu.Unlock()
		log.Debug("params changed")
	}
	log.Debug("parameters", "mode", p.mode, "source", p.source, "rate", p.rate, "period", p.period, "channels", p.channels, "bits", p.bits)
	return changed
}

// open or re-open the recording device with the given name and prepare it to record.
// If name is empty, the first recording device is used.
func (ac *audioClient) open() error {
	if ac.dev != nil {
		log.Debug("closing", "source", ac.source)
		ac.dev.Close()
		ac.dev = nil
	}
	log.Debug("opening", "source", ac.source)

	cards, err := yalsa.OpenCards()
	if err != nil {
		return err
	}
	defer yalsa.CloseCards(cards)

	for _, card := range cards {
		devices, err := card.Devices()
		if err != nil {
			return err
		}
		for _, dev := range devices {
			if dev.Type != yalsa.PCM || !dev.Record {
				continue
			}
			if dev.Title == ac.source || ac.source == "" {
				ac.dev = dev
				break
			}
		}
	}

	if ac.dev == nil {
		return errors.New("No audio source found")
	}
	log.Debug("found audio source", "source", ac.dev.Title)

	// ToDo: time out if Open takes too long.
	err = ac.dev.Open()
	if err != nil {
		return err
	}
	log.Debug("opened audio source")

	_, err = ac.dev.NegotiateChannels(defaultChannels)
	if err != nil {
		return err
	}

	// Try to negotiate a rate to record in that is divisible by the wanted rate
	// so that it can be easily downsampled to the wanted rate.
	// Note: if a card thinks it can record at a rate but can't actually, this can cause a failure. Eg.
	// the audioinjector is supposed to record at 8000Hz and 16000Hz but it can't due to a firmware issue,
	// to fix this 8000 and 16000 must be removed from this slice.
	rates := [8]int{8000, 16000, 32000, 44100, 48000, 88200, 96000, 192000}
	foundRate := false
	for _, r := range rates {
		if r < ac.rate {
			continue
		}
		if r%ac.rate == 0 {
			_, err = ac.dev.NegotiateRate(r)
			if err == nil {
				foundRate = true
				log.Debug("sample rate set", "rate", r)
				break
			}
		}
	}

	// If no easily divisible rate is found, then use the default rate.
	if !foundRate {
		log.Warning("no available device sample-rates are divisible by the requested rate. Default rate will be used. Resampling may fail.", "rateRequested", ac.rate)
		_, err = ac.dev.NegotiateRate(defaultFrameRate)
		if err != nil {
			return err
		}
		log.Debug("sample rate set", "rate", defaultFrameRate)
	}

	var fmt yalsa.FormatType
	switch ac.bits {
	case 16:
		fmt = yalsa.S16_LE
	case 32:
		fmt = yalsa.S32_LE
	default:
		return errors.New("unsupported sample bits")
	}
	_, err = ac.dev.NegotiateFormat(fmt)
	if err != nil {
		return err
	}

	// Either 8192 or 16384 bytes is a reasonable ALSA buffer size.
	_, err = ac.dev.NegotiateBufferSize(8192, 16384)
	if err != nil {
		return err
	}

	if err = ac.dev.Prepare(); err != nil {
		return err
	}
	log.Debug("successfully negotiated ALSA params")
	return nil
}

// input continously records audio and writes it to the ringbuffer.
// Re-opens the device and tries again if ASLA returns an error.
// Spends a lot of time sleeping in Paused mode.
// ToDo: Currently, reading audio and writing to the ringbuffer are synchronous.
// Need a way to asynchronously read from the buf, i.e.,  _while_ it is recording to avoid any gaps.
func (ac *audioClient) input() {
	for {
		ac.mu.Lock()
		mode := ac.mode
		ac.mu.Unlock()
		if mode == "Paused" {
			time.Sleep(time.Duration(ac.period) * time.Second)
			continue
		}
		log.Debug("recording audio for period", "seconds", ac.period)
		ac.mu.Lock()
		err := ac.dev.Read(ac.pb.Data)
		ac.mu.Unlock()
		if err != nil {
			log.Debug("device.Read failed", "error", err.Error())
			ac.mu.Lock()
			err = ac.open() // re-open
			if err != nil {
				log.Fatal("yalsa.open failed", "error", err.Error())
			}
			ac.mu.Unlock()
			continue
		}

		toWrite := ac.formatBuffer()

		log.Debug("audio format conversion has been performed where needed")

		var n int
		n, err = ac.buf.Write(toWrite.Data)
		switch err {
		case nil:
			log.Debug("wrote audio to ringbuffer", "length", n)
		case pool.ErrDropped:
			log.Warning("dropped audio")
		default:
			log.Error("unexpected ringbuffer error", "error", err.Error())
			return
		}
	}
}

// output continously reads audio from the ringbuffer and sends it to the cloud via poll requests.
// When "B0" is configured as one of the inputs, audio data is posted as "B0".
// When "B0" is not an input, the poll request happens without any audio data
// (although other inputs may still be present via URL parameters).
// When paused, polling continues but without sending audio (B0) data.
// Sending is throttled so as to complete one pass of this loop approximately every audio period,
// since cycling more frequently is pointless.
// Finally while audio data is sent every audio period, other data is reported only every monitor period.
// This function also handles cloud configuration requests and updating of cloud vars.
func (ac *audioClient) output() {
	// Calculate the size of the output data based on wanted channels and rate.
	outLen := (((len(ac.pb.Data) / int(ac.pb.Format.Channels)) * ac.channels) / int(ac.pb.Format.Rate)) * ac.rate
	buf := make([]byte, outLen)

	mime := "audio/x-wav;codec=pcm;rate=" + strconv.Itoa(ac.rate) + ";channels=" + strconv.Itoa(ac.channels) + ";bits=" + strconv.Itoa(ac.bits)
	ip := ac.ns.Param("ip")
	mp, err := strconv.Atoi(ac.ns.Param("mp"))
	if err != nil {
		log.Fatal("mp not an integer")
	}

	report := true         // Report non-audio data.
	reported := time.Now() // When we last did so.

	for {
		var rc int
		start := time.Now()
		audio := false
		var pins []netsender.Pin

		if ac.mode == "Paused" {

			// Only send X data when paused (if any).
			if report {
				pins = netsender.MakePins(ip, "X")
			}
		} else {
			n, err := read(ac.buf, buf)
			if err != nil {
				return
			}
			if n == 0 {
				goto sleep
			}
			if n != len(buf) {
				log.Error("unexpected length from read", "length", n)
				return
			}
			if report {
				pins = netsender.MakePins(ip, "")
			} else {
				pins = netsender.MakePins(ip, "B")
			}
			for i, pin := range pins {
				if pin.Name == "B0" {
					audio = true
					pins[i].Value = n
					pins[i].Data = buf
					pins[i].MimeType = mime
				}
			}
		}

		if !(report || audio) {
			goto sleep // nothing to do
		}

		// Populate X pins, if any.
		for i, pin := range pins {
			if pin.Name[0] == 'X' {
				err := sds.ReadSystem(&pins[i])
				if err != nil {
					log.Warning("sds.ReadSystem failed", "error", err.Error())
					// Pin.Value defaults to -1 upon error, so OK to continue.
				}
			}
		}
		_, rc, err = ac.ns.Send(netsender.RequestPoll, pins)
		if err != nil {
			log.Debug("netsender.Send failed", "error", err.Error())
			goto sleep
		}
		if report {
			reported = start
			report = false
		}
		if rc == netsender.ResponseUpdate {
			_, err = ac.ns.Config()
			if err != nil {
				log.Warning("netsender.Config failed", "error", err.Error())
				goto sleep
			}
			ip = ac.ns.Param("ip")
			mp, err = strconv.Atoi(ac.ns.Param("mp"))
			if err != nil {
				log.Fatal("mp not an integer")
			}
		}

		if ac.vs != ac.ns.VarSum() {
			vars, err := ac.ns.Vars()
			if err != nil {
				log.Error("netsender.Vars failed", "error", err.Error())
				goto sleep
			}
			ac.params(vars) // ToDo: re-open device if audio params have changed.
			ac.vs = ac.ns.VarSum()
		}

	sleep:
		pause := ac.period*1000 - int(time.Since(start).Seconds()*1000)
		if pause > 0 {
			time.Sleep(time.Duration(pause) * time.Millisecond)
		}
		if time.Since(reported).Seconds() >= float64(mp) {
			report = true
		}

	}
}

// read reads a full PCM chunk from the ringbuffer, returning the number of bytes read upon success.
// Any errors returned are unexpected and should be considered fatal.
func read(rb *pool.Buffer, buf []byte) (int, error) {
	chunk, err := rb.Next(rbNextTimeout)
	switch err {
	case nil:
		// Do nothing.
	case pool.ErrTimeout:
		return 0, nil
	case io.EOF:
		log.Error("unexpected EOF from pool.Next")
		return 0, io.ErrUnexpectedEOF
	default:
		log.Error("unexpected error from pool.Next", "error", err.Error())
		return 0, err
	}

	n, err := io.ReadFull(rb, buf[:chunk.Len()])
	if err != nil {
		log.Error("unexpected error from pool.Read", "error", err.Error())
		return n, err
	}

	log.Debug("read audio from ringbuffer", "length", n)
	return n, nil
}

// formatBuffer returns a Buffer that has the recording data from the ac's original Buffer but stored
// in the desired format specified by the ac's parameters.
func (ac *audioClient) formatBuffer() pcm.Buffer {
	var err error
	ac.mu.Lock()
	wantChannels := ac.channels
	wantRate := ac.rate
	ac.mu.Unlock()

	// If nothing needs to be changed, return the original.
	if int(ac.pb.Format.Channels) == wantChannels && int(ac.pb.Format.Rate) == wantRate {
		return ac.pb
	}

	formatted := pcm.Buffer{Format: ac.pb.Format}
	bufCopied := false
	if int(ac.pb.Format.Channels) != wantChannels {

		// Convert channels.
		if ac.pb.Format.Channels == 2 && wantChannels == 1 {
			if formatted, err = pcm.StereoToMono(ac.pb); err != nil {
				log.Warning("channel conversion failed, audio has remained stereo", "error", err.Error())
			} else {
				formatted.Format.Channels = 1
			}
			bufCopied = true
		}
	}

	if int(ac.pb.Format.Rate) != wantRate {

		// Convert rate.
		if bufCopied {
			formatted, err = pcm.Resample(formatted, uint(wantRate))
		} else {
			formatted, err = pcm.Resample(ac.pb, uint(wantRate))
		}
		if err != nil {
			log.Warning("rate conversion failed, audio has remained original rate", "error", err.Error())
		} else {
			formatted.Format.Rate = uint(wantRate)
		}
	}
	return formatted
}

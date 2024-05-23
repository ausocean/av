/*
NAME
  alsa.go

AUTHOR
  Alan Noble <alan@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

// Package alsa provides access to input from ALSA audio devices.
package alsa

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	yalsa "github.com/yobert/alsa"

	"github.com/ausocean/av/codec/adpcm"
	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/codec/pcm"
	"github.com/ausocean/av/device"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
	"github.com/ausocean/utils/pool"
)

const (
	pkg           = "alsa: "
	rbTimeout     = 100 * time.Millisecond
	rbNextTimeout = 2000 * time.Millisecond
	rbLen         = 200
	pbSize        = 11520000         // 60 seconds of pcm data.
	longRecLength = 10 * time.Second // Longer record period to minimise skips between recordings.
)

// "running" means the input goroutine is reading from the ALSA device and writing to the ringbuffer.
// "paused" means the input routine is sleeping until unpaused or stopped.
// "stopped" means the input routine is stopped and the ALSA device is closed.
const (
	running = iota + 1
	paused
	stopped
)

const (
	defaultSampleRate = 48000
	defaultBitDepth   = 16
	defaultChannels   = 1
	defaultRecPeriod  = 1.0
	defaultCodec      = codecutil.PCM
)

// Configuration field errors.
var (
	errInvalidSampleRate = errors.New("invalid sample rate, defaulting")
	errInvalidChannels   = errors.New("invalid number of channels, defaulting")
	errInvalidBitDepth   = errors.New("invalid bitdepth, defaulting")
	errInvalidRecPeriod  = errors.New("invalid record period, defaulting")
	errInvalidCodec      = errors.New("invalid audio codec, defaulting")
)

// An ALSA device holds everything we need to know about the audio input stream and implements io.Reader and device.AVDevice.
type ALSA struct {
	l      logging.Logger // Logger for device's routines to log to.
	mode   uint8          // Operating mode, either running, paused, or stopped.
	mu     sync.Mutex     // Provides synchronisation when changing modes concurrently.
	title  string         // Name of audio title, or empty for the default title.
	dev    *yalsa.Device  // ALSA device's Audio input device.
	pb     pcm.Buffer     // Buffer to contain the direct audio from ALSA.
	buf    *pool.Buffer   // Ring buffer to contain processed audio ready to be read.
	Config                // Configuration parameters for this device.
}

// Config provides parameters used by the ALSA device.
type Config struct {
	SampleRate uint
	Channels   uint
	BitDepth   uint
	RecPeriod  float64
	Codec      string
}

// New initializes and returns an ALSA device which has its logger set as the given logger.
func New(l logging.Logger) *ALSA { return &ALSA{l: l} }

// Name returns the name of the device.
func (d *ALSA) Name() string {
	return "ALSA"
}

// Setup will take a Config struct, check the validity of the relevant fields
// and then perform any configuration necessary. If fields are not valid,
// an error is added to the multiError and a default value is used.
// It then initialises the ALSA device which can then be started, read from, and stopped.
func (d *ALSA) Setup(c config.Config) error {
	var errs device.MultiError
	if c.SampleRate <= 0 {
		errs = append(errs, errInvalidSampleRate)
		c.SampleRate = defaultSampleRate
	}
	if c.Channels <= 0 {
		errs = append(errs, errInvalidChannels)
		c.Channels = defaultChannels
	}
	if c.BitDepth <= 0 {
		errs = append(errs, errInvalidBitDepth)
		c.BitDepth = defaultBitDepth
	}
	if c.RecPeriod <= 0 {
		errs = append(errs, errInvalidRecPeriod)
		c.RecPeriod = defaultRecPeriod
	}
	if c.InputCodec != codecutil.ADPCM && c.InputCodec != codecutil.PCM {
		errs = append(errs, errInvalidCodec)
		c.InputCodec = defaultCodec
	}
	d.Config = Config{
		SampleRate: c.SampleRate,
		Channels:   c.Channels,
		BitDepth:   c.BitDepth,
		RecPeriod:  c.RecPeriod,
		Codec:      c.InputCodec,
	}

	// Open the requested audio device.
	err := d.open()
	if err != nil {
		return fmt.Errorf("failed to open device: %w", err)
	}

	// Create a buffer for longer continuous recordings.
	ab := d.dev.NewBufferDuration(longRecLength)
	sf, err := pcm.SFFromString(ab.Format.SampleFormat.String())
	if err != nil {
		return fmt.Errorf("unable to get sample format from string: %w", err)
	}
	cf := pcm.BufferFormat{
		SFormat:  sf,
		Channels: uint(ab.Format.Channels),
		Rate:     uint(ab.Format.Rate),
	}
	d.pb = pcm.Buffer{
		Format: cf,
		Data:   ab.Data,
	}

	// Create pool buffer with appropriate chunk size.
	cs := d.DataSize()
	d.buf = pool.NewBuffer(rbLen, cs, rbTimeout)
	pool.MaxAlloc(pbSize * 2)

	// Start device in paused mode.
	d.mode = paused
	go d.input()

	if len(errs) != 0 {
		return errs
	}
	return nil
}

// Set exists to satisfy the implementation of the Device interface that revid uses.
// Everything that would usually be in Set is in the Setup function.
// This is because an ALSA device is different to other devices in that it
// outputs binary non-packetised data and it requires a different configuration procedure.
func (d *ALSA) Set(c config.Config) error {
	return nil
}

// Start will start recording audio and writing to the ringbuffer.
// Once an ALSA device has been stopped it cannot be started again. This is likely to change in future.
func (d *ALSA) Start() error {
	d.mu.Lock()
	mode := d.mode
	d.mu.Unlock()
	switch mode {
	case paused:
		d.mu.Lock()
		d.mode = running
		d.mu.Unlock()
		return nil
	case stopped:
		// TODO(Trek): Make this reopen device and start recording.
		return errors.New("device is stopped")
	case running:
		return nil
	default:
		return fmt.Errorf("invalid mode: %d", mode)
	}
}

// Stop will stop recording audio and close the device.
// Once an ALSA device has been stopped it cannot be started again. This is likely to change in future.
func (d *ALSA) Stop() error {
	d.mu.Lock()
	d.mode = stopped
	d.mu.Unlock()
	return nil
}

// open the recording device with the given name and prepare it to record.
// If name is empty, the first recording device is used.
func (d *ALSA) open() error {
	// Close any existing device.
	if d.dev != nil {
		d.l.Debug("closing device", "title", d.title)
		d.dev.Close()
		d.dev = nil
	}

	// Open sound card and open recording device.
	d.l.Debug("opening sound card")
	cards, err := yalsa.OpenCards()
	if err != nil {
		return err
	}
	defer yalsa.CloseCards(cards)

	d.l.Debug("finding audio device")
	for _, card := range cards {
		devices, err := card.Devices()
		if err != nil {
			continue
		}
		for _, dev := range devices {
			if dev.Type != yalsa.PCM || !dev.Record {
				continue
			}
			if dev.Title == d.title || d.title == "" {
				d.dev = dev
				break
			}
		}
	}
	if d.dev == nil {
		return errors.New("no ALSA device found")
	}

	d.l.Debug("opening ALSA device", "title", d.dev.Title)
	err = d.dev.Open()
	if err != nil {
		return err
	}

	// Try to configure device with chosen channels.
	channels, err := d.dev.NegotiateChannels(int(d.Channels))
	if err != nil && d.Channels == 1 {
		d.l.Info("device is unable to record in mono, trying stereo", "error", err)
		channels, err = d.dev.NegotiateChannels(2)
	}
	if err != nil {
		return fmt.Errorf("device is unable to record with requested number of channels: %w", err)
	}
	d.l.Debug("alsa device channels set", "channels", channels)

	// Try to negotiate a rate to record in that is divisible by the wanted rate
	// so that it can be easily downsampled to the wanted rate.
	// rates is a slice of common sample rates including the standard for CD (44100Hz) and standard for professional audio recording (48000Hz).
	// Note: if a card thinks it can record at a rate but can't actually, this can cause a failure.
	// Eg. the audioinjector sound card is supposed to record at 8000Hz and 16000Hz but it can't due to a firmware issue,
	// a fix for this is to remove 8000 and 16000 from the rates slice.
	var rates = [8]int{8000, 16000, 32000, 44100, 48000, 88200, 96000, 192000}

	var rate int
	foundRate := false
	for _, r := range rates {
		if r < int(d.SampleRate) {
			continue
		}
		if r%int(d.SampleRate) == 0 {
			rate, err = d.dev.NegotiateRate(r)
			if err == nil {
				foundRate = true
				d.l.Debug("alsa device sample rate set", "rate", rate)
				break
			}
		}
	}

	// If no easily divisible rate is found, then use the default rate.
	if !foundRate {
		d.l.Warning("unable to sample at requested rate, default used.", "rateRequested", d.SampleRate)
		rate, err = d.dev.NegotiateRate(defaultSampleRate)
		if err != nil {
			return err
		}
		d.l.Debug("alsa device sample rate set", "rate", rate)
	}

	var aFmt yalsa.FormatType
	switch d.BitDepth {
	case 16:
		aFmt = yalsa.S16_LE
	case 32:
		aFmt = yalsa.S32_LE
	default:
		return fmt.Errorf("unsupported sample bits %v", d.BitDepth)
	}
	devFmt, err := d.dev.NegotiateFormat(aFmt)
	if err != nil {
		return err
	}
	var bitdepth int
	switch devFmt {
	case yalsa.S16_LE:
		bitdepth = 16
	case yalsa.S32_LE:
		bitdepth = 32
	default:
		return fmt.Errorf("unsupported sample bits %v", d.BitDepth)
	}
	d.l.Debug("alsa device bit depth set", "bitdepth", bitdepth)

	// A 50ms period is a sensible value for low-ish latency. (this could be made configurable if needed)
	// Some devices only accept even period sizes while others want powers of 2.
	// So we will find the closest power of 2 to the desired period size.
	const wantPeriod = 0.05 //seconds
	bytesPerSecond := rate * channels * (bitdepth / 8)
	wantPeriodSize := int(float64(bytesPerSecond) * wantPeriod)
	nearWantPeriodSize := nearestPowerOfTwo(wantPeriodSize)
	periodSize, err := d.dev.NegotiatePeriodSize(nearWantPeriodSize)
	if err != nil {
		return err
	}
	d.l.Debug("alsa device period size set", "periodsize", periodSize)

	// At least four period sizes should fit within the buffer.
	bufSize, err := d.dev.NegotiateBufferSize(periodSize * 4)
	if err != nil {
		return err
	}
	d.l.Debug("alsa device buffer size set", "buffersize", bufSize)

	if err = d.dev.Prepare(); err != nil {
		return err
	}

	d.l.Debug("successfully negotiated device params")
	return nil
}

// input continously records audio and writes it to the ringbuffer.
// Re-opens the device and tries again if the ASLA device returns an error.
func (d *ALSA) input() {
	// Make a channel to communicate between continuous recording and processing.
	// The channel has a capacity of 5 minutes of audio, which it should never reach.
	ch := make(chan []byte, int(5*60/d.RecPeriod))

	// Read audio in longer sections (length of longRecPeriod).
	go chunkingRead(d, ch)

	goodCount := 0
	badCount := 0

	ticker := time.NewTicker(time.Duration(d.RecPeriod) * time.Second)

	for {
		// Check mode.
		d.mu.Lock()
		mode := d.mode
		d.mu.Unlock()
		switch mode {
		case paused:
			time.Sleep(time.Duration(d.RecPeriod) * time.Second)
			continue
		case stopped:
			if d.dev != nil {
				d.l.Debug("closing ALSA device", "title", d.title)
				d.dev.Close()
				d.dev = nil
			}
			err := d.buf.Close()
			if err != nil {
				d.l.Error("unable to close pool buffer", "error", err)
			}
			return
		}

		// Read audio chunk from channel.
		<-ticker.C
		timeout := time.NewTimer(time.Duration(d.RecPeriod) * time.Second)
		select {
		case d.pb.Data = <-ch:
		case <-timeout.C:
			continue
		}

		// Process audio.
		d.l.Debug("processing audio")
		toWrite := d.formatBuffer()

		// Write audio to ringbuffer.
		n, err := d.buf.Write(toWrite.Data)
		switch err {
		case nil:
			goodCount++
			d.l.Debug("wrote audio to ringbuffer", "length", n, "full chunks", d.buf.Len(), "throughput", fmt.Sprintf("%.2f", float64(goodCount)/float64(goodCount+badCount)))
		case pool.ErrDropped:
			badCount++
			d.l.Warning("old audio data overwritten", "full chunks", d.buf.Len(), "throughput", fmt.Sprintf("%.2f", float64(goodCount)/float64(goodCount+badCount)))
		default:
			badCount++
			d.l.Error("unexpected ringbuffer error", "error", err.Error())
		}
	}
}

// chunkingRead reads continuously from the ALSA buffer in long sections. The
// audio is then chunked into the recording period set by d.RecPeriod and sent over
// the channel.
func chunkingRead(d *ALSA, ch chan []byte) {
	d.l.Debug("Datasize of recperiod", "datasize", d.DataSize())
	for {
		buf := d.dev.NewBufferDuration(time.Minute)
		// Read audio in 1 minute sections.
		d.l.Debug("Reading audio", "recording length", longRecLength.String())
		err := d.dev.Read(buf.Data)
		if err != nil {
			d.l.Debug("read failed", "error", err.Error())
			err = d.open() // re-open
			if err != nil {
				d.l.Fatal("reopening device failed", "error", err.Error())
				return
			}
			continue
		}

		// Chunk the audio into length of RecPeriod.
		// We won't wait for this to finish executing because we want
		// to start recording again ASAP.
		go chunkingSender(buf.Data, d.DataSize(), ch, d.l)
	}
}

func chunkingSender(buf []byte, size int, ch chan []byte, log logging.Logger) {
	log.Debug("starting chunkingSender")
	for i := 0; i < len(buf); i += size {
		ch <- buf[i:(i + size)]
	}
	log.Debug("finish chunkingSender")
}

// Read reads from the ringbuffer, returning the number of bytes read upon success.
func (d *ALSA) Read(p []byte) (int, error) {
	// Ready ringbuffer for read.
	d.l.Debug(pkg + "getting next chunk ready")
	chunk, err := d.buf.Next(rbNextTimeout)
	if err != nil {
		switch err {
		case io.EOF:
			d.l.Debug(pkg + "EOF from Next")
			return 0, err
		case pool.ErrTimeout:
			d.l.Debug(pkg + "pool buffer timeout")
			return 0, err
		default:
			d.l.Error(pkg+"unexpected error from Next", "error", err.Error())
			return 0, err
		}
	}

	n := copy(p, chunk.Bytes())
	err = chunk.Close()
	if err != nil {
		d.l.Debug("chunk close error:", "err", err)
		return n, err
	}

	// Read from pool buffer.
	d.l.Debug(pkg+"reading from buffer", "full chunks", d.buf.Len())
	d.l.Debug(fmt.Sprintf("%v read %v bytes", pkg, n))
	return n, nil
}

// formatBuffer returns audio that has been converted to the desired format.
func (d *ALSA) formatBuffer() pcm.Buffer {
	var err error

	// If nothing needs to be changed, return the original.
	if d.pb.Format.Channels == d.Channels && d.pb.Format.Rate == d.SampleRate {
		return d.pb
	}
	var formatted pcm.Buffer
	if d.pb.Format.Channels != d.Channels {
		// Convert channels.
		// TODO(Trek): Make this work for conversions other than stereo to mono.
		if d.pb.Format.Channels == 2 && d.Channels == 1 {
			formatted, err = pcm.StereoToMono(d.pb)
			if err != nil {
				d.l.Fatal("channel conversion failed", "error", err.Error())
			}
		}
	}

	if d.pb.Format.Rate != d.SampleRate {
		// Convert rate.
		formatted, err = pcm.Resample(formatted, d.SampleRate)
		if err != nil {
			d.l.Fatal("rate conversion failed", "error", err.Error())
		}
	}

	switch d.Codec {
	case codecutil.PCM:
	case codecutil.ADPCM:
		b := bytes.NewBuffer(make([]byte, 0, adpcm.EncBytes(len(formatted.Data))))
		enc := adpcm.NewEncoder(b)
		_, err = enc.Write(formatted.Data)
		if err != nil {
			d.l.Fatal("unable to encode", "error", err.Error())
		}
		formatted.Data = b.Bytes()
	default:
		d.l.Error("unhandled audio codec")
	}

	return formatted
}

// DataSize returns the size in bytes of the data ALSA device d will
// output in the duration of a single recording period.
func (d *ALSA) DataSize() int {
	s := pcm.DataSize(d.SampleRate, d.Channels, d.BitDepth, d.RecPeriod)
	if d.Codec == codecutil.ADPCM {
		s = adpcm.EncBytes(s)
	}
	return s
}

// nearestPowerOfTwo finds and returns the nearest power of two to the given integer.
// If the lower and higher power of two are the same distance, it returns the higher power.
// For negative values, 1 is returned.
// Source: https://stackoverflow.com/a/45859570
func nearestPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	if n == 1 {
		return 2
	}
	v := n
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++         // higher power of 2
	x := v >> 1 // lower power of 2
	if (v - n) > (n - x) {
		return x
	}
	return v
}

// IsRunning is used to determine if the ALSA device is running.
func (d *ALSA) IsRunning() bool {
	return d.mode == running
}

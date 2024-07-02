/*
DESCRIPTION
  pipeline.go provides functionality for set up of the revid processing pipeline.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Alan Noble <alan@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Trek Hopton <trek@ausocean.org>
  Scott Barnard <scott@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved.

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

package revid

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/codec/h264"
	"github.com/ausocean/av/codec/h265"
	"github.com/ausocean/av/codec/jpeg"
	"github.com/ausocean/av/container/flv"
	"github.com/ausocean/av/container/mts"
	"github.com/ausocean/av/device"
	"github.com/ausocean/av/device/file"
	"github.com/ausocean/av/device/geovision"
	"github.com/ausocean/av/device/raspistill"
	"github.com/ausocean/av/device/raspivid"
	"github.com/ausocean/av/device/webcam"
	"github.com/ausocean/av/filter"
	"github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/ioext"
	"github.com/ausocean/utils/pool"
)

// TODO(Saxon): put more thought into error severity and how to handle these.
func (r *Revid) handleErrors() {
	for {
		err := <-r.err
		if err != nil {
			r.cfg.Logger.Error("async error", "error", err.Error())
		}
	}
}

// reset swaps the current config of a Revid with the passed
// configuration; checking validity and returning errors if not valid. It then
// sets up the data pipeline accordingly to this configuration.
func (r *Revid) reset(c config.Config) error {
	r.cfg.Logger.Debug("setting config")
	err := r.setConfig(c)
	if err != nil {
		return fmt.Errorf("could not set config: %w", err)
	}
	r.cfg.Logger.Info("config set")

	r.cfg.Logger.Debug("setting up revid pipeline")

	err = r.setupPipeline(
		func(dst io.WriteCloser, rate float64) (io.WriteCloser, error) {
			var st int
			var encOptions []func(*mts.Encoder) error

			switch r.cfg.Input {
			case config.InputRaspivid, config.InputRaspistill, config.InputFile, config.InputV4L, config.InputRTSP:
				switch r.cfg.InputCodec {
				case codecutil.H265:
					if r.cfg.Input != config.InputRTSP {
						return nil, errors.New("h.265 codec valid only for InputRTSP")
					}
					st = mts.EncodeH265
				case codecutil.H264:
					st = mts.EncodeH264
				case codecutil.MJPEG:
					st = mts.EncodeMJPEG
					encOptions = append(encOptions, mts.TimeBasedPSI(time.Duration(r.cfg.PSITime)*time.Second))
					r.cfg.CBR = true
				case codecutil.JPEG:
					st = mts.EncodeJPEG
					encOptions = append(encOptions, mts.TimeBasedPSI(time.Duration(r.cfg.PSITime)*time.Second), mts.Rate(1/r.cfg.TimelapseInterval.Seconds()))
					r.cfg.CBR = true
				case codecutil.PCM, codecutil.ADPCM:
					return nil, fmt.Errorf("invalid input codec: %v for input: %v", r.cfg.InputCodec, r.cfg.Input)
				default:
					panic("unknown input codec for Raspivid, Raspistill, File, V4l or RTSP input")
				}
			case config.InputAudio:
				switch r.cfg.InputCodec {
				case codecutil.PCM:
					st = mts.EncodePCM
					encOptions = append(encOptions, mts.TimeBasedPSI(time.Duration(r.cfg.PSITime)*time.Second))
					r.cfg.ClipDuration = 10 * time.Second
					rate = 1 / r.cfg.RecPeriod
				case codecutil.ADPCM:
					st = mts.EncodeADPCM
					encOptions = append(encOptions, mts.TimeBasedPSI(time.Duration(r.cfg.PSITime)*time.Second))
					r.cfg.ClipDuration = 10 * time.Second
				case codecutil.H264, codecutil.H265, codecutil.MJPEG:
					return nil, fmt.Errorf("invalid input codec: %v for input: %v", r.cfg.InputCodec, r.cfg.Input)
				default:
					panic("unknown input codec")
				}
			default:
				panic("unknown input type")
			}
			encOptions = append(encOptions, mts.MediaType(st), mts.Rate(rate))
			return mts.NewEncoder(dst, r.cfg.Logger, encOptions...)
		},
		func(dst io.WriteCloser, fps int) (io.WriteCloser, error) {
			return flv.NewEncoder(dst, true, true, fps)
		},
		ioext.MultiWriteCloser,
	)
	r.cfg.Logger.Info("finished setting pipeline")

	if err != nil {
		return fmt.Errorf("could not set up pipeline: %w", err)
	}

	return nil
}

// setConfig takes a config, checks it's validity and then replaces the current
// revid config.
func (r *Revid) setConfig(config config.Config) error {
	r.cfg.Logger = config.Logger
	r.cfg.Logger.Debug("validating config")
	err := config.Validate()
	if err != nil {
		return errors.New("Config struct is bad: " + err.Error())
	}
	r.cfg.Logger.Info("config validated")
	r.cfg = config
	r.cfg.Logger.SetLevel(r.cfg.LogLevel)
	return nil
}

// setupPipeline constructs the revid dataPipeline. Inputs, encoders and
// senders are created and linked based on the current revid config.
//
// mtsEnc and flvEnc will be called to obtain an mts encoder and flv encoder
// respectively. multiWriter will be used to create an ioext.multiWriteCloser
// so that encoders can write to multiple senders.
func (r *Revid) setupPipeline(mtsEnc func(dst io.WriteCloser, rate float64) (io.WriteCloser, error), flvEnc func(dst io.WriteCloser, rate int) (io.WriteCloser, error), multiWriter func(...io.WriteCloser) io.WriteCloser) error {
	// encoders will hold the encoders that are required for revid's current
	// configuration.
	var encoders []io.WriteCloser

	// mtsSenders will hold the senders the require MPEGTS encoding, and flvSenders
	// will hold senders that require FLV encoding.
	var mtsSenders, flvSenders []io.WriteCloser

	// Calculate no. of pool buffer elements based on starting element size
	// const and config directed max pool buffer size, then create buffer.
	// This is only used if the selected output uses a pool buffer.
	nElements := r.cfg.PoolCapacity / r.cfg.PoolStartElementSize
	writeTimeout := time.Duration(r.cfg.PoolWriteTimeout) * time.Second

	// We will go through our outputs and create the corresponding senders to add
	// to mtsSenders if the output requires MPEGTS encoding, or flvSenders if the
	// output requires FLV encoding.
	var w io.WriteCloser
	rtmpUrlIdx := 0
	for _, out := range r.cfg.Outputs {
		switch out {
		case config.OutputHTTP:
			r.cfg.Logger.Debug("using HTTP output")
			pb := pool.NewBuffer(int(r.cfg.PoolStartElementSize), int(nElements), writeTimeout)
			hs, err := newHTTPSender(r.ns, r.cfg.Logger, withReportCallback(r.bitrate.Report), withHTTPAddress(r.cfg.HTTPAddress))
			if err != nil {
				return fmt.Errorf("coult not create http sender: %w", err)
			}
			w = newMTSSender(hs, r.cfg.Logger, pb, r.cfg.ClipDuration)
			mtsSenders = append(mtsSenders, w)

		case config.OutputRTP:
			r.cfg.Logger.Debug("using RTP output")
			w, err := newRtpSender(r.cfg.RTPAddress, r.cfg.Logger, r.cfg.FrameRate, r.bitrate.Report)
			if err != nil {
				r.cfg.Logger.Warning("rtp connect error", "error", err.Error())
			}
			mtsSenders = append(mtsSenders, w)
		case config.OutputFile:
			r.cfg.Logger.Debug("using File output")
			w, err := newFileSender(r.cfg.Logger, r.cfg.OutputPath, false, r.cfg.MaxFileSize)
			if err != nil {
				return err
			}
			mtsSenders = append(mtsSenders, w)
		case config.OutputFiles:
			r.cfg.Logger.Debug("using Files output")
			pb := pool.NewBuffer(int(r.cfg.PoolStartElementSize), int(nElements), writeTimeout)
			fs, err := newFileSender(r.cfg.Logger, r.cfg.OutputPath, true, r.cfg.MaxFileSize)
			if err != nil {
				return err
			}
			w = newMTSSender(fs, r.cfg.Logger, pb, r.cfg.ClipDuration)
			mtsSenders = append(mtsSenders, w)
		case config.OutputRTMP:
			r.cfg.Logger.Debug("using RTMP output")
			if rtmpUrlIdx > len(r.cfg.RTMPURL)-1 {
				r.cfg.Logger.Warning("rtmp outputs exceed available rtmp urls")
				break
			}
			pb := pool.NewBuffer(int(r.cfg.PoolStartElementSize), int(nElements), writeTimeout)
			w, err := newRtmpSender(r.cfg.RTMPURL[rtmpUrlIdx], rtmpConnectionMaxTries, pb, r.cfg.Logger, r.bitrate.Report)
			if err != nil {
				r.cfg.Logger.Warning("rtmp connect error", "error", err.Error())
			}
			rtmpUrlIdx++
			flvSenders = append(flvSenders, w)
		}
	}

	// If we have some senders that require MPEGTS encoding then add an MPEGTS
	// encoder to revid's encoder slice, and give this encoder the mtsSenders
	// as a destination.
	if len(mtsSenders) != 0 {
		mw := multiWriter(mtsSenders...)
		e, err := mtsEnc(mw, float64(r.cfg.FrameRate))
		if err != nil {
			return fmt.Errorf("error from setting up MTS encoder: %w", err)
		}
		encoders = append(encoders, e)
	}

	// If we have some senders that require FLV encoding then add an FLV
	// encoder to revid's encoder slice, and give this encoder the flvSenders
	// as a destination.
	if len(flvSenders) != 0 {
		mw := multiWriter(flvSenders...)
		e, err := flvEnc(mw, int(r.cfg.FrameRate))
		if err != nil {
			return fmt.Errorf("error from setting up FLV encoder: %w", err)
		}
		encoders = append(encoders, e)
	}

	r.encoders = multiWriter(encoders...)

	l := len(r.cfg.Filters)
	r.filters = []filter.Filter{filter.NewNoOp(r.encoders)}
	if l != 0 {
		r.cfg.Logger.Debug("setting up filters", "filters", r.cfg.Filters)
		r.filters = make([]filter.Filter, l)
		dst := r.encoders

		for i := l - 1; i >= 0; i-- {
			switch r.cfg.Filters[i] {
			case config.FilterNoOp:
				r.cfg.Logger.Debug("using NoOp filter")
				r.filters[i] = filter.NewNoOp(dst)
			case config.FilterMOG:
				r.cfg.Logger.Debug("using MOG filter")
				r.filters[i] = filter.NewMOG(dst, r.cfg)
			case config.FilterVariableFPS:
				r.cfg.Logger.Debug("using Variable FPS MOG filter")
				r.filters[i] = filter.NewVariableFPS(dst, r.cfg.MinFPS, filter.NewMOG(dst, r.cfg))
			case config.FilterKNN:
				r.cfg.Logger.Debug("using KNN filter")
				r.filters[i] = filter.NewKNN(dst, r.cfg)
			case config.FilterDiff:
				r.cfg.Logger.Debug("using gocv difference filter")
				r.filters[i] = filter.NewDiff(dst, r.cfg)
			case config.FilterBasic:
				r.cfg.Logger.Debug("using go difference filter")
				r.filters[i] = filter.NewBasic(dst, r.cfg)
			default:
				panic("unknown filter")
			}
			dst = r.filters[i]
		}
		r.cfg.Logger.Info("filters set up")
	}

	var err error
	switch r.cfg.Input {
	case config.InputRaspivid:
		r.cfg.Logger.Debug("using raspivid input")
		r.input = raspivid.New(r.cfg.Logger)
		err = r.setLexer(r.cfg.InputCodec, false)

	case config.InputRaspistill:
		r.cfg.Logger.Debug("using raspistill input")
		r.input = raspistill.New(r.cfg.Logger)
		r.setLexer(r.cfg.InputCodec, false)

	case config.InputV4L:
		r.cfg.Logger.Debug("using V4L input")
		r.input = webcam.New(r.cfg.Logger)
		err = r.setLexer(r.cfg.InputCodec, false)

	case config.InputFile:
		r.cfg.Logger.Debug("using file input")
		r.input = file.New(r.cfg.Logger)
		err = r.setLexer(r.cfg.InputCodec, false)

	case config.InputRTSP:
		r.cfg.Logger.Debug("using RTSP input")
		r.input = geovision.New(r.cfg.Logger)
		err = r.setLexer(r.cfg.InputCodec, true)

	case config.InputAudio:
		r.cfg.Logger.Debug("using audio input")
		err = r.setupAudio()

	case config.InputManual:
		r.cfg.Logger.Debug("using manual input")
		r.input = device.NewManualInput()
		err = r.setLexer(r.cfg.InputCodec, false)

	default:
		return fmt.Errorf("unrecognised input type: %v", r.cfg.Input)
	}
	if err != nil {
		return fmt.Errorf("could not set lexer: %w", err)
	}

	// Configure the input device. We know that defaults are set, so no need to
	// return error, but we should log.
	r.cfg.Logger.Debug("configuring input device")
	err = r.input.Set(r.cfg)
	if err != nil {
		r.cfg.Logger.Warning("errors from configuring input device", "errors", err)
	}
	r.cfg.Logger.Info("input device configured")

	return nil
}

// setLexer sets the revid input lexer based on input codec and whether input
// is RTSP or not, in which case an RTP/<codec> extractor is used.
func (r *Revid) setLexer(c string, isRTSP bool) error {
	switch c {
	case codecutil.H264:
		r.cfg.Logger.Debug("using H.264 codec")
		r.lexTo = h264.Lex
		if isRTSP {
			r.lexTo = h264.NewExtractor().Extract
		}
	case codecutil.H264_AU:
		r.cfg.Logger.Debug("using H.264 AU codec")
		r.lexTo = codecutil.Noop
	case codecutil.H265:
		r.cfg.Logger.Debug("using H.265 codec")
		r.lexTo = h265.NewExtractor(false).Extract
		if !isRTSP {
			return errors.New("byte stream h.265 lexing not implemented")
		}
	case codecutil.MJPEG, codecutil.JPEG:
		r.cfg.Logger.Debug("using MJPEG/JPEG codec")
		r.lexTo = jpeg.Lex
		jpeg.Log = r.cfg.Logger
		if isRTSP {
			r.lexTo = jpeg.NewExtractor().Extract
		}

	case codecutil.PCM, codecutil.ADPCM:
		return errors.New("invalid codec for this selected input")
	default:
		panic("unrecognised codec")
	}
	return nil
}

// processFrom is run as a routine to read from a input data source, lex and
// then send individual access units to revid's encoders.
func (r *Revid) processFrom(delay time.Duration) {
	defer r.wg.Done()

	if r.input != nil {
		err := r.input.Start()
		if err != nil {
			r.err <- fmt.Errorf("could not start input device: %w", err)
			return
		}
	}

	// Lex data from input device, in, until finished or an error is encountered.
	// For a continuous source e.g. a camera or microphone, we should remain
	// in this call indefinitely unless in.Stop() is called and an io.EOF is forced.
	r.cfg.Logger.Debug("lexing")
	var w io.Writer
	w = r.filters[0]
	if r.probe != nil {
		w = ioext.MultiWriteCloser(r.filters[0], r.probe)
	}

	err := r.lexTo(w, r.input, delay)
	switch err {
	case nil, io.EOF:
		r.cfg.Logger.Info("end of file")
	case io.ErrUnexpectedEOF:
		r.cfg.Logger.Info("unexpected EOF from input")
	default:
		r.err <- err
	}
	r.cfg.Logger.Info("finished reading input")

	r.cfg.Logger.Debug("stopping input")
	err = r.input.Stop()
	if err != nil {
		r.err <- fmt.Errorf("could not stop input source: %w", err)
	} else {
		r.cfg.Logger.Info("input stopped")
	}
}

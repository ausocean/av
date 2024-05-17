/*
DESCRIPTION
  geovision.go provides an implementation of the AVDevice interface for the
  GeoVision IP camera.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package geovision provides an implementation of the AVDevice interface
// for the GeoVision IP camera.
package geovision

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/av/device"
	gvconfig "github.com/ausocean/av/device/geovision/config"
	"github.com/ausocean/av/protocol/rtcp"
	"github.com/ausocean/av/protocol/rtp"
	"github.com/ausocean/av/protocol/rtsp"
	avconfig "github.com/ausocean/av/revid/config"
	"github.com/ausocean/utils/logging"
	"github.com/ausocean/utils/sliceutils"
)

// Indicate package when logging.
const pkg = "geovision: "

// Constants for real time clients.
const (
	rtpPort               = 60000
	rtcpPort              = 60001
	defaultServerRTCPPort = 17301
)

// TODO: remove this when config has configurable user and pass.
const (
	ipCamUser = "admin"
	ipCamPass = "admin"
)

// Configuration defaults.
const (
	defaultCameraIP   = "192.168.1.50"
	defaultCodec      = codecutil.H264
	defaultHeight     = 720
	defaultFrameRate  = 25
	defaultBitrate    = 400
	defaultVBRBitrate = 400
	defaultMinFrames  = 3
	defaultVBRQuality = avconfig.QualityStandard
	defaultCameraChan = 2
)

// Configuration field errors.
var (
	errGVBadCameraIP   = errors.New("camera IP bad or unset, defaulting")
	errGVBadCodec      = errors.New("codec bad or unset, defaulting")
	errGVBadFrameRate  = errors.New("frame rate bad or unset, defaulting")
	errGVBadBitrate    = errors.New("bitrate bad or unset, defaulting")
	errGVBadVBRQuality = errors.New("VBR quality bad or unset, defaulting")
	errGVBadHeight     = errors.New("height bad or unset, defaulting")
	errGVBadMinFrames  = errors.New("min frames bad or unset, defaulting")
)

// GeoVision is an implementation of the AVDevice interface for a GeoVision
// IP camera. This has been designed to implement the GV-BX4700-8F in particular.
// Any other models are untested.
type GeoVision struct {
	cfg       avconfig.Config
	log       logging.Logger
	rtpClt    *rtp.Client
	rtspClt   *rtsp.Client
	rtcpClt   *rtcp.Client
	isRunning bool
}

// NewGeoVision returns a new GeoVision.
func New(l logging.Logger) *GeoVision { return &GeoVision{log: l} }

// Name returns the name of the device.
func (g *GeoVision) Name() string {
	return "GeoVision"
}

// Set will take a Config struct, check the validity of the relevant fields
// and then performs any configuration necessary using config to control the
// GeoVision web interface. If fields are not valid, an error is added to the
// multiError and a default value is used for that particular field.
func (g *GeoVision) Set(c avconfig.Config) error {
	var errs device.MultiError
	if c.CameraIP == "" {
		errs = append(errs, errGVBadCameraIP)
		c.CameraIP = defaultCameraIP
	}

	switch c.InputCodec {
	case codecutil.H264, codecutil.H265, codecutil.MJPEG:
	default:
		errs = append(errs, errGVBadCodec)
		c.InputCodec = defaultCodec
	}

	if c.Height <= 0 {
		errs = append(errs, errGVBadHeight)
		c.Height = defaultHeight
	}

	if c.FrameRate <= 0 {
		errs = append(errs, errGVBadFrameRate)
		c.FrameRate = defaultFrameRate
	}

	if c.Bitrate <= 0 {
		errs = append(errs, errGVBadBitrate)
		c.Bitrate = defaultBitrate
	}

	refresh := float64(c.MinFrames) / float64(c.FrameRate)
	if refresh < 1 || refresh > 5 {
		errs = append(errs, errGVBadMinFrames)
		c.MinFrames = 4 * c.FrameRate
	}

	// If we're using RTMP then we should default to constant bitrate.
	if sliceutils.ContainsUint8(c.Outputs, avconfig.OutputRTMP) {
		c.CBR = true
	}

	switch c.VBRQuality {
	case avconfig.QualityStandard, avconfig.QualityFair, avconfig.QualityGood, avconfig.QualityGreat, avconfig.QualityExcellent:
	default:
		errs = append(errs, errGVBadVBRQuality)
		c.VBRQuality = defaultVBRQuality
	}

	if c.VBRBitrate <= 0 {
		errs = append(errs, errGVBadVBRQuality)
		c.VBRBitrate = defaultVBRBitrate
	}

	if c.CameraChan != 1 && c.CameraChan != 2 {
		errs = append(errs, errGVBadVBRQuality)
		c.CameraChan = defaultCameraChan
	}

	g.cfg = c

	err := gvconfig.Set(
		g.cfg.CameraIP,
		gvconfig.Channel(g.cfg.CameraChan),
		gvconfig.CodecOut(
			map[string]gvconfig.Codec{
				codecutil.H264:  gvconfig.CodecH264,
				codecutil.H265:  gvconfig.CodecH265,
				codecutil.MJPEG: gvconfig.CodecMJPEG,
			}[g.cfg.InputCodec],
		),
		gvconfig.Height(g.cfg.Height),
		gvconfig.FrameRate(g.cfg.FrameRate),
		gvconfig.VariableBitrate(!g.cfg.CBR),
		gvconfig.VBRQuality(
			map[avconfig.Quality]gvconfig.Quality{
				avconfig.QualityStandard:  gvconfig.QualityStandard,
				avconfig.QualityFair:      gvconfig.QualityFair,
				avconfig.QualityGood:      gvconfig.QualityGood,
				avconfig.QualityGreat:     gvconfig.QualityGreat,
				avconfig.QualityExcellent: gvconfig.QualityExcellent,
			}[g.cfg.VBRQuality],
		),
		gvconfig.VBRBitrate(g.cfg.VBRBitrate),
		gvconfig.CBRBitrate(g.cfg.Bitrate),
		gvconfig.Refresh(float64(g.cfg.MinFrames)/float64(g.cfg.FrameRate)),
	)
	if err != nil {
		return fmt.Errorf("could not set IPCamera settings: %w", err)
	}

	// Give the camera some time to change it's configuration.
	const setupDelay = 5 * time.Second
	time.Sleep(setupDelay)

	if len(errs) != 0 {
		return errs
	}
	return nil
}

// Start uses an RTSP client to communicate with the GeoVision RTSP server and
// request a stream that is then received by an RTP client, from which packets
// can be read from using the Read method.
func (g *GeoVision) Start() error {
	var (
		local, remote *net.TCPAddr
		err           error
	)

	g.rtspClt, local, remote, err = rtsp.NewClient("rtsp://" + ipCamUser + ":" + ipCamPass + "@" + g.cfg.CameraIP + ":8554/" + "CH002.sdp")
	if err != nil {
		return fmt.Errorf("could not create RTSP client: %w", err)
	}

	g.log.Info(pkg + "created RTSP client")

	resp, err := g.rtspClt.Options()
	if err != nil {
		return fmt.Errorf("options request unsuccessful: %w", err)
	}
	g.log.Debug(pkg+"RTSP OPTIONS response", "response", resp.String())

	resp, err = g.rtspClt.Describe()
	if err != nil {
		return fmt.Errorf("describe request unsuccessful: %w", err)
	}
	g.log.Debug(pkg+"RTSP DESCRIBE response", "response", resp.String())

	resp, err = g.rtspClt.Setup("track1", fmt.Sprintf("RTP/AVP;unicast;client_port=%d-%d", rtpPort, rtcpPort))
	if err != nil {
		return fmt.Errorf("setup request unsuccessful: %w", err)
	}
	g.log.Debug(pkg+"RTSP SETUP response", "response", resp.String())

	rtpCltAddr, rtcpCltAddr, rtcpSvrAddr, err := formAddrs(local, remote, *resp)
	if err != nil {
		return fmt.Errorf("could not format addresses: %w", err)
	}

	g.log.Info(pkg + "RTSP session setup complete")

	g.rtpClt, err = rtp.NewClient(rtpCltAddr)
	if err != nil {
		return fmt.Errorf("could not create RTP client: %w", err)
	}

	g.rtcpClt, err = rtcp.NewClient(rtcpCltAddr, rtcpSvrAddr, g.rtpClt, g.log.Log)
	if err != nil {
		return fmt.Errorf("could not create RTCP client: %w", err)
	}

	g.log.Info(pkg + "RTCP and RTP clients created")

	// Check errors from RTCP client until it has stopped running.
	go func() {
		for {
			err, ok := <-g.rtcpClt.Err()
			if ok {
				g.log.Warning(pkg+"RTCP error", "error", err.Error())
			} else {
				return
			}
		}
	}()

	// Start the RTCP client.
	g.rtcpClt.Start()
	g.log.Info(pkg + "RTCP client started")

	resp, err = g.rtspClt.Play()
	if err != nil {
		return fmt.Errorf("play request unsuccessful: %w", err)
	}
	g.log.Debug(pkg+"RTSP server PLAY response", "response", resp.String())
	g.log.Info(pkg + "play requested, now receiving stream")
	g.isRunning = true

	return nil
}

// Stop will close the RTSP, RTCP, and RTP connections and in turn end the
// stream from the GeoVision. Future reads using Read will result in error.
func (g *GeoVision) Stop() error {
	err := g.rtpClt.Close()
	if err != nil {
		return fmt.Errorf("could not close RTP client: %w", err)
	}

	err = g.rtspClt.Close()
	if err != nil {
		return fmt.Errorf("could not close RTSP client: %w", err)
	}

	g.rtcpClt.Stop()

	g.log.Info(pkg + "RTP, RTSP and RTCP clients stopped and closed")

	g.isRunning = false

	return nil
}

// Read implements io.Reader. If the GeoVision has not been started an error is
// returned.
func (g *GeoVision) Read(p []byte) (int, error) {
	if g.rtpClt != nil {
		return g.rtpClt.Read(p)
	}
	return 0, errors.New("cannot read, GeoVision not streaming")
}

// formAddrs is a helper function to form the addresses for the RTP client,
// RTCP client, and the RTSP server's RTCP addr using the local, remote addresses
// of the RTSP conn, and the SETUP method response.
func formAddrs(local, remote *net.TCPAddr, setupResp rtsp.Response) (rtpCltAddr, rtcpCltAddr, rtcpSvrAddr string, err error) {
	svrRTCPPort, err := parseSvrRTCPPort(setupResp)
	if err != nil {
		return "", "", "", err
	}
	rtpCltAddr = strings.Split(local.String(), ":")[0] + ":" + strconv.Itoa(rtpPort)
	rtcpCltAddr = strings.Split(local.String(), ":")[0] + ":" + strconv.Itoa(rtcpPort)
	rtcpSvrAddr = strings.Split(remote.String(), ":")[0] + ":" + strconv.Itoa(svrRTCPPort)
	return
}

// parseServerRTCPPort is a helper function to get the RTSP server's RTCP port.
func parseSvrRTCPPort(resp rtsp.Response) (int, error) {
	transport := resp.Header.Get("Transport")
	for _, p := range strings.Split(transport, ";") {
		if strings.Contains(p, "server_port") {
			port, err := strconv.Atoi(strings.Split(p, "-")[1])
			if err != nil {
				return 0, err
			}
			return port, nil
		}
	}
	return 0, errors.New("SETUP response did not provide RTCP port")
}

// IsRunning is used to determine if the geovision is running.
func (g *GeoVision) IsRunning() bool {
	return g.isRunning
}

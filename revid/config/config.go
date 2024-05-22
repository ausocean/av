/*
NAME
  Config.go

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


// Package config contains the configuration settings for revid.
package config

import (
	"time"

	"github.com/ausocean/utils/logging"
)

// Enums to define inputs, outputs and codecs.
const (
	// Indicates no option has been set.
	NothingDefined = iota

	// Input/Output.
	InputFile
	InputRaspivid
	InputRaspistill
	InputV4L
	InputRTSP
	InputAudio
	InputManual

	// Outputs.
	OutputRTMP
	OutputRTP
	OutputHTTP
	OutputMPEGTS
	OutputFile
	OutputFiles

	// Codecs.
	H264    // h264 bytestream.
	H264_AU // Discrete h264 access units.
	H265
	MJPEG
	JPEG
)

// Quality represents video quality.
type Quality int

// The different video qualities that can be used for variable bitrate when
// using the GeoVision camera.
const (
	QualityStandard Quality = iota
	QualityFair
	QualityGood
	QualityGreat
	QualityExcellent
)

// The different media filters.
const (
	FilterNoOp = iota
	FilterMOG
	FilterVariableFPS
	FilterKNN
	FilterDiff
	FilterBasic
)

// Config provides parameters relevant to a revid instance. A new config must
// be passed to the constructor. Default values for these fields are defined
// as consts above.
type Config struct {
	// AutoWhiteBalance defines the auto white balance mode used by Raspivid input.
	// Valid modes are defined in the exported []string AutoWhiteBalanceModes
	// defined at the start of the file.
	AutoWhiteBalance string

	// AWBGains sets the blue and red channel gains of an image/video capture device.
	AWBGains string

	BitDepth    uint // Sample bit depth.
	Bitrate     uint // Bitrate specifies the bitrate for constant bitrate in kbps.
	Brightness  uint
	BurstPeriod uint  // BurstPeriod defines the revid burst period in seconds.
	CameraChan  uint8 // This is the channel we're using for the GeoVision camera.

	// CameraIP is the IP address of the camera in the case of the input camera
	// being an IP camera.
	CameraIP string

	// CBR indicates whether we wish to use constant or variable bitrate. If CBR
	// is true then we will use constant bitrate, and variable bitrate otherwise.
	// In the case of the Pi camera, variable bitrate quality is controlled by
	// the Quantization parameter below. In the case of the GeoVision camera,
	// variable bitrate quality is controlled by firstly the VBRQuality parameter
	// and second the VBRBitrate parameter.
	CBR bool

	Channels uint // Number of audio channels, 1 for mono, 2 for stereo.

	// ClipDuration is the duration of MTS data that is sent using HTTP or RTP
	// output. This defaults to 0, therefore MinFrames will determine the length of
	// clips by default.
	ClipDuration time.Duration

	// Contrast is the contrast of captured video/images from a capture device.
	Contrast int

	// Exposure defines the exposure mode used by the Raspivid input. Valid modes
	// are defined in the exported []string ExposureModes defined at the start
	// of the file.
	Exposure string

	// EV is the exposure value for image/video capture devices.
	EV int

	FileFPS uint   // Defines the rate at which frames from a file source are processed.
	Filters []uint // Defines the methods of filtering to be used in between lexing and encoding.

	// FrameRate defines the input frame rate if configurable by the chosen input.
	// Raspivid input supports custom framerate.
	FrameRate uint

	Height         uint // Height defines the input video height Raspivid input.
	HorizontalFlip bool // HorizontalFlip flips video horizontally for Raspivid input.

	// HTTPAddress defines a custom HTTP destination if we do not wish to use that
	// defined in /etc/netsender.conf.
	HTTPAddress string

	// Input defines the input data source.
	//
	// Valid values are defined by enums:
	// InputRaspivid:
	//		Use raspivid utility to capture video from Raspberry Pi Camera.
	// InputRaspistill:
	//		Use raspistill utility to capture images from the Raspberry Pi Camera.
	// InputV4l:
	//		Read from webcam.
	// InputFile:
	//		Read h.264 bytestream from a file.
	// 		Location must be specified in InputPath field.
	// InputRTSP:
	//		Read from a camera supporting RTSP communication.
	//		CameraIP should also be defined.
	// InputAudio:
	//		Read from a ALSA audio source.
	Input uint8

	// InputCodec defines the input codec we wish to use, and therefore defines the
	// lexer for use in the pipeline. This defaults to H264, but H265 is also a
	// valid option if we expect this from the input.
	InputCodec string

	// InputPath defines the input file location for File Input. This must be
	// defined if File input is to be used.
	InputPath string

	// ISO sets the image/video capture device's sensitivity to light.
	ISO uint

	// Logger holds an implementation of the Logger interface as defined in revid.go.
	// This must be set for revid to work correctly.
	Logger logging.Logger

	// LogLevel is the revid logging verbosity level.
	// Valid values are defined by enums from the logger package: logging.Debug,
	// logging.Info, logging.Warning logging.Error, logging.Fatal.
	LogLevel int8

	Loop        bool // If true will restart reading of input after an io.EOF.
	MaxFileSize uint // Maximum size in bytes that a file will be written when File output is to be used. A value of 0 means unlimited.
	MinFPS      uint // The reduced framerate of the video when there is no motion.

	// MinFrames defines the frequency of key NAL units SPS, PPS and IDR in
	// number of NAL units. This will also determine the frequency of PSI if the
	// output container is MPEG-TS. If ClipDuration is less than MinFrames,
	// ClipDuration will default to MinFrames.
	MinFrames uint

	MotionDownscaling uint    // Downscaling factor of frames used for motion detection.
	MotionHistory     uint    // Length of filter's history (KNN & MOG only).
	MotionInterval    uint    // Sets the number of frames that are held before the filter is used (on the nth frame).
	MotionKernel      uint    // Size of kernel used for filling holes and removing noise (KNN only).
	MotionMinArea     float64 // Used to ignore small areas of motion detection (KNN & MOG only).
	MotionPadding     uint    // Number of frames to keep before and after motion detected.
	MotionPixels      uint    // Number of pixels with motion that is needed for a whole frame to be considered as moving (Basic only).
	MotionThreshold   float64 // Intensity value that is considered motion.

	// OutputPath defines the output destination for File output. This must be
	// defined if File output is to be used.
	OutputPath string

	// Outputs define the outputs we wish to output data too.
	//
	// Valid outputs are defined by enums:
	// OutputFile & OutputFiles:
	// 		Location must be defined by the OutputPath field. MPEG-TS packetization
	//		is used.
	// OutputHTTP:
	// 		Destination is defined by the sh field located in /etc/netsender.conf.
	// 		MPEGT-TS packetization is used.
	// OutputRTMP:
	// 		Destination URL must be defined in the RtmpUrl field. FLV packetization
	//		is used.
	// OutputRTP:
	// 		Destination is defined by RtpAddr field, otherwise it will default to
	//		localhost:6970. MPEGT-TS packetization is used.
	Outputs []uint8

	PSITime              uint     // Sets the time between a packet being sent.
	Quantization         uint     // Quantization defines the quantization level, which will determine variable bitrate quality in the case of input from the Pi Camera.
	PoolCapacity         uint     // The number of bytes the pool buffer will occupy.
	PoolStartElementSize uint     // The starting element size of the pool buffer from which element size will increase to accomodate frames.
	PoolWriteTimeout     uint     // The pool buffer write timeout in seconds.
	RecPeriod            float64  // How many seconds to record at a time.
	Rotation             uint     // Rotation defines the video rotation angle in degrees Raspivid input.
	RTMPURL              []string // RTMPURL specifies the RTMP output destination URLs. This must be defined if RTMP is to be used as an output.
	RTPAddress           string   // RTPAddress defines the RTP output destination.
	SampleRate           uint     // Samples a second (Hz).
	Saturation           int

	// Sharpness is the sharpness of capture image/video from a capture device.
	Sharpness int

	// JPEGQuality is a value 0-100 inclusive, controlling JPEG compression of the
	// timelapse snaps. 100 represents minimal compression and 0 represents the most
	// compression.
	JPEGQuality int

	Suppress bool // Holds logger suppression state.

	// TimelapseInterval defines the interval between timelapse images when using
	// raspistill input.
	TimelapseInterval time.Duration

	// TimelapseDuration defines the duration of timelapse i.e. duration over
	// which all snaps are taken, when using raspistill input.
	TimelapseDuration time.Duration

	VBRBitrate uint // VBRBitrate describes maximal variable bitrate.

	// VBRQuality describes the general quality of video from the GeoVision camera
	// under variable bitrate. VBRQuality can be one 5 consts defined:
	// qualityStandard, qualityFair, qualityGood, qualityGreat and qualityExcellent.
	VBRQuality Quality

	VerticalFlip bool // VerticalFlip flips video vertically for Raspivid input.
	Width        uint // Width defines the input video width Raspivid input.

	// TransformMatrix describes the projective transformation matrix to extract a target from the
	// video data for turbidty calculations.
	TransformMatrix []float64
}

// Validate checks for any errors in the config fields and defaults settings
// if particular parameters have not been defined.
func (c *Config) Validate() error {
	for _, v := range Variables {
		if v.Validate != nil {
			v.Validate(c)
		}
	}
	return nil
}

// Update takes a map of configuration variable names and their corresponding
// values, parses the string values and converting into correct type, and then
// sets the config struct fields as appropriate.
func (c *Config) Update(vars map[string]string) {
	for _, value := range Variables {
		if v, ok := vars[value.Name]; ok && value.Update != nil {
			value.Update(c, v)
		}
	}
}

func (c *Config) LogInvalidField(name string, def interface{}) {
	c.Logger.Info(name+" bad or unset, defaulting", name, def)
}

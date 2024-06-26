/*
DESCRIPTION
  variables.go contains a list of structs that provide a variable Name, type in
  a string format, a function for updating the variable in the Config struct
  from a string, and finally, a validation function to check the validity of the
  corresponding field value in the Config.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ausocean/av/codec/codecutil"
	"github.com/ausocean/utils/logging"
)

// Config map Keys.
const (
	KeyAutoWhiteBalance     = "AutoWhiteBalance"
	KeyAWBGains             = "AWBGains"
	KeyBitDepth             = "BitDepth"
	KeyBitrate              = "Bitrate"
	KeyBrightness           = "Brightness"
	KeyBurstPeriod          = "BurstPeriod"
	KeyCameraChan           = "CameraChan"
	KeyCameraIP             = "CameraIP"
	KeyCBR                  = "CBR"
	KeyClipDuration         = "ClipDuration"
	KeyChannels             = "Channels"
	KeyContrast             = "Contrast"
	KeyExposure             = "Exposure"
	KeyEV                   = "EV"
	KeyFileFPS              = "FileFPS"
	KeyFilters              = "Filters"
	KeyFrameRate            = "FrameRate"
	KeyHeight               = "Height"
	KeyHorizontalFlip       = "HorizontalFlip"
	KeyHTTPAddress          = "HTTPAddress"
	KeyInput                = "Input"
	KeyInputCodec           = "InputCodec"
	KeyInputPath            = "InputPath"
	KeyISO                  = "ISO"
	KeyLogging              = "logging"
	KeyLoop                 = "Loop"
	KeyMaxFileSize          = "MaxFileSize"
	KeyMinFPS               = "MinFPS"
	KeyMinFrames            = "MinFrames"
	KeyMode                 = "mode"
	KeyMotionDownscaling    = "MotionDownscaling"
	KeyMotionHistory        = "MotionHistory"
	KeyMotionInterval       = "MotionInterval"
	KeyMotionKernel         = "MotionKernel"
	KeyMotionMinArea        = "MotionMinArea"
	KeyMotionPadding        = "MotionPadding"
	KeyMotionPixels         = "MotionPixels"
	KeyMotionThreshold      = "MotionThreshold"
	KeyOutput               = "Output"
	KeyOutputPath           = "OutputPath"
	KeyOutputs              = "Outputs"
	KeyPSITime              = "PSITime"
	KeyQuantization         = "Quantization"
	KeyPoolCapacity         = "PoolCapacity"
	KeyPoolStartElementSize = "PoolStartElementSize"
	KeyPoolWriteTimeout     = "PoolWriteTimeout"
	KeyRecPeriod            = "RecPeriod"
	KeyRotation             = "Rotation"
	KeyRTMPURL              = "RTMPURL"
	KeyRTPAddress           = "RTPAddress"
	KeySampleRate           = "SampleRate"
	KeySaturation           = "Saturation"
	KeySharpness            = "Sharpness"
	KeyJPEGQuality          = "JPEGQuality"
	KeySuppress             = "Suppress"
	KeyTimelapseDuration    = "TimelapseDuration"
	KeyTimelapseInterval    = "TimelapseInterval"
	KeyVBRBitrate           = "VBRBitrate"
	KeyVBRQuality           = "VBRQuality"
	KeyVerticalFlip         = "VerticalFlip"
	KeyWidth                = "Width"
	KeyTransformMatrix      = "TransformMatrix"
)

// Config map parameter types.
const (
	typeString = "string"
	typeInt    = "int"
	typeUint   = "uint"
	typeBool   = "bool"
	typeFloat  = "float"
)

// Default variable values.
const (
	// General revid defaults.
	defaultInput        = InputRaspivid
	defaultOutput       = OutputHTTP
	defaultInputCodec   = codecutil.H264
	defaultVerbosity    = logging.Error
	defaultRTPAddr      = "localhost:6970"
	defaultCameraIP     = "192.168.1.50"
	defaultBurstPeriod  = 10 // Seconds
	defaultMinFrames    = 100
	defaultFrameRate    = 25
	defaultClipDuration = 0
	defaultPSITime      = 2
	defaultFileFPS      = 0

	// Ring buffer defaults.
	defaultPoolCapacity         = 50000000 // => 50MB
	defaultPoolStartElementSize = 1000     // bytes
	defaultPoolWriteTimeout     = 5        // Seconds.

	// Motion filter parameter defaults.
	defaultMinFPS = 1.0
)

// Variables describes the variables that can be used for revid control.
// These structs provide the name and type of variable, a function for updating
// this variable in a Config, and a function for validating the value of the variable.
var Variables = []struct {
	Name     string
	Type     string
	Update   func(*Config, string)
	Validate func(*Config)
}{
	{
		Name: KeyTransformMatrix,
		Type: typeString,
		Update: func(c *Config, v string) {
			c.Logger.Debug("updating transform matrix", "string", v)
			v = strings.Replace(v, " ", "", -1)
			vals := make([]float64, 0)
			if v == "" {
				c.TransformMatrix = vals
				return
			}

			elements := strings.Split(v, ",")
			for _, e := range elements {
				vFloat, err := strconv.ParseFloat(e, 64)
				if err != nil {
					c.Logger.Warning("invalid TransformMatrix param", "value", e)
				}
				vals = append(vals, vFloat)
			}
			c.TransformMatrix = vals
		},
	},
	{
		Name:   KeyAutoWhiteBalance,
		Type:   "enum:off,auto,sun,cloud,shade,tungsten,fluorescent,incandescent,flash,horizon",
		Update: func(c *Config, v string) { c.AutoWhiteBalance = v },
	},
	{
		Name:   KeyAWBGains,
		Type:   typeString,
		Update: func(c *Config, v string) { c.AWBGains = v },
	},
	{
		Name:   KeyBitDepth,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.BitDepth = parseUint(KeyBitDepth, v, c) },
	},
	{
		Name:   KeyBitrate,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.Bitrate = parseUint(KeyBitrate, v, c) },
	},
	{
		Name:   KeyBrightness,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.Brightness = parseUint(KeyBrightness, v, c) },
	},
	{
		Name:   KeyBurstPeriod,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.BurstPeriod = parseUint(KeyBurstPeriod, v, c) },
		Validate: func(c *Config) {
			if c.BurstPeriod <= 0 {
				c.LogInvalidField(KeyBurstPeriod, defaultBurstPeriod)
				c.BurstPeriod = defaultBurstPeriod
			}
		},
	},
	{
		Name:   KeyCameraChan,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.CameraChan = uint8(parseUint(KeyCameraChan, v, c)) },
	},
	{
		Name:   KeyCameraIP,
		Type:   typeString,
		Update: func(c *Config, v string) { c.CameraIP = v },
		Validate: func(c *Config) {
			if c.CameraIP == "" {
				c.LogInvalidField(KeyCameraIP, defaultCameraIP)
				c.CameraIP = defaultCameraIP
			}
		},
	},
	{
		Name:   KeyCBR,
		Type:   typeBool,
		Update: func(c *Config, v string) { c.CBR = parseBool(KeyCBR, v, c) },
	},
	{
		Name: KeyClipDuration,
		Type: typeUint,
		Update: func(c *Config, v string) {
			_v, err := strconv.Atoi(v)
			if err != nil {
				c.Logger.Warning("invalid ClipDuration param", "value", v)
			}
			c.ClipDuration = time.Duration(_v) * time.Second
		},
		Validate: func(c *Config) {
			if c.ClipDuration <= 0 {
				c.LogInvalidField(KeyClipDuration, defaultClipDuration)
				c.ClipDuration = defaultClipDuration
			}
		},
	},
	{
		Name:   KeyChannels,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.Channels = parseUint(KeyChannels, v, c) },
	},
	{
		Name:   KeyContrast,
		Type:   typeInt,
		Update: func(c *Config, v string) { c.Contrast = parseInt(KeyContrast, v, c) },
	},
	{
		Name:   KeyEV,
		Type:   typeInt,
		Update: func(c *Config, v string) { c.EV = parseInt(KeyEV, v, c) },
	},
	{
		Name:   KeyExposure,
		Type:   "enum:auto,night,nightpreview,backlight,spotlight,sports,snow,beach,verylong,fixedfps,antishake,fireworks",
		Update: func(c *Config, v string) { c.Exposure = v },
	},
	{
		Name:   KeyFileFPS,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.FileFPS = parseUint(KeyFileFPS, v, c) },
		Validate: func(c *Config) {
			if c.FileFPS <= 0 || (c.FileFPS > 0 && c.Input != InputFile) {
				c.LogInvalidField(KeyFileFPS, defaultFileFPS)
				c.FileFPS = defaultFileFPS
			}
		},
	},
	{
		Name: KeyFilters,
		Type: "enums:NoOp,MOG,VariableFPS,KNN,Difference,Basic",
		Update: func(c *Config, v string) {
			filters := strings.Split(v, ",")
			m := map[string]uint{"NoOp": FilterNoOp, "MOG": FilterMOG, "VariableFPS": FilterVariableFPS, "KNN": FilterKNN, "Difference": FilterDiff, "Basic": FilterBasic}
			c.Filters = make([]uint, len(filters))
			for i, filter := range filters {
				v, ok := m[filter]
				if !ok {
					c.Logger.Warning("invalid Filters param", "value", v)
				}
				c.Filters[i] = uint(v)
			}
		},
	},
	{
		Name:   KeyFrameRate,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.FrameRate = parseUint(KeyFrameRate, v, c) },
		Validate: func(c *Config) {
			if c.FrameRate <= 0 || c.FrameRate > 60 {
				c.LogInvalidField(KeyFrameRate, defaultFrameRate)
				c.FrameRate = defaultFrameRate
			}
		},
	},
	{
		Name:   KeyHeight,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.Height = parseUint(KeyHeight, v, c) },
	},
	{
		Name:   KeyHorizontalFlip,
		Type:   typeBool,
		Update: func(c *Config, v string) { c.HorizontalFlip = parseBool(KeyHorizontalFlip, v, c) },
	},
	{
		Name:   KeyHTTPAddress,
		Type:   typeString,
		Update: func(c *Config, v string) { c.HTTPAddress = v },
	},
	{
		Name: KeyInput,
		Type: "enum:raspivid,raspistill,rtsp,v4l,file,audio,manual",
		Update: func(c *Config, v string) {
			c.Input = parseEnum(
				KeyInput,
				v,
				map[string]uint8{
					"raspivid":   InputRaspivid,
					"raspistill": InputRaspistill,
					"rtsp":       InputRTSP,
					"v4l":        InputV4L,
					"file":       InputFile,
					"audio":      InputAudio,
					"manual":     InputManual,
				},
				c,
			)
		},
		Validate: func(c *Config) {
			switch c.Input {
			case InputRaspivid, InputRaspistill, InputV4L, InputFile, InputAudio, InputRTSP, InputManual:
			default:
				c.LogInvalidField(KeyInput, defaultInput)
				c.Input = defaultInput
			}
		},
	},
	{
		Name: KeyInputCodec,
		Type: "enum:h264,h264_au,h265,mjpeg,jpeg,pcm,adpcm",
		Update: func(c *Config, v string) {
			c.InputCodec = v
		},
		Validate: func(c *Config) {
			if !codecutil.IsValid(c.InputCodec) {
				c.LogInvalidField(KeyInputCodec, defaultInputCodec)
				c.InputCodec = defaultInputCodec
			}
		},
	},
	{
		Name:   KeyInputPath,
		Type:   typeString,
		Update: func(c *Config, v string) { c.InputPath = v },
	},
	{
		Name:   KeyISO,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.ISO = parseUint(KeyISO, v, c) },
	},
	{
		Name: KeyLogging,
		Type: "enum:Debug,Info,Warning,Error,Fatal",
		Update: func(c *Config, v string) {
			switch v {
			case "Debug":
				c.LogLevel = logging.Debug
			case "Info":
				c.LogLevel = logging.Info
			case "Warning":
				c.LogLevel = logging.Warning
			case "Error":
				c.LogLevel = logging.Error
			case "Fatal":
				c.LogLevel = logging.Fatal
			default:
				c.Logger.Warning("invalid Logging param", "value", v)
			}
		},
		Validate: func(c *Config) {
			switch c.LogLevel {
			case logging.Debug, logging.Info, logging.Warning, logging.Error, logging.Fatal:
			default:
				c.LogInvalidField("LogLevel", defaultVerbosity)
				c.LogLevel = defaultVerbosity
			}
		},
	},
	{
		Name:   KeyLoop,
		Type:   typeBool,
		Update: func(c *Config, v string) { c.Loop = parseBool(KeyLoop, v, c) },
	},
	{
		Name:   KeyMaxFileSize,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.MaxFileSize = parseUint(KeyMaxFileSize, v, c) },
	},
	{
		Name:     KeyMinFPS,
		Type:     typeUint,
		Update:   func(c *Config, v string) { c.MinFPS = parseUint(KeyMinFPS, v, c) },
		Validate: func(c *Config) { c.MinFPS = lessThanOrEqual(KeyMinFPS, c.MinFPS, 0, c, defaultMinFPS) },
	},
	{
		Name:   KeyMinFrames,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.MinFrames = parseUint(KeyMinFrames, v, c) },
		Validate: func(c *Config) {
			const maxMinFrames = 1000
			if c.MinFrames <= 0 || c.MinFrames > maxMinFrames {
				c.LogInvalidField(KeyMinFrames, defaultMinFrames)
				c.MinFrames = defaultMinFrames
			}
		},
	},
	{
		Name:   KeyMode,
		Type:   "enum:Normal,Paused,Burst,Shutdown,Completed",
		Update: func(c *Config, v string) {},
	},
	{
		Name:   KeyMotionDownscaling,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.MotionDownscaling = parseUint(KeyMotionDownscaling, v, c) },
	},
	{
		Name:   KeyMotionHistory,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.MotionHistory = parseUint(KeyMotionHistory, v, c) },
	},
	{
		Name:   KeyMotionInterval,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.MotionInterval = parseUint(KeyMotionInterval, v, c) },
	},
	{
		Name:   KeyMotionKernel,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.MotionKernel = parseUint(KeyMotionKernel, v, c) },
	},
	{
		Name: KeyMotionMinArea,
		Type: typeFloat,
		Update: func(c *Config, v string) {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				c.Logger.Warning("invalid MotionMinArea var", "value", v)
			}
			c.MotionMinArea = f
		},
	},
	{
		Name:   KeyMotionPadding,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.MotionPadding = parseUint(KeyMotionPadding, v, c) },
	},
	{
		Name:   KeyMotionPixels,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.MotionPixels = parseUint(KeyMotionPixels, v, c) },
	},
	{
		Name: KeyMotionThreshold,
		Type: typeFloat,
		Update: func(c *Config, v string) {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				c.Logger.Warning("invalid MotionThreshold var", "value", v)
			}
			c.MotionThreshold = f
		},
	},
	{
		Name: KeyOutput,
		Type: "enum:File,HTTP,RTMP,RTP",
		Update: func(c *Config, v string) {
			c.Outputs = make([]uint8, 1)
			switch strings.ToLower(v) {
			case "file":
				c.Outputs[0] = OutputFile
			case "files":
				c.Outputs[0] = OutputFiles
			case "http":
				c.Outputs[0] = OutputHTTP
			case "rtmp":
				c.Outputs[0] = OutputRTMP
			case "rtp":
				c.Outputs[0] = OutputRTP
			default:
				c.Logger.Warning("invalid output param", "value", v)
			}
		},
	},
	{
		Name:   KeyOutputPath,
		Type:   typeString,
		Update: func(c *Config, v string) { c.OutputPath = v },
	},
	{
		Name: KeyOutputs,
		Type: "enums:File,HTTP,RTMP,RTP",
		Update: func(c *Config, v string) {
			outputs := strings.Split(v, ",")
			c.Outputs = make([]uint8, len(outputs))
			for i, output := range outputs {
				switch strings.ToLower(output) {
				case "file":
					c.Outputs[i] = OutputFile
				case "files":
					c.Outputs[i] = OutputFiles
				case "http":
					c.Outputs[i] = OutputHTTP
				case "rtmp":
					c.Outputs[i] = OutputRTMP
				case "rtp":
					c.Outputs[i] = OutputRTP
				default:
					c.Logger.Warning("invalid outputs param", "value", v)
				}
			}
		},
		Validate: func(c *Config) {
			if c.Outputs == nil {
				c.LogInvalidField(KeyOutputs, defaultOutput)
				c.Outputs = append(c.Outputs, defaultOutput)
			}
		},
	},
	{
		Name:     KeyPSITime,
		Type:     typeUint,
		Update:   func(c *Config, v string) { c.PSITime = parseUint(KeyPSITime, v, c) },
		Validate: func(c *Config) { c.PSITime = lessThanOrEqual(KeyPSITime, c.PSITime, 0, c, defaultPSITime) },
	},
	{
		Name:   KeyQuantization,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.Quantization = parseUint(KeyQuantization, v, c) },
	},
	{
		Name:   KeyPoolCapacity,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.PoolCapacity = parseUint(KeyPoolCapacity, v, c) },
		Validate: func(c *Config) {
			c.PoolCapacity = lessThanOrEqual(KeyPoolCapacity, c.PoolCapacity, 0, c, defaultPoolCapacity)
		},
	},
	{
		Name:   KeyPoolStartElementSize,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.PoolStartElementSize = parseUint("PoolStartElementSize", v, c) },
		Validate: func(c *Config) {
			c.PoolStartElementSize = lessThanOrEqual("PoolStartElementSize", c.PoolStartElementSize, 0, c, defaultPoolStartElementSize)
		},
	},
	{
		Name:   KeyPoolWriteTimeout,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.PoolWriteTimeout = parseUint(KeyPoolWriteTimeout, v, c) },
		Validate: func(c *Config) {
			c.PoolWriteTimeout = lessThanOrEqual(KeyPoolWriteTimeout, c.PoolWriteTimeout, 0, c, defaultPoolWriteTimeout)
		},
	},
	{
		Name: KeyRecPeriod,
		Type: typeFloat,
		Update: func(c *Config, v string) {
			_v, err := strconv.ParseFloat(v, 64)
			if err != nil {
				c.Logger.Warning(fmt.Sprintf("invalid %s param", KeyRecPeriod), "value", v)
			}
			c.RecPeriod = _v
		},
	},
	{
		Name:   KeyRotation,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.Rotation = parseUint(KeyRotation, v, c) },
	},
	{
		Name: KeyRTMPURL,
		Type: typeString,
		Update: func(c *Config, v string) {
			v = strings.ReplaceAll(v, " ", "")
			c.RTMPURL = strings.Split(v, ",")
		},
	},
	{
		Name:   KeyRTPAddress,
		Type:   typeString,
		Update: func(c *Config, v string) { c.RTPAddress = v },
		Validate: func(c *Config) {
			if c.RTPAddress == "" {
				c.LogInvalidField(KeyRTPAddress, defaultRTPAddr)
				c.RTPAddress = defaultRTPAddr
			}
		},
	},
	{
		Name:   KeySampleRate,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.SampleRate = parseUint(KeySampleRate, v, c) },
	},
	{
		Name:   KeySaturation,
		Type:   typeInt,
		Update: func(c *Config, v string) { c.Saturation = parseInt(KeySaturation, v, c) },
	},
	{
		Name:   KeySharpness,
		Type:   typeInt,
		Update: func(c *Config, v string) { c.Sharpness = parseInt(KeySharpness, v, c) },
	},
	{
		Name: KeyJPEGQuality,
		Type: typeUint,
		Update: func(c *Config, v string) {
			_v, err := strconv.Atoi(v)
			if err != nil {
				c.Logger.Warning("invalid JPEGQuality param", "value", v)
			}
			c.JPEGQuality = _v
		},
	},
	{
		Name: KeySuppress,
		Type: typeBool,
		Update: func(c *Config, v string) {
			c.Suppress = parseBool(KeySuppress, v, c)
			c.Logger.(*logging.JSONLogger).SetSuppress(c.Suppress)
		},
	},
	{
		Name: KeyTimelapseInterval,
		Type: typeUint,
		Update: func(c *Config, v string) {
			_v, err := strconv.Atoi(v)
			if err != nil {
				c.Logger.Warning("invalid TimelapseInterval param", "value", v)
			}
			c.TimelapseInterval = time.Duration(_v) * time.Second
		},
	},
	{
		Name: KeyTimelapseDuration,
		Type: typeUint,
		Update: func(c *Config, v string) {
			_v, err := strconv.Atoi(v)
			if err != nil {
				c.Logger.Warning("invalid TimelapseDuration param", "value", v)
			}
			c.TimelapseDuration = time.Duration(_v) * time.Second
		},
	},
	{
		Name:   KeyVBRBitrate,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.VBRBitrate = parseUint(KeyVBRBitrate, v, c) },
	},
	{
		Name: KeyVBRQuality,
		Type: "enum:standard,fair,good,great,excellent",
		Update: func(c *Config, v string) {
			c.VBRQuality = Quality(parseEnum(
				KeyVBRQuality,
				v,
				map[string]uint8{
					"standard":  uint8(QualityStandard),
					"fair":      uint8(QualityFair),
					"good":      uint8(QualityGood),
					"great":     uint8(QualityGreat),
					"excellent": uint8(QualityExcellent),
				},
				c,
			))
		},
	},
	{
		Name:   KeyVerticalFlip,
		Type:   typeBool,
		Update: func(c *Config, v string) { c.VerticalFlip = parseBool(KeyVerticalFlip, v, c) },
	},
	{
		Name:   KeyWidth,
		Type:   typeUint,
		Update: func(c *Config, v string) { c.Width = parseUint(KeyWidth, v, c) },
	},
}

func parseUint(n, v string, c *Config) uint {
	_v, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		c.Logger.Warning(fmt.Sprintf("expected unsigned int for param %s", n), "value", v)
	}
	return uint(_v)
}

func parseInt(n, v string, c *Config) int {
	_v, err := strconv.Atoi(v)
	if err != nil {
		c.Logger.Warning(fmt.Sprintf("expected integer for param %s", n), "value", v)
	}
	return _v
}

func parseBool(n, v string, c *Config) (b bool) {
	switch strings.ToLower(v) {
	case "true":
		b = true
	case "false":
		b = false
	default:
		c.Logger.Warning(fmt.Sprintf("expect bool for param %s", n), "value", v)
	}
	return
}

func parseEnum(n, v string, enums map[string]uint8, c *Config) uint8 {
	_v, ok := enums[strings.ToLower(v)]
	if !ok {
		c.Logger.Warning(fmt.Sprintf("invalid value for %s param", n), "value", v)
	}
	return _v
}

func lessThanOrEqual(n string, v, cmp uint, c *Config, def uint) uint {
	if v <= cmp {
		c.LogInvalidField(n, def)
		return def
	}
	return v
}

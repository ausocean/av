/*
NAME
  list.go

AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package codecutil

// All available codecs for reference in any application.
// When adding or removing a codec from this list, the IsValid function below must be updated.
const (
	PCM     = "pcm"
	ADPCM   = "adpcm"
	H264    = "h264"    // h264 bytestream (requires lexing).
	H264_AU = "h264_au" // Discrete h264 access units.
	H265    = "h265"
	MJPEG   = "mjpeg"
	JPEG    = "jpeg"
)

// IsValid checks if a string is a known and valid codec in the right format.
func IsValid(s string) bool {
	switch s {
	case PCM, ADPCM, H264, H264_AU, H265, MJPEG, JPEG:
		return true
	default:
		return false
	}
}

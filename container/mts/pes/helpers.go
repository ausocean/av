/*
DESCRIPTIONS
  helpers.go provides general codec related helper functions.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package pes

import "errors"

// Stream types AKA stream IDs as per ITU-T Rec. H.222.0 / ISO/IEC 13818-1 [1], tables 2-22 and 2-34.
const (
	H264SID  = 27
	H265SID  = 36
	MJPEGSID = 136
	JPEGSID  = 137
	PCMSID   = 192
	ADPCMSID = 193
)

// SIDToMIMEType will return the corresponding MIME type for passed stream ID.
func SIDToMIMEType(id int) (string, error) {
	switch id {
	case H264SID:
		return "video/h264", nil
	case H265SID:
		return "video/h265", nil
	case MJPEGSID:
		return "video/x-motion-jpeg", nil
	case JPEGSID:
		return "image/jpeg", nil
	case PCMSID:
		return "audio/pcm", nil
	case ADPCMSID:
		return "audio/adpcm", nil
	default:
		return "", errors.New("unknown stream ID")
	}
}

/*
NAME
  audio_windows.go

AUTHORS
  Alan Noble <alan@ausocean.org>

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
)

func (r *Revid) setupAudio() error {
	return errors.New("audio not implemented on Windows")
}

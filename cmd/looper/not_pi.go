// +build !pi0,!pi3

/*
DESCRIPTION
  not_pi.go lets looper build on non-RPi environments, but leaves a
  non-functional executable.

AUTHORS
  Dan Kortschak <dan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package main

import (
	"github.com/ausocean/utils/logging"
)

const audioCmd = "play"

func initCommand(l logging.Logger) {
	panic("looper is intended to be run on a Raspberry Pi 0 or 3.")
}

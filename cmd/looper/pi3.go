// +build pi3

/*
DESCRIPTION
  pi3.go defines an initialisation function for use when running on the
  Raspberry Pi 3.

AUTHORS
  Ella Pietraroia <ella@ausocean.org>
  Scott Barnard <scott@ausocean.org>
  Saxon Nelson-Milton <saxon@ausocean.org>

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

const audioCmd = "omxplayer"

func initCommand(l logging.Logger) { checkPath(audioCmd, l) }

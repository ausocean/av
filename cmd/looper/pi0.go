// +build pi0

/*
DESCRIPTION
  pi0.go defines an initialisation function for use when running on the
  Raspberry Pi 0.

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
	"os/exec"
	"time"

	"github.com/ausocean/utils/logging"
)

const audioCmd = "play"

func initCommand(l logging.Logger) {
	const (
		cardPath = "/usr/share/doc/audioInjector/asound.state.RCA.thru.test"
		retryDur = 5 * time.Second
		alsactl  = "alsactl"
	)

	// Make sure utility to set up sound card, alsactl, exists.
	checkPath(alsactl, l)

	// Set up sound card using alsactl.
	cmdInit := exec.Command(alsactl, "-f", cardPath, "restore")
	err := cmdInit.Run()
	for err != nil {
		l.Warning("alsactl run failed, retrying...", "error", err)
		time.Sleep(retryDur)
		err = cmdInit.Run()
	}

	// Make sure utility to play audio exists.
	checkPath(audioCmd, l)
}

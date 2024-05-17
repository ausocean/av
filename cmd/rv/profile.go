// +build profile

/*
DESCRIPTION
  profile.go provides an init to change canProfile flag to true if profile tag
  provided on build.

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

import _ "net/http/pprof"

func init() {
	canProfile = true
}

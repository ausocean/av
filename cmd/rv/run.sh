#!/bin/sh
REVIDPATH=$HOME/go/src/github.com/ausocean/av/cmd/rv
cd $REVIDPATH
sudo "PATH=$PATH:$REVIDPATH" ./rv &

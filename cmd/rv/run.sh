#!/bin/sh
REVIDPATH=$HOME/go/src/bitbucket.org/ausocean/av/cmd/rv
cd $REVIDPATH
sudo "PATH=$PATH:$REVIDPATH" ./rv &

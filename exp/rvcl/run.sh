#!/bin/sh
REVIDPATH=$HOME/go/src/github.com/ausocean/av/cmd/rvcl
cd $REVIDPATH
sudo "PATH=$PATH:$REVIDPATH" ./rvcl -NetSender &

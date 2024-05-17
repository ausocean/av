#!/bin/sh
REVIDPATH=$HOME/go/src/bitbucket.org/ausocean/av/cmd/rvcl
cd $REVIDPATH
sudo "PATH=$PATH:$REVIDPATH" ./rvcl -NetSender &

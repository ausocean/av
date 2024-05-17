#!/bin/sh -e
# This script launches treatment on a pi, intended to run at boot time.

TREATPATH=/home/pi/go/src/bitbucket.org/ausocean/av/cmd/treatment

echo Set kernel parameters:
# kernel settings to improve performance on Raspberry Pi
# tell Linux to fork optimistically
sudo sysctl -w vm.overcommit_memory=1
# minimize swapping, without disabling it completely
sudo sysctl -w vm.swappiness=1

# the following required directories _should_ already exist
if [ ! -d /var/log/netsender ]; then
  sudo mkdir /var/log/netsender
  chmod guo+rwx /var/log/netsender
fi
if [ ! -d /var/netsender ]; then
  sudo mkdir /var/netsender
  chmod guo+rwx /var/netsender
fi

# show IP addresses
echo Our IP addresses:
sudo ip addr show | grep inet

# capture stdout and stderr to a secondary log file (just in case)
exec 2> /var/log/netsender/stream.log
exec 1>&2

# set env, working dir and run treatment as pi user
HOME=/home/pi
GOPATH=$HOME/go
TREATPATH=$GOPATH/src/bitbucket.org/ausocean/av/cmd/treatment
PATH=$PATH:/usr/local/go/bin:$TREATPATH
cd $TREATPATH
sudo HOME=$HOME GOPATH=$GOPATH PATH=$PATH ./treatment
if [ $? -eq 0 ]
then
  echo "Successfully exited treatment"
  exit 0
else
  echo "treatment exited with code: $?" >&2
  exit 1
fi

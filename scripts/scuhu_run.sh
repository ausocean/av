#!/bin/sh -e
# This script configures and runs a binary on a pi, intended to run at boot time.
BINNAME=rvcl
ARGS="-config-file config-scuhu.json"

echo Set kernel parameters:
# kernel settings to improve performance on Raspberry Pi
# tell Linux to fork optimistically
sudo sysctl -w vm.overcommit_memory=1
# minimize swapping, without disabling it completely
sudo sysctl -w vm.swappiness=1

# turn off the pi LED to conserve power
echo default-on | sudo tee /sys/class/leds/led0/trigger
echo none | sudo tee /sys/class/leds/led0/trigger

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

# set env, working dir and run as pi user
HOME=/home/pi
GOPATH=$HOME/go
BINPATH=$GOPATH/src/github.com/ausocean/av/exp/$BINNAME
PATH=$PATH:/usr/local/go/bin:$BINPATH
cd $BINPATH
sudo HOME=$HOME GOPATH=$GOPATH PATH=$PATH ./$BINNAME $ARGS
if [ $? -eq 0 ]
then
  echo "Successfully exited $BINNAME"
  exit 0
else
  echo "$BINNAME exited with code: $?" >&2
  exit 1
fi

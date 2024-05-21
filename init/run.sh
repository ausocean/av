#!/bin/bash -e
# This script launches rv. This is used by the rv.service file that will become
# a systemd service after using the Makefile install* targets.

# Check that we have the correct number of arguments passed.
if [ $# -ne 2 ]; then
  echo "incorrect number of arguments, expected gopath user and binary directory"
  exit 1
fi

# This is the user in the GOPATH e.g. for /home/pi/go the user is pi.
gopath_user=$1

# This is the dir of the binary from the GOPATH e.g. /src/github.com/ausocean/av/cmd/rv.
bin_dir=$2

# We'll get the bin name from the bin dir (assuming this is same as the bin dir name).
bin_name=$(basename $bin_dir)

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

# Now set all required variables.
HOME=/home/$gopath_user
GOPATH=$HOME/go
RVPATH=$GOPATH$bin_dir
PATH=$PATH:/usr/local/go/bin:$RVPATH
cd $RVPATH
sudo -u $gopath_user HOME=$HOME GOPATH=$GOPATH PATH=$PATH ./$bin_name
if [ $? -eq 0 ]
then
  echo "Successfully exited rv"
  exit 0
else
  echo "rv exited with code: $?" >&2
  exit 1
fi

#!/bin/bash -e
# This script runs AusOcean client software on a Raspberry Pi.
# It expects the binary to be under /opt/ausocean/bin.
# Usage: run_pi_client.sh <user> <binary>
version="v1.0.0"
if [ "$1" == "-version" ] || [ "$1" == "-v" ]; then
  echo "$version"
  exit 0
fi

# Check that we have the correct number of arguments passed.
if [ $# -ne 2 ]; then
  echo "incorrect number of arguments, expected <user> and <binary>"
  exit 1
fi

# This is the runtime user, typically 'pi'.
user=$1

# Form the binary path and check it exists.
bin_dir=/opt/ausocean/bin
bin_path=$bin_dir/$2
if [ ! -f $bin_path ]; then
  echo "$bin_path not found"
  exit 1
fi
echo Running $bin_path

# Lernel settings to improve performance on Raspberry Pi.
echo Set kernel parameters:
# Tell Linux to fork optimistically.
sudo sysctl -w vm.overcommit_memory=1
# Minimize swapping, without disabling it completely.
sudo sysctl -w vm.swappiness=1

# The following required directories _should_ already exist
if [ ! -d /var/log/netsender ]; then
  sudo mkdir /var/log/netsender
  chmod guo+rwx /var/log/netsender
fi
if [ ! -d /var/netsender ]; then
  sudo mkdir /var/netsender
  chmod guo+rwx /var/netsender
fi

# Show IP addresses.
echo Our IP addresses:
sudo ip addr show | grep inet

# Capture stdout and stderr to a secondary log file (just in case).
exec 2> /var/log/netsender/stream.log
exec 1>&2

# Prepend binary directory to the PATH.
PATH=$bin_dir:$PATH

# Launch the binary running as the given user.
sudo -u $user PATH=$PATH $bin_path
if [ $? -eq 0 ]
then
  echo "Successfully exited $bin_path"
  exit 0
else
  echo "$bin_path exited with code: $?" >&2
  exit 1
fi

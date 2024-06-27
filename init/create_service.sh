#!/bin/bash
# This script is utilised by Makefile for creation of a service responsible for the
# running of a netsender client. The service lines are stored in a string allowing us
# to substitute the GOPATH into the ExecStart path.

if [ $# -ne 2 ]; then
  echo "incorrect number of arguments, expected run script and binary directories"
  exit 1
fi

# Path to run script,e.g. /opt/ausocean/bin/run.sh
run_script_path=$1

# Path to the binary, e.g. /opt/ausocean/bin/rv
bin_path=$2

# We always run as the pi user.
user=pi

# Get the bin name (assuming this is at the end of the bin_path).
bin_name=$(basename $bin_path)

# Here are the lines that will go into the rv.service file. We'll set the
# ExecStart field as the GOPATH we've obtained + the passed run script dir.
service="
[Unit]
Description=Netsender Client for Media Collection and Forwarding

[Service]
Type=simple
ExecStart=$run_script_path $user $bin_path
Restart=on-failure

[Install]
WantedBy=multi-user.target
"

# The service name will just use the bin name.
service_name="$bin_name.service"

# Now overwrite the service if it exists, or create the service then write.
service_path=/etc/systemd/system/$service_name
if [ -f $service_path ]; then
  echo "$service" > $service_path
else
  touch $service_path
  echo "$service" > $service_path
fi

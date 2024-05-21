#!/bin/bash
# This script is utilised by Makefile for creation of a service responsible for the
# running of a netsender client. The service lines are stored in a string allowing us
# to substitute the GOPATH into the ExecStart path.

if [ $# -ne 2 ]; then
  echo "incorrect number of arguments, expected run script and binary directories"
  exit 1
fi

# This corresponds to the path beyond the GOPATH corresponding to the run script
# of the client. e.g. "/src/github.com/ausocean/av/init/run.sh"
run_script_dir=$1

# This corresponds to the binary dir. e.g. /src/github.com/ausocean/av/cmd/rv
bin_dir=$2

# Get the bin name (assuming this is at the end of the bin_dir).
bin_name=$(basename $bin_dir)

# First find the user that corresponds to this path (which is assumed to be at the
# base of the current working directory).
in=$(pwd)
arr_in=(${in//// })
gopath_user=${arr_in[1]}

# We can now form the gopath from the obtained user.
gopath="/home/$gopath_user/go"

# Here are the lines that will go into the rv.service file. We'll set the
# ExecStart field as the GOPATH we've obtained + the passed run script dir.
service="
[Unit]
Description=Netsender Client for Media Collection and Forwarding

[Service]
Type=simple
ExecStart=$gopath$run_script_dir $gopath_user $bin_dir
Restart=on-failure

[Install]
WantedBy=multi-user.target
"

# The service name will just use the bin name.
service_name="$bin_name.service"

# Now overwrite the service if it exists, or create the service then write.
service_dir=/etc/systemd/system/$service_name
if [ -f $service_dir ]; then
  echo "$service" > $service_dir
else
  touch $service_dir
  echo "$service" > $service_dir
fi

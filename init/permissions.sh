#!/bin/bash
# This script is used by Makefile to modify permissions required by some targets.
in=$(pwd)
arr_in=(${in//// })
gopath_user=${arr_in[1]}
chown $gopath_user /etc/netsender.conf

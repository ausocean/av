#!/bin/sh

# This script is intended to run at startup and will turn the wifi interface off after 20 minutes.
# If a user doesn't want the wifi to turn off, the service that controls this script can be 
# disabled before the delay by running: "sudo service wifi-off-delay stop".

sleep 180
sudo ifconfig wlan0 down

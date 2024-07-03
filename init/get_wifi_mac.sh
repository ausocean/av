#!/bin/bash
# Find the MAC address of the first WiFi device and create an AusOcean MAC address.
# We do this heuristically by finding the first device that starts with DeviceLetter.
# For ethernet, use "e"; for WiFi, use "w"; for loopback, use "l".
DeviceLetter="w"
Prefix="a0:a0:a0"

# Get the output of the ip link show command.
Output=$(ip link show)

# Iterate over each line of the ip link output until we match DeviceLetter.
# The following line contains the MAC address.
MacLine=
while IFS= read -r line; do
  if [[ ${line:3:1} == "$DeviceLetter" ]]; then
    IFS= read -r MacLine
    break
  fi
done <<< "$Output"

if [ -z "$MacLine" ]; then
  # We failed to find a device that matches DeviceLetter.
  exit 1
fi

# Split MacLine by space to get the MAC address.
read -r -a array <<< "$MacLine"
MAC=${array[1]}

# Split MacLine by space to get the MAC address.
read -r -a lineparts <<< "$MacLine"
MAC=${lineparts[1]}

# Split MAC by colon to get the MAC address parts.
IFS=: read -r -a MACparts <<< "$MAC"
hex3=${MACparts[3]}
hex4=${MACparts[4]}
hex5=${MACparts[5]}

# Print the orginal MAC followed by the AusOcean MAC.
echo "$MAC $Prefix:$hex3:$hex4:$hex5"
exit 0

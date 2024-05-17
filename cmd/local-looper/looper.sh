#!/bin/bash

systemd-notify --ready

export AUDIODEV=hw:1,0
while true; do
  play -q -V0 -t raw -r 44.1k -e signed -b 16 -c 1 /home/pi/shrimp.pcm
  systemd-notify --status="service is running" WATCHDOG=1
done

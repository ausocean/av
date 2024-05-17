# DESCRIPTION
	local looper is a process which will continually loop a pcm audio file
	(like looper: "av/cmd/looper/"). The intended hardware for this system is a raspberry pi
	zero W, with a USB sound card (AUDIODEV=hw:(1,0)).

# AUTHORS
	David Sutton <davidsutton@ausocean.org>

# SETUP
	1) move looper.sh to home/pi and run "chmod u+x looper.sh.
	2) move looper.service to /etc/systemd/system.
	3) run systemctl enable looper.service.
	4) run sudo systemctl start looper.service.
	5) restart the device, and wait for boot and audio should play.
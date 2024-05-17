# DESCRIPTION
  looper is a process that will continually repeat playback of an audio file.
  Intended hardware is raspberry pi 3 or raspberry pi zero (with audio injector
  sound card hat, see http://www.audioinjector.net/rpi-zero).

# AUTHORS
  Ella Pietraroia <ella@ausocean.org>
  Saxon Nelson-Milton <saxon@ausocean.org>

# Pi Setup:
  1) sudo make install_hard
  2) modify ma field in /etc/netsender.conf file to be mac of device
  3) modify audio file flag in ./looper execution command in /etc/rc.local to be path of file we'd like to play
  4) systemctl enable rc-local
  5) sudo systemctl start rc-local.service
  6) To check that install steps were successful, restart device and confirm that sound plays.

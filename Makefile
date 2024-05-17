all:	rv audio-netsender

rv:
	cd cmd/rv; go build

audio-netsender:
	cd cmd/audio-netsender; go build

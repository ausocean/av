# Install files and directories required by speaker (netsender client, speaker driver software).

# SET_MAC=true will create a dhcpcd.enter-hook for setting the MAC address. (MA must be specified). This may not be desired if installing for a generic image.
SET_MAC := true

# CONFIG=true will write or overwrite the configuration file. (MA and DK must be specified). This may not be desired if installing for a generic image.
CONFIG := true

# BUILD=true will run go build to build the binary. This may not be desirable if cross compiling or installing for a generic image.
BUILD := true

# USER can be passed to customise the user under which the code is located.
USER := $(shell whoami)

PATH := /usr/local/go/bin:$(PATH)
BIN_NAME := speaker
BIN_DIR := /src/github.com/ausocean/av/cmd/$(BIN_NAME)
SPEAKER_CONF := '{"Path": "/home/pi/audio.wav", "Volume": "100"}'


.SILENT:clean

install: as_root make_dirs write_service
ifeq ($(BUILD),true)
	$(MAKE) rebuild
	echo "binary built"
endif
ifeq ($(CONFIG),true)
	$(MAKE) write_conf
	echo "conf and service files created/overwritten"
endif
ifeq ($(SET_MAC),true)
	$(MAKE) set_mac
	echo "MAC address set to $(MA)"
endif
	$(MAKE) syncreboot
	echo "install complete"

as_root:
ifneq ($(USER),root)
	$(error Must run as superuser!)
endif

set_conf_treat:
	if [ -f /etc/speaker.json ] ; then \
		echo "/etc/speaker.json overwritten" ; \
	fi
	printf $(SPEAKER_CONF) > /etc/speaker.json

rebuild:
	@chmod +x speaker_run.sh
	@go build

make_dirs:
	@if [ ! -d /var/netsender ] ; then \
		mkdir /var/netsender; \
		chmod guo+rwx /var/netsender; \
	fi
	@if [ ! -d /var/log/netsender ] ; then \
		mkdir /var/log/netsender; \
		chmod guo+rwx /var/log/netsender; \
	fi

write_service:
	@if [ -f /etc/systemd/system/$(BIN_NAME).service ] ; then \
		echo "/etc/systemd/system/$(BIN_NAME).service overwritten" ; \
	fi
	@cp -f $(BIN_NAME).service /etc/systemd/system
	@systemctl enable $(BIN_NAME).service

write_conf:
ifeq ($(MA),)
	$(error "Must provide MAC using MA=")
endif
ifeq ($(DK),)
	$(error "Must provide device key using DK=")
endif
	@if [ -f /etc/netsender.conf ] ; then \
		echo "Backed up netsender.conf to /etc/netsender.conf.bak"; \
		cp /etc/netsender.conf /etc/netsender.conf.bak ; \
	fi
	@printf "ma $(MA)\ndk $(DK)\n" > /etc/netsender.conf
	@chown $(USER) /etc/netsender.conf

set_mac:
ifeq ($(MA),)
	$(error "Must provide MAC using MA=")
endif
	@printf "ip link set eth0 address $(MA)\n" > /etc/dhcpcd.enter-hook
	@chmod guo+x /etc/dhcpcd.enter-hook

syncreboot:
	@if [ -e ../../../utils ]; then \
		cd ../../../utils; git checkout -f master; git pull; \
	else \
		cd ../../../; git clone https://github.com/ausocean/utils; \
	fi
	@cd ../../../utils/cmd/syncreboot; make; make install

clean: as_root
	rm -rf /var/netsender
	rm -rf /var/log/netsender
	rm -rf /etc/systemd/system/$(BIN_NAME).service
	rm -rf /etc/netsender.conf
	echo "Clean complete"

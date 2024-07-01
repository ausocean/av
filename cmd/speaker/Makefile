# Install files and directories required by speaker (netsender client, speaker driver software).
# The /opt/ausocean/systemd directory MUST already exist and contain required files.

# SET_MAC=true will create a dhcpcd.enter-hook for setting the MAC address. (MA must be specified). This may not be desired if installing for a generic image.
SET_MAC := true

# CONFIG=true will write or overwrite the configuration file. (MA and DK must be specified). This may not be desired if installing for a generic image.
CONFIG := true
AUDIO := audio.wav

USER := $(shell whoami)
RUNTIME_USER := pi
INSTALL_DIR := /opt/ausocean
BIN_DIR := $(INSTALL_DIR)/bin
AUDIO_DIR := $(INSTALL_DIR)/data/audio
SYSTEMD_DIR := $(INSTALL_DIR)/systemd
BIN_NAME := speaker
WIFI_SVC_NAME = wifi-off-delay
SPEAKER_CONF := '{"Path": "$(AUDIO_DIR)/$(AUDIO)", "Volume": "100"}'

.SILENT:clean

install: as_root make_dirs write_services set_conf_speaker
ifeq ($(CONFIG),true)
	$(MAKE) write_conf
	echo "conf and service files created/overwritten"
endif
ifeq ($(SET_MAC),true)
	$(MAKE) set_mac
	echo "MAC address set to $(MA)"
endif
	echo "install complete"

as_root:
ifneq ($(USER),root)
	$(error Must run as superuser!)
endif

set_conf_speaker:
	if [ -f /etc/speaker.json ] ; then \
		echo "/etc/speaker.json overwritten" ; \
	fi
	printf $(SPEAKER_CONF) > /etc/speaker.json

make_dirs:
	@if [ ! -d /var/netsender ] ; then \
		mkdir /var/netsender; \
		chmod guo+rwx /var/netsender; \
	fi
	@if [ ! -d /var/log/netsender ] ; then \
		mkdir /var/log/netsender; \
		chmod guo+rwx /var/log/netsender; \
	fi
	@if [ ! -d $(INSTALL_DIR) ] ; then \
		mkdir $(INSTALL_DIR); \
		chmod guo+rwx $(INSTALL_DIR); \
	fi
	@if [ ! -d $(INSTALL_DIR)/bin ] ; then \
		mkdir $(INSTALL_DIR)/bin; \
		chmod guo+rwx $(INSTALL_DIR)/bin; \
	fi
	@if [ ! -d $(INSTALL_DIR)/doc ] ; then \
		mkdir $(INSTALL_DIR)/doc; \
		chmod guo+rwx $(INSTALL_DIR)/doc; \
	fi
	@if [ ! -d $(INSTALL_DIR)/data ] ; then \
		mkdir $(INSTALL_DIR)/data; \
		chmod guo+rwx $(INSTALL_DIR)/data; \
	fi
	@if [ ! -d $(INSTALL_DIR)/data/audio ] ; then \
		mkdir $(INSTALL_DIR)/data/audio; \
		chmod guo+rwx $(INSTALL_DIR)/data/audio; \
	fi
	@if [ ! -d $(SYSTEMD_DIR) ] ; then \
		$(error "$(SYSTEMD_DIR) does not exist or is empty"); \
	fi

write_services:
	@if [ -f /etc/systemd/system/$(BIN_NAME).service ] ; then \
		echo "/etc/systemd/system/$(BIN_NAME).service overwritten" ; \
	fi
	@cp -f $(SYSTEMD_DIR)/$(BIN_NAME).service /etc/systemd/system
	@systemctl enable $(BIN_NAME).service
	@if [ -f /etc/systemd/system/$(WIFI_SVC_NAME).service ] ; then \
		echo "/etc/systemd/system/$(WIFI_SVC_NAME).service overwritten" ; \
	fi
	@cp -f $(SYSTEMD_DIR)/$(WIFI_SVC_NAME).service /etc/systemd/system
	@systemctl enable $(WIFI_SVC_NAME).service

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
	@chown $(RUNTIME_USER) /etc/netsender.conf

set_mac:
ifeq ($(MA),)
	$(error "Must provide MAC using MA=")
endif
	@printf "ip link set eth0 address $(MA)\n" > /etc/dhcpcd.enter-hook
	@chmod guo+x /etc/dhcpcd.enter-hook

clean: as_root
	rm -rf /var/netsender
	rm -rf /var/log/netsender
	rm -rf /etc/systemd/system/$(BIN_NAME).service
	rm -rf /etc/systemd/system/$(WIFI_SVC_NAME).service
	rm -rf /etc/netsender.conf
	echo "Clean complete"

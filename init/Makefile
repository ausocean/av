# Install files and directories required by NetSender clients (such as gpio-netsender, rv, etc.).
# Optionally you can create a dhcpcd.enter-hook for setting the MAC address. (install_set_mac).
# MA and DK must be passed to Make for an installation:
# 	sudo make install MA=mac DK=dk
# Dependencies:
#   pkg-upgrade.sh
#   run.sh
#   create_service.sh
# TODO: Make client type (CT) a parameter so this Makefile can be used for different devices.
USER := $(shell whoami)
BIN_DIR := /opt/ausocean/bin
PATH := $(BIN_DIR):$(PATH)
DEV_TYPE := camera
BIN_NAME := rv
BIN_PATH := $(BIN_DIR)/rv
RUN_SCRIPT_PATH := $(BIN_DIR)/run.sh
UPGRADER_PATH := $(BIN_DIR)/pkg-upgrade.sh

# Change this to withcv if you wish to use CV.
BUILD_TAG :=

.SILENT:make_dirs
.SILENT:hard_copy_files
.SILENT:set_mac
.SILENT:clean

download: not_as_root upgrade
	@echo "Download complete"

install: as_root make_dirs hard_copy_files 
	@echo "Install complete"

install_set_mac: as_root make_dirs hard_copy_files set_mac
	@echo "Install complete, MAC address set to $(MA)"

as_root:
ifneq ($(USER),root)
	$(error Must run as superuser!)
endif

not_as_root:
ifeq ($(USER),root)
	$(error Must not run as superuser!)
endif

upgrade:
	$(UPGRADER_PATH) $(DEV_TYPE)

make_dirs:
	@if [ ! -d /var/netsender ] ; then \
		mkdir /var/netsender; \
		chmod guo+rwx /var/netsender; \
	fi
	@if [ ! -d /var/log/netsender ] ; then \
		mkdir /var/log/netsender; \
		chmod guo+rwx /var/log/netsender; \
	fi

hard_copy_files:
ifeq ($(MA),)
	$(error "Must provide MAC using MA=")
endif
ifeq ($(DK),)
	$(error "Must provide device key using DK=")
endif
	if [ -f /etc/systemd/system/$(BIN_NAME).service ] ; then \
		echo "/etc/systemd/system/$(BIN_NAME).service overwritten" ; \
	fi
	bash create_service.sh $(RUN_SCRIPT_PATH) $(BIN_PATH)
	@systemctl enable $(BIN_NAME).service
	@if [ -f /etc/netsender.conf ] ; then \
		echo "Backed up netsender.conf to /etc/netsender.conf.bak"; \
		cp /etc/netsender.conf /etc/netsender.conf.bak ; \
	fi
	@printf "ma $(MA)\ndk $(DK)\n" > /etc/netsender.conf
	chown $(USER) /etc/netsender.conf

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
	rm -rf /etc/netsender.conf
	@echo "Clean complete"

# Install files and directories required by NetSender clients (such as gpio-netsender, rv, etc.).
# Optionally you can create a dhcpcd.enter-hook for setting the MAC address. (install_set_mac).
# MA and DK must be passed to Make for an installation:
# 	sudo make install BIN=binary MA=mac DK=dk CT=ct [SVC=service]
# BIN is the binary name (defaults to rv).
# CT is the client type (defaults to camera).
# SVC is an optional service to be installed (without the .service extension)
# Dependencies:
#   pkg-upgrade.sh
BIN := rv
CT := camera
USER := $(shell whoami)
RUNTIME_USER := pi
INSTALL_DIR := /opt/ausocean
BIN_DIR := $(INSTALL_DIR)/bin
PATH := $(BIN_DIR):$(PATH)
UPGRADER_PATH := $(BIN_DIR)/pkg-upgrade.sh
SYSTEMD_DIR := $(INSTALL_DIR)/systemd

.SILENT:make_dirs
.SILENT:hard_copy_files
.SILENT:set_mac
.SILENT:clean

install: as_root pre_reqs make_dirs download hard_copy_files 
	@echo "Install complete"

install_set_mac: as_root pre_reqs make_dirs download hard_copy_files set_mac
	@echo "Install complete, MAC address set to $(MA)"

as_root:
ifneq ($(USER),root)
	$(error Must run as superuser!)
endif

pre_reqs:
	@if [ ! -d $(SYSTEMD_DIR) ] ; then \
		echo "$(SYSTEMD_DIR) does not exist" ; \
		exit 1; \
	fi
	@if [ ! -f $(SYSTEMD_DIR)/$(BIN).service ] ; then \
		echo "$(SYSTEMD_DIR)/$(BIN).service does not exist"; \
		exit 1; \
	fi
ifneq ($(SVC),)
	@if [ ! -f $(SYSTEMD_DIR)/$(SVC).service ] ; then \
		echo "$(SYSTEMD_DIR)/$(SVC).service does not exist"; \
		exit 1; \
	fi
endif

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
		chmod guo+rwx $(INSTALL_DIR)/data; \
	fi
	@echo "Made directories"

download:
	$(UPGRADER_PATH) $(CT)
	@chown -R $(RUNTIME_USER) $(BIN_DIR)
	@echo "Dowloaded $(CT) release"

hard_copy_files:
ifeq ($(MA),)
	$(error "Must provide MAC using MA=")
endif
ifeq ($(DK),)
	$(error "Must provide device key using DK=")
endif
	@if [ -f /etc/systemd/system/$(BIN).service ] ; then \
		echo "/etc/systemd/system/$(BIN).service overwritten" ; \
	fi
	@cp -f $(SYSTEMD_DIR)/$(BIN).service /etc/systemd/system
	@systemctl enable $(BIN).service
ifneq ($(SVC),)
	@if [ -f /etc/systemd/system/$(SVC).service ] ; then \
		echo "/etc/systemd/system/$(SVC).service overwritten" ; \
	fi
	@cp -f $(SYSTEMD_DIR)/$(SVC).service /etc/systemd/system
	@systemctl enable $(SVC).service
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
	rm -rf /etc/systemd/system/$(BIN).service
ifneq ($(SVC),)
	rm -rf /etc/systemd/system/$(SVC).service
endif
	rm -rf /etc/netsender.conf
	@echo "Clean complete"

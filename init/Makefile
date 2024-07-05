# Install files and directories required by NetSender clients (such as gpio-netsender, rv, etc.).
# Optionally you can create a dhcpcd.enter-hook for setting the MAC address. (install_set_mac).
# MA and DK must be passed to Make for an installation:
# 	sudo make install MA=mac DK=dk [BIN=binary] [CT=cient-type] [RU=runtime-user]
# BIN is the binary name (defaults to rv).
# CT is the client type (defaults to camera).
# RU is the runtime user (defaults to pi)
# SVC is an optional service to be installed (without the .service extension)
# Dependencies:
#   pkg-upgrade.sh
#   ausocean.service
BIN := rv
CT := camera
RU := pi
SVC := ausocean.service
USER := $(shell whoami)
INSTALL_DIR := /opt/ausocean
BIN_DIR := $(INSTALL_DIR)/bin
DOC_DIR := $(INSTALL_DIR)/doc
DATA_DIR := $(INSTALL_DIR)/data
PATH := $(BIN_DIR):$(PATH)
UPGRADER_SCRIPT := pkg-upgrade.sh
UPGRADER_PATH := $(BIN_DIR)/$(UPGRADER_SCRIPT)

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
	@if [ ! -f $(UPGRADER_SCRIPT) ] ; then \
		echo "$(UPGRADER_SCRIPT) does not exist"; \
		exit 1; \
	fi
	@if [ ! -f $(SVC) ] ; then \
		echo "$(SVC) does not exist"; \
		exit 1; \
	fi
	@echo "Pre-reqs OK"

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
	@if [ ! -d $(BIN_DIR) ] ; then \
		mkdir $(BIN_DIR); \
		chmod guo+rwx $(BIN_DIR); \
	fi
	@if [ ! -d $(DOC_DIR) ] ; then \
		mkdir $(DOC_DIR); \
		chmod guo+rwx $(DOC_DIR); \
	fi
	@if [ ! -d $(DATA_DIR) ] ; then \
		mkdir $(DATA_DIR); \
		chmod guo+rwx $(DATA_DIR); \
	fi
	@if [ ! -d $(DATA_DIR)/audio ] ; then \
		mkdir $(DATA_DIR)/audio; \
		chmod guo+rwx $(DATA_DIR); \
	fi
	@echo "Made directories"

download:
	@cp -pf $(UPGRADER_SCRIPT) $(UPGRADER_PATH)
	@chmod guo+x $(UPGRADER_PATH)
	@chown $(RU) $(UPGRADER_PATH)
	@echo "$(UPGRADER_PATH) installed"
	$(UPGRADER_PATH) $(CT)
	@chown -R $(RU) $(BIN_DIR)
	@chown -R $(RU) $(DOC_DIR)
	@chown -R $(RU) $(DATA_DIR)
	@echo "Dowloaded $(CT) release"

hard_copy_files:
ifeq ($(MA),)
	$(error "Must provide MAC using MA=")
endif
ifeq ($(DK),)
	$(error "Must provide device key using DK=")
endif
	@if [ -f /etc/systemd/system/$(SVC) ] ; then \
		echo "/etc/systemd/system/$(SVC) overwritten" ; \
	fi
	@cp -f $(SVC) /etc/systemd/system
	@systemctl enable $(SVC)
ifneq ($(SVC),)
	@if [ -f /etc/systemd/system/$(SVC).service ] ; then \
		echo "/etc/systemd/system/$(SVC).service overwritten" ; \
	fi
	@cp -f $(SVC).service /etc/systemd/system
	@systemctl enable $(SVC).service
endif
	@if [ -f /etc/netsender.conf ] ; then \
		echo "Backed up netsender.conf to /etc/netsender.conf.bak"; \
		cp /etc/netsender.conf /etc/netsender.conf.bak ; \
	fi
	@printf "ma $(MA)\ndk $(DK)\n" > /etc/netsender.conf
	@chown $(RU) /etc/netsender.conf

set_mac:
ifeq ($(MA),)
	$(error "Must provide MAC using MA=")
endif
	@printf "ip link set eth0 address $(MA)\n" > /etc/dhcpcd.enter-hook
	@chmod guo+x /etc/dhcpcd.enter-hook
	@echo "Created /etc/dhcpcd.enter-hook"

clean: as_root
	rm -rf /var/netsender
	rm -rf /var/log/netsender
	rm -rf /etc/systemd/system/$(SVC)
	rm -rf /etc/netsender.conf
	@echo "Clean complete"

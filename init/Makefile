# Install files and directories required by NetSender clients (such as gpio-netsender, rv, etc.).
# Optionally you can create a dhcpcd.enter-hook for setting the MAC address. (install_set_mac).
# MA and DK must be passed to Make for an installation:
# 	sudo make install MA=mac DK=dk
USER := $(shell whoami)
PATH := /usr/local/go/bin:$(PATH)
BIN_NAME := rv
BIN_DIR := /src/github.com/ausocean/av/cmd/$(BIN_NAME)
RUN_SCRIPT_DIR := /src/github.com/ausocean/av/init/run.sh

# Change this to withcv if you wish to use CV.
BUILD_TAG :=

.SILENT:make_dirs
.SILENT:hard_copy_files
.SILENT:set_mac
.SILENT:clean

rebuild:
	@chmod +x run.sh
	@cd ../cmd/$(BIN_NAME); \
	if [ -z "$(BUILD_TAG)" ]; then \
		go build; \
	else \
		go build -tags $(BUILD_TAG); \
	fi

install: as_root make_dirs hard_copy_files rebuild syncreboot
	@echo "Install complete"

install_no_rebuild: as_root make_dirs hard_copy_files syncreboot
	@echo "Install complete"

install_set_mac: as_root make_dirs hard_copy_files set_mac rebuild syncreboot
	@echo "Install complete, MAC address set to $(MA)"

as_root:
ifneq ($(USER),root)
	$(error Must run as superuser!)
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
	bash create_service.sh $(RUN_SCRIPT_DIR) $(BIN_DIR)
	@systemctl enable $(BIN_NAME).service
	@if [ -f /etc/netsender.conf ] ; then \
		echo "Backed up netsender.conf to /etc/netsender.conf.bak"; \
		cp /etc/netsender.conf /etc/netsender.conf.bak ; \
	fi
	@printf "ma $(MA)\ndk $(DK)\n" > /etc/netsender.conf
	bash permissions.sh

set_mac:
ifeq ($(MA),)
	$(error "Must provide MAC using MA=")
endif
	@printf "ip link set eth0 address $(MA)\n" > /etc/dhcpcd.enter-hook
	@chmod guo+x /etc/dhcpcd.enter-hook

syncreboot:
	@if [ -e ../../utils ]; then \
		cd ../../utils; git checkout -f master; git pull; \
	else \
		cd ../../; git clone https://github.com/ausocean/utils; \
	fi
	@cd ../../utils/cmd/syncreboot; make; make install

clean: as_root
	rm -rf /var/netsender
	rm -rf /var/log/netsender
	rm -rf /etc/systemd/system/$(BIN_NAME).service
	rm -rf /etc/netsender.conf
	@echo "Clean complete"

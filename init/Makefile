# Install files and directories required by NetSender clients (such as gpio-netsender, rv, etc.)
# and create a dhcpcd.enter-hook for setting the MAC address.
# MA and DK can be optionally passed to Make, e.g, for a hard (first-time) installation:
# 	sudo MA=mac DK=dk install_hard
# NB: The default (soft) install does not override conf files.
# USR can also be passsed to customise user under which the code is housed.
USER := $(shell whoami)
PATH := /usr/local/go/bin:$(PATH)
BIN_NAME := rv
BIN_DIR := /src/bitbucket.org/ausocean/av/cmd/$(BIN_NAME)
RUN_SCRIPT_DIR := /src/bitbucket.org/ausocean/av/init/run.sh

# Change this to withcv if you wish to use CV.
BUILD_TAG :=

ifeq ($(MA),)
	MA := "00:E0:4C:00:00:01"
endif
ifeq ($(DK),)
	DK := 0
endif

.SILENT:make_dirs
.SILENT:soft_copy_files
.SILENT:hard_copy_files
.SILENT:set_mac
.SILENT:clean

rebuild:
	chmod +x run.sh
	cd ../cmd/$(BIN_NAME); \
	if [ -z "$(BUILD_TAG)" ]; then \
		go build; \
	else \
		go build -tags $(BUILD_TAG); \
	fi

install: as_root make_dirs soft_copy_files rebuild syncreboot
	@echo "Install complete"

install_hard: as_root make_dirs hard_copy_files set_mac rebuild syncreboot
	@echo "Hard install complete"

as_root:
ifneq ($(USER),root)
	$(error Must run as superuser!)
endif

make_dirs:
	if [ ! -d /var/netsender ] ; then \
		mkdir /var/netsender; \
		chmod guo+rwx /var/netsender; \
	fi
	if [ ! -d /var/log/netsender ] ; then \
		mkdir /var/log/netsender; \
		chmod guo+rwx /var/log/netsender; \
	fi

soft_copy_files:
	if [ -f /etc/systemd/system/$(BIN_NAME).service ] ; then \
		echo "/etc/systemd/system/$(BIN_NAME).service left unmodified" ; \
	else \
		bash create_service.sh $(RUN_SCRIPT_DIR) $(BIN_DIR); \
	fi
	systemctl enable $(BIN_NAME).service
	if [ -f /etc/netsender.conf ] ; then \
		echo "/etc/netsender.conf left unmodified" ; \
	else \
		printf "ma $(MA)\ndk $(DK)\n" > /etc/netsender.conf; \
		bash permissions.sh; \
	fi

hard_copy_files:
	if [ -f /etc/systemd/system/$(BIN_NAME).service ] ; then \
		echo "/etc/systemd/system/$(BIN_NAME).service overwritten" ; \
	fi
	bash create_service.sh $(RUN_SCRIPT_DIR) $(BIN_DIR)
	systemctl enable $(BIN_NAME).service
	if [ -f /etc/netsender.conf ] ; then \
		echo "Backed up netsender.conf to /etc/netsender.conf.bak"; \
		cp /etc/netsender.conf /etc/netsender.conf.bak ; \
	fi
	printf "ma $(MA)\ndk $(DK)\n" > /etc/netsender.conf
	bash permissions.sh

set_mac:
	printf "ip link set eth0 address $(MA)\n" > /etc/dhcpcd.enter-hook
	chmod guo+x /etc/dhcpcd.enter-hook

syncreboot:
	if [ -e ../../utils ]; then \
		cd ../../utils; git checkout -f master; git pull; \
	else \
		cd ../../; git clone https://bitbucket.org/ausocean/utils; \
	fi
	cd ../../utils/cmd/syncreboot; make; make install

clean: as_root
	rm -rf /var/netsender
	rm -rf /var/log/netsender
	rm -rf /etc/systemd/system/$(BIN_NAME).service
	rm -rf /etc/netsender.conf
	@echo "Clean complete"

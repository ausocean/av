# Install files and directories required by speaker (speaker driver software).
# This can also create a dhcpcd.enter-hook for setting the MAC address.
# MA and DK can be passed to Make if it is for a hard (first-time) installation:
# 	sudo MA=mac DK=dk make install_hard
# NB: The default (soft) install does not override conf files or set the MAC address.
# USR can also be passsed to customise user under which the code is housed.
USER := $(shell whoami)
PATH := /usr/local/go/bin:$(PATH)
BIN_NAME := speaker
BIN_DIR := /src/bitbucket.org/ausocean/av/cmd/$(BIN_NAME)

.SILENT:make_dirs
.SILENT:hard_copy_files
.SILENT:set_mac
.SILENT:clean

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

hard_copy_files:
ifeq ($(MA),)
	$(error "Must provide MAC using MA=")
endif
ifeq ($(DK),)
	$(error "Must provide device key using DK=")
endif
	@if [ -f /etc/systemd/system/$(BIN_NAME).service ] ; then \
		echo "/etc/systemd/system/$(BIN_NAME).service overwritten" ; \
	fi
	@cp -f $(BIN_NAME).service /etc/systemd/system
	@systemctl enable $(BIN_NAME).service
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
		cd ../../../; git clone https://bitbucket.org/ausocean/utils; \
	fi
	@cd ../../../utils/cmd/syncreboot; make; make install

clean: as_root
	rm -rf /var/netsender
	rm -rf /var/log/netsender
	rm -rf /etc/systemd/system/$(BIN_NAME).service
	rm -rf /etc/netsender.conf
	@echo "Clean complete"

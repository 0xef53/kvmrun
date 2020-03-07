PROJECT_NAME := kvmrun
PROJECT_REPO := github.com/0xef53/kvmrun

CWD := $(shell pwd)

ifeq (,$(wildcard /etc/debian_version))
    SYSTEMD_UNITDIR ?= /usr/lib/systemd/system
else
    SYSTEMD_UNITDIR ?= /lib/systemd/system
endif

BUILD_MOUNTS := \
    -v $(PROJECT_NAME)_pkg:/go/pkg \
    -v $(CWD):/go/$(PROJECT_NAME) \
    -v $(CWD)/scripts/build.sh:/usr/local/bin/build.sh

PKG_MOUNTS := \
    -v $(CWD):/root/source:ro \
    -v $(CWD)/packages:/root/source/packages \
    -v $(CWD)/scripts/build-deb.sh:/usr/local/bin/build-deb.sh

binaries = bin/kvmhelper bin/kvmrund bin/launcher \
           bin/netinit bin/finisher bin/control \
           bin/gencert


.PHONY: all build deb-package clean

all: build

$(binaries):
	@echo "##########################"
	@echo "#  Building binaries     #"
	@echo "##########################"
	@echo
	docker run --rm -i -w /go/$(PROJECT_NAME) $(BUILD_MOUNTS) golang:latest build.sh
	@echo
	@echo "==================="
	@echo "Successfully built:"
	ls -lh bin/
	@echo

build: $(binaries)

install: $(binaries)
	install -d $(DESTDIR)/usr/sbin $(DESTDIR)/usr/lib/kvmrun $(DESTDIR)/etc/kvmrun
	cp -t $(DESTDIR)/usr/lib/kvmrun $(binaries) contrib/svlog/svlog_run
	mv -t $(DESTDIR)/usr/sbin $(DESTDIR)/usr/lib/kvmrun/kvmhelper
	cp -t $(DESTDIR)/etc/kvmrun contrib/kvmrun.ini
	install -d $(DESTDIR)$(SYSTEMD_UNITDIR)
	cp -t $(DESTDIR)$(SYSTEMD_UNITDIR) contrib/kvmrund.service
	install -d $(DESTDIR)/etc/rsyslog.d
	cp -t $(DESTDIR)/etc/rsyslog.d contrib/rsyslog/kvmrun.conf
	install -d $(DESTDIR)/usr/share/kvmrun/tls
	install -d $(DESTDIR)/etc/bash_completion.d
	cp -t $(DESTDIR)/etc/bash_completion.d contrib/bash-completion/kvmhelper
	install -d $(DESTDIR)/usr/share/kvmrun
	cp -t $(DESTDIR)/usr/share/kvmrun scripts/mk-debian-image
	@echo

deb-package: $(binaries)
	@echo "##########################"
	@echo "#  Building deb package  #"
	@echo "##########################"
	@echo
	install -d packages
	docker run --rm -i -w /root/source $(PKG_MOUNTS) 0xef53/debian-dev:latest build-deb.sh
	@echo
	@echo "==================="
	@echo "Successfully built:"
	@find packages -type f -name '*.deb' -printf "%p\n"
	@echo

clean:
	rm -Rvf bin packages
	rm -Rvf debian/changelog

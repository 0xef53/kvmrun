PROJECT_NAME := kvmrun
PROJECT_REPO := github.com/0xef53/$(PROJECT_NAME)

CWD := $(shell pwd)

ifeq (,$(wildcard /etc/debian_version))
    SYSTEMD_UNITDIR ?= /usr/lib/systemd/system
else
    SYSTEMD_UNITDIR ?= /lib/systemd/system
endif

DOCKER_BUILD_ARGS := \
    -w /go/$(PROJECT_NAME) \
    -v $(PROJECT_NAME)-grpc_pkg:/go/pkg \
    -v $(CWD):/go/$(PROJECT_NAME) \
    -v $(CWD)/scripts/build.sh:/usr/local/bin/build.sh \
    -e GOBIN=/go/$(PROJECT_NAME)/bin \
    --entrypoint build.sh

DOCKER_TESTS_ARGS := \
    -w /go/$(PROJECT_NAME) \
    -v $(PROJECT_NAME)-grpc_pkg:/go/pkg \
    -v $(CWD):/go/$(PROJECT_NAME)

DOCKER_PB_ARGS := \
    -w /go/$(PROJECT_NAME) \
    -v $(CWD):/go/$(PROJECT_NAME)

DOCKER_DEB_ARGS := \
    -w /root/source \
    -v $(CWD):/root/source:ro \
    -v $(CWD)/packages:/root/source/packages \
    -v $(CWD)/scripts/build-deb.sh:/usr/local/bin/build-deb.sh \
    -e PROJECT_NAME=$(PROJECT_NAME) \
    --entrypoint build-deb.sh

binaries = \
    bin/kvmrund bin/vmm bin/launcher \
    bin/netinit bin/vnetctl bin/gencert bin/proxy-launcher \
    bin/printpci bin/update-kvmrun-package

scripts = \
    scripts/delegate-cgroup-v1-controller

proto_files = \
    api/types/types.proto \
    api/services/machines/v1/machines.proto \
    api/services/tasks/v1/tasks.proto \
    api/services/system/v1/system.proto \
    api/services/network/v1/network.proto \
    api/services/hardware/v1/hardware.proto \
    api/services/cloudinit/v1/cloudinit.proto

.PHONY: all build clean protobufs $(proto_files)

all: build

$(binaries):
	@echo "##########################"
	@echo "#  Building binaries     #"
	@echo "##########################"
	@echo
	install -d bin
	docker run --rm -it $(DOCKER_BUILD_ARGS) golang:1.18-buster
	@echo
	@echo "==================="
	@echo "Successfully built:"
	ls -lh bin/
	@echo

build: $(binaries)

tests:
	@echo "##########################"
	@echo "#  Running tests         #"
	@echo "##########################"
	@echo
	docker run --rm -i $(DOCKER_TESTS_ARGS) golang:latest go test ./...
	@echo
	@echo

$(proto_files):
	docker run --rm -i $(DOCKER_PB_ARGS) 0xef53/go-proto-compiler:latest \
		--proto_path api \
		--proto_path /go/src/github.com/gogo/googleapis \
		--proto_path /go/src \
		--gogofast_out=plugins=grpc,paths=source_relative,\
	Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
	Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
	Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types:\
	./api $@

protobufs: $(proto_files)

install: $(binaries)
	install -d $(DESTDIR)/usr/bin $(DESTDIR)/usr/lib/$(PROJECT_NAME) $(DESTDIR)/etc/$(PROJECT_NAME)
	cp -t $(DESTDIR)/usr/lib/$(PROJECT_NAME) $(binaries)
	cp -t $(DESTDIR)/usr/lib/$(PROJECT_NAME) $(scripts)
	cp -t $(DESTDIR)/usr/lib/$(PROJECT_NAME) contrib/qemu.wrapper
	ln -fs vnetctl $(DESTDIR)/usr/lib/$(PROJECT_NAME)/ifup
	ln -fs vnetctl $(DESTDIR)/usr/lib/$(PROJECT_NAME)/ifdown
	mv -t $(DESTDIR)/usr/bin $(DESTDIR)/usr/lib/$(PROJECT_NAME)/vmm
	cp -t $(DESTDIR)/etc/$(PROJECT_NAME) contrib/kvmrun.ini
	install -d $(DESTDIR)$(SYSTEMD_UNITDIR)
	cp -t $(DESTDIR)$(SYSTEMD_UNITDIR) contrib/kvmrund.service contrib/kvmrun@.service contrib/kvmrun-proxy@.service
	install -d $(DESTDIR)/etc/rsyslog.d
	cp -t $(DESTDIR)/etc/rsyslog.d contrib/rsyslog/kvmrun.conf
	install -d $(DESTDIR)/usr/share/kvmrun/tls
	install -d $(DESTDIR)/etc/bash_completion.d
	cp -t $(DESTDIR)/etc/bash_completion.d contrib/bash-completion/vmm
	install -d $(DESTDIR)/usr/share/$(PROJECT_NAME)
	cp -t $(DESTDIR)/usr/share/$(PROJECT_NAME) scripts/mk-debian-image
	@echo

deb-package: $(binaries)
	@echo "##########################"
	@echo "#  Building deb package  #"
	@echo "##########################"
	@echo
	install -d packages
	docker run --rm -i $(DOCKER_DEB_ARGS) 0xef53/debian-dev:latest
	@echo
	@echo "==================="
	@echo "Successfully built:"
	@find packages -type f -name '*.deb' -printf "%p\n"
	@echo

clean:
	rm -Rvf bin packages vendor

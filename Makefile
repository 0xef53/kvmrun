PROJECT_NAME := kvmrun
PROJECT_REPO := github.com/0xef53/$(PROJECT_NAME)

GOLANG_IMAGE := golang:1.24-bullseye
DEVTOOLS_IMAGE := 0xef53/devtools:debian-bullseye

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

protofiles_grpc = \
    types/v2/common.proto \
    types/v2/machines.proto \
    types/v2/tasks.proto \
    types/v2/network.proto \
    types/v2/hardware.proto \
    services/machines/v2/machines.proto \
    services/tasks/v2/tasks.proto \
    services/system/v2/system.proto \
    services/network/v2/network.proto \
    services/hardware/v2/hardware.proto \
    services/cloudinit/v2/cloudinit.proto

protofiles_grpc_gw = \
    services/machines/v2/machines.proto \
    services/tasks/v2/tasks.proto

DOCKER_DEB_ARGS := \
    -w /root/source \
    -v $(CWD):/root/source:ro \
    -v $(CWD)/packages:/root/source/packages \
    -v $(CWD)/scripts/build-deb.sh:/usr/local/bin/build-deb.sh \
    -e PROJECT_NAME=$(PROJECT_NAME) \
    --entrypoint build-deb.sh

binaries = \
    bin/kvmrund bin/vmm bin/launcher \
    bin/netinit bin/vnetctl bin/gencert \
    bin/printpci bin/update-kvmrun-package

scripts = \
    scripts/delegate-cgroup-v1-controller

.PHONY: all build clean protobufs $(proto_files)

all: build

$(binaries):
	@echo "##########################"
	@echo "#  Building binaries     #"
	@echo "##########################"
	@echo
	install -d bin
	docker run --rm -it $(DOCKER_BUILD_ARGS) $(GOLANG_IMAGE)
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
	docker run --rm -i $(DOCKER_TESTS_ARGS) $(GOLANG_IMAGE) go test ./...
	@echo
	@echo

protobufs:
	docker run --rm -it $(DOCKER_PB_ARGS) 0xef53/go-proto-compiler:v3.18 \
		--proto_path api \
		--go_opt "plugins=grpc,paths=source_relative" \
		--go_out ./api \
		$(protofiles_grpc)
	docker run --rm -it $(DOCKER_PB_ARGS) 0xef53/go-proto-compiler:v3.18 \
		--proto_path api \
		--grpc-gateway_opt "logtostderr=true,paths=source_relative" \
		--grpc-gateway_out ./api \
		$(protofiles_grpc_gw)
	scripts/fix-proto-names.sh $(shell find api/ -type f -name '*.pb.go')

install: $(binaries)
	install -d $(DESTDIR)/usr/bin $(DESTDIR)/usr/lib/$(PROJECT_NAME) $(DESTDIR)/etc/$(PROJECT_NAME)
	cp -t $(DESTDIR)/usr/lib/$(PROJECT_NAME) $(binaries)
	cp -t $(DESTDIR)/usr/lib/$(PROJECT_NAME) $(scripts)
	cp -t $(DESTDIR)/usr/lib/$(PROJECT_NAME) contrib/qemu.wrapper
	ln -fs vnetctl $(DESTDIR)/usr/lib/$(PROJECT_NAME)/ifup
	ln -fs vnetctl $(DESTDIR)/usr/lib/$(PROJECT_NAME)/ifdown
	mv -t $(DESTDIR)/usr/bin $(DESTDIR)/usr/lib/$(PROJECT_NAME)/vmm
	ln -fs /usr/lib/$(PROJECT_NAME)/vnetctl $(DESTDIR)/usr/bin/vnetctl
	cp -t $(DESTDIR)/etc/$(PROJECT_NAME) contrib/kvmrun.ini
	install -d $(DESTDIR)$(SYSTEMD_UNITDIR)
	cp -t $(DESTDIR)$(SYSTEMD_UNITDIR) contrib/kvmrund.service contrib/kvmrun@.service
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
	docker run --rm -i $(DOCKER_DEB_ARGS) $(DEVTOOLS_IMAGE)
	@echo
	@echo "==================="
	@echo "Successfully built:"
	@find packages -type f -name '*.deb' -printf "%p\n"
	@echo

clean:
	rm -Rvf bin packages vendor

#!/usr/bin/make -f

# Copyright (c) 2022  The Go-Enjin Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#: uncomment to echo instead of execute
#CMD=echo

BE_PATH ?= ../be

BIN_NAME ?= enjenv


PWD = $(shell pwd)
SHELL = /bin/bash

BUILD_OS ?= linux
BUILD_ARCH ?= `uname -m | perl -pe 's!aarch64!arm64!;s!x86_64!amd64!;'`

prefix ?= /usr
BIN_PATH ?= ${DESTDIR}${prefix}/bin
ETC_PATH ?= ${DESTDIR}/etc

SYSTEMD_PATH ?= ${ETC_PATH}/systemd/system
NISEROKU_PATH ?= ${ETC_PATH}/niseroku
LOGROTATE_PATH ?= ${ETC_PATH}/logrotate.d
SYSV_INIT_PATH ?= ${ETC_PATH}/init.d
AUTOCOMPLETE_PATH ?= ${ETC_PATH}/bash_completion.d

ENJENV_AUTOCOMPLETE_FILE ?= ${AUTOCOMPLETE_PATH}/enjenv
NISEROKU_AUTOCOMPLETE_FILE ?= ${AUTOCOMPLETE_PATH}/niseroku
NISEROKU_TOML_FILE ?= ${NISEROKU_PATH}/niseroku.toml
NISEROKU_LOGROTATE_FILE ?= ${LOGROTATE_PATH}/niseroku

NISEROKU_PROXY_SERVICE_FILE ?= ${SYSTEMD_PATH}/niseroku-proxy.service
NISEROKU_PROXY_SYSV_INIT_FILE ?= ${SYSV_INIT_PATH}/niseroku-proxy

NISEROKU_REPOS_SERVICE_FILE ?= ${SYSTEMD_PATH}/niseroku-repos.service
NISEROKU_REPOS_SYSV_INIT_FILE ?= ${SYSV_INIT_PATH}/niseroku-repos

define _trim_path =
$(shell \
if [ "${GOPATH}" != "" ]; then \
	echo "${GOPATH};${PWD}"; \
else \
	echo "${PWD}"; \
fi)
endef

define _tag_ver =
$(shell (git describe 2> /dev/null) || echo "untagged")
endef

GIT_STATUS := $(git status 2> /dev/null)

define _rel_ver =
$(shell \
	if [ "$(GIT_STATUS)" = "" ]; then \
		git rev-parse --short=10 HEAD; \
	else \
		git diff 2> /dev/null \
			| sha256sum - 2> /dev/null \
			| perl -pe 's!^\s*([a-f0-9]{10}).*!\1!'; \
	fi \
)
endef

# 1: bin-name, 2: goos, 3: goarch
define _build_target =
	echo "# building $(2)-$(3) (release): ${BIN_NAME} (${BUILD_VERSION}, ${BUILD_RELEASE})"; \
	${CMD} GOOS="$(2)" GOARCH="$(3)" go build -v \
		-o "$(1)" \
		-ldflags="\
-s -w \
-buildid='' \
-X 'github.com/go-enjin/enjenv/pkg/globals.BuildVersion=${BUILD_VERSION}' \
-X 'github.com/go-enjin/enjenv/pkg/globals.BuildRelease=${BUILD_RELEASE}'\
" \
		-gcflags="-trimpath='${TRIM_PATHS}'" \
		-asmflags="-trimpath='${TRIM_PATHS}'" \
		-trimpath \
		./cmd/enjenv || exit 1
endef

define _build_debug =
	@echo "# building $(2)-$(3) (debug): ${BIN_NAME} (${BUILD_VERSION}, ${BUILD_RELEASE})"
	@${CMD} GOOS="$(2)" GOARCH="$(3)" go build -v \
		-o "$(1)" \
		-gcflags="all=-N -l" \
		-ldflags="\
-buildid='' \
-X 'github.com/go-enjin/enjenv/pkg/globals.BuildVersion=${BUILD_VERSION}' \
-X 'github.com/go-enjin/enjenv/pkg/globals.BuildRelease=${BUILD_RELEASE}'\
" \
		-gcflags="-trimpath='${TRIM_PATHS}'" \
		-asmflags="-trimpath='${TRIM_PATHS}'" \
		-trimpath \
		./cmd/enjenv
endef

define _upx_build =
	if [ -x /usr/bin/upx ]; then \
		echo -n "# packing: $(1) - "; \
		du -hs "$(1)" | awk '{print $$1}'; \
		/usr/bin/upx -qq -7 --no-color --no-progress "$(1)"; \
		echo -n "# packed: $(1) - "; \
		du -hs "$(1)" | awk '{print $$1}'; \
		sha256sum "$(1)"; \
	else \
		echo "# upx command not found, skipping binary packing stage"; \
	fi
endef

.PHONY: all help clean build install local unlocal tidy

help:
	@echo "usage: make <help|clean|local|unlocal|tidy>"
	@echo "       make <build|build-all|build-amd64|build-arm64>"
	@echo "       make <install>"
	@echo "       make <install-autocomplete>"
	@echo "       make <install-niseroku>"
	@echo "       make <install-niseroku-systemd>"
	@echo "       make <install-niseroku-logrotate>"
	@echo "       make <install-niseroku-sysv-init>"

clean:
	@rm -fv "${BIN_NAME}"
	@rm -fv ${BIN_NAME}.*.*

distclean: clean
	@rm -rfv _dist

debug: BUILD_VERSION=$(call _tag_ver)
debug: BUILD_RELEASE=$(call _rel_ver)
debug: TRIM_PATHS=$(call _trim_path)
debug:
	@$(call _build_debug,"${BIN_NAME}",`go env GOOS`,`go env GOARCH`)

build: BUILD_VERSION=$(call _tag_ver)
build: BUILD_RELEASE=$(call _rel_ver)
build: TRIM_PATHS=$(call _trim_path)
build:
	@$(call _build_target,"${BIN_NAME}.linux.${BUILD_ARCH}",${BUILD_OS},${BUILD_ARCH})

build-amd64: BUILD_VERSION=$(call _tag_ver)
build-amd64: BUILD_RELEASE=$(call _rel_ver)
build-amd64: TRIM_PATHS=$(call _trim_path)
build-amd64: export CGO_ENABLED=1
build-amd64: export CC=x86_64-linux-gnu-gcc
build-amd64:
	@$(call _build_target,"${BIN_NAME}.linux.amd64",linux,amd64)
	@sha256sum "${BIN_NAME}.linux.amd64"

build-arm64: BUILD_VERSION=$(call _tag_ver)
build-arm64: BUILD_RELEASE=$(call _rel_ver)
build-arm64: TRIM_PATHS=$(call _trim_path)
build-arm64: export CGO_ENABLED=1
build-arm64: export CC=aarch64-linux-gnu-gcc
build-arm64:
	@$(call _build_target,"${BIN_NAME}.linux.arm64",linux,arm64)
	@sha256sum "${BIN_NAME}.linux.arm64"

build-all: build-amd64 build-arm64

release: build
	@$(call _upx_build,"${BIN_NAME}.linux.${BUILD_ARCH}")

release-arm64: build-arm64
	@$(call _upx_build,"${BIN_NAME}.linux.arm64")

release-amd64: build-amd64
	@$(call _upx_build,"${BIN_NAME}.linux.amd64")

release-all: release-amd64 release-arm64

define _install_build =
	echo "# installing $(1) to: $(2)"; \
	[ -d "${BIN_PATH}" ] || mkdir -vp "${BIN_PATH}"; \
	${CMD} /usr/bin/install -v -m 0775 -T "$(1)" "${BIN_PATH}/$(2)"; \
	${CMD} sha256sum "${BIN_PATH}/$(2)"
endef

install:
	@if [ -f "enjenv.linux.${BUILD_ARCH}" ]; then \
		$(call _install_build,"enjenv.linux.${BUILD_ARCH}","enjenv"); \
	else \
		echo "error: missing enjenv.linux.${BUILD_ARCH} binary" 1>&2; \
	fi

install-autocomplete:
	@[ -d "${AUTOCOMPLETE_PATH}" ] || mkdir -vp "${AUTOCOMPLETE_PATH}"
	@echo "# installing enjenv bash_autocomplete to: ${ENJENV_AUTOCOMPLETE_FILE}"
	@${CMD} /usr/bin/install -v -m 0775 -T "_templates/bash_autocomplete" "${ENJENV_AUTOCOMPLETE_FILE}"
	@${CMD} sha256sum "${ENJENV_AUTOCOMPLETE_FILE}"

install-niseroku:
	@[ -d "${NISEROKU_PATH}" ] || mkdir -vp "${NISEROKU_PATH}"
	@if [ -f "${NISEROKU_TOML_FILE}" ]; then \
		echo "# skipping ${NISEROKU_TOML_FILE} (exists already)"; \
	else \
		echo "# installing ${NISEROKU_TOML_FILE}"; \
		if [ ! -d "${NISEROKU_PATH}" ]; then mkdir -p "${NISEROKU_PATH}"; fi; \
		${CMD} /usr/bin/install -v -b -m 0664 -T "_templates/niseroku.toml" "${NISEROKU_TOML_FILE}"; \
		${CMD} sha256sum "${NISEROKU_TOML_FILE}"; \
	fi

install-niseroku-logrotate:
	@[ -d "${LOGROTATE_PATH}" ] || mkdir -vp "${LOGROTATE_PATH}"
	@echo "# installing ${NISEROKU_LOGROTATE_FILE}"
	@${CMD} /usr/bin/install -v -b -m 0664 -T "_templates/niseroku.logrotate" "${NISEROKU_LOGROTATE_FILE}"
	@${CMD} sha256sum "${NISEROKU_LOGROTATE_FILE}"

install-niseroku-sysv-init:
	@[ -d "${SYSV_INIT_PATH}" ] || mkdir -vp "${SYSV_INIT_PATH}"
	@echo "# installing ${NISEROKU_PROXY_SYSV_INIT_FILE}"
	@${CMD} /usr/bin/install -v -b -m 0775 -T "_templates/niseroku-proxy.init" "${NISEROKU_PROXY_SYSV_INIT_FILE}"
	@${CMD} sha256sum "${NISEROKU_PROXY_SYSV_INIT_FILE}"
	@echo "# installing ${NISEROKU_REPOS_SYSV_INIT_FILE}"
	@${CMD} /usr/bin/install -v -b -m 0775 -T "_templates/niseroku-repos.init" "${NISEROKU_REPOS_SYSV_INIT_FILE}"
	@${CMD} sha256sum "${NISEROKU_REPOS_SYSV_INIT_FILE}"

install-niseroku-systemd:
	@[ -d "${SYSTEMD_PATH}" ] || mkdir -vp "${SYSTEMD_PATH}"
	@echo "# installing ${NISEROKU_PROXY_SERVICE_FILE}"
	@${CMD} /usr/bin/install -v -b -m 0664 -T "_templates/niseroku-proxy.service" "${NISEROKU_PROXY_SERVICE_FILE}"
	@$${CMD} sha256sum "${NISEROKU_PROXY_SERVICE_FILE}"
	@echo "# installing ${NISEROKU_REPOS_SERVICE_FILE}"
	@${CMD} /usr/bin/install -v -b -m 0664 -T "_templates/niseroku-repos.service" "${NISEROKU_REPOS_SERVICE_FILE}"
	@${CMD} sha256sum "${NISEROKU_REPOS_SERVICE_FILE}"

install-niseroku-utils:
	@if [ -f "_templates/niseroku.sh" ]; then \
		echo "# installing niseroku wrapper script"; \
		$(call _install_build,"_templates/niseroku.sh","niseroku"); \
	else \
		echo "error: missing niseroku wrapper script" 1>&2; \
	fi
	@if [ -f "_templates/niseroku-tail.sh" ]; then \
		echo "# installing niseroku-tail wrapper script"; \
		$(call _install_build,"_templates/niseroku-tail.sh","niseroku-tail"); \
	else \
		echo "error: missing niseroku-tail wrapper script" 1>&2; \
	fi
	@[ -d "${AUTOCOMPLETE_PATH}" ] || mkdir -vp "${AUTOCOMPLETE_PATH}"
	@echo "# installing niseroku bash_autocomplete to: ${NISEROKU_AUTOCOMPLETE_FILE}"
	@${CMD} /usr/bin/install -v -m 0775 -T "_templates/bash_autocomplete" "${NISEROKU_AUTOCOMPLETE_FILE}"
	@${CMD} sha256sum "${NISEROKU_AUTOCOMPLETE_FILE}"

local:
	@if [ -d "${BE_PATH}" ]; then \
		go mod edit -replace="github.com/go-enjin/be=${BE_PATH}"; \
	else \
		echo "BE_PATH not set or not a directory: \"${BE_PATH}\""; \
	fi

unlocal:
	@go mod edit -dropreplace="github.com/go-enjin/be"

tidy:
	@go mod tidy

be-update: export GOPROXY=direct
be-update:
	@go get github.com/go-enjin/be@latest
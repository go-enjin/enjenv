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

#BUILD_ALL_GOOS = linux darwin
BUILD_ALL_GOOS = linux
BUILD_ALL_GOARCH = amd64 arm64

prefix ?= /usr/local
BIN_PATH ?= ${DESTDIR}${prefix}/bin
ETC_PATH ?= ${DESTDIR}${prefix}/etc
SYSTEMD_PATH ?= ${ETC_PATH}/systemd/system
NISEROKU_PATH ?= ${ETC_PATH}/niseroku

NISEROKU_TOML_FILE ?= ${NISEROKU_PATH}/niseroku.toml
NISEROKU_SERVICE_FILE ?= ${SYSTEMD_PATH}/niseroku.service

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
		-ldflags="-w -s -buildid='' -X 'main.BuildVersion=${BUILD_VERSION}' -X 'main.BuildRelease=${BUILD_RELEASE}'" \
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
-X 'main.BuildVersion=${BUILD_VERSION}' \
-X 'main.BuildRelease=${BUILD_RELEASE}' \
" \
		-gcflags="-trimpath='${TRIM_PATHS}'" \
		-asmflags="-trimpath='${TRIM_PATHS}'" \
		-trimpath \
		./cmd/enjenv
endef

.PHONY: all help clean build install local unlocal tidy

help:
	@echo "usage: make <help|clean|local|unlocal|tidy>"
	@echo "       make <build|build-all|build-amd64|build-arm64>"
	@echo "       make <install|install-niseroku>"

clean:
	@if [ -f "${BIN_NAME}" ]; then rm -fv "${BIN_NAME}"; fi
	@rm -fv ${BIN_NAME}.*.*

debug: BUILD_VERSION=$(call _tag_ver)
debug: BUILD_RELEASE=$(call _rel_ver)
debug: TRIM_PATHS=$(call _trim_path)
debug:
	@$(call _build_debug,"${BIN_NAME}",`go env GOOS`,`go env GOARCH`)

build: BUILD_VERSION=$(call _tag_ver)
build: BUILD_RELEASE=$(call _rel_ver)
build: TRIM_PATHS=$(call _trim_path)
build:
	@$(call _build_target,"${BIN_NAME}",`go env GOOS`,`go env GOARCH`)

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

install:
	@if [ ! -f enjenv ]; then \
		echo "error: missing enjenv binary"; \
		false; \
	fi
	@if [ -d "${BIN_PATH}" ]; then \
			echo "# installing enjenv to: ${BIN_PATH}"; \
			${CMD} /usr/bin/install -t ${BIN_PATH} -v enjenv; \
			${CMD} sha256sum "${BIN_PATH}/enjenv"; \
	fi

install-niseroku-systemd: install
	@if [ ! -d "${SYSTEMD_PATH}" ]; then \
		echo "# error: SYSTEMD_PATH not found - ${SYSTEMD_PATH}"; \
		false; \
	fi
	@echo "# installing niseroku service configs and paths"
	@echo "#   niseroku cfg: ${NISEROKU_TOML_FILE}"
	@echo "#   service file: ${NISEROKU_SERVICE_FILE}"
	@if [ ! -d "${NISEROKU_PATH}" ]; then \
		mkdir -vp "${NISEROKU_PATH}"; \
	fi
	@${CMD} /usr/bin/install -t ${NISEROKU_PATH} -m 0664 \
		-v _templates/niseroku.toml
	@cat _templates/systemd/niseroku.service \
		| perl -pe "s!{ENJENV_PATH}!${BIN_PATH}/enjenv!msg" \
		> "${NISEROKU_SERVICE_FILE}"
	@sha256sum "${NISEROKU_SERVICE_FILE}"
	@sha256sum "${NISEROKU_PATH}/niseroku.toml"
	@systemctl daemon-reload

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

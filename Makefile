#!/usr/bin/make --no-print-directory --jobs=1 --environment-overrides -f

# Copyright (c) 2023  The Go-Enjin Authors
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

-include .env
#export

.PHONY: all help
.PHONY: clean distclean realclean
.PHONY: local unlocal tidy be-update
.PHONY: debug build build-all build-amd64 build-arm64
.PHONY: release release-all release-amd64 release-arm64
.PHONY: profile.proxy.cpu profile.proxy.mem
.PHONY: profile.repos.cpu profile.repos.mem
.PHONY: profile.watch.cpu profile.watch.mem
.PHONY: install install-autocomplete
.PHONY: install-niseroku install-niseroku-logrotate install-niseroku-utils
.PHONY: install-niseroku-systemd install-niseroku-sysv-init

BE_PATH ?= ../be
CDK_PATH ?= ../../go-curses/cdk
CTK_PATH ?= ../../go-curses/ctk

BIN_NAME ?= enjenv
UNTAGGED_VERSION ?= v0.1.6
UNTAGGED_COMMIT ?= 0000000000

PWD = $(shell pwd)
SHELL = /bin/bash

BUILD_OS   := `uname -os | awk '{print $$1}' | perl -pe '$$_=lc($$_)'`
BUILD_ARCH := `uname -m | perl -pe 's!aarch64!arm64!;s!x86_64!amd64!;'`

prefix ?= /usr

GIT_STATUS := $([ -d .git ] && git status 2> /dev/null)

CLEAN_FILES     ?= "${BIN_NAME}" ${BIN_NAME}.*.* pprof.{proxy,repos,watch}
DISTCLEAN_FILES ?=
REALCLEAN_FILES ?=

UPX_BIN := $(shell which upx)

define _trim_path =
$(shell \
if [ "${GOPATH}" != "" ]; then \
	echo "${GOPATH};${PWD}"; \
else \
	echo "${PWD}"; \
fi)
endef

define _tag_ver =
$(shell ([ -d .git ] && git describe 2> /dev/null) || echo "${UNTAGGED_VERSION}")
endef

define _rel_ver =
$(shell \
	if [ -d .git ]; then \
		if [ -z "${GIT_STATUS}" ]; then \
			git rev-parse --short=10 HEAD; \
		else \
			[ -d .git ] && git diff 2> /dev/null \
				| sha256sum - 2> /dev/null \
				| perl -pe 's!^\s*([a-f0-9]{10}).*!\1!'; \
		fi; \
	else \
		echo "${UNTAGGED_COMMIT}"; \
	fi \
)
endef


# 1: bin-name, 2: goos, 3: goarch, 4: ldflags, 5: gcflags, 6: asmflags, 7: argv
define _cmd_go_build
$(shell echo "\
GOOS=\"$(2)\" GOARCH=\"$(3)\" \
go build -v \
		-o \"$(1)\" \
		-ldflags=\"$(4) \
-buildid='' \
-X 'github.com/go-enjin/enjenv/pkg/globals.BuildVersion=${BUILD_VERSION}' \
-X 'github.com/go-enjin/enjenv/pkg/globals.BuildRelease=${BUILD_RELEASE}' \
\" \
		-gcflags=\"$(5)\" \
		-asmflags=\"$(6)\" \
		$(7) \
		./cmd/enjenv")
endef

# 1: bin-name, 2: goos, 3: goarch, 4: ldflags
define _cmd_go_build_trimpath
$(call _cmd_go_build,$(1),$(2),$(3),$(4),-trimpath='${TRIM_PATHS}',-trimpath='${TRIM_PATHS}',-trimpath)
endef

# 1: bin-name, 2: goos, 3: goarch
define _build_target =
	echo "# building $(2)-$(3) (release): ${BIN_NAME} (${BUILD_VERSION}, ${BUILD_RELEASE})"; \
	echo $(call _cmd_go_build_trimpath,$(1),$(2),$(3),-s -w); \
	$(call _cmd_go_build_trimpath,$(1),$(2),$(3),-s -w)
endef

# 1: bin-name, 2: goos, 3: goarch
define _build_debug =
	echo "# building $(2)-$(3) (debug): ${BIN_NAME} (${BUILD_VERSION}, ${BUILD_RELEASE})"; \
	echo $(call _cmd_go_build,$(1),$(2),$(3),,-N -l); \
	$(call _cmd_go_build,$(1),$(2),$(3),,-N -l)
endef

define _upx_build =
	if [ -n "${UPX_BIN}" -a -x "${UPX_BIN}" ]; then \
		echo -n "# packing: $(1) - "; \
		du -hs "$(1)" | awk '{print $$1}'; \
		${UPX_BIN} -qq -7 --no-color --no-progress "$(1)"; \
		echo -n "# packed: $(1) - "; \
		du -hs "$(1)" | awk '{print $$1}'; \
		sha256sum "$(1)"; \
	else \
		echo "# upx command not found, skipping binary packing stage"; \
	fi
endef

define _profile_run =
	@if [ -f "${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}" ]; then \
		echo "# starting niseroku $(1)..."; \
		case "$(1)" in \
			"proxy") \
				./${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH} niseroku reverse-proxy;; \
			"repos") \
				./${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH} niseroku git-repository;; \
			"watch") \
				./${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH} niseroku status watch;; \
		esac; \
		if [ -f pprof.$(1)/$(2).pprof ]; then \
			echo "# ./pprof.$(1)/$(2).pprof found; ready to run pprof"; \
			echo "# press <ENTER> to continue, <CTRL+c> to stop"; \
			read -N 1 -s -p "" JUNK; \
			echo "# running: go tool pprof --http:12345 ..."; \
			( go tool pprof --http=:12345 ./pprof.$(1)/$(2).pprof 2>/dev/null ); \
		else \
			echo "# ./pprof.$(1)/$(2).pprof not found"; \
		fi; \
	else \
		echo "# ${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH} not found"; \
	fi
endef

define _clean =
	for FOUND in $(1); do \
		if [ -n "$${FOUND}" ]; then \
			rm -rfv $${FOUND}; \
		fi; \
	done
endef

help:
	@echo "usage: make <help|clean|local|unlocal|tidy>"
	@echo "       make <debug>"
	@echo "       make <build|build-amd64|build-arm64|build-all>"
	@echo "       make <release|release-amd64|release-arm64|release-all>"
	@echo "       make <profile.proxy.cpu|profile.proxy.mem>"
	@echo "       make <profile.repos.cpu|profile.repos.mem>"
	@echo "       make <profile.watch.cpu|profile.watch.mem>"
	@echo "       make <install>"
	@echo "       make <install-autocomplete>"
	@echo "       make <install-niseroku>"
	@echo "       make <install-niseroku-systemd>"
	@echo "       make <install-niseroku-logrotate>"
	@echo "       make <install-niseroku-sysv-init>"

clean:
	@$(call _clean,${CLEAN_FILES})

distclean: clean
	@$(call _clean,${DISTCLEAN_FILES})

realclean: distclean
	@$(call _clean,${REALCLEAN_FILES})

debug: BUILD_VERSION=$(call _tag_ver)
debug: BUILD_RELEASE=$(call _rel_ver)
debug: TRIM_PATHS=$(call _trim_path)
debug: export CGO_ENABLED=1
ifeq (${BUILD_OS},linux)
ifeq (${BUILD_ARCH},arm64)
debug: export CC=aarch64-linux-gnu-gcc
else
debug: export CC=x86_64-linux-gnu-gcc
endif
endif
debug:
	@$(call _build_debug,"${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}",${BUILD_OS},${BUILD_ARCH})
	@sha256sum "${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}"

profile.proxy.cpu: export ENJENV_ENABLE_PROFILING=true
profile.proxy.cpu: export ENJENV_PROFILING_TYPE=cpu
profile.proxy.cpu: export ENJENV_PROFILING_PATH=./pprof.proxy
profile.proxy.cpu: debug
	@$(call _profile_run,"proxy","cpu")

profile.proxy.mem: export ENJENV_ENABLE_PROFILING=true
profile.proxy.mem: export ENJENV_PROFILING_TYPE=mem
profile.proxy.mem: export ENJENV_PROFILING_PATH=./pprof.proxy
profile.proxy.mem: debug
	@$(call _profile_run,"proxy","mem")

profile.repos.cpu: export ENJENV_ENABLE_PROFILING=true
profile.repos.cpu: export ENJENV_PROFILING_TYPE=cpu
profile.repos.cpu: export ENJENV_PROFILING_PATH=./pprof.repos
profile.repos.cpu: debug
	@$(call _profile_run,"repos","cpu")

profile.repos.mem: export ENJENV_ENABLE_PROFILING=true
profile.repos.mem: export ENJENV_PROFILING_TYPE=mem
profile.repos.mem: export ENJENV_PROFILING_PATH=./pprof.repos
profile.repos.mem: debug
	@$(call _profile_run,"repos","mem")

profile.watch.cpu: export ENJENV_ENABLE_PROFILING=true
profile.watch.cpu: export ENJENV_PROFILING_TYPE=cpu
profile.watch.cpu: export ENJENV_PROFILING_PATH=./pprof.watch
profile.watch.cpu: debug
	@$(call _profile_run,"watch","cpu")

profile.watch.mem: export ENJENV_ENABLE_PROFILING=true
profile.watch.mem: export ENJENV_PROFILING_TYPE=mem
profile.watch.mem: export ENJENV_PROFILING_PATH=./pprof.watch
profile.watch.mem: debug
	@$(call _profile_run,"watch","mem")

build: BUILD_VERSION=$(call _tag_ver)
build: BUILD_RELEASE=$(call _rel_ver)
build: TRIM_PATHS=$(call _trim_path)
build: export CGO_ENABLED=1
ifeq (${BUILD_OS},linux)
ifeq (${BUILD_ARCH},arm64)
build: export CC=aarch64-linux-gnu-gcc
else
build: export CC=x86_64-linux-gnu-gcc
endif
endif
build:
	@$(call _build_target,"${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}",${BUILD_OS},${BUILD_ARCH})
	@sha256sum "${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}"

build-amd64: BUILD_VERSION=$(call _tag_ver)
build-amd64: BUILD_RELEASE=$(call _rel_ver)
build-amd64: TRIM_PATHS=$(call _trim_path)
build-amd64: export CGO_ENABLED=1
build-amd64: export CC=x86_64-linux-gnu-gcc
build-amd64:
	@$(call _build_target,"${BIN_NAME}.${BUILD_OS}.amd64",${BUILD_OS},amd64)
	@sha256sum "${BIN_NAME}.${BUILD_OS}.amd64"

build-arm64: BUILD_VERSION=$(call _tag_ver)
build-arm64: BUILD_RELEASE=$(call _rel_ver)
build-arm64: TRIM_PATHS=$(call _trim_path)
build-arm64: export CGO_ENABLED=1
build-arm64: export CC=aarch64-linux-gnu-gcc
build-arm64:
	@$(call _build_target,"${BIN_NAME}.${BUILD_OS}.arm64",${BUILD_OS},arm64)
	@sha256sum "${BIN_NAME}.${BUILD_OS}.arm64"

build-all: build-amd64 build-arm64

release: build
	@$(call _upx_build,"${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}")

release-arm64: build-arm64
	@$(call _upx_build,"${BIN_NAME}.${BUILD_OS}.arm64")

release-amd64: build-amd64
	@$(call _upx_build,"${BIN_NAME}.${BUILD_OS}.amd64")

release-all: release-amd64 release-arm64

define _install_build =
	BIN_PATH="${DESTDIR}${prefix}/bin"; \
	echo "# installing $(1) to: $${BIN_PATH}/$(2)"; \
	[ -d "$${BIN_PATH}" ] || mkdir -vp "$${BIN_PATH}"; \
	${CMD} /usr/bin/install -v -m 0775 -T "$(1)" "$${BIN_PATH}/$(2)"; \
	${CMD} sha256sum "$${BIN_PATH}/$(2)"
endef

install:
	@if [ -f "${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}" ]; then \
		$(call _install_build,"${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}","${BIN_NAME}"); \
	else \
		echo "error: missing ${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH} binary" 1>&2; \
	fi

install-autocomplete: ETC_PATH=${DESTDIR}/etc
install-autocomplete: AUTOCOMPLETE_PATH=${ETC_PATH}/bash_completion.d
install-autocomplete: ENJENV_AUTOCOMPLETE_FILE=${AUTOCOMPLETE_PATH}/${BIN_NAME}
install-autocomplete: NISEROKU_AUTOCOMPLETE_FILE=${AUTOCOMPLETE_PATH}/niseroku
install-autocomplete:
	@[ -d "${AUTOCOMPLETE_PATH}" ] || mkdir -vp "${AUTOCOMPLETE_PATH}"
	@echo "# installing ${BIN_NAME} bash_autocomplete to: ${ENJENV_AUTOCOMPLETE_FILE}"
	@${CMD} /usr/bin/install -v -m 0775 -T "_templates/bash_autocomplete" "${ENJENV_AUTOCOMPLETE_FILE}"
	@${CMD} sha256sum "${ENJENV_AUTOCOMPLETE_FILE}"
	@echo "# installing niseroku bash_autocomplete to: ${NISEROKU_AUTOCOMPLETE_FILE}"
	@${CMD} /usr/bin/install -v -m 0775 -T "_templates/bash_autocomplete" "${NISEROKU_AUTOCOMPLETE_FILE}"
	@${CMD} sha256sum "${NISEROKU_AUTOCOMPLETE_FILE}"

install-niseroku: ETC_PATH=${DESTDIR}/etc
install-niseroku: NISEROKU_PATH=${ETC_PATH}/niseroku
install-niseroku: NISEROKU_TOML_FILE=${NISEROKU_PATH}/niseroku.toml
install-niseroku:
	@if [ -f "${NISEROKU_TOML_FILE}" ]; then \
		echo "# skipping ${NISEROKU_TOML_FILE} (exists already)"; \
	else \
		echo "# installing ${NISEROKU_TOML_FILE}"; \
		[ -d "${NISEROKU_PATH}" ] || mkdir -vp "${NISEROKU_PATH}"; \
		if [ ! -d "${NISEROKU_PATH}" ]; then mkdir -p "${NISEROKU_PATH}"; fi; \
		${CMD} /usr/bin/install -v -b -m 0664 -T "_templates/niseroku.toml" "${NISEROKU_TOML_FILE}"; \
		${CMD} sha256sum "${NISEROKU_TOML_FILE}"; \
	fi

install-niseroku-logrotate: ETC_PATH=${DESTDIR}/etc
install-niseroku-logrotate: LOGROTATE_PATH=${ETC_PATH}/logrotate.d
install-niseroku-logrotate: NISEROKU_LOGROTATE_FILE=${LOGROTATE_PATH}/niseroku
install-niseroku-logrotate:
	@echo "# installing ${NISEROKU_LOGROTATE_FILE}"
	@[ -d "${LOGROTATE_PATH}" ] || mkdir -vp "${LOGROTATE_PATH}"
	@${CMD} /usr/bin/install -v -b -m 0664 -T "_templates/niseroku.logrotate" "${NISEROKU_LOGROTATE_FILE}"
	@${CMD} sha256sum "${NISEROKU_LOGROTATE_FILE}"

install-niseroku-sysv-init: ETC_PATH=${DESTDIR}/etc
install-niseroku-sysv-init: SYSV_INIT_PATH=${ETC_PATH}/init.d
install-niseroku-sysv-init: NISEROKU_PROXY_SYSV_INIT_FILE=${SYSV_INIT_PATH}/niseroku-proxy
install-niseroku-sysv-init: NISEROKU_REPOS_SYSV_INIT_FILE=${SYSV_INIT_PATH}/niseroku-repos
install-niseroku-sysv-init:
	@echo "# installing ${NISEROKU_PROXY_SYSV_INIT_FILE}"
	@[ -d "${SYSV_INIT_PATH}" ] || mkdir -vp "${SYSV_INIT_PATH}"
	@${CMD} /usr/bin/install -v -b -m 0775 -T "_templates/niseroku-proxy.init" "${NISEROKU_PROXY_SYSV_INIT_FILE}"
	@${CMD} sha256sum "${NISEROKU_PROXY_SYSV_INIT_FILE}"
	@echo "# installing ${NISEROKU_REPOS_SYSV_INIT_FILE}"
	@${CMD} /usr/bin/install -v -b -m 0775 -T "_templates/niseroku-repos.init" "${NISEROKU_REPOS_SYSV_INIT_FILE}"
	@${CMD} sha256sum "${NISEROKU_REPOS_SYSV_INIT_FILE}"

install-niseroku-systemd: ETC_PATH=${DESTDIR}/etc
install-niseroku-systemd: SYSTEMD_PATH=${ETC_PATH}/systemd/system
install-niseroku-systemd: NISEROKU_PROXY_SERVICE_FILE=${SYSTEMD_PATH}/niseroku-proxy.service
install-niseroku-systemd: NISEROKU_REPOS_SERVICE_FILE=${SYSTEMD_PATH}/niseroku-repos.service
install-niseroku-systemd:
	@[ -d "${SYSTEMD_PATH}" ] || mkdir -vp "${SYSTEMD_PATH}"
	@echo "# installing ${NISEROKU_PROXY_SERVICE_FILE}"
	@${CMD} /usr/bin/install -v -b -m 0664 -T "_templates/niseroku-proxy.service" "${NISEROKU_PROXY_SERVICE_FILE}"
	@$${CMD} sha256sum "${NISEROKU_PROXY_SERVICE_FILE}"
	@echo "# installing ${NISEROKU_REPOS_SERVICE_FILE}"
	@${CMD} /usr/bin/install -v -b -m 0664 -T "_templates/niseroku-repos.service" "${NISEROKU_REPOS_SERVICE_FILE}"
	@${CMD} sha256sum "${NISEROKU_REPOS_SERVICE_FILE}"

install-niseroku-utils: ETC_PATH=${DESTDIR}/etc
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

local:
	@if [ -d "${BE_PATH}" ]; then \
		go mod edit -replace="github.com/go-enjin/be=${BE_PATH}"; \
	else \
		echo "BE_PATH not set or not a directory: \"${BE_PATH}\""; \
	fi
	@if [ -d "${CDK_PATH}" ]; then \
		go mod edit -replace="github.com/go-curses/cdk=${CDK_PATH}"; \
	else \
		echo "CDK_PATH not set or not a directory: \"${CDK_PATH}\""; \
	fi
	@if [ -d "${CTK_PATH}" ]; then \
		go mod edit -replace="github.com/go-curses/ctk=${CTK_PATH}"; \
	else \
		echo "CTK_PATH not set or not a directory: \"${CTK_PATH}\""; \
	fi

unlocal:
	@go mod edit -dropreplace="github.com/go-enjin/be"
	@go mod edit -dropreplace="github.com/go-curses/cdk"
	@go mod edit -dropreplace="github.com/go-curses/ctk"

tidy:
	@go mod tidy

be-update: export GOPROXY=direct
be-update:
	@echo "# go get github.com/go-enjin/be@latest github.com/go-curses/cdk@latest github.com/go-curses/ctk@latest"
	@go get github.com/go-enjin/be@latest github.com/go-curses/ctk@latest

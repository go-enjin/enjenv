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

BE_LOCAL_PATH ?= ../../go-enjin/be

GOPKG_KEYS ?= CDK CTK GOXT

CDK_GO_PACKAGE ?= github.com/go-curses/cdk
CDK_LOCAL_PATH ?= ../../go-curses/cdk

CTK_GO_PACKAGE ?= github.com/go-curses/ctk
CTK_LOCAL_PATH ?= ../../go-curses/ctk

GOXT_GO_PACKAGE ?= github.com/go-enjin/golang-org-x-text
GOXT_LOCAL_PATH ?= ../golang-org-x-text
GOXT_LATEST_VER ?= v0.12.1-enjin.2

BIN_NAME ?= enjenv
UNTAGGED_VERSION ?= v0.1.20
UNTAGGED_COMMIT ?= 0000000000

CLEAN_FILES     ?= "${BIN_NAME}" ${BIN_NAME}.*.* pprof.{proxy,repos,watch}
DISTCLEAN_FILES ?=
REALCLEAN_FILES ?=

BUILD_VERSION_VAR := github.com/go-enjin/enjenv/pkg/globals.BuildVersion
BUILD_RELEASE_VAR := github.com/go-enjin/enjenv/pkg/globals.BuildRelease

_BUILD_TAGS += page_funcmaps exclude_pages_formats _templates

include Golang.cmd.mk

define _profile_run
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
	@$(call __clean,${CLEAN_FILES})

distclean: clean
	@$(call __clean,${DISTCLEAN_FILES})

realclean: distclean
	@$(call __clean,${REALCLEAN_FILES})

debug: BUILD_VERSION=$(call __tag_ver)
debug: BUILD_RELEASE=$(call __rel_ver)
debug: TRIM_PATHS=$(call __go_trim_path)
debug: __golang
	@$(call __go_build_debug,"${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}",${BUILD_OS},${BUILD_ARCH},./cmd/enjenv)
	@${SHASUM_CMD} "${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}"

build: BUILD_VERSION=$(call __tag_ver)
build: BUILD_RELEASE=$(call __rel_ver)
build: TRIM_PATHS=$(call __go_trim_path)
build: __golang
	@$(call __go_build_release,"${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}",${BUILD_OS},${BUILD_ARCH},./cmd/enjenv)
	@${SHASUM_CMD} "${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}"

build-amd64: BUILD_VERSION=$(call __tag_ver)
build-amd64: BUILD_RELEASE=$(call __rel_ver)
build-amd64: TRIM_PATHS=$(call __go_trim_path)
build-amd64: __golang
	@$(call __go_build_release,"${BIN_NAME}.${BUILD_OS}.amd64",${BUILD_OS},amd64,./cmd/enjenv)
	@${SHASUM_CMD} "${BIN_NAME}.${BUILD_OS}.amd64"

build-arm64: BUILD_VERSION=$(call __tag_ver)
build-arm64: BUILD_RELEASE=$(call __rel_ver)
build-arm64: TRIM_PATHS=$(call __go_trim_path)
build-arm64: __golang
	@$(call __go_build_release,"${BIN_NAME}.${BUILD_OS}.arm64",${BUILD_OS},arm64,./cmd/enjenv)
	@${SHASUM_CMD} "${BIN_NAME}.${BUILD_OS}.arm64"

build-all: build-amd64 build-arm64

release: build
	@$(call __upx_build,"${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}")

release-arm64: build-arm64
	@$(call __upx_build,"${BIN_NAME}.${BUILD_OS}.arm64")

release-amd64: build-amd64
	@$(call __upx_build,"${BIN_NAME}.${BUILD_OS}.amd64")

release-all: release-amd64 release-arm64

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

install:
	@if [ -f "${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}" ]; then \
		echo "# ${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH} present"; \
		$(call __install_exe,"${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}","${INSTALL_BIN_PATH}/${BIN_NAME}"); \
	else \
		echo "error: missing ${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH} binary" 1>&2; \
	fi

install-arm64:
	@if [ -f "${BIN_NAME}.${BUILD_OS}.arm64" ]; then \
		echo "# ${BIN_NAME}.${BUILD_OS}.arm64 present"; \
		$(call __install_exe,"${BIN_NAME}.${BUILD_OS}.arm64","${INSTALL_BIN_PATH}/${BIN_NAME}"); \
	else \
		echo "error: missing ${BIN_NAME}.${BUILD_OS}.arm64 binary" 1>&2; \
	fi

install-amd64:
	@if [ -f "${BIN_NAME}.${BUILD_OS}.amd64" ]; then \
		echo "# ${BIN_NAME}.${BUILD_OS}.amd64 present"; \
		$(call __install_exe,"${BIN_NAME}.${BUILD_OS}.amd64","${INSTALL_BIN_PATH}/${BIN_NAME}"); \
	else \
		echo "error: missing ${BIN_NAME}.${BUILD_OS}.amd64 binary" 1>&2; \
	fi

install-autocomplete: ENJENV_AUTOCOMPLETE_FILE=${INSTALL_AUTOCOMPLETE_PATH}/${BIN_NAME}
install-autocomplete: NISEROKU_AUTOCOMPLETE_FILE=${INSTALL_AUTOCOMPLETE_PATH}/niseroku
install-autocomplete:
	@echo "# installing ${BIN_NAME} bash_autocomplete to: ${ENJENV_AUTOCOMPLETE_FILE}"
	@$(call __install_exe,_templates/bash_autocomplete,${ENJENV_AUTOCOMPLETE_FILE})
	@echo "# installing niseroku bash_autocomplete to: ${NISEROKU_AUTOCOMPLETE_FILE}"
	@$(call __install_exe,_templates/bash_autocomplete,${NISEROKU_AUTOCOMPLETE_FILE})

install-niseroku: NISEROKU_PATH=${INSTALL_ETC_PATH}/niseroku
install-niseroku: NISEROKU_TOML_FILE=${NISEROKU_PATH}/niseroku.toml
install-niseroku:
	@if [ -f "${NISEROKU_TOML_FILE}" ]; then \
		echo "# skipping ${NISEROKU_TOML_FILE} (exists already)"; \
	else \
		echo "# installing ${NISEROKU_TOML_FILE}"; \
		$(call __install_file,0664 -b,_templates/niseroku.toml,${NISEROKU_TOML_FILE}); \
	fi

install-niseroku-logrotate: LOGROTATE_PATH=${INSTALL_ETC_PATH}/logrotate.d
install-niseroku-logrotate: NISEROKU_LOGROTATE_FILE=${LOGROTATE_PATH}/niseroku
install-niseroku-logrotate:
	@echo "# installing ${NISEROKU_LOGROTATE_FILE}"
	@$(call __install_file,0664 -b,_templates/niseroku.logrotate,${NISEROKU_LOGROTATE_FILE})

install-niseroku-sysv-init: SYSV_INIT_PATH=${INSTALL_ETC_PATH}/init.d
install-niseroku-sysv-init: NISEROKU_PROXY_SYSV_INIT_FILE=${SYSV_INIT_PATH}/niseroku-proxy
install-niseroku-sysv-init: NISEROKU_REPOS_SYSV_INIT_FILE=${SYSV_INIT_PATH}/niseroku-repos
install-niseroku-sysv-init:
	@echo "# installing ${NISEROKU_PROXY_SYSV_INIT_FILE}"
	@$(call __install_exe,_templates/niseroku-proxy.init,${NISEROKU_PROXY_SYSV_INIT_FILE})
	@echo "# installing ${NISEROKU_REPOS_SYSV_INIT_FILE}"
	@$(call __install_exe,_templates/niseroku-repos.init,${NISEROKU_REPOS_SYSV_INIT_FILE})

install-niseroku-systemd: SYSTEMD_PATH=${INSTALL_ETC_PATH}/systemd/system
install-niseroku-systemd: NISEROKU_PROXY_SERVICE_FILE=${SYSTEMD_PATH}/niseroku-proxy.service
install-niseroku-systemd: NISEROKU_REPOS_SERVICE_FILE=${SYSTEMD_PATH}/niseroku-repos.service
install-niseroku-systemd:
	@echo "# installing ${NISEROKU_PROXY_SERVICE_FILE}"
	@$(call __install_file,0664,_templates/niseroku-proxy.service,${NISEROKU_PROXY_SERVICE_FILE})
	@echo "# installing ${NISEROKU_REPOS_SERVICE_FILE}"
	@$(call __install_file,0664,_templates/niseroku-repos.service,${NISEROKU_REPOS_SERVICE_FILE})

install-niseroku-utils:
	@if [ -f "_templates/niseroku.sh" ]; then \
		echo "# installing niseroku wrapper script"; \
		$(call __install_exe,"_templates/niseroku.sh","${INSTALL_BIN_PATH}/niseroku"); \
	else \
		echo "error: missing niseroku wrapper script" 1>&2; \
	fi
	@if [ -f "_templates/niseroku-tail.sh" ]; then \
		echo "# installing niseroku-tail wrapper script"; \
		$(call __install_exe,"_templates/niseroku-tail.sh","${INSTALL_BIN_PATH}/niseroku-tail"); \
	else \
		echo "error: missing niseroku-tail wrapper script" 1>&2; \
	fi

local: __local

unlocal: __unlocal

tidy: __tidy

be-update: __be_update

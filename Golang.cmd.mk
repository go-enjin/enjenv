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

###############################################################################
#:
#: CHANGELOG
#:
#: v0.1.3 - GOPKG_KEYS support for (key)_LATEST_VER
#:        * correct debug build flags
#:
#: v0.1.2 - go-curses/coreutils updates
#:        * allow non-enjin things, set GO_ENJIN_PKG and BE_LOCAL_PATH to 'nil'
#:        * updates to how be-update derives PKG_LIST
#:        * provide BUILD_NAME based on BIN_NAME BUILD_OS and BUILD_ARCH
#:        * git command handling cleanups
#: v0.1.1 - enjenv go binary updates
#:        * abstract GO_BIN variable into a defined __go_bin make function
#:        * __go_bin prefers enjenv's activate script over existing env PATH
#:        * __go_bin falls back with that 'which enjenv' is found
#:        * replace all traces of global GO_BIN with calling __go_bin
#:        * update __golang target to use __go_bin
#:        * __golang target echos the go version when present
#:
#: v0.1.0 - initial implementation
#:
###############################################################################

ENJENV_MK_VERSION := v0.1.4

.PHONY: __golang __tidy __local __unlocal __be_update

PWD := $(shell pwd)
SHELL := /bin/bash

UNTAGGED_VERSION ?= v0.0.0
UNTAGGED_COMMIT ?= 0000000000

BUILD_OS   := $(shell uname -s | awk '{print $$1}' | perl -pe '$$_=lc($$_)')
BUILD_ARCH := $(shell uname -m | perl -pe 's!aarch64!arm64!;s!x86_64!amd64!;')
BUILD_NAME := ${BIN_NAME}.${BUILD_OS}.${BUILD_ARCH}

GIT_STATUS := $(git status 2> /dev/null)

UPX_BIN := $(shell which upx)

SHASUM_BIN := $(shell which shasum)
SHASUM_CMD := ${SHASUM_BIN} -a 256

GOPKG_KEYS ?=

GO_ENJIN_PKG ?= github.com/go-enjin/be

DESTDIR ?=

prefix ?= /usr
prefix_etc ?= /etc

INSTALL_BIN_PATH := ${DESTDIR}${prefix}/bin
INSTALL_ETC_PATH := ${DESTDIR}${prefix_etc}
INSTALL_AUTOCOMPLETE_PATH := ${INSTALL_ETC_PATH}/bash_completion.d

_INTERNAL_BUILD_LOG_ ?= /dev/null

define __clean
	for FOUND in $(1); do \
		if [ -n "$${FOUND}" ]; then \
			${CMD} rm -rfv $${FOUND}; \
		fi; \
	done
endef

define __install_file
	echo "# installing $(2) to: $(3) [$(1)]"; \
	${CMD} mkdir -vp `dirname $(3)`; \
	${CMD} /usr/bin/install -v -m $(1) "$(2)" "$(3)"; \
	${CMD} ${SHASUM_CMD} "$(3)"
endef

define __install_exe
$(call __install_file,0775,$(1),$(2))
endef

define __go_trim_path
$(shell \
if [ "${GOPATH}" != "" ]; then \
	echo "${GOPATH};${PWD}"; \
else \
	echo "${PWD}"; \
fi)
endef

define __tag_ver
$(shell (git describe 2>/dev/null) || echo "${UNTAGGED_VERSION}")
endef

define __rel_ver
$(shell \
	if [ -d .git ]; then \
		if [ -z "${GIT_STATUS}" ]; then \
			(git rev-parse --short=10 HEAD 2>/dev/null) || echo "${UNTAGGED_COMMIT}"; \
		else \
			[ -d .git ] && ( git diff 2>/dev/null || true ) \
				| ${SHASUM_CMD} - 2>/dev/null \
				| perl -pe 's!^\s*([a-f0-9]{10}).*!\1!'; \
		fi; \
	else \
		echo "${UNTAGGED_COMMIT}"; \
	fi \
)
endef

define __go_bin
$(shell \
	export ENJENV_BIN=`which enjenv`; \
	if [ -n "$${ENJENV_BIN}" -a -x "$${ENJENV_BIN}" ]; then \
		export ENJENV_PATH=`"$${ENJENV_BIN}"`; \
		if [ -n "$${ENJENV_PATH}" -a "$${ENJENV_PATH}/activate" ]; then \
			( source "$${ENJENV_PATH}/activate" 2>&1 ) > /dev/null; \
		fi; \
	fi; \
	export GO_BIN=`which go`; \
	if [ -z "$${GO_BIN}" -o ! -x "$${GO_BIN}" ]; then \
		echo "error: missing go binary" 1>&2; \
		false; \
	fi; \
	echo "$${GO_BIN}" \
)
endef

# __go_build 1=bin-name, 2=goos, 3=goarch, 4=ldflags, 5=gcflags, 6=asmflags, 7=extra, 8=src
define __go_build
$(shell \
	export ERR=false; \
	if [ "$(2)" == "linux" ]; then \
		if [ "$(3)" == "arm64" ]; then \
			export CC_VAL=aarch64-linux-gnu-gcc; \
			export CXX_VAL=aarch64-linux-gnu-g++; \
		elif [ "$(3)" == "amd64" ]; then \
			export CC_VAL=x86_64-linux-gnu-gcc; \
			export CXX_VAL=x86_64-linux-gnu-g++; \
		else \
			echo "error: unsupported architecture: $(3)" 1>&2; \
			export ERR=true; \
		fi; \
	fi; \
	if [ "$${ERR}" == "false" ]; then \
		echo "${CMD} \
 GOOS=\"$(2)\" GOARCH=\"$(3)\" \
 CGO_ENABLED=1 CC=$${CC_VAL} CXX=$${CXX_VAL} \
  $(call __go_bin) \
    build -v \
    -o \"$(1)\" \
    -ldflags=\"-buildid='' $(4)\" \
    -gcflags=\"$(5)\" \
    -asmflags=\"$(6)\" \
    -tags \"${_BUILD_TAGS}\" \
    $(7) \
    $(8)"; \
	else \
		echo "echo ERROR missing go binary"; \
	fi
)
endef

# 1: bin-name, 2: goos, 3: goarch, 4: ldflags, 5: gcflags, 6: asmflags, 7: argv, 8: src
define __cmd_go_build
$(call __go_build,$(1),$(2),$(3),$(4) -X '${BUILD_VERSION_VAR}=${BUILD_VERSION}' -X '${BUILD_RELEASE_VAR}=${BUILD_RELEASE}',$(5),$(6),$(7),$(8))
endef

# 1: bin-name, 2: goos, 3: goarch, 4: ldflags, 5: src
define __cmd_go_build_trimpath
$(call __cmd_go_build,$(1),$(2),$(3),$(4),-trimpath='${TRIM_PATHS}',-trimpath='${TRIM_PATHS}',-trimpath,$(5))
endef

# 1: bin-name, 2: goos, 3: goarch, 4: src
define __go_build_release
	echo "# building $(2)-$(3) (release): ${BIN_NAME} (${BUILD_VERSION}, ${BUILD_RELEASE})"; \
	echo $(call __cmd_go_build_trimpath,$(1),$(2),$(3),-s -w,$(4)); \
	$(call __cmd_go_build_trimpath,$(1),$(2),$(3),-s -w,$(4))
endef

# 1: bin-name, 2: goos, 3: goarch, 4: src
define __go_build_debug
	echo "# building $(2)-$(3) (debug): ${BIN_NAME} (${BUILD_VERSION}, ${BUILD_RELEASE})"; \
	echo $(call __cmd_go_build,$(1),$(2),$(3),,all=-l -N -l,,,$(4)); \
	$(call __cmd_go_build,$(1),$(2),$(3),,-N -l,,,$(4))
endef

define __upx_build
	if [ "${BUILD_OS}" == "darwin" ]; then \
		echo "# upx command not supported on darwin, nothing to do"; \
	elif [ -n "${UPX_BIN}" -a -x "${UPX_BIN}" ]; then \
		echo -n "# packing: $(1) - "; \
		du -hs "$(1)" | awk '{print $$1}'; \
		${UPX_BIN} -qq -7 --no-color --no-progress "$(1)"; \
		echo -n "# packed: $(1) - "; \
		du -hs "$(1)" | awk '{print $$1}'; \
		${SHASUM_CMD} "$(1)"; \
	else \
		echo "# upx command not found, nothing to do"; \
	fi
endef

define __pkg_list_latest
$(shell \
	if [ "${GO_ENJIN_PKG}" != "nil" ]; then \
		echo -n "${GO_ENJIN_PKG}@latest "; \
	fi)\
$(if ${GOPKG_KEYS},$(foreach key,${GOPKG_KEYS},$(shell \
		if [ \
			-n "$($(key)_GO_PACKAGE)" \
			-a "$($(key)_GO_PACKAGE)" != "nil" \
		]; then \
			echo "$($(key)_GO_PACKAGE)@$(if $($(key)_LATEST_VER),$($(key)_LATEST_VER),latest)"; \
		fi \
)))
endef

define __validate_extra_pkgs
$(if ${GOPKG_KEYS},$(foreach key,${GOPKG_KEYS},$(shell \
		if [ \
			-z "$($(key)_GO_PACKAGE)" \
			-o -z "$($(key)_LOCAL_PATH)" \
			-o ! -d "$($(key)_LOCAL_PATH)" \
		]; then \
			echo "echo \"# $(key)_GO_PACKAGE and/or $(key)_LOCAL_PATH not found\"; false;"; \
		fi \
)))
endef

define __make_go_local
echo "__make_go_local $(1) $(2)" >> ${_INTERNAL_BUILD_LOG_}; \
if [ -n "$(2)" -a "$(2)" != "nil" ]; then \
	echo "# go.mod local: $(1)"; \
	${CMD} $(call __go_bin) mod edit -replace "$(1)=$(2)"; \
fi
endef

define __make_go_unlocal
echo "__make_go_unlocal $(1)" >> ${_INTERNAL_BUILD_LOG_}; \
if [ -n "$(1)" -a "$(1)" != "nil" ]; then \
	echo "# go.mod unlocal $(1)"; \
	${CMD} $(call __go_bin) mod edit -dropreplace "$(1)"; \
fi
endef

define _make_extra_pkgs
$(if ${GOPKG_KEYS},$(foreach key,${GOPKG_KEYS},$($(key)_GO_PACKAGE)@latest))
endef

__golang: export GO_BIN=$(call __go_bin)
__golang:
	@if [ -n "${GO_BIN}" -a -x "${GO_BIN}" ]; then \
		export GO_BIN_VERSION=`${GO_BIN} version`; \
		echo "# $${GO_BIN_VERSION}"; \
	else \
		echo "# missing go binary" 1>&2; \
		echo "# ${GO_BIN} -- $${GO_BIN}"; \
		false; \
	fi

__tidy: __golang
	@echo "# go mod tidy"
	@$(call __go_bin) mod tidy

__local: __golang
	@`echo "_make_extra_locals" >> ${_INTERNAL_BUILD_LOG_}`
	@$(call __validate_extra_pkgs)
	@$(if ${GOPKG_KEYS},$(foreach key,${GOPKG_KEYS},$(call __make_go_local,$($(key)_GO_PACKAGE),$($(key)_LOCAL_PATH));))
	@$(call __make_go_local,${GO_ENJIN_PKG},${BE_LOCAL_PATH})

__unlocal: __golang
	@`echo "_make_extra_unlocals" >> ${_INTERNAL_BUILD_LOG_}`
	@$(call __validate_extra_pkgs)
	@$(if ${GOPKG_KEYS},$(foreach key,${GOPKG_KEYS},$(call __make_go_unlocal,$($(key)_GO_PACKAGE));))
	@$(call __make_go_unlocal,${GO_ENJIN_PKG})

__be_update: PKG_LIST=$(call __pkg_list_latest)
__be_update: __golang
	@$(call __validate_extra_pkgs)
	@echo "# go getting: ${PKG_LIST}"
	@GOPROXY=direct $(call __go_bin) get ${PKG_LIST}

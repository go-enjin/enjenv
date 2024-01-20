#!/usr/bin/make --no-print-directory --jobs=1 --environment-overrides -f

# Copyright (c) 2024  The Go-Enjin Authors
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

MAKEFILE_KEYS += DEBIAN
DEBIAN_MK_FILE := Debian.mk
DEBIAN_MK_VERSION := v0.2.0
DEBIAN_MK_DESCRIPTION := debian packaging support

#:: begin changelog
#
# v0.2.0:
#   * support Golang makefile usage (MAKEFILE_KEYS)
#
# v0.1.2:
#   * disable unit tests with DEB_BUILD_OPTIONS="nocheck"
#
# v0.1.0:
#   * initial versioning of Debian.mk
#   * updates and fixes for better arm/amd cross compilation support
#
#:: end changelog

#
#: Global Settings
#

export APT_FLAVOUR ?= debian
export AE_ARCHIVES ?= apt-archives
export AE_GPG_HOME ?= $(shell realpath .gpg)
export AE_GPG_FILE ?=
export AE_SIGN_KEY ?=

export PACKAGING_NAME ?= ${BIN_NAME}

#: bin checks

export WHICH_GPG := $(shell which gpg)
export WHICH_DPKG_BUILDPACKAGE := $(shell which dpkg-buildpackage)

#
#: Helper Defines
#

define _gpg =
$(shell \
	if [ -n "${AE_GPG_HOME}" ]; then \
		echo -n "GNUPGHOME=\"${AE_GPG_HOME}\" "; \
		echo -n " gpg "; \
	else \
		echo -n " echo gpg "; \
	fi
)
endef

define _get_gpg_key_id =
$(shell \
	if [ -n "${AE_SIGN_KEY}" -a -d "${AE_GPG_HOME}" ]; then \
		$(call _gpg) --list-keys "${AE_SIGN_KEY}" \
		| head -2 | tail -1 \
		| awk '{print $$1}'; \
	fi \
)
endef

define _move_deb_files =
	mkdir -vp ${AE_ARCHIVES}/${APT_FLAVOUR}; \
	mv -v ../${PACKAGING_NAME}[-_]*.{dsc,xz,buildinfo,changes,deb} ${AE_ARCHIVES}/${APT_FLAVOUR}/
endef

define _dpkg_buildpackage =
	if [ -n "${AE_SIGN_KEY}" ]; then \
		env GNUPGHOME="${AE_GPG_HOME}" \
			${DPKG_ARCH_EXPORTS} \
			DEB_BUILD_OPTIONS="nocheck" \
			${WHICH_DPKG_BUILDPACKAGE} \
			--build=full \
			--post-clean \
			--build-profiles=cross,nocheck,nostrip \
			--host-arch $(1) \
			--sign-key="${AE_SIGN_KEY}"; \
	else \
		env ${DPKG_ARCH_EXPORTS} \
			DEB_BUILD_OPTIONS="nocheck" \
			${WHICH_DPKG_BUILDPACKAGE} \
			--build=full \
			--post-clean \
			--build-profiles=cross,nocheck,nostrip \
			--host-arch $(1) \
			--no-sign; \
	fi
endef

#
#: Preparation Targets
#

_check_commands:
	@if [ ! -x "${WHICH_GPG}" ]; then \
		echo "# error: gpg not found"; \
		echo "# please use \"make debian-deps\" to install all build dependencies"; \
		false; \
	elif [ ! -x "${WHICH_DPKG_BUILDPACKAGE}" ]; then \
		echo "dpkg-buildpackage not found, please install gpg"; \
		echo "# please use \"make debian-deps\" to install all build dependencies"; \
		false; \
	fi

_setup_gpg: _check_commands
	@if [ ! -d "${AE_GPG_HOME}" ]; then \
		mkdir -v -p "${AE_GPG_HOME}"; \
		chmod -v 0700 "${AE_GPG_HOME}"; \
		if [ -n "${AE_GPG_FILE}" -a -f "${AE_GPG_FILE}" ]; then \
			echo "# importing gpg key..."; \
			$(call _gpg) --import "${AE_GPG_FILE}"; \
		fi; \
	else \
		echo "# existing gpg home found, nothing to do"; \
	fi

_prepare_gpg: _setup_gpg
	@if [ -n "${AE_SIGN_KEY}" ]; then \
		if [ -n "$(call _get_gpg_key_id)" ]; then \
			echo "# gpg key <${AE_SIGN_KEY}> verified"; \
		else \
			echo "# gpg key not found: ${AE_SIGN_KEY}"; \
		fi; \
	fi

#
#: debian packaging targets
#

debian-clean: distclean
	@echo "# cleaning all debian package building artifacts"
	@rm -rfv apt-archives 

debian-deps:
	@echo "# installing debian package build dependencies"
	@sudo apt-get install gpg build-essential dh-make devscripts \
		gcc-multilib-x86-64-linux-gnu libc6-dev-amd64-cross linux-libc-dev-amd64-cross \
		gcc-multilib-aarch64-linux-gnu libc6-dev-arm64-cross linux-libc-dev-arm64-cross

deb-arm64: export CC=aarch64-linux-gnu-gcc
deb-arm64: export CXX=aarch64-linux-gnu-g++
deb-arm64: export CROSS_COMPILE="aarch64-linux-gnu-"
deb-arm64: export DPKG_ARCH_EXPORTS="$(shell dpkg-architecture -a arm64 2> /dev/null)"
deb-arm64: _prepare_gpg
	@echo "# building arm64 debian package"
	@$(call _dpkg_buildpackage,arm64)
	@$(call _move_deb_files);

deb-amd64: export CC=x86_64-linux-gnu-gcc
deb-amd64: export CXX=x86_64-linux-gnu-g++
deb-amd64: export CROSS_COMPILE="x86_64-linux-gnu-"
deb-amd64: export DPKG_ARCH_EXPORTS="$(shell dpkg-architecture -a amd64 2> /dev/null)"
deb-amd64: _prepare_gpg
	@echo "# building amd64 debian package"
	@$(call _dpkg_buildpackage,amd64)
	@$(call _move_deb_files);

debian-buildpackage: deb-arm64 deb-amd64

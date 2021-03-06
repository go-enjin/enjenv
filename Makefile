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

PREFIX ?= ${GOPATH}

PWD = $(shell pwd)

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

.PHONY: all help clean build install local unlocal tidy

help:
	@echo "usage: make <help|clean|build|install|local|unlocal|tidy>"

clean:
	@if [ -f "${BIN_NAME}" ]; then rm -fv "${BIN_NAME}"; fi

build: BUILD_VERSION=$(call _tag_ver)
build: BUILD_RELEASE=$(call _rel_ver)
build: TRIM_PATHS=$(call _trim_path)
build:
	@echo "# building: ${BIN_NAME} (${BUILD_VERSION}, ${BUILD_RELEASE})"
	@${CMD} go build -v \
		-o "${BIN_NAME}" \
		-ldflags="-w -s -buildid='' -X 'main.BuildVersion=${BUILD_VERSION}' -X 'main.BuildRelease=${BUILD_RELEASE}'" \
		-gcflags="-trimpath='${TRIM_PATHS}'" \
		-asmflags="-trimpath='${TRIM_PATHS}'" \
		-trimpath \
		./cmd/enjenv

install:
	@if [ ! -f enjenv ]; then \
		echo "error: missing enjenv binary"; \
		false; \
	fi
	@echo "# installing enjenv to: $(PREFIX)/bin/"
	@if [ -d "$(PREFIX)/bin/" ]; then \
			${CMD} /usr/bin/install -t $(PREFIX)/bin -v enjenv; \
	fi

local:
	@if [ -d "${BE_PATH}" ]; then \
		go mod edit -replace="github.com/go-enjin/be=${BE_PATH}"; \
	else \
		echo "BE_PATH not set or not a directory: \"${BE_PATH}\""; \
	fi

unlocal:
	@go mod edit -dropreplace="github.com/go-enjin/be"

tidy:
	@go mod tidy -go=1.16 && go mod tidy -go=1.17

be-update: export GOPROXY=direct
be-update:
	@go get -tags all -u github.com/go-enjin/be

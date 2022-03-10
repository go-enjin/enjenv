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

ENV_PATH = $(shell enjenv)

define _trimPath =
$(shell \
if [ "${GOPATH}" != "" ]; then \
	echo "${GOPATH};${ENV_PATH};${PWD}"; \
else \
	echo "${ENV_PATH};${PWD}"; \
fi)
endef

.PHONY: all help clean build install local unlocal tidy

help:
	@echo "usage: make <help|clean|build|install|local|unlocal|tidy>"

clean:
	@if [ -f "${BIN_NAME}" ]; then rm -fv "${BIN_NAME}"; fi

build: TRIMPATH=$(call _trimPath)
build:
	@echo "# building: ${BIN_NAME} (${TRIMPATH})"
	@${CMD} go build -v \
		-o "${BIN_NAME}" \
		-ldflags="-w -s -buildid=''" \
		-gcflags="-trimpath='${TRIMPATH}'" \
		-asmflags="-trimpath='${TRIMPATH}'" \
		-trimpath \
		./cmd/enjenv

enjenv: build

install: enjenv
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

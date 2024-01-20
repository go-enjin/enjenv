// Copyright (c) 2023  The Go-Enjin Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package niseroku

import "regexp"

var (
	RxLogFileName = regexp.MustCompile(`(?:/|^)([^/]+?)\.?(access|info|error|)\.log$`)

	RxSlugArchiveName = regexp.MustCompile(`(?:/|^)([^/]+?)--([a-f0-9]+)\.zip$`)
	RxSlugRunningName = regexp.MustCompile(`(?:/|^)([^/]+?)--([a-f0-9]+).([a-f0-9]{10})(\.pid|\.port|)$`)

	RxSockCommand         = regexp.MustCompile(`^\s*([a-z][-.a-z0-9]+?)\s*$`)
	RxSockCommandWithArgs = regexp.MustCompile(`^\s*([a-z][-.a-z0-9]+?)\s+(.+?)\s*$`)

	RxTangoTags = regexp.MustCompile(`<.+?>`)
)

var RxDscFileName = regexp.MustCompile(`^\s*(.+?)_(.+?)\.dsc\s*$`)

var RxDebFileName = regexp.MustCompile(`^\s*(.+?)_(.+?)_(.+?)\.u?deb\s*$`)

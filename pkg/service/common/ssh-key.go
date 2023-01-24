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

package common

import "regexp"

var (
	RxSshPubKey = regexp.MustCompile(`^(\S+)\s+(\S+)((?:\s*).*)$`)
)

func ParseSshKey(input string) (prefix, data, comment, id string, ok bool) {
	if ok = RxSshPubKey.MatchString(input); ok {
		m := RxSshPubKey.FindAllStringSubmatch(input, 1)
		prefix, data, comment = m[0][1], m[0][2], m[0][3]
		id = prefix + " " + data
	}
	return
}
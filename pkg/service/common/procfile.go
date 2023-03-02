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

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/go-enjin/be/pkg/path"
)

var RxSlugProcfileEntry = regexp.MustCompile(`^([a-zA-Z0-9]+):\s*(.+?)\s*$`)

func ReadProcfile(procfile string) (procTypes map[string]string, err error) {
	procTypes = make(map[string]string)
	if path.IsFile(procfile) {
		var data []byte
		if data, err = os.ReadFile(procfile); err != nil {
			return
		}
		for _, contents := range strings.Split(string(data), "\n") {
			if RxSlugProcfileEntry.MatchString(contents) {
				m := RxSlugProcfileEntry.FindAllStringSubmatch(contents, 1)
				for _, mm := range m {
					procTypes[mm[1]] = mm[2]
				}
			}
		}
	} else {
		err = fmt.Errorf("slug Procfile not found")
	}
	return
}
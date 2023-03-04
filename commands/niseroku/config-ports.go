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

import (
	"strings"

	bePath "github.com/go-enjin/be/pkg/path"

	pkgIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (c *Config) GetAllRunningPorts() (ports map[int]*Application) {
	ports = make(map[int]*Application)
	files, _ := bePath.ListFiles(c.Paths.TmpRun)
	for _, file := range files {
		if strings.HasSuffix(file, ".port") {
			if v, err := common.GetIntFromFile(file); err != nil {
				pkgIo.StderrF("error getting int from file: %v - %v\n", file, err)
			} else if RxSlugRunningName.MatchString(file) {
				m := RxSlugRunningName.FindAllStringSubmatch(file, 1)
				if app, ok := c.Applications[m[0][1]]; ok {
					ports[v] = app
				} else {
					pkgIo.StderrF("application not found from slug running name: %v\n", file)
				}
			}
		}
	}
	return
}
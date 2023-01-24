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
	"fmt"
	"strings"

	pkgRun "github.com/go-enjin/enjenv/pkg/run"
)

func (a *Application) Invoke() (err error) {
	var o, e string
	if o, e, err = pkgRun.EnjenvCmd("niseroku", "app", "start", a.Name); err != nil {
		err = fmt.Errorf("error invoking app: %v - %v", a.Name, err)
		return
	}
	for _, line := range strings.Split(o, "\n") {
		if line != "" {
			a.LogInfoF("invoke[stdout]: %v", line)
		}
	}
	for _, line := range strings.Split(e, "\n") {
		if line != "" {
			a.LogInfoF("invoke[stderr]: %v", line)
		}
	}
	return
}
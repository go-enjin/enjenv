// Copyright (c) 2022  The Go-Enjin Authors
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

package basepath

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/path"
)

var EnjenvPath = FindEnjenvDir()

var (
	EnjenvDirName = ".enjenv"
)

func EnjenvPresent() (present bool) {
	present = path.IsDir(EnjenvPath)
	return
}

func FindEnjenvDir() string {
	var name string
	if name = env.Get("ENJENV_DIR_NAME", ""); name == "" {
		name = EnjenvDirName
	}
	if wd, err := os.Getwd(); err == nil {
		for len(wd) > 1 && wd != "/" {
			beDir := fmt.Sprintf("%v/%v", wd, name)
			if path.IsDir(beDir) {
				if abs, err := path.Abs(beDir); err == nil {
					return abs
				}
			}
			wd = path.Dir(wd)
		}
	}
	if bmp := env.Get("ENJENV_PATH", ""); bmp != "" {
		if path.IsDir(bmp) {
			return bmp
		}
	}
	return fmt.Sprintf("%v/%v", path.Pwd(), EnjenvDirName)
}

func MakeEnjenvPath(names ...string) (path string) {
	name := strings.Join(names, "/")
	path = fmt.Sprintf("%v/%v", EnjenvPath, name)
	return
}
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
	"github.com/go-enjin/be/pkg/hash/sha"
	"github.com/go-enjin/be/pkg/path"
)

var (
	EnjenvPath    = FindEnjenvDir()
	EnjenvDirName = ".enjenv"
)

func WhichBin() (enjenvBinPath string) {
	if enjenvBinPath = os.Getenv("ENJENV_BIN"); enjenvBinPath != "" {
		return
	}
	enjenvBinPath = path.Which(os.Args[0])
	return
}

func BinCheck() (absPath, buildBinHash string, err error) {
	if absPath = path.Which(os.Args[0]); absPath == "" {
		err = fmt.Errorf("could not find self: %v\n", os.Args[0])
		return
	}
	if buildBinHash, err = sha.FileHash10(absPath); err != nil {
		err = fmt.Errorf("enjenv sha256 error %v: %v\n", absPath, err)
		return
	}
	return
}

func EnjenvPresent() (present bool) {
	present = path.IsDir(EnjenvPath)
	return
}

func EnjenvIsInPwd() (present bool) {
	pwd := path.Pwd()
	if envPathValue := os.Getenv("ENJENV_PATH"); envPathValue != "" {
		present = envPathValue[0:len(pwd)] == pwd
		return
	}
	if path.IsDir(pwd + "/" + EnjenvDirName) {
		present = true
		return
	}
	return
}

func FindEnjenvDir() string {
	if envPathValue := os.Getenv("ENJENV_PATH"); envPathValue != "" {
		return envPathValue
	}
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
	return fmt.Sprintf("%v/%v", path.Pwd(), EnjenvDirName)
}

func MakeEnjenvPath(names ...string) (path string) {
	name := strings.Join(names, "/")
	path = fmt.Sprintf("%v/%v", EnjenvPath, name)
	return
}
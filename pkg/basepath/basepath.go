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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-corelibs/env"
	"github.com/go-corelibs/path"
	sha "github.com/go-corelibs/shasum"
)

var (
	EnjenvPath    = FindEnjenvDir()
	EnjenvDirName = ".enjenv"
	EnjenvBinPath = ""
	EnjenvBinHash = ""
)

func WhichBin() (enjenvBinPath string) {
	// TODO: figure out if ENJENV_BIN is necessary
	// if enjenvBinPath = os.Getenv("ENJENV_BIN"); enjenvBinPath != "" {
	// 	return
	// }
	if EnjenvBinPath != "" {
		enjenvBinPath = EnjenvBinPath
		return
	}
	var err error
	if enjenvBinPath, err = exec.LookPath(os.Args[0]); err == nil {
		return
	}
	enjenvBinPath = path.Which(os.Args[0])
	return
}

func BinCheck() (absPath, buildBinHash string, err error) {
	if EnjenvBinPath != "" && EnjenvBinHash != "" {
		absPath = EnjenvBinPath
		buildBinHash = EnjenvBinHash
		return
	}
	argv0 := os.Args[0]

	if strings.HasPrefix(argv0, "./") || strings.HasPrefix(argv0, "../") {
		if absPath, err = path.Abs(argv0); err != nil {
			err = fmt.Errorf("error finding absolute path to: %v - %v", argv0, err)
			return
		}
	} else if strings.HasPrefix(argv0, "/") {
		absPath = argv0
	} else if absPath = path.Which(argv0); absPath == "" {
		err = fmt.Errorf("error program not found: %v", argv0)
		return
	}

	if buildBinHash, err = sha.BriefFile(absPath); err != nil {
		err = fmt.Errorf("enjenv sha256 error %v: %v\n", absPath, err)
		return
	}
	EnjenvBinPath = absPath
	EnjenvBinHash = buildBinHash
	return
}

func EnjenvPresent() (present bool) {
	present = path.IsDir(EnjenvPath)
	return
}

func EnjenvIsInPwd() (present bool) {
	pwd, _ := os.Getwd()
	if envPathValue := env.String("ENJENV_PATH", ""); envPathValue != "" {
		if len(envPathValue) >= len(pwd) {
			present = envPathValue[0:len(pwd)] == pwd
		}
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
	var err error
	var pwd, wd string
	name := env.String("ENJENV_DIR_NAME", EnjenvDirName)

	if pwd, err = os.Getwd(); err == nil {
		wd = pwd
		for len(wd) > 1 && wd != "/" {
			beDir := fmt.Sprintf("%v/%v", wd, name)
			if path.IsDir(beDir) {
				if abs, ee := path.Abs(beDir); ee == nil {
					return abs
				}
			}
			wd = path.Dir(wd)
		}
	}
	return fmt.Sprintf("%v/%v", pwd, EnjenvDirName)
}

func MakeEnjenvPath(names ...string) (path string) {
	path = filepath.Join(append([]string{EnjenvPath}, names...)...)
	return
}

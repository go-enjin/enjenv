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
	"strings"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/hash/sha"
	"github.com/go-enjin/be/pkg/path"
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

	if strings.HasPrefix(argv0, "./") {
		if absPath, err = path.Abs(argv0); err != nil {
			err = fmt.Errorf("error finding absolute path to: %v - %v", argv0, err)
			return
		}
	} else if strings.HasPrefix(argv0, "/") {
		absPath = argv0
	} else {
		binPaths := env.GetPaths()
		for _, binPath := range binPaths {
			if path.IsFile(binPath + "/" + argv0) {
				absPath = binPath + "/" + argv0
			}
		}
		if absPath == "" {
			err = fmt.Errorf("error finding program: %v - not in any of: %v", argv0, binPaths)
			return
		}
	}

	if buildBinHash, err = sha.FileHash10(absPath); err != nil {
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
	pwd := path.Pwd()
	if envPathValue := os.Getenv("ENJENV_PATH"); envPathValue != "" {
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
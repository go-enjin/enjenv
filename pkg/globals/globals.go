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

package globals

import (
	"os"

	"github.com/go-enjin/enjenv/pkg/basepath"
)

var (
	DefaultGolangVersion = "1.21.6"
	DefaultNodejsVersion = "18.16.1"
)

var (
	BuildVersion   = "v0.2.2"
	BuildRelease   = "trunk"
	BuildBinPath   = ""
	BuildBinHash   = "0000000000"
	DisplayVersion = BuildVersion + " (trunk) [0000000000]"
	OsHostname, _  = os.Hostname()
)

func init() {
	var err error
	if BuildBinPath, BuildBinHash, err = basepath.BinCheck(); err == nil {
		DisplayVersion = BuildVersion + " (" + BuildRelease + ") [" + BuildBinHash + "]"
		_ = os.Setenv("ENJENV_BIN", BuildBinPath)
	}
}

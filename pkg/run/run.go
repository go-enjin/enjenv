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

package run

import (
	"fmt"
	"os"
	"strconv"

	"github.com/go-enjin/be/pkg/cli/env"
	"github.com/go-enjin/be/pkg/cli/run"
	"github.com/go-enjin/enjenv/pkg/basepath"
)

func AddPathToEnv(path string) {
	_ = env.SetPathRemoved(path)
	_ = env.SetPathPrefixed(path)
	return
}

func RemovePathFromEnv(path string) {
	_ = env.SetPathRemoved(path)
	return
}

func MakeExe(argv ...string) (err error) {
	err = run.CheckExe("make", argv...)
	return
}

func EnjenvExe(argv ...string) (err error) {
	if enjenvBin := basepath.WhichBin(); enjenvBin != "" {
		err = run.CheckExe(enjenvBin, argv...)
	} else {
		err = fmt.Errorf("enjenv not found")
	}
	return
}

func EnjenvCmd(argv ...string) (o, e string, err error) {
	if enjenvBin := basepath.WhichBin(); enjenvBin != "" {
		o, e, err = run.CheckCmd(enjenvBin, argv...)
	} else {
		err = fmt.Errorf("enjenv not found")
	}
	return
}

func GetPidFromFile(pidFile string) (pid int, err error) {
	var data []byte
	if data, err = os.ReadFile(pidFile); err != nil {
		return
	}
	pid, err = strconv.Atoi(string(data))
	return
}
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
	"strings"

	"github.com/go-enjin/be/pkg/cli/run"

	"github.com/go-corelibs/path"
)

func Gpg(home string, argv ...string) (o, e string, status int, err error) {
	if !path.Exists(home) {
		if err = os.Mkdir(home, 0700); err != nil {
			return
		}
	}
	arguments := append([]string{"--homedir", home}, argv...)
	o, e, status, err = run.Cmd("gpg", arguments...)
	return
}

func GpgShowOnly(home, file string) (fingerprints []string, err error) {
	var o, e string
	var ee error
	if o, e, _, ee = Gpg(home, "--with-colons", "--import-options", "show-only", "--import", file); ee != nil {
		msg := "begin GpgShowOnly error: %v\n"
		msg += "begin GpgShowOnly stdout:\n"
		msg += "%v"
		msg += "end GpgShowOnly stdout.\n"
		msg += "begin GpgShowOnly stderr:\n"
		msg += "%v"
		msg += "end GpgShowOnly stderr."
		err = fmt.Errorf(msg, ee, o, e)
		return
	}

	for _, line := range strings.Split(o, "\n") {
		if strings.HasPrefix(line, "fpr") {
			fields := strings.Split(line, ":")
			if len(fields) >= 10 {
				fingerprints = append(fingerprints, fields[9])
			}
		}
	}
	return
}

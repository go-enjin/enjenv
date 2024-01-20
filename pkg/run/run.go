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

	"github.com/go-corelibs/env"
	"github.com/go-enjin/be/pkg/cli/run"

	"github.com/go-enjin/enjenv/pkg/basepath"
)

func AddPathToEnv(path string) {
	_ = env.PrependPATH(path)
	return
}

func MakeExe(argv ...string) (err error) {
	err = run.CheckExe("make", argv...)
	return
}

func EnjenvExe(argv ...string) (err error) {
	if enjenvBin := basepath.WhichBin(); enjenvBin != "" {
		_, err = run.Exe(enjenvBin, argv...)
	} else {
		err = fmt.Errorf("enjenv not found")
	}
	return
}

func EnjenvExeWith(path string, environ []string, argv ...string) (err error) {
	if enjenvBin := basepath.WhichBin(); enjenvBin != "" {
		err = run.ExeWith(&run.Options{
			Path:    path,
			Name:    enjenvBin,
			Argv:    argv,
			Environ: environ,
		})
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

func EnjenvCmdWith(path string, environ []string, argv ...string) (o, e string, err error) {
	if enjenvBin := basepath.WhichBin(); enjenvBin != "" {
		o, e, _, err = run.CmdWith(&run.Options{
			Path:    path,
			Name:    enjenvBin,
			Argv:    argv,
			Environ: environ,
		})
	} else {
		err = fmt.Errorf("enjenv not found")
	}
	return
}

func EnjenvBg(so, se string, argv ...string) (pid int, err error) {
	if so == "-" && se != "-" {
		so = se
	} else if so != "-" && se == "-" {
		se = so
	}
	if so == "" {
		so = "/dev/null"
	}
	if se == "" {
		se = "/dev/null"
	}
	if enjenvBin := basepath.WhichBin(); enjenvBin != "" {
		pid, err = run.BackgroundWith(&run.Options{
			Name:   enjenvBin,
			Argv:   argv,
			Stdout: so,
			Stderr: se,
		})
	} else {
		err = fmt.Errorf("enjenv not found")
	}
	return
}

//go:build debug || all

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

package profiling

import (
	"fmt"
	"strings"

	"github.com/pkg/profile"

	pkgIo "github.com/go-enjin/enjenv/pkg/io"

	"github.com/go-corelibs/env"
	clpath "github.com/go-corelibs/path"
	clstrings "github.com/go-corelibs/strings"
)

var (
	BuildEnabled       = ""
	BuildProfileType   = ""
	BuildProfilePath   = ""
	EnvNameEnabled     = "ENJENV_ENABLE_PROFILING"
	EnvNameProfileType = "ENJENV_PROFILING_TYPE"
	EnvNameProfilePath = "ENJENV_PROFILING_PATH"
)

var (
	Enabled     = false
	ProfileType = "cpu"
	ProfilePath = "./out.pprof"
)

func init() {
	if BuildEnabled != "" {
		Enabled = clstrings.IsTrue(BuildEnabled)
	}
	if BuildProfileType != "" {
		ProfileType = BuildProfileType
	}
	if BuildProfilePath != "" {
		ProfilePath = BuildProfilePath
	}
	if v := env.String(EnvNameEnabled, ""); v != "" {
		Enabled = clstrings.IsTrue(v)
	}
	if v := env.String(EnvNameProfileType, ""); v != "" {
		ProfileType = v
	}
	if v := env.String(EnvNameProfilePath, ""); v != "" {
		ProfilePath = v
	}
	switch v := strings.ToLower(ProfileType); v {
	case "cpu", "mem", "go":
		ProfileType = v
	}
	if v, err := clpath.Abs(ProfilePath); err == nil {
		ProfilePath = v
	} else {
		panic(fmt.Errorf("error getting absolute profile path: %v", err))
	}
}

func newProfiler(profileType func(*profile.Profile), profilePath string) (profiler interface{ Stop() }) {
	profiler = profile.Start(
		profileType,
		profile.Quiet,
		profile.NoShutdownHook,
		profile.ProfilePath(profilePath),
	)
	return
}

var instance interface{ Stop() }

func Start() {
	pkgIo.STDOUT("pkg/profiling: enabled=%v, type=%v, path=%v\n", Enabled, ProfileType, ProfilePath)
	if !Enabled {
		return
	}
	switch ProfileType {
	case "cpu":
		instance = newProfiler(profile.CPUProfile, ProfilePath)
	case "mem":
		instance = newProfiler(profile.MemProfile, ProfilePath)
	case "go":
		instance = newProfiler(profile.GoroutineProfile, ProfilePath)
	}
}

func Stop() {
	if Enabled && instance != nil {
		instance.Stop()
	}
}

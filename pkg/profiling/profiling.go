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

	"github.com/go-enjin/be/pkg/cli/env"
	bePath "github.com/go-enjin/be/pkg/path"
	beStrings "github.com/go-enjin/be/pkg/strings"
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
		Enabled = beStrings.IsTrue(BuildEnabled)
	}
	if BuildProfileType != "" {
		ProfileType = BuildProfileType
	}
	if BuildProfilePath != "" {
		ProfilePath = BuildProfilePath
	}
	if v := env.Get(EnvNameEnabled, ""); v != "" {
		Enabled = beStrings.IsTrue(v)
	}
	if v := env.Get(EnvNameProfileType, ""); v != "" {
		ProfileType = v
	}
	if v := env.Get(EnvNameProfilePath, ""); v != "" {
		ProfilePath = v
	}
	switch v := strings.ToLower(ProfileType); v {
	case "cpu", "mem", "go":
		ProfileType = v
	}
	if v, err := bePath.Abs(ProfilePath); err == nil {
		ProfilePath = v
	} else {
		panic(fmt.Errorf("error getting absolute profile path: %v", err))
	}
	// _, _ = fmt.Fprintf(os.Stderr, "pkg/profiling: enabled=%v, type=%v, path=%v\n", Enabled, ProfileType, ProfilePath)
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
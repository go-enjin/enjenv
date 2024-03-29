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

package system

import (
	"sort"

	"github.com/fvbommel/sortorder"
	"github.com/urfave/cli/v2"

	"github.com/go-corelibs/slices"
)

func ListFeatures(commands []*cli.Command) (names []string) {
	for _, cmd := range commands {
		names = append(names, cmd.Name)
		for _, sn := range ListFeatures(cmd.Subcommands) {
			names = append(names, cmd.Name+"--"+sn)
		}
	}
	sort.Sort(sortorder.Natural(names))
	return
}

func HasFeature(name string, commands []*cli.Command) (present bool) {
	present = slices.Present(name, ListFeatures(commands)...)
	return
}

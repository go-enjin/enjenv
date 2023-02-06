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

package niseroku

import "github.com/fvbommel/sortorder"

type WatchingByUsage []WatchProc

func (a WatchingByUsage) Len() int      { return len(a) }
func (a WatchingByUsage) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a WatchingByUsage) Less(i, j int) bool {
	if a[i].Cpu == a[j].Cpu {
		return sortorder.NaturalLess(a[i].Name, a[j].Name)
	}
	return a[i].Cpu > a[j].Cpu
}
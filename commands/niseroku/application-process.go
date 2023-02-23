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

import (
	"fmt"
	"os"

	"github.com/iancoleman/strcase"

	"github.com/go-enjin/be/pkg/maps"
	bePath "github.com/go-enjin/be/pkg/path"
)

func (a *Application) GetThisSlug() (slug *Slug) {
	a.RLock()
	defer a.RUnlock()
	if a.ThisSlug != "" {
		name := bePath.Base(a.ThisSlug)
		if found, ok := a.Slugs[name]; ok {
			slug = found
		}
	}
	return
}

func (a *Application) GetNextSlug() (slug *Slug) {
	a.RLock()
	defer a.RUnlock()
	if a.NextSlug != "" {
		name := bePath.Base(a.NextSlug)
		if found, ok := a.Slugs[name]; ok {
			slug = found
		}
	}
	return
}

func (a *Application) ApplySettings(envDir string) (err error) {
	// a.LogInfoF("applying settings to: %v\n", envDir)
	a.RLock()
	defer a.RUnlock()
	for _, k := range maps.SortedKeys(a.Settings) {
		key := strcase.ToScreamingSnake(k)
		value := fmt.Sprintf("%v", a.Settings[k])
		if err = os.WriteFile(envDir+"/"+key, []byte(value), 0660); err != nil {
			return
		}
	}
	return
}

func (a *Application) OsEnviron() (environment []string) {
	a.RLock()
	defer a.RUnlock()
	environment = os.Environ()
	for _, k := range maps.SortedKeys(a.Settings) {
		key := strcase.ToScreamingSnake(k)
		environment = append(environment, fmt.Sprintf("%v=%v", key, a.Settings[k]))
	}
	return
}

func (a *Application) GetSlugInstanceByPid(pid int) (si *SlugInstance) {
	a.RLock()
	defer a.RUnlock()
	for _, slug := range a.Slugs {
		if si = slug.GetInstanceByPid(pid); si != nil {
			return
		}
	}
	return
}
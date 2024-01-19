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
	"bytes"
	"os"

	"github.com/BurntSushi/toml"

	clpath "github.com/go-corelibs/path"

	"github.com/go-enjin/enjenv/pkg/service/common"
)

type SlugSettings struct {
	Live []string `toml:"live"`
	Next []string `toml:"next,omitempty"`

	Path         string        `toml:"-"`
	TomlMetaData toml.MetaData `toml:"-"`
	RunAs        RunAsConfig   `toml:"-"`
}

func NewSlugSettings(path string, config RunAsConfig) (sw *SlugSettings, err error) {
	sw = &SlugSettings{
		Path:  path,
		RunAs: config,
	}
	if clpath.IsFile(path) {
		sw.TomlMetaData, err = toml.DecodeFile(path, sw)
	}
	return
}

func (s *SlugSettings) Save() (err error) {
	buf := bytes.NewBuffer([]byte{})
	_ = toml.NewEncoder(buf).Encode(s)
	if err = os.WriteFile(s.Path, buf.Bytes(), 0660); err != nil {
		return
	}
	_ = common.RepairOwnership(s.Path, s.RunAs.User, s.RunAs.Group)
	return
}

func (s *SlugSettings) Reload() (err error) {
	path := s.Path
	s.TomlMetaData, err = toml.DecodeFile(path, s)
	s.Path = path
	return
}

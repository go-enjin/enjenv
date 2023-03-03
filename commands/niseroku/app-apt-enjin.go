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
	"strings"
)

type AptEnjinConfig struct {
	Enable    bool                      `toml:"enable,omitempty"`
	SiteKey   string                    `toml:"site-key,omitempty"`
	SiteName  string                    `toml:"site-name,omitempty"`
	SiteMail  string                    `toml:"site-mail,omitempty"`
	SiteMaint string                    `toml:"site-maint,omitempty"`
	SiteUrl   string                    `toml:"site-url,omitempty"`
	GpgKeys   map[string][]string       `toml:"gpg-keys,omitempty"`
	Flavours  map[string][]Distribution `toml:"flavours,omitempty"`
}

type Distribution struct {
	Label         string   `toml:"label,omitempty"`
	Origin        string   `toml:"origin,omitempty"`
	Version       string   `toml:"version,omitempty"`
	Description   string   `toml:"description,omitempty"`
	Codename      string   `toml:"codename,omitempty"`
	Components    []string `toml:"components,omitempty"`
	Architectures []string `toml:"architectures,omitempty"`
	SignWith      string   `toml:"sign-with,omitempty"`
	Contents      string   `toml:"contents,omitempty"`
	Tracking      string   `toml:"tracking,omitempty"`
}

func (d Distribution) String() (conf string) {
	add := func(k, v string) string {
		if v != "" {
			return fmt.Sprintf("%v: %v\n", k, v)
		}
		return ""
	}
	conf += add("Label", d.Label)
	conf += add("Origin", d.Origin)
	conf += add("Version", d.Version)
	conf += add("Description", d.Description)
	conf += add("Codename", d.Codename)
	conf += add("Components", strings.Join(d.Components, " "))
	conf += add("Architectures", strings.Join(d.Architectures, " "))
	conf += add("SignWith", d.SignWith)
	conf += add("Contents", d.Contents)
	conf += add("Tracking", d.Tracking)
	return
}

type Distributions []Distribution

func (d Distributions) String() (conf string) {
	last := len(d) - 1
	for idx, dist := range d {
		if v := dist.String(); v != "" {
			conf += v
		}
		if idx < last {
			conf += "\n"
		}
	}
	return
}
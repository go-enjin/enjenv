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

package enjin

import (
	"fmt"
	"strings"

	"github.com/go-enjin/golang-org-x-text/language"
	"github.com/urfave/cli/v2"
)

func tagInTags(tag language.Tag, tags ...language.Tag) (found bool) {
	for _, check := range tags {
		if found = language.Compare(tag, check); found {
			return
		}
	}
	return
}

func parseLangOutArgv(ctx *cli.Context) (outDir string, tags []language.Tag, err error) {
	if languageTags := ctx.String("lang"); languageTags == "" {
		err = fmt.Errorf("error: --lang argument requires at least one locale tag\n")
		return
	} else if parts := strings.Split(languageTags, ","); len(parts) == 0 {
		err = fmt.Errorf("error: --lang argument requires at least one locale tag\n")
		return
	} else {
		for _, part := range parts {
			if t, e := language.Parse(part); e != nil {
				err = fmt.Errorf("error parsing language tag: %v - %v\n", part, e)
				return
			} else {
				tags = append(tags, t)
			}
		}
	}

	if outDir = ctx.String("out"); outDir == "" {
		err = fmt.Errorf("error: --out argument requires a path value")
		return
	}
	return
}
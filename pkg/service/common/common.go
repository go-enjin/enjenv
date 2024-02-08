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
	"encoding/csv"
	"strings"

	"github.com/gofrs/uuid"

	sha "github.com/go-corelibs/shasum"
)

func ParseControlArgv(input string) (argv []string, err error) {
	r := csv.NewReader(strings.NewReader(input))
	r.Comma = ' '
	argv, err = r.Read()
	return
}

func UniqueHash() (hash string) {
	unique, _ := uuid.NewV4()
	hash, _ = sha.BriefSum(unique.Bytes())
	return
}

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

	"github.com/BurntSushi/toml"
	bePath "github.com/go-enjin/be/pkg/path"

	beIo "github.com/go-enjin/enjenv/pkg/io"
)

type Users []*User

func (u Users) String() (details string) {
	for _, user := range u {
		details += user.String()
		details += ";"
	}
	return
}

type User struct {
	Name           string   `toml:"name"`
	AuditLog       string   `toml:"audit-log"`
	Applications   []string `toml:"applications"`
	AuthorizedKeys []string `toml:"ssh-keys"`
}

func LoadUsers(path string) (users Users, err error) {
	var files []string
	if files, err = bePath.ListAllFiles(path); err != nil {
		return
	}
	for _, file := range files {
		u := &User{}
		if _, ee := toml.DecodeFile(file, u); ee != nil {
			err = fmt.Errorf("error decoding user file: %v - %v", file, ee)
			return
		}
		users = append(users, u)
	}
	return
}

func (u *User) String() (details string) {
	details = fmt.Sprintf("{Name:%v,Keys:%d,Apps:%v}", u.Name, len(u.AuthorizedKeys), u.Applications)
	return
}

func (u *User) HasKey(given string) (has bool) {
	if _, _, _, id, ok := parseSshKey(given); ok {
		for _, key := range u.AuthorizedKeys {
			if _, _, _, keyId, valid := parseSshKey(key); valid {
				if has = id == keyId; has {
					return
				}
			}
		}
	}
	return
}

func (u *User) Log(format string, argv ...interface{}) {
	message := fmt.Sprintf("[user:%v] %v\n", u.Name, fmt.Sprintf(format, argv...))
	if u.AuditLog != "" {
		if outFH, err := os.OpenFile(u.AuditLog, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0660); err == nil {
			_, _ = outFH.WriteString(message + "\n")
			return
		} else {
			beIo.StderrF("[user:%v] error writing to audit log: %v\n", u.Name, err)
		}
	}
	beIo.StdoutF(message)
}
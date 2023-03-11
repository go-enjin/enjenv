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
	"fmt"
	"os"
	"syscall"

	"github.com/go-enjin/be/pkg/path"
)

func PerformMkdirChownChmod(uid, gid int, fileMode, dirMode os.FileMode, directories ...string) (err error) {

	canChown := syscall.Geteuid() == 0 && uid > 0 && gid > 0

	for _, target := range directories {

		if err = os.MkdirAll(target, 0770); err != nil {
			err = fmt.Errorf("mkdir [path] %v - %v", target, err)
			return
		}

		if canChown {
			if err = PerformChown(target, uid, gid); err != nil {
				err = fmt.Errorf("chown (%d:%d) [path] %v - %v", uid, gid, target, err)
				return
			}
		}

		if err = PerformChmod(target, dirMode); err != nil {
			err = fmt.Errorf("chmod (%v) [path] %v - %v", dirMode, target, err)
			return
		}

		if err = PerformChownChmod(uid, gid, fileMode, dirMode, target); err != nil {
			return
		}
	}
	return
}

func PerformChown(target string, uid, gid int) (err error) {
	if path.Exists(target) && uid > 0 && gid > 0 {
		err = os.Chown(target, uid, gid)
	}
	return
}

func PerformChmod(target string, mode os.FileMode) (err error) {
	if path.Exists(target) {
		err = os.Chmod(target, mode)
	}
	return
}

func PerformChownChmod(uid, gid int, fileMode, dirMode os.FileMode, paths ...string) (err error) {
	canChown := syscall.Geteuid() == 0 && uid > 0 && gid > 0

	getModifiedFileMode := func(tgt string) (modified os.FileMode) {
		modified = fileMode
		var stat os.FileInfo
		if stat, err = os.Stat(tgt); err != nil {
			// no stat, no mode to modify
			return
		}
		actual := stat.Mode()
		if actual != fileMode {
			if actual&syscall.S_IXUSR != 0 {
				modified |= syscall.S_IXUSR
			}
			if actual&syscall.S_IXGRP != 0 {
				modified |= syscall.S_IXGRP
			}
		}
		return
	}

	for _, target := range paths {
		if !path.Exists(target) {
			continue
		}

		if canChown {
			if err = PerformChown(target, uid, gid); err != nil {
				err = fmt.Errorf("chown (%d:%d) [file] %v - %v\n", uid, gid, target, err)
				return
			}
		}

		if path.IsFile(target) {
			modifiedFileMode := getModifiedFileMode(target)
			if err = PerformChmod(target, modifiedFileMode); err != nil {
				err = fmt.Errorf("chmod (%v) [file] %v - %v\n", modifiedFileMode, target, err)
				return
			}
			continue
		}

		if err = PerformChmod(target, dirMode); err != nil {
			err = fmt.Errorf("chmod (%v) [path] %v - %v\n", dirMode, target, err)
			return
		}

		if found, ee := path.ListAllDirs(target); ee == nil {
			for _, tgt := range found {
				if canChown {
					if err = PerformChown(tgt, uid, gid); err != nil {
						err = fmt.Errorf("chown (%d:%d) [path] %v - %v\n", uid, gid, tgt, err)
						return
					}
				}
				if err = PerformChmod(tgt, dirMode); err != nil {
					err = fmt.Errorf("chmod (%v) [path] %v - %v\n", dirMode, tgt, err)
					return
				}
			}
		}

		if found, ee := path.ListAllFiles(target); ee == nil {
			for _, tgt := range found {
				if canChown {
					if err = PerformChown(tgt, uid, gid); err != nil {
						err = fmt.Errorf("chown (%d:%d) [file] %v - %v\n", uid, gid, tgt, err)
						return
					}
				}
				modifiedFileMode := getModifiedFileMode(tgt)
				if err = PerformChmod(tgt, modifiedFileMode); err != nil {
					err = fmt.Errorf("chmod (%v) [file] %v - %v\n", modifiedFileMode, tgt, err)
					return
				}
			}
		}

	}
	return
}
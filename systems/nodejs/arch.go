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

package nodejs

import (
	"runtime"
)

func RuntimeSupported() (supported bool) {
	return RuntimeOS() != "" && RuntimeArch() != ""
}

func RuntimeOS() (osName string) {
	osName = CheckOS(runtime.GOOS)
	return
}

func RuntimeArch() (archName string) {
	archName = CheckArch(runtime.GOARCH)
	return
}

func CheckOS(check string) (osName string) {
	switch check {
	case "darwin":
		osName = "darwin"
	case "linux":
		osName = "linux"
	case "windows", "win":
		osName = "win"
	}
	return
}

func CheckArch(check string) (archName string) {
	switch check {
	case "amd64", "x64":
		archName = "x64"
	case "arm64", "arm":
		archName = "arm64"
	case "386", "x86":
		archName = "x86"
	}
	return
}

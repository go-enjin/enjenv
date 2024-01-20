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

package service

import (
	"fmt"
	"strings"

	beIo "github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/service/common"
)

func (s *Service) LogInfoF(format string, argv ...interface{}) {
	format = strings.TrimSpace(format)
	beIo.StdoutF("# [%v] [%v] %v\n", common.Datestamp(), s.Name, fmt.Sprintf(format, argv...))
}

func (s *Service) LogError(err error) {
	beIo.StdoutF("# [%v] [%v] ERROR %v\n", common.Datestamp(), s.Name, err)
}

func (s *Service) LogErrorF(format string, argv ...interface{}) {
	format = strings.TrimSpace(format)
	beIo.StdoutF("# [%v] [%v] ERROR %v\n", common.Datestamp(), s.Name, fmt.Sprintf(format, argv...))
}

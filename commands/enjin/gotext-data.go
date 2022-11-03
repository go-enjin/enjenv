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

type GotextData struct {
	Language string    `json:"language"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Id                string `json:"id"`
	Key               string `json:"key"`
	Message           string `json:"message"`
	Translation       string `json:"translation"`
	TranslatorComment string `json:"translatorComment"`
	Fuzzy             bool   `json:"fuzzy,omitempty"`
}
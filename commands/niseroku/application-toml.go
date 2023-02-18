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

var applicationTomlComments = TomlComments{
	{
		Statement: "domains",
		Lines: []string{
			": domains           (string...)",
			":    * one or more domains routed to this app",
		},
	},
	{
		Statement: "maintenance",
		Lines: []string{
			": maintenance       (bool)",
			":    * omit this app in the reverse-proxy service",
			":    * does not prevent git-repository deployments",
			":    * all requests are 503 - Service Unavailable",
		},
	},
	{
		Statement: "this-slug",
		Lines: []string{
			": this-slug         (path)",
			":     * the real path to this app's current slug",
			":     * this setting is overwritten during deployments",
		},
	},
	{
		Statement: "next-slug",
		Lines: []string{
			": next-slug         (path)",
			":     * the real path to this app's next slug deployment",
			":     * this setting is overwritten during deployments",
		},
	},
	{
		Statement: "[timeouts]",
		Lines: []string{
			": [timeouts]        (section)",
			":     * per-app timeout settings",
			":     * uses the Go time.Duration format, see: https://pkg.go.dev/time#ParseDuration",
		},
	},
	{
		Statement: "slug-startup",
		Lines: []string{
			": slug-startup      (time.Duration)",
			":     * maximum time to allow slugs to open the expected port",
		},
	},
	{
		Statement: "read-interval",
		Lines: []string{
			": ready-interval    (time.Duration)",
			":     * frequency at which niseroku checks expected ports to open",
		},
	},
	{
		Statement: "origin-request",
		Lines: []string{
			": origin-request    (time.Duration)",
			":     * maximum time to allow slugs to perform a given request",
		},
	},
	{
		Statement: "[settings]",
		Lines: []string{
			": [settings]        (section)",
			":     * per-application custom environment variables",
			":     * all keys are converted to SCREAMING_SNAKE_CASE",
			":     * this section, and it's comments, are retained unmodified",
		},
	},
	{
		Statement: "[origin]",
		Lines: []string{
			": [origin]          (section)",
			":     * slug endpoint settings",
			":     * this section is overwritten during deployments",
		},
	},
	{
		Statement: "scheme",
		Inline:    ": the URL scheme, must be one of http or https",
	},
	{
		Statement: "host",
		Inline:    ": the localhost IP address to proxy requests with",
	},
}

func MergeApplicationToml(current TomlComments, keepCustomComments bool) (modified TomlComments) {
	var defaults TomlComments
	for _, dtc := range applicationTomlComments {
		var exists bool
		for _, ctc := range current {
			if exists = dtc.Statement == ctc.Statement; exists {
				if keepCustomComments {
					ctc.Inline = CheckAB(ctc.Inline, dtc.Inline, ctc.Inline != "")
					ctc.Lines = CheckAB(ctc.Lines, dtc.Lines, len(ctc.Lines) > 0)
				} else {
					ctc.Inline = dtc.Inline
					ctc.Lines = dtc.Lines
				}
				break
			}
		}
		if exists {
			continue
		}
		defaults = append(defaults, dtc)
	}
	modified = append(modified, current...)
	modified = append(modified, defaults...)
	return
}
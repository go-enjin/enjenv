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

var configTomlComments = TomlComments{
	{
		Statement: "__header__",
		Lines: []string{
			": BEGIN NISEROKU CONFIG - enjenv niseroku config --help",
		},
	},
	{
		Statement: "__footer__",
		Lines: []string{
			": END NISEROKU CONFIG - enjenv niseroku config --help",
		},
	},
	{
		Statement: "bind-addr",
		Lines: []string{
			": bind-addr         (string)",
			":     * ip address for niseroku services to bind listeners",
			":     * requires niseroku-{proxy,repos} restart if changed",
			"",
		},
	},
	{
		Statement: "enable-ssl",
		Lines: []string{
			": enable-ssl        (bool)",
			":     * reverse-proxy will bind an HTTPS autocert listener",
			":     * requires account-email to be set",
			":     * requires niseroku-proxy restart if changed",
			"",
		},
	},
	{
		Statement: "account-email",
		Lines: []string{
			": account-email     (email@address.string)",
			":     * email address used for the Let's Encrypt account",
			":     * implies agreement with Let's Encrypt terms of service",
			":     * requires niseroku-proxy restart if changed",
			"",
		},
	},
	{
		Statement: "buildpack-path",
		Lines: []string{
			": buildpack-path    (path)",
			":     * specifies an enjenv-heroku-buildpack checkout to use",
			":     * requires niseroku-repos reload or restart if changed",
			"",
		},
	},
	{
		Statement: "log-file",
		Lines: []string{
			": log-file          (path)",
			":     * specifies the path to use for logging services",
			":     * both reverse-proxy and git-repository log to this file",
			":     * requires niseroku-{proxy,repos} reload or restart if changed",
			"",
		},
	},
	{
		Statement: "keep-slugs",
		Lines: []string{
			": keep-slugs        (bool)",
			":     * keep slugs after new deployments",
			"",
		},
	},
	{
		Statement: "slug-nice",
		Lines: []string{
			": slug-nice         (number: -20 to 20)",
			":     * renice all slugs run to the given priority",
			":     * be careful with this setting!",
			":     * requires niseroku app restart if changed",
			"",
		},
	},
	{
		Statement: "[include-slugs]",
		Lines: []string{
			": [include-slugs]   (section)",
			":     * configures when to include slugs in the niseroku lifecycle",
			":     * requires niseroku-proxy reload or restart if changed",
			"",
		},
	},
	{
		Statement: "on-start",
		Inline:    ": start all stopped slugs on reverse-proxy startup",
	},
	{
		Statement: "on-stop",
		Inline:    ": stop all running slugs on reverse-proxy shutdown",
	},
	{
		Statement: "[timeouts]",
		Lines: []string{
			": [timeouts]        (section)",
			":     * global reverse-proxy timeout settings",
			":     * uses the Go time.Duration format, see: https://pkg.go.dev/time#ParseDuration",
			":     * requires niseroku-proxy reload or restart if changed",
			"",
		},
	},
	{
		Statement: "slug-startup",
		Lines: []string{
			": slug-startup      (time.Duration)",
			":     * maximum time to allow slugs to open the expected port",
			"",
		},
	},
	{
		Statement: "ready-interval",
		Lines: []string{
			": ready-interval    (time.Duration)",
			":     * frequency at which niseroku checks expected ports to open",
			"",
		},
	},
	{
		Statement: "origin-request",
		Lines: []string{
			": origin-request    (time.Duration)",
			":     * maximum time to allow slugs to perform a given request",
			"",
		},
	},
	{
		Statement: "[proxy-limit]",
		Lines: []string{
			": [proxy-limit]     (section)",
			":     * reverse-proxy request rate-limiting settings",
			":     * requires niseroku-proxy reload or restart if changed",
			"",
		},
	},
	{
		Statement: "ttl",
		Lines: []string{
			": ttl (time.Duration) - rate-limiter cached values lifetime",
			"",
		},
	},
	{
		Statement: "max",
		Lines: []string{
			": max (int) - concurrent requests allowed before rate limiting",
			"",
		},
	},
	{
		Statement: "burst",
		Lines: []string{
			": burst (int) - concurrent requests allowed within a brief timeframe before rate limiting",
			"",
		},
	},
	{
		Statement: "max-delay",
		Lines: []string{
			": max-delay (time.Duration) - maximum time to delay requests before 429 response",
			"",
		},
	},
	{
		Statement: "delay-scale",
		Lines: []string{
			": delay-scale (int) - number of limit-check intervals within the max-delay timeframe",
			"",
		},
	},
	{
		Statement: "log-allowed",
		Lines: []string{
			": log-allowed (bool) - log when already delayed requests are allowed to pass",
			"",
		},
	},
	{
		Statement: "log-delayed",
		Lines: []string{
			": log-delayed (bool) - log each time a request is delayed by rate-limiting",
			"",
		},
	},
	{
		Statement: "log-limited",
		Lines: []string{
			": log-limited (bool) - log each time a request is limited (429 response)",
			"",
		},
	},
	{
		Statement: "[run-as]",
		Lines: []string{
			": [run-as]          (section)",
			":     * when run as root, drop privileges to the specified user and group",
			":     * requires niseroku-{proxy,repos} restart if changed",
			":     * may also require \"sudo enjenv niseroku fix-fs\" to correct file ownership errors",
			"",
		},
	},
	{
		Statement: "[ports]",
		Lines: []string{
			": [ports]           (section)",
			":     * all ports specified are used with the bind-addr setting",
			"",
		},
	},
	{
		Statement: "git",
		Lines: []string{
			": git               (number: 1 to 65534)",
			":     * requires niseroku-repos restart if changed",
			"",
		},
	},
	{
		Statement: "http",
		Lines: []string{
			": http               (number: 1 to 65534)",
			":     * requires niseroku-proxy restart if changed",
			"",
		},
	},
	{
		Statement: "https",
		Lines: []string{
			": https              (number: 1 to 65534)",
			":     * setting to anything other than 443 has not been tested",
			":     * requires niseroku-proxy restart if changed",
			"",
		},
	},
	{
		Statement: "app-start",
		Lines: []string{
			": app-start          (number: 1 to 65534)",
			":     * start of application port range",
			"",
		},
	},
	{
		Statement: "app-end",
		Lines: []string{
			": app-end            (number: 1 to 65534)",
			":     * end of application port range",
			"",
		},
	},
	{
		Statement: "[paths]",
		Lines: []string{
			": [paths]            (section)",
			":     * top-levels of where niseroku files live",
			":     * requires niseroku-{proxy,repos} restart if changed",
			":     * may also require \"sudo enjenv niseroku fix-fs\" to correct file ownership errors",
			"",
		},
	},
	{
		Statement: "etc",
		Lines: []string{
			": etc                (path)",
			":     * where configuration files live",
			"",
		},
	},
	{
		Statement: "tmp",
		Lines: []string{
			": tmp                (path)",
			":     * where temporary files live",
			"",
		},
	},
	{
		Statement: "var",
		Lines: []string{
			": var                (path)",
			":     * where persistent files live",
			"",
		},
	},
}

func MergeConfigToml(current TomlComments, keepCustomComments bool) (modified TomlComments) {
	var defaults TomlComments
	for _, dtc := range configTomlComments {
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
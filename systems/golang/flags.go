package golang

import (
	"github.com/urfave/cli/v2"
)

var CliBuildFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:    "optimize",
		Usage:   "configure for reproducible go builds (-trimpath, etc)",
		EnvVars: []string{"ENJENV_GO_OPTIMIZE"},
	},
	&cli.BoolFlag{
		Name:    "finalize",
		Usage:   "create a Shasums from a Shafile and embed the integrity values for --strict runtime checks",
		EnvVars: []string{"ENJENV_GO_FINALIZE"},
	},
	&cli.StringFlag{
		Name:    "be-app-name",
		Usage:   "set -o and -ldflags -X for be.AppName",
		EnvVars: []string{"BE_APP_NAME"},
	},
	&cli.StringFlag{
		Name:    "be-summary",
		Usage:   "set -ldflags -X for be.Summary",
		EnvVars: []string{"BE_SUMMARY"},
	},
	&cli.StringFlag{
		Name:    "be-version",
		Usage:   "set -ldflags -X for be.Version",
		EnvVars: []string{"BE_VERSION"},
	},
	&cli.StringFlag{
		Name:    "be-release",
		Usage:   "set -ldflags -X for be.Release",
		EnvVars: []string{"BE_RELEASE"},
	},
	&cli.StringFlag{
		Name:    "be-env-prefix",
		Usage:   "set -ldflags -X for be.EnvPrefix",
		EnvVars: []string{"BE_ENV_PREFIX"},
	},
	&cli.StringFlag{
		Name:    "be-bin-name",
		Usage:   "specify the binary name to produce (overrides --be-app-name)",
		Aliases: []string{"o"},
		EnvVars: []string{"ENJENV_GO_OUTPUT"},
	},
	&cli.StringFlag{
		Name:    "ldflags",
		Usage:   "specify additional ldflags to include",
		EnvVars: []string{"ENJENV_GO_LDFLAGS"},
	},
	&cli.StringFlag{
		Name:    "gcflags",
		Usage:   "specify additional gcflags to include",
		EnvVars: []string{"ENJENV_GO_GCFLAGS"},
	},
	&cli.BoolFlag{
		Name:    "verbose",
		Usage:   "pass -v to go build invocation",
		EnvVars: []string{"ENJENV_GO_VERBOSE"},
	},
	&cli.StringFlag{
		Name:    "config",
		Usage:   "specify a TOML config file to use",
		EnvVars: []string{"ENJENV_GO_CONFIG"},
	},
}
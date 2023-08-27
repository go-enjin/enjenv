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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kevinburke/ssh_config"
	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/cli/run"
	"github.com/go-enjin/enjenv/pkg/io"
)

func makeCommandAppPromote(c *Command, app *cli.App) (cmd *cli.Command) {
	cmd = &cli.Command{
		Name:      "promote",
		Usage:     "deploy one or more application slugs to a remote niseroku",
		UsageText: app.Name + " niseroku app promote <ssh-profile> <app-name> [app-name...]",
		Action:    c.actionAppPromote,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "no-sudo",
				Usage:   "run remote deploy-slug without sudo",
				Aliases: []string{"N"},
			},
		},
	}
	return
}

func (c *Command) actionAppPromote(ctx *cli.Context) (err error) {
	if err = c.Prepare(ctx); err != nil {
		return
	}
	io.LogFile = ""

	if argc := ctx.NArg(); argc < 2 {
		cli.ShowSubcommandHelpAndExit(ctx, 1)
	}

	cliArgv := ctx.Args().Slice()
	sshProfile := cliArgv[0]

	var cfg *ssh_config.Config
	if cfg, err = sshLoadConfig(); err != nil {
		err = fmt.Errorf("error loading .ssh/config: %v", err)
		return
	}

	var foundHost *ssh_config.Host
	for _, host := range cfg.Hosts {
		if host.Matches(sshProfile) {
			foundHost = host
			break
		}
	}
	if foundHost == nil {
		err = fmt.Errorf("error: ssh host not found - %v", sshProfile)
		return
	}

	var sshBin, scpBin string
	if sshBin, err = exec.LookPath("ssh"); err != nil || sshBin == "" {
		err = fmt.Errorf("error: missing ssh binary")
		return
	}
	if scpBin, err = exec.LookPath("scp"); err != nil || scpBin == "" {
		err = fmt.Errorf("error: missing scp binary")
		return
	}

	io.STDOUT("# ssh profile verified\n")

	var so, se string
	var status int

	if so, se, status, err = execSSH(sshBin, sshProfile, "/bin/uname -m"); err != nil {
		if so != "" {
			io.STDOUT("%v", so)
		}
		if se != "" {
			io.STDERR("%v", se)
		}
		io.STDERR("error running ssh 'uname -m' command: %v\n", err)
		err = nil
		return
	}
	remoteUname := strings.TrimSpace(so)

	if so, se, _, err = run.Cmd("uname", "-m"); err != nil {
		if so != "" {
			io.STDOUT("%v", so)
		}
		if se != "" {
			io.STDERR("%v", se)
		}
		err = fmt.Errorf("error running local 'uname -m' command: %v", err)
		return
	}
	localUname := strings.TrimSpace(so)

	if remoteUname != localUname {
		io.STDERR("# incompatible architectures: remote %v != local %v\n", remoteUname, localUname)
		return
	}

	if so, se, status, err = execSSH(sshBin, sshProfile, "which enjenv || exit 1"); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() != 1 {
			if so != "" {
				io.STDOUT("%v", so)
			}
			if se != "" {
				io.STDERR("%v", se)
			}
			// io.STDERR("error: unexpected ssh which enjenv command exit status\n")
			err = fmt.Errorf("error running ssh 'which enjenv' command: %v", err)
		} else {
			err = fmt.Errorf("error: enjenv not present on " + sshProfile)
		}
		return
	}
	if status != 0 || so == "" {
		err = fmt.Errorf("error: enjenv not present on " + sshProfile)
		return
	}
	remoteEnjenvPath := strings.TrimSpace(so)
	io.STDOUT("# remote enjenv verified\n")

	var count int
	for _, arg := range cliArgv[1:] {
		if ee := c.processAppName(ctx, sshBin, scpBin, sshProfile, remoteEnjenvPath, arg); ee != nil {
			io.STDERR("error processing %v: %v\n", arg, ee)
			count += 1
		}
	}

	if count > 0 {
		err = fmt.Errorf("%d errors encountered", count)
	}

	return
}

func execSSH(sshBin string, sshProfile string, args ...string) (stdout, stderr string, status int, err error) {
	stdout, stderr, status, err = run.CmdWith(&run.Options{
		Path:    ".",
		Name:    sshBin,
		Argv:    append([]string{sshProfile}, args...),
		Environ: os.Environ(),
	})
	return
}

func execSCP(scpBin string, sshProfile string, args ...string) (stdout, stderr string, status int, err error) {
	stdout, stderr, status, err = run.CmdWith(&run.Options{
		Path:    ".",
		Name:    scpBin,
		Argv:    args,
		Environ: os.Environ(),
	})
	return
}

func (c *Command) processAppName(ctx *cli.Context, sshBin, scpBin, sshProfile, enjenvPath, appName string) (err error) {

	var ok bool
	var slug *Slug
	var app *Application

	if app, ok = c.config.Applications[appName]; !ok {
		err = fmt.Errorf("app not found: %v", appName)
		return
	} else if slug = app.GetThisSlug(); slug == nil {
		err = fmt.Errorf("app slug not found: %v", appName)
		return
	}

	/*
		- validate ssh profile
		- call ssh remote: enjenv present
		- transfer slug archive to ssh remote /tmp/
		- call ssh remote: enjenv niseroku deploy-slug /tmp/...
	*/

	// scp /archive.zip sshProfile:/tmp/...

	slugZipName := filepath.Base(slug.Archive)
	io.STDOUT("# transferring slug: %v\n", slug.Name)
	if so, se, _, ee := execSCP(scpBin, sshProfile, slug.Archive, sshProfile+":/tmp/"+slugZipName); ee != nil {
		io.STDOUT("%v\n", so)
		io.STDERR("%v\n", se)
		err = fmt.Errorf("error transferring slug: %v", ee)
		return
	}

	io.STDOUT("# deploying remote slug: %v\n", slug.Name)
	argv := []string{enjenvPath, "niseroku", "deploy-slug", "/tmp/" + slugZipName}
	if !ctx.Bool("no-sudo") {
		argv = append([]string{"sudo"}, argv...)
	}
	if so, se, _, ee := execSSH(sshBin, sshProfile, argv...); ee != nil {
		io.STDOUT("%v\n", so)
		io.STDERR("%v\n", se)
		err = fmt.Errorf("error running remote enjenv niseroku deploy-slug: %v", ee)
		return
	}

	io.STDOUT("# %v deployed to %v\n", appName, sshProfile)

	return
}

func validateSshConfig(profile string) {
}

func sshLoadConfig() (cfg *ssh_config.Config, err error) {
	var fh *os.File
	if fh, err = os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "config")); err != nil {
		return
	}
	defer fh.Close()
	if cfg, err = ssh_config.Decode(fh); err != nil {
		return
	}
	return
}
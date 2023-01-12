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

package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/go-enjin/be/pkg/log"
	bePath "github.com/go-enjin/be/pkg/path"

	"github.com/go-enjin/enjenv/commands/enjin"
	herokuCmd "github.com/go-enjin/enjenv/commands/heroku"
	"github.com/go-enjin/enjenv/commands/niseroku"
	"github.com/go-enjin/enjenv/pkg/basepath"
	"github.com/go-enjin/enjenv/pkg/io"
	"github.com/go-enjin/enjenv/pkg/system"
	"github.com/go-enjin/enjenv/systems/golang"
	"github.com/go-enjin/enjenv/systems/ngrok"
	"github.com/go-enjin/enjenv/systems/nodejs"
)

var BinName = bePath.Base(os.Args[0])

var (
	BuildVersion = "v0.0.0"
	BuildRelease = "development"
	BuildBinPath = ""
	BuildBinHash = ""
)

func init() {
	var err error
	if BuildBinPath, BuildBinHash, err = basepath.BinCheck(); err != nil {
		panic(err)
	}
	_ = os.Setenv("ENJENV_BIN", BuildBinPath)
}

func main() {
	log.Config.AppName = BinName
	log.Config.DisableTimestamp = true
	log.Config.LoggingFormat = log.FormatText
	log.Config.Apply()
	app := &cli.App{
		Name:    BinName,
		Usage:   "Go-Enjin environment management utility",
		Version: BuildVersion + " (" + BuildRelease + ") [" + BuildBinHash + "]",
		Action: func(ctx *cli.Context) (err error) {
			if ctx.NArg() > 0 {
				cli.ShowAppHelpAndExit(ctx, 1)
				return
			}
			io.StdoutF("%v\n", basepath.EnjenvPath)
			return
		},
		Flags: []cli.Flag{
			cli.BashCompletionFlag,
		},
	}
	app.EnableBashCompletion = true
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s %s\n", BinName, c.App.Version)
	}
	var err error
	if err = system.Manager().
		AddCommand(niseroku.New()).
		AddCommand(enjin.New()).
		AddCommand(herokuCmd.New()).
		AddSystem(golang.New()).
		AddSystem(nodejs.New()).
		AddSystem(ngrok.New()).
		Setup(app); err == nil {
		err = app.Run(os.Args)
		system.Manager().Shutdown()
	}
	if err != nil {
		io.StderrF("error: %v\n", err)
		os.Exit(1)
	}
}
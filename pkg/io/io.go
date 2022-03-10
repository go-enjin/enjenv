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

package io

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-enjin/be/pkg/cli/git"
	"github.com/go-enjin/be/pkg/notify"
	bePath "github.com/go-enjin/be/pkg/path"
)

var SlackChannel string

var SlackPrefix string

var BinName = filepath.Base(os.Args[0])

var _slackBuffer = make(chan string, 25)

var _slackTimer *time.Timer

func Shutdown() {
	if _slackTimer != nil {
		_slackTimer.Stop()
		_slackTimer = nil
	}
	if SlackChannel != "" {
		processSlackNotifications()
	}
	close(_slackBuffer)
}

func getSlackPrefix() (prefix string) {
	if SlackPrefix == "" {
		var gitTag, relVer string
		if gitTag, _ = git.Describe(); gitTag == "" {
			gitTag = "untagged"
		}
		relVer = git.MakeCustomVersion("release", "c", "d")
		name := bePath.Base(bePath.Pwd())
		SlackPrefix = fmt.Sprintf("(%v) %v %v %v", BinName, name, gitTag, relVer)
	}
	prefix = SlackPrefix
	return
}

func notifySlack(message string) {
	var channel string
	if channel = notify.SlackUrl(SlackChannel); channel == "" {
		return
	}
	_slackBuffer <- message
	if _slackTimer != nil {
		_slackTimer.Stop()
		_slackTimer = nil
	}
	_slackTimer = time.AfterFunc(
		time.Duration(2)*time.Second,
		processSlackNotifications,
	)
}

func processSlackNotifications() {
	var channel string
	if channel = notify.SlackUrl(SlackChannel); channel == "" {
		return
	}
	_slackTimer = nil
	prefix := getSlackPrefix()
	messages := ""
	count := 0
	for i := 0; i < 25; i++ {
		select {
		case msg := <-_slackBuffer:
			lines := strings.Split(msg, "\n")
			for _, line := range lines {
				if len(line) > 0 {
					line = strings.TrimSpace(line)
					messages += "\t- " + line + "\n"
					count += 1
				}
			}
		default:
		}
	}
	if messages != "" {
		fancyPrefix := "*" + prefix + "*"
		var output string
		if count > 1 {
			output = fmt.Sprintf("%v:\n%v", fancyPrefix, messages)
		} else {
			output = fmt.Sprintf("%v: %v", fancyPrefix, messages)
		}
		if err := notify.SlackF(channel, output); err != nil {
			StderrF("error notifying slack channel: %v\n", err)
		}
	}
}

func SetupSlackIfPresent(ctx *cli.Context) (err error) {
	channel := SlackChannel
	if channel == "" {
		channel = ctx.String("slack")
	}
	if channel != "" {
		if webhook := notify.SlackUrl(channel); webhook != "" {
			_ = os.Setenv(EnvPrefix+"_SLACK", channel)
			SlackChannel = webhook
			// StdoutF("# using slack channel: %v\n", channel)
			return
		}
		err = fmt.Errorf("invalid slack channel given: %v", channel)
	}
	return
}

func NotifyF(format string, argv ...interface{}) {
	msg := fmt.Sprintf(fmt.Sprintf("%v\n", strings.TrimSpace(format)), argv...)
	fmt.Printf("# " + msg)
	notifySlack(msg)
}

func StdoutF(format string, argv ...interface{}) {
	fmt.Printf(format, argv...)
}

func StderrF(format string, argv ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format, argv...)
}

func FatalF(format string, argv ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format, argv...)
	os.Exit(1)
}
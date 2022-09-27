// Copyright (c) 2022 IndyKite
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package supervisor

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"

	"github.com/sirupsen/logrus"
)

var removeLogTimeRegex = regexp.MustCompile(
	`^[0-9]{4}-[0-9]{2}-[0-9]{2}(?: |T)[0-9]{2}:[0-9]{2}(:[0-9]{2}(\.[0-9]+)?)?([+-][0-9]{2}:?[0-9]{2}|Z)?\s*`,
)

type errorWrap struct {
	err error
}

// TSCmd is embedding os/exec.Cmd and adding WaitTS (wait thread safe) function
type TSCmd struct {
	errWrap *errorWrap
	exec.Cmd
	sync.Mutex
}

// WaitTS (Wait Thread Safe) will wait until command stops. This can be called multiple times with same result.
//
// Original Wait() on original os/exec.Cmd when called multiple times can return different results,
// depends on which lifecycle of command the Wait() is called.
func (c *TSCmd) WaitTS() error {
	c.Lock()
	defer c.Unlock()
	if c.errWrap == nil {
		c.errWrap = &errorWrap{
			err: c.Wait(),
		}
	}
	return c.errWrap.err
}

// StartCmd starts command inside wrapper with thread-safe Wait method.
// First element of args array is taken as command, others are used as arguments of the command.
// Also stdout and stderr are redirected into log.
// Argument stdin is redirected into the command, if is set.
func StartCmd(log *logrus.Entry, stdin io.Reader, args ...string) (cmd *TSCmd, err error) {
	log.Debug("Executing: ", args)
	cmd = &TSCmd{
		// #nosec G204
		Cmd: *exec.Command(args[0], args[1:]...),
	}
	cmd.Stdin = os.Stdin
	if stdin != nil {
		cmd.Stdin = stdin
	}
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = outPipe.Close()
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		_ = outPipe.Close()
		_ = errPipe.Close()
		return nil, err
	}

	// Scanning the StdOut  pipe until is closed
	go func() {
		log.Trace("Listening for StdOut")
		s := bufio.NewScanner(outPipe)
		for s.Scan() {
			log.Info(removeLogTimeRegex.ReplaceAllString(s.Text(), "> "))
		}
		log.Trace("Scanner of StdOut stopped")
	}()
	go func() {
		log.Trace("Listening for StdErr")
		s := bufio.NewScanner(errPipe)
		for s.Scan() {
			log.Warn(removeLogTimeRegex.ReplaceAllString(s.Text(), "> "))
		}
		log.Trace("Scanner of StdErr stopped")
	}()

	return cmd, nil
}

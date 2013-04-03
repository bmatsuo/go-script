// Copyright 2013, Bryan Matsuo. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*  Filename:    script.go
 *  Author:      Bryan Matsuo <bryan.matsuo@gmail.com>
 *  Created:     Wed,  3 Apr 2013
 */

// Package script does ....
package script

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	Args   = os.Args
	Stdin  = os.Stdin
	Stdout = os.Stdout
	Stderr = os.Stderr
)

var Environ = make(map[string]string, 1)

func Set(key, value string) {
	Environ[key] = value
}
func Delete(key string) {
	delete(Environ, key)
}

type NopReadCloser struct{ io.Reader }
type NopWriteCloser struct{ io.Writer }

func (r NopReadCloser) Close() error  { return nil }
func (r NopWriteCloser) Close() error { return nil }

func command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdin = Stdin
	cmd.Stdout = NopWriteCloser{Stdout}
	cmd.Stderr = NopWriteCloser{Stderr}
	cmd.Env = append(make([]string, 0), os.Environ()...)
	for k, v := range Environ {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return cmd
}

func Shell(name string, args ...string) error {
	return command(name, args...).Run()
}

type PipeCommand struct {
	name string
	args []string
}

func P(name string, args ...string) PipeCommand {
	return PipeCommand{name, args}
}

type pipeStatus struct {
	index int
	cmd   *exec.Cmd
	err   error
}

func runpipe(i int, p *exec.Cmd, out io.WriteCloser, done chan<- *pipeStatus) {
	err := p.Run()
	if out != nil {
		out.Close()
	}
	done <- &pipeStatus{i, p, err}
}

func Pipe(p ...PipeCommand) error {
	n := len(p)
	if n == 0 {
		return nil
	}

	cmds := make([]*exec.Cmd, n)
	for i := range p {
		cmds[i] = command(p[i].name, p[i].args...)
	}

	done := make(chan *pipeStatus, 1)
	for i := 0; i < n-1; i++ {
		nextin, out := io.Pipe()
		cmds[i+1].Stdin, cmds[i].Stdout = nextin, out
		go runpipe(i, cmds[i], out, done)
	}
	go runpipe(n-1, cmds[n-1], nil, done)

	statuses := make([]*pipeStatus, 0, n)
	for status := range done {
		statuses = append(statuses, status)
		if len(statuses) == n {
			break
		}
	}
	close(done)

	// take on the error of the last process
	var err error
	for _, status := range statuses {
		if status.index == n-1 {
			err = status.err
			break
		}
	}

	return err
}

func Must(err error) {
	if err != nil {
		Log("=> FAIL", "--", err)
		os.Exit(1)
	}
}

func Log(v ...interface{}) {
	fmt.Println(v...)
}

func Getenv(key string, def string) string {
	val := os.Getenv(key)
	if val == "" {
		val = def
	}
	return val
}

func Path(nodes ...string) string {
	return filepath.Join(nodes...)
}

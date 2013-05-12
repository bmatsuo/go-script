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

type NopReadCloser struct{ io.Reader }
type NopWriteCloser struct{ io.Writer }

func (r NopReadCloser) Close() error  { return nil }
func (r NopWriteCloser) Close() error { return nil }

func Run(path string, args ...string) error {
	return Cmd(path, args...).Run()
}

type Command struct {
	Path string
	Args []string
}

func (cmd *Command) Start() <-chan error {
	ch := make(chan error, 1)
	_cmd := cmd.Cmd()
	err := _cmd.Start()
	if err != nil {
		ch <- err
		return ch
	}
	go func() {
		ch <- _cmd.Wait()
		close(ch)
	}()
	return ch
}
func (cmd *Command) Run() error {
	return <-cmd.Start()
}
func (cmd *Command) Cmd() *exec.Cmd {
	_cmd := exec.Command(cmd.Path, cmd.Args...)
	_cmd.Stdin = Stdin
	_cmd.Stdout = NopWriteCloser{Stdout}
	_cmd.Stderr = NopWriteCloser{Stderr}
	_cmd.Env = os.Environ()
	return _cmd
}
func Cmd(path string, args ...string) *Command {
	return &Command{path, args}
}
func Or(cmds ...*Command) error {
	for i := range cmds {
		if err := cmds[i].Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("none")
}
func And(cmds ...*Command) error {
	for i := range cmds {
		if err := cmds[i].Run(); err != nil {
			return err
		}
	}
	return nil
}
func Pipe(p ...*Command) error {
	n := len(p)
	if n == 0 {
		return nil
	}
	cmds := make([]*exec.Cmd, n)
	for i := range p {
		cmds[i] = p[i].Cmd()
	}
	done := pipe(cmds)
	var err error
	for i := 0; i < n; i++ {
		if e, ok := (<-done).(errPipeLast); ok {
			err = e.err
		}
	}
	return err
}

type errPipeLast struct {
	cmd *exec.Cmd
	err error
}

func (err errPipeLast) Error() string {
	return err.err.Error()
}

func pipe(cmds []*exec.Cmd) <-chan error {
	done := make(chan error, 1)
	for i, curr := range cmds {
		if i < len(cmds)-1 {
			in, out := io.Pipe()
			cmds[i+1].Stdin = in
			curr.Stdout = out
			go runpipe(false, curr, out, done)
		} else {
			go runpipe(true, curr, nil, done)
		}
	}
	return done
}

func runpipe(last bool, p *exec.Cmd, out io.WriteCloser, done chan<- error) {
	err := p.Run()
	if out != nil {
		out.Close()
	}
	if last {
		done <- errPipeLast{p, err}
	} else {
		done <- err
	}
}

func Must(err error) {
	if err != nil {
		Println(err)
		os.Exit(1)
	}
}

func Print(v ...interface{}) {
	fmt.Fprint(Stdout, v...)
}
func Println(v ...interface{}) {
	fmt.Fprintln(Stdout, v...)
}
func Printf(format string, v ...interface{}) {
	fmt.Fprintf(Stdout, format, v...)
}

func Getenv(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		val = def
	}
	return val
}
func Setenv(key, value string) {
	os.Setenv(key, value)
}

func Path(nodes ...string) string {
	return filepath.Join(nodes...)
}

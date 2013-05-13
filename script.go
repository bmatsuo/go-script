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
	"bytes"
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

func Bytes(path string, args ...string) ([]byte, error) {
	stdout := new(bytes.Buffer)
	err := Cmd(path, args...).
		Stdout(stdout).
		Run()
	p := stdout.Bytes()
	return p, err
}

func String(path string, args ...string) (string, error) {
	p, err := Bytes(path, args...)
	return string(p), err
}

func must(err error) {
	if err != nil {
		Println(err)
		os.Exit(1)
	}
}

func Must(path string, args ...string) {
	must(Run(path, args...))
}

func MustBytes(path string, args ...string) []byte {
	p, err := Bytes(path, args...)
	must(err)
	return p
}

func MustString(path string, args ...string) string {
	return string(MustBytes(path, args...))
}

type Command struct {
	Path   string
	Args   []string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// redirect stderr to stdout
func (cmd *Command) Combine() *Command {
	cmd.stderr = cmd.stdout
	return cmd
}

// redirect stdout to stderr
func (cmd *Command) CombineErr() *Command {
	cmd.stdout = cmd.stderr
	return cmd
}

// redirect stdout
func (cmd *Command) Stdout(out io.Writer) *Command {
	cmd.stdout = out
	return cmd
}

// redirect stderr
func (cmd *Command) Stderr(err io.Writer) *Command {
	cmd.stderr = err
	return cmd
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
	_cmd.Stdin = cmd.stdin
	_cmd.Stdout = NopWriteCloser{cmd.stdout}
	_cmd.Stderr = NopWriteCloser{cmd.stderr}
	_cmd.Env = os.Environ()
	return _cmd
}
func Cmd(path string, args ...string) *Command {
	return &Command{path, args, Stdin, Stdout, Stderr}
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

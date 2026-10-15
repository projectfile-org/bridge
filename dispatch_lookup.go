// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

//go:build lookup

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
)

// searchDirs lists where siblings are looked up: this executable's own directory first, then PATH.
func searchDirs() []string {
	dirs := filepath.SplitList(os.Getenv("PATH"))
	if self, err := os.Executable(); err == nil {
		dirs = append([]string{filepath.Dir(self)}, dirs...)
	}
	return dirs
}

// resolve returns the path of the pf-bridge-<name> binary, preferring the one beside this executable.
func resolve(name string) (string, error) {
	file := prefix + name
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	if self, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(self), file)
		if info, err := os.Stat(sibling); err == nil && !info.IsDir() {
			return sibling, nil
		}
	}
	return exec.LookPath(prefix + name)
}

// installed reports whether a pf-bridge-<name> binary is beside this executable or on PATH.
func installed(name string) bool {
	_, err := resolve(name)
	return err == nil
}

// discover returns every pf-bridge-* binary beside this executable or on PATH.
func discover() []string {
	return discoverIn(searchDirs())
}

// dispatch runs the sibling binary with this process's streams and exits with its status.
func dispatch(name string, args []string) {
	cmd := command(context.Background(), name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	genlog.Debug("dispatch to sibling", "command", name, "path", cmd.Path)
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		os.Exit(exit.ExitCode())
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error: run %s: %s\n", cmd.Path, err)
		genlog.FlushDebug()
		os.Exit(1)
	}
}

// command runs the pf-bridge-<name> sibling, falling back to the bare name when it is not installed.
func command(ctx context.Context, name string, args ...string) *exec.Cmd {
	path, err := resolve(name)
	if err != nil {
		path = prefix + name
	}
	return exec.CommandContext(ctx, path, args...) // #nosec G204 -- resolves a sibling on a fixed prefix; dispatching is this command's job
}

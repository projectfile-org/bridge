// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os"
	"os/exec"
	"sort"

	"kiota.ch/projectfile/core/v2/pkg/genlog"

	"projectfile.org/projectfile/bridge/v4/internal/commands"
)

// installed reports whether name is a subcommand linked into this binary.
func installed(name string) bool {
	_, ok := commands.Mains[name]
	return ok
}

// discover returns every subcommand linked into this binary, sorted.
func discover() []string {
	out := make([]string, 0, len(commands.Mains))
	for name := range commands.Mains {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// dispatch runs the linked subcommand in this process, as its standalone binary would run.
func dispatch(name string, args []string) {
	genlog.Debug("dispatch in-process", "command", name, "args", len(args))
	os.Args = append([]string{prefix + name}, args...)
	commands.Mains[name](prefix + name)
}

// command runs a subcommand as a child of this same executable.
func command(ctx context.Context, name string, args ...string) *exec.Cmd {
	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}
	// Re-runs this executable with a linked subcommand name. exec.Command uses argv (no shell), so forwarded CLI flags cannot inject commands; G702 taint originates from os.Args and gosec provides no sanitizer, so the directive below is the only honest remediation.
	return exec.CommandContext(ctx, self, append([]string{name}, args...)...) // #nosec G204 G702 -- re-runs this executable with a linked subcommand name
}

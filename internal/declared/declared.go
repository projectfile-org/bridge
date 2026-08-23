// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package declared answers "which bridges does THIS project maintain?" from
// the project's own document, so a fan-out stops meaning "everything installed
// on PATH".
//
// The answer already exists: every derived file ships as a check/generate pair
// of `org.projectfile.ci.tools` rows, and the check half spells the exact
// invocation — `run: pf-bridge ignore .gitignore --check`. Reading those rows
// keeps ONE declaration of what a project derives (the CI manifest its build
// already runs) instead of a second list to keep in sync with it.
package declared

import (
	"fmt"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

const (
	// dispatcherName is the argv[0] a row must name for its run line to be a
	// bridge invocation this package understands.
	dispatcherName = "pf-bridge"
	// checkFlag marks the read-only half of a derived-file pair.
	checkFlag = "--check"

	ciNS     = "org.projectfile.ci"
	toolsKey = "tools"
	runKey   = "run"
)

// Run is one declared invocation: the child binary suffix plus the arguments
// the row spelled after it (filename, flags), minus the `pf-bridge` head.
type Run struct {
	Tool string   // manifest row name, e.g. pf-bridge-gitignore-check
	Name string   // bridge name → pf-bridge-<Name>, e.g. ignore
	Args []string // what the row said after the bridge name, e.g. [.gitignore --check]
}

// Checks reads the project document at dir and returns every declared
// `pf-bridge … --check` invocation, ordered by row name so a sweep reports in
// the same order twice. An empty result means the project declares no bridge
// checks — the caller decides what to do with that, since "no manifest" and
// "a manifest with no bridges" are the same answer to this question.
func Checks(dir string, opts projectfile.ReadOptions) ([]Run, error) {
	pf, _, err := projectfile.ReadWithOptions(dir, opts)
	if err != nil {
		return nil, fmt.Errorf("read projectfile: %w", err)
	}
	return ChecksOf(pf), nil
}

// ChecksOf extracts the declared checks from an already-read document.
func ChecksOf(pf *projectfile.Document) []Run {
	tools, ok := toolRows(pf)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)

	var runs []Run
	seen := map[string]bool{}
	for _, name := range names {
		row, ok := tools[name].(map[string]any)
		if !ok {
			continue
		}
		line, ok := row[runKey].(string)
		if !ok {
			continue
		}
		run, ok := parseRun(name, line)
		if !ok || seen[run.Name+" "+strings.Join(run.Args, " ")] {
			continue
		}
		seen[run.Name+" "+strings.Join(run.Args, " ")] = true
		runs = append(runs, run)
	}
	return runs
}

// toolRows resolves org.projectfile.ci.tools on the merged document.
func toolRows(pf *projectfile.Document) (map[string]any, bool) {
	raw, ok := projectfile.LookupExtension(pf, ciNS)
	if !ok {
		return nil, false
	}
	ci, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	tools, ok := ci[toolsKey].(map[string]any)
	return tools, ok
}

// parseRun turns one `run:` line into a Run, keeping only the read-only bridge
// invocations. A row is ours when it drives the dispatcher (`pf-bridge …`),
// names a bridge, and carries --check; anything else (a generate row, pf-cli,
// a project's own script) is somebody else's tool and stays untouched.
//
// The line is split on whitespace, never handed to a shell: a manifest row is
// as trusted as the make recipe that already runs it, and exec-ing the words
// keeps it that way.
func parseRun(tool, line string) (Run, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != dispatcherName {
		return Run{}, false
	}
	args := fields[2:]
	if !hasCheck(args) {
		return Run{}, false
	}
	return Run{Tool: tool, Name: fields[1], Args: args}, true
}

func hasCheck(args []string) bool {
	for _, a := range args {
		if a == checkFlag {
			return true
		}
	}
	return false
}

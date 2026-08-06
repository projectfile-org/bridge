// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package rootflags carries the persistent CLI flags shared by every pf-bridge
// binary (the dispatcher, each per-bridge binary, forge, scan, init). Extracted
// so a per-bridge main can bind the same global surface without linking the
// other commands. Mirrors pf-cli's global flags so a projectfile behaves
// identically whichever binary drives it.
package rootflags

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/spdx"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
)

var (
	quietFlag            bool
	verboseFlag          bool
	ignoreUserConfigFlag bool
	offlineFlag          bool
	sortedFlag           bool
	failOnFlag           = failOnFlagValue{level: projectfile.FailOnError}
)

// Offline reports whether --offline was set. Read after flag parsing.
func Offline() bool { return offlineFlag }

// ReadOpts builds the shared ReadOptions from the include-resolution flags so
// every command resolves a projectfile the same way.
func ReadOpts() projectfile.ReadOptions {
	return projectfile.ReadOptions{Offline: offlineFlag, FailOn: failOnFlag.level}
}

// Bind registers the persistent flags on root and installs the shared
// PersistentPreRun that pushes the parsed values into the core packages.
func Bind(root *cobra.Command) {
	root.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		genlog.SetQuiet(quietFlag)
		if !verboseFlag {
			if v, _ := strconv.ParseBool(os.Getenv("PF_CLI_VERBOSE")); v {
				verboseFlag = true
			}
		}
		genlog.SetVerbose(verboseFlag)
		userconfig.SetIgnored(ignoreUserConfigFlag)
		projectfile.SetYAMLOutputSorted(sortedFlag)
		// Claim the "bridge" cache slot so pf-bridge reads/writes its own
		// SPDX + includes cache ($XDG_CACHE_HOME/projectfile/bridge/) and never
		// collides with pf-cli or ci-resolver. Both the spdx and the include
		// resolvers route through this slot.
		projectfile.SetCacheApp("bridge")
		spdx.SetCacheApp("bridge")
		if offlineFlag {
			genlog.Info("offline mode", "message", "network fetches disabled")
		}
	}

	pf := root.PersistentFlags()
	pf.BoolVarP(&quietFlag, "quiet", "q", false,
		"suppress info/decision-trace output; warnings and errors still print")
	pf.BoolVarP(&verboseFlag, "verbose", "v", false,
		"show operational log lines (file detection, includes, locks); also PF_CLI_VERBOSE=1")
	pf.BoolVar(&ignoreUserConfigFlag, "ignore-user-config", false,
		"skip $XDG_CONFIG_HOME/projectfile/cli.* loading — run as if no personal config existed")
	pf.BoolVar(&offlineFlag, "offline", false,
		"refuse all network fetches; use embedded and cached data only")
	pf.BoolVar(&sortedFlag, "sorted", false,
		"write YAML keys in sorted (alphabetical) order; disable canvas round-trip key preservation")
	pf.Var(&failOnFlag, "fail-on",
		"abort when an include-resolution problem reaches this severity: "+
			"'error' (default; a missing local include warns and is skipped) or "+
			"'warning' (a missing local include aborts the command)")
}

// failOnFlagValue parses --fail-on. pflag calls Set during parse, so an invalid
// value is rejected before any command runs. Default FailOnError: a missing
// local include warns and is skipped so consumers are not blocked while an
// include is fixed upstream. --fail-on=warning restores hard-fail strictness.
type failOnFlagValue struct{ level projectfile.IncludeFailLevel }

func (f *failOnFlagValue) String() string {
	if f.level == projectfile.FailOnWarning {
		return "warning"
	}
	return "error"
}

func (f *failOnFlagValue) Set(s string) error {
	switch s {
	case "", "error":
		f.level = projectfile.FailOnError
	case "warning":
		f.level = projectfile.FailOnWarning
	default:
		return fmt.Errorf("invalid value %q for --fail-on (must be 'error' or 'warning')", s)
	}
	return nil
}

func (f *failOnFlagValue) Type() string { return "string" }

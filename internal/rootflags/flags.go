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
	"time"

	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/netfetch"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
)

var (
	quietFlag            bool
	verboseFlag          bool
	ignoreUserConfigFlag bool
	offlineFlag          bool
	sortedFlag           bool
	failOnFlag           = failOnFlagValue{level: projectfile.FailOnError}
	colorsFlag           = genlog.ColorAuto
	timeoutFlag          = netfetch.DefaultTimeout
)

// Offline reports whether --offline was set. Read after flag parsing.
func Offline() bool { return offlineFlag }

// ReadOpts builds the shared ReadOptions from the include-resolution flags so
// every command resolves a projectfile the same way.
func ReadOpts() projectfile.ReadOptions {
	return projectfile.ReadOptions{Offline: offlineFlag, FailOn: failOnFlag.level}
}

// envBool reports whether the named env var parses as true.
func envBool(name string) bool {
	v, err := strconv.ParseBool(os.Getenv(name))
	return err == nil && v
}

// Bind registers the persistent flags on root and installs the shared
// PersistentPreRun that pushes the parsed values into the core packages.
// Every flag defaults from its PF_BRIDGE_* env var so CI can set behaviour
// without editing command lines; an explicit CLI flag always wins.
func Bind(root *cobra.Command) {
	quietFlag = quietFlag || envBool("PF_BRIDGE_QUIET")
	verboseFlag = verboseFlag || envBool("PF_BRIDGE_VERBOSE") || envBool("PF_CLI_VERBOSE")
	ignoreUserConfigFlag = ignoreUserConfigFlag || envBool("PF_BRIDGE_IGNORE_USER_CONFIG")
	offlineFlag = offlineFlag || envBool("PF_BRIDGE_OFFLINE")
	sortedFlag = sortedFlag || envBool("PF_BRIDGE_SORTED")
	if v := os.Getenv("PF_BRIDGE_FAIL_ON"); v != "" {
		_ = failOnFlag.Set(v)
	}
	if v := os.Getenv("PF_BRIDGE_COLORS"); v != "" {
		colorsFlag = v
	}
	if envBool("PF_BRIDGE_NO_COLOR") {
		colorsFlag = genlog.ColorNever
	}
	if d, err := time.ParseDuration(os.Getenv("PF_BRIDGE_TIMEOUT")); err == nil {
		timeoutFlag = d
	}
	root.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		if err := genlog.SetColor(colorsFlag); err != nil {
			return err
		}
		genlog.SetResultOutput(c.OutOrStdout())
		netfetch.SetTimeout(timeoutFlag)
		genlog.SetQuiet(quietFlag)
		genlog.SetVerbose(verboseFlag)
		userconfig.SetIgnored(ignoreUserConfigFlag)
		projectfile.SetYAMLOutputSorted(sortedFlag)
		if offlineFlag {
			genlog.Info("offline mode", "message", "network fetches disabled")
		}
		genlog.Debug("output", "colors", colorsFlag, "timeout", netfetch.Timeout())
		return nil
	}
	pf := root.PersistentFlags()
	pf.BoolVarP(&quietFlag, "quiet", "q", false,
		"suppress info/decision-trace output; warnings and errors still print")
	pf.BoolVarP(&verboseFlag, "verbose", "v", false,
		"show operational log lines (file detection, includes, locks)")
	pf.BoolVar(&ignoreUserConfigFlag, "ignore-user-config", false,
		"skip user config loading — run as if no personal config existed")
	pf.BoolVar(&offlineFlag, "offline", false,
		"refuse all network fetches; use embedded and cached data only")
	pf.BoolVar(&sortedFlag, "sorted", false,
		"write YAML keys in sorted (alphabetical) order; disable canvas round-trip key preservation")
	pf.Var(&failOnFlag, "fail-on",
		"abort when an include-resolution problem reaches this severity: "+
			"'error' (default; a missing local include warns and is skipped) or "+
			"'warning' (a missing local include aborts the command)")
	pf.StringVar(&colorsFlag, "colors", colorsFlag, "colour output: auto, always or never; PF_BRIDGE_NO_COLOR=1 means never")
	pf.DurationVar(&timeoutFlag, "timeout", timeoutFlag, "per-attempt network timeout; retries and backoff are built in")
	root.SetOut(genlog.Styled(os.Stdout))
	root.SetErr(genlog.Styled(os.Stderr))
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(c *cobra.Command, args []string) {
		if err := genlog.SetColor(colorsFlag); err != nil {
			genlog.Warn("colors", "err", err.Error())
		}
		defaultHelp(c, args)
	})
	if root.Version != "" {
		root.Flags().BoolP("version", "V", false, "print the version and exit")
	}
	root.SetHelpTemplate(root.HelpTemplate() + "\nEvery --flag above defaults from its PF_BRIDGE_* env var.\n")
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

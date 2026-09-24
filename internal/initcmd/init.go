// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package initcmd is the pf-bridge-init binary's command: scaffold a new
// projectfile document.
package initcmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/buildinfo"
	"projectfile.org/projectfile/bridge/internal/describe"
	"projectfile.org/projectfile/bridge/internal/rootflags"
	"projectfile.org/projectfile/bridge/internal/scaffold"
)

var (
	initFormat         string
	initNonInteractive bool
	initNamespace      string
	initName           string
	initLicense        string
	initNoScan         bool
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Scaffold a new projectfile document (prefer pf-cli init)",
	Long: "Create a new projectfile.yaml (or .toml/.json) by discovering metadata from\n" +
		"known sources (CITATION.cff, etc.) and prompting for missing required fields.\n" +
		"Prefer `pf-cli init`, which delegates here when pf-bridge is installed and\n" +
		"falls back to a basic scaffold otherwise.",
	Aliases:       []string{"scaffold"},
	Args:          cobra.MaximumNArgs(1),
	Version:       buildinfo.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(_ *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		opts := scaffold.Options{
			Dir:            dir,
			Format:         initFormat,
			NonInteractive: initNonInteractive,
			Namespace:      initNamespace,
			Name:           initName,
			License:        initLicense,
			NoScan:         initNoScan,
		}

		return scaffold.Run(opts)
	},
}

// Main is the pf-bridge-init entry point.
func Main(binName string) {
	initCmd.Use = binName + " [directory]"
	initCmd.Flags().StringVarP(&initFormat, "format", "f", "", "output format: yaml, toml, json (default: prompt)")
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "fail if required fields are missing instead of prompting")
	initCmd.Flags().StringVarP(&initNamespace, "namespace", "", "", "identity.namespace (reverse-DNS, e.g. org.example)")
	initCmd.Flags().StringVarP(&initName, "name", "", "", "identity.name (project slug)")
	initCmd.Flags().StringVarP(&initLicense, "license", "", "", "license SPDX expression (e.g. MIT)")
	initCmd.Flags().BoolVar(&initNoScan, "no-scan", false, "skip init-time scanners (git history, stack detection)")
	rootflags.Bind(initCmd)

	if describe.Handled(initCmd, os.Args[1:]) {
		return
	}
	if err := initCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		genlog.FlushDebug()
		os.Exit(1)
	}
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package bookcmd is the pf-bridge-book binary's command.
package bookcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/book"
	"projectfile.org/projectfile/bridge/v4/internal/buildinfo"
	"projectfile.org/projectfile/bridge/v4/internal/derive"
	"projectfile.org/projectfile/bridge/v4/internal/describe"
	"projectfile.org/projectfile/bridge/v4/internal/rootflags"
)

var (
	outputDir   string
	renderedDir string
)

var rootCmd = &cobra.Command{
	Use:   "book [directory]",
	Short: "Stage one mdBook per language",
	Long: "Write one mdBook source tree per declared language under --output/<lang>/:\n" +
		"book.toml, src/SUMMARY.md and a copy of every page at its repository path.\n" +
		"The README opens the book; the parts follow org.projectfile.conventions.book,\n" +
		"and each org.projectfile.pages collection is read from --rendered/<lang>/<name>/,\n" +
		"where its executable rendered it, each page followed by its hand-written\n" +
		"docs/<name>.d/<id>.md when one exists. A page with no translation is the\n" +
		"default-language page under a note saying so.",
	Example: "  pf-bridge book\n" +
		"  pf-bridge book --output dist/book --rendered dist/pages ../project",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		pf, _, err := projectfile.ReadWithOptions(dir, rootflags.ReadOpts())
		if err != nil {
			return fmt.Errorf("read projectfile: %w", err)
		}
		derive.AddVirtual(pf)
		langs, err := book.Stage(dir, under(dir, renderedDir), under(dir, outputDir), pf)
		if err != nil {
			return err
		}
		for _, lang := range langs {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(under(dir, outputDir), lang))
		}
		return nil
	},
}

// under resolves a relative flag path against the project directory.
func under(dir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// Main is the pf-bridge-book entry point.
func Main(binName string) {
	rootCmd.Use = binName + " [directory]"
	rootCmd.Version = buildinfo.Version
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	f := rootCmd.Flags()
	f.StringVar(&outputDir, "output", "dist/book", "directory each language’s book is staged under")
	f.StringVar(&renderedDir, "rendered", "dist/pages", "directory the page collections were rendered under, one <lang>/<name>/ each")
	rootflags.Bind(rootCmd)

	if describe.Handled(rootCmd, os.Args[1:]) {
		return
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		genlog.FlushDebug()
		os.Exit(1)
	}
}

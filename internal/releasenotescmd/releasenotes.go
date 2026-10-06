// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package releasenotescmd is the pf-bridge-release-notes binary's command.
package releasenotescmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/buildinfo"
	"projectfile.org/projectfile/bridge/internal/describe"
	"projectfile.org/projectfile/bridge/internal/releasenotes"
	"projectfile.org/projectfile/bridge/internal/rootflags"
)

var (
	opts     releasenotes.Options
	noteFile string
)

var rootCmd = &cobra.Command{
	Use:   "release-notes",
	Short: "Render release notes from git",
}

var renderCmd = &cobra.Command{
	Use:   "render [directory]",
	Short: "Print the release notes of a range as Markdown",
	Long: "Print the human half of a release body to stdout: a summary line, the\n" +
		"maintainer note, upgrade notes, the features.d fragments the range adds,\n" +
		"the roadmap.d fragments it deletes, the release-notes.d fragments, the\n" +
		"installation and usage blocks of the README pinned to --tag, the\n" +
		"Conventional Commits grouped by type, suppression changes and new\n" +
		"contributors. Empty sections are left out.\n" +
		"\n" +
		"With no --from, the range starts at the newest tag below --tag: a\n" +
		"candidate (-rc.N) follows the previous tag, a final the previous final.",
	Example: "  pf-bridge release-notes render --tag 1.4.0\n" +
		"  pf-bridge release-notes render --from 1.3.0 --to HEAD --note-file NOTES.md",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts.Dir = "."
		if len(args) > 0 {
			opts.Dir = args[0]
		}
		if noteFile != "" {
			note, err := os.ReadFile(noteFile) // #nosec G304 -- the caller names the note file to read
			if err != nil {
				return fmt.Errorf("read note file: %w", err)
			}
			opts.Note = string(note)
		}
		opts.Read = rootflags.ReadOpts()
		body, err := releasenotes.Render(opts)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), body)
		return err
	},
}

// Main is the pf-bridge-release-notes entry point.
func Main(binName string) {
	rootCmd.Use = binName
	rootCmd.Version = buildinfo.Version
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	f := renderCmd.Flags()
	f.StringVar(&opts.Tag, "tag", "", "name of the release being described, picks the previous tag")
	f.StringVar(&opts.From, "from", "", "start of the range (default: the previous tag)")
	f.StringVar(&opts.To, "to", "HEAD", "end of the range")
	f.StringVar(&opts.Prefix, "tag-prefix", "", "prefix of every version tag")
	f.StringVar(&noteFile, "note-file", "", "Markdown placed under the summary line")
	f.DurationVar(&opts.Timeout, "timeout", 30*time.Second, "limit for each git call")
	rootCmd.AddCommand(renderCmd)
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

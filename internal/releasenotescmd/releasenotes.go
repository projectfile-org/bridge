// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package releasenotescmd is the pf-bridge-release-notes binary's command.
package releasenotescmd

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	pub      releasenotes.PublishOptions
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
		"Conventional Commits grouped by type, suppression changes and new\n" +
		"contributors. Empty sections are left out. What one forge published\n" +
		"is added by publish.\n" +
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

var publishCmd = &cobra.Command{
	Use:   "publish [directory]",
	Short: "Add what this forge published to the release of a tag",
	Long: "Append the forge half to the body of one forge’s release of --tag:\n" +
		"installation and usage pinned to the tag and narrowed to the registries\n" +
		"this forge pushed to, cosign and gpg verify commands, magnet links read\n" +
		"from the release’s .magnet assets and a compare link. A re-run replaces\n" +
		"the forge half it wrote before; an unchanged body is not written.\n" +
		"\n" +
		"Every flag defaults from the CI environment both GitHub Actions and\n" +
		"Forgejo Actions set; the forge is the first label of the server host.\n" +
		"The token is read from FORGEJO_TOKEN, GH_TOKEN or GITHUB_TOKEN.",
	Example: "  pf-bridge release-notes publish --tag 1.4.0\n" +
		"  pf-bridge release-notes publish --tag 1.4.0 --server https://kiota.ch --repository o/r --dry-run",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pub.Dir = "."
		if len(args) > 0 {
			pub.Dir = args[0]
		}
		if pub.Tag == "" {
			return fmt.Errorf("--tag is required (or GITHUB_REF_NAME)")
		}
		if pub.Server == "" || pub.Repo == "" {
			return fmt.Errorf("--server and --repository are required (or GITHUB_SERVER_URL and GITHUB_REPOSITORY)")
		}
		if pub.API == "" {
			pub.API = strings.TrimSuffix(pub.Server, "/") + "/api/v1"
		}
		if pub.Forge == "" {
			pub.Forge = forgeSlug(pub.Server)
		}
		pub.Token = firstEnv("FORGEJO_TOKEN", "GH_TOKEN", "GITHUB_TOKEN")
		if pub.Token == "" && !pub.DryRun {
			return fmt.Errorf("no token: set FORGEJO_TOKEN, GH_TOKEN or GITHUB_TOKEN to write the release")
		}
		pub.Read = rootflags.ReadOpts()
		genlog.Info("publish", "forge", pub.Forge, "repo", pub.Repo, "tag", pub.Tag, "dry-run", pub.DryRun)
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*pub.Timeout)
		defer cancel()
		body, err := releasenotes.Publish(ctx, pub)
		if err != nil {
			return err
		}
		if pub.DryRun {
			_, err = fmt.Fprint(cmd.OutOrStdout(), body)
		}
		return err
	},
}

// forgeSlug is the first label of the server host, the rule pf-ci keys publish routes by.
func forgeSlug(server string) string {
	host := server
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	host, _, _ = strings.Cut(host, "/")
	label, _, _ := strings.Cut(host, ".")
	return label
}

// firstEnv returns the value of the first set variable among names.
func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			genlog.Info("token read", "env", n)
			return v
		}
	}
	return ""
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

	p := publishCmd.Flags()
	p.StringVar(&pub.Tag, "tag", os.Getenv("GITHUB_REF_NAME"), "release to describe")
	p.StringVar(&pub.Prefix, "tag-prefix", "", "prefix of every version tag")
	p.StringVar(&pub.Server, "server", os.Getenv("GITHUB_SERVER_URL"), "forge base URL")
	p.StringVar(&pub.API, "api", os.Getenv("GITHUB_API_URL"), "forge API base URL (default: <server>/api/v1)")
	p.StringVar(&pub.Repo, "repository", os.Getenv("GITHUB_REPOSITORY"), "owner/name on the forge")
	p.StringVar(&pub.Forge, "forge", "", "publish route of this forge (default: first label of the server host)")
	p.BoolVar(&pub.DryRun, "dry-run", false, "print the new body instead of writing it")
	p.DurationVar(&pub.Timeout, "timeout", 30*time.Second, "limit for each git and HTTP call")
	rootCmd.AddCommand(publishCmd)
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

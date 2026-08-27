// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package forgecmd is the pf-bridge-forge binary's command: push projectfile
// identity metadata out to the repository forge.
package forgecmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/buildinfo"
	"projectfile.org/projectfile/bridge/internal/describe"
	forgecore "projectfile.org/projectfile/bridge/internal/forge/core"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
	"projectfile.org/projectfile/bridge/internal/rootflags"

	// Blank-import every driver so its init() populates the registry.
	// Adding a new driver is a single import line here, mirroring sync.go.
	_ "projectfile.org/projectfile/bridge/internal/forge/drivers/forgejo"
	_ "projectfile.org/projectfile/bridge/internal/forge/drivers/github"
	_ "projectfile.org/projectfile/bridge/internal/forge/drivers/gitlab"
)

var (
	forgePushDryRun      bool
	forgePushRepos       []string
	forgePushFields      []string
	forgePushTimeout     time.Duration
	forgePushInsecureTLS bool
)

var forgeCmd = &cobra.Command{
	Use:   "forge",
	Short: "Push projectfile metadata out to the repository forge",
	Long: "Sync identity fields (description, homepage, topics) from\n" +
		"projectfile to every non-archive entry in top-level repositories[].\n" +
		"\n" +
		"Tokens are read from environment variables — standard GITHUB_TOKEN /\n" +
		"GITLAB_TOKEN / FORGEJO_TOKEN, plus PF_FORGE_TOKEN_<HOST> for self-hosted\n" +
		"instances. Use `forge list` to see which repos have a token configured.\n" +
		"\n" +
		"Self-hosted instances on bare hostnames (where the hostname doesn't\n" +
		"contain 'gitea.' / 'forgejo.' / 'gitlab.') are not auto-detected.\n" +
		"Declare them via [org.projectfile.forge.kinds] in projectfile.toml, e.g.\n" +
		"    [org.projectfile.forge.kinds]\n" +
		"    \"code.example.com\" = \"forgejo\"",
	Version:       buildinfo.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var forgePushCmd = &cobra.Command{
	Use:   "push [forge] [directory]",
	Short: "Push description/homepage/topics from projectfile to each forge",
	Long: "For every entry in top-level repositories[] (role != archive),\n" +
		"compare the projectfile's identity fields against the forge's current\n" +
		"state and push the per-field delta. Idempotent: a second run is a no-op.\n" +
		"\n" +
		"Optional [forge] restricts to repos on a single forge (matched by host\n" +
		"substring, e.g. 'github', 'codeberg', 'kiota'). If the first arg is a\n" +
		"directory that contains a projectfile, it is used as [directory] instead\n" +
		"and every forge runs.",
	Args: cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if rootflags.Offline() {
			return errors.New("forge push requires network access; remove --offline to proceed")
		}

		forge, dir := parsePushArgs(args)

		pf, _, err := projectfile.ReadWithOptions(dir, rootflags.ReadOpts())
		if err != nil {
			return fmt.Errorf("read projectfile: %w", err)
		}

		// Honour SIGINT mid-loop so a user can ctrl-C between repos without
		// leaving the process unresponsive during the retry sleep.
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		opts := forgecore.PushOptions{
			DryRun: forgePushDryRun,
			Repos:  forgePushRepos,
			Fields: forgePushFields,
			Forge:  forge,
			Resolve: forgecore.NewResolver(forgecore.HTTPOptions{
				Timeout:         forgePushTimeout,
				UserAgent:       "pf-cli/" + buildinfo.Version,
				InsecureSkipTLS: forgePushInsecureTLS,
			}),
		}

		if forgePushDryRun {
			genlog.Plain("DRY RUN — no forge metadata will be written")
		}

		res, err := forgecore.Push(ctx, pf, opts)
		if err != nil {
			return err
		}
		genlog.Plain(res.Format())
		return nil
	},
}

var forgeListCmd = &cobra.Command{
	Use:   "list [directory]",
	Short: "List repositories and whether a forge token is available",
	Long: "Print a table of every top-level repositories[] entry: the\n" +
		"resolved forge kind, whether a token env var is set, and the next-\n" +
		"action status (\"ready\" / \"no token (set ...)\" / \"unsupported\").",
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		pf, _, err := projectfile.ReadWithOptions(dir, rootflags.ReadOpts())
		if err != nil {
			return fmt.Errorf("read projectfile: %w", err)
		}
		if len(pf.Repositories) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "(no entries in top-level repositories[])")
			return nil
		}
		// `forge list` should reflect what `forge push` would actually do —
		// including any extension-declared kinds override — so the user
		// doesn't see "unknown" for a host they've already configured.
		ext, _ := pfmodel.GetForgeExtension(pf)
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "REPO\tFORGE\tTOKEN\tSTATUS")
		for _, repo := range pf.Repositories {
			rule, host := hostmatch.Resolve(repo.URL)
			kind := resolveListKind(ext, rule, host)
			token, status := "missing", "unsupported host"
			if kind == "" {
				kind = "unknown"
			}
			if kind != "unknown" {
				if t, _ := forgecore.ResolveToken(kind, host); t != "" {
					token, status = "present", "ready"
				} else {
					envName := forgecore.HostOverrideEnvVar(host)
					if v := forgecore.CanonicalEnvVar(kind); v != "" {
						envName = envName + " or " + v
					}
					status = "no token (set " + envName + ")"
				}
			}
			if repo.Role == projectfile.RepositoryRoleArchive {
				status = "skipped (archive role)"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", repo.URL, kind, token, status)
		}
		return w.Flush()
	},
}

// Main is the pf-bridge-forge entry point.
func Main(binName string) {
	forgeCmd.Use = binName
	forgePushCmd.Flags().BoolVarP(&forgePushDryRun, "dry-run", "n", false, "preview the diff; make no PATCH/PUT calls")
	forgePushCmd.Flags().StringSliceVar(&forgePushRepos, "repo", nil, "restrict to specific repository URLs (repeatable)")
	forgePushCmd.Flags().StringSliceVar(&forgePushFields, "field", nil,
		"restrict to specific fields: "+strings.Join(forgecore.AllFields, ", ")+" (repeatable)")
	forgePushCmd.Flags().BoolVar(&forgePushInsecureTLS, "insecure-skip-tls", false,
		"skip TLS verification (for self-hosted instances with self-signed certs)")
	forgePushCmd.Flags().DurationVar(&forgePushTimeout, "timeout", 10*time.Second, "per-request HTTP timeout")

	forgeCmd.AddCommand(forgePushCmd)
	forgeCmd.AddCommand(forgeListCmd)
	rootflags.Bind(forgeCmd)

	if describe.Handled(forgeCmd, os.Args[1:]) {
		return
	}
	if err := forgeCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// parsePushArgs disambiguates the optional [forge] and [directory]
// positionals. If the first arg is a directory containing a projectfile,
// it is used as [directory] and every forge runs. Otherwise the first
// arg is the forge filter and the second (if any) is the directory.
func parsePushArgs(args []string) (forge, dir string) {
	if len(args) == 0 {
		return "", "."
	}
	if isProjectfileDir(args[0]) {
		return "", args[0]
	}
	forge = args[0]
	if len(args) > 1 {
		dir = args[1]
	} else {
		dir = "."
	}
	return forge, dir
}

// isProjectfileDir reports whether dir contains a readable projectfile.
func isProjectfileDir(dir string) bool {
	_, _, err := projectfile.ReadWithOptions(dir, rootflags.ReadOpts())
	return err == nil
}

// resolveListKind mirrors forge/core.resolveKind: the extension's per-host
// declaration wins over hostmatch. Lives here rather than the core package
// because the list command also needs the lookup but doesn't want to pull
// in the rest of the push algorithm.
func resolveListKind(ext *pfmodel.ForgeExtension, rule *hostmatch.Rule, host string) string {
	if ext != nil && ext.Kinds != nil {
		if k, ok := ext.Kinds[host]; ok && k != "" {
			return k
		}
	}
	if rule != nil {
		return string(rule.Kind)
	}
	return ""
}

// _ keeps the context import alive across refactors; the cmd builder uses
// cmd.Context() but the package import surface should never go unused.
var _ = context.Background

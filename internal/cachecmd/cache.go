// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package cachecmd is the pf-bridge-cache binary's command: warm, inspect, and
// purge the SPDX license-text cache (and the bridge's HTTP include cache).
//
// SPDX lives here because pf-bridge is the sole reader of license boilerplate
// (the license bridge calls spdx.Text); warming it from the binary that reads
// it keeps the corpus and its cache in one place. Deprecated SPDX ids are
// skipped during warm — they have no upstream text/ file, so fetching one only
// ever 404s.
package cachecmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/spdx"
	"projectfile.org/projectfile/bridge/internal/buildinfo"
	"projectfile.org/projectfile/bridge/internal/describe"
	"projectfile.org/projectfile/bridge/internal/rootflags"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage pf-bridge's local SPDX and include cache",
	Long: "pf-bridge keeps a local copy of SPDX license texts and HTTP includes\n" +
		"so it can render LICENSE files and read projectfiles without a network\n" +
		"connection once warmed.\n" +
		"\n" +
		"The cache lives at:\n" +
		"  ${XDG_CACHE_HOME:-~/.cache}/pf/\n" +
		"    spdx/      license boilerplate texts (warmed here, read by the license bridge)\n" +
		"    includes/  HTTP includes pf-bridge resolves while reading a projectfile\n" +
		"\n" +
		"This slot is shared with pf-cli and pf-ci — a purge here clears the cache\n" +
		"they all read.",
}

var cacheStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show what is cached and where",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		out := cmd.OutOrStdout()

		spdxDir, err := spdx.CacheDir()
		if err != nil {
			return err
		}
		s := spdx.Status()
		includes := countCachedIncludes()

		fmt.Fprintf(out, "cache location: %s\n", spdxDir)
		fmt.Fprintf(out, "SPDX:     %d embedded, %d cached\n", s.Embedded, s.Cached)
		fmt.Fprintf(out, "includes: %d cached\n", includes)
		return nil
	},
}

var cacheWarmCmd = &cobra.Command{
	Use:   "warm [directory]",
	Short: "Pre-fetch SPDX license texts and HTTP includes",
	Long: "Download SPDX license texts and the projectfile's HTTP includes so\n" +
		"pf-bridge works fully offline afterwards. Requires a network connection.\n" +
		"\n" +
		"Deprecated SPDX ids are skipped: they have no upstream text file, so\n" +
		"fetching them only ever fails. Includes are read from the projectfile\n" +
		"in [directory] (default: the current directory); if none is found,\n" +
		"only SPDX is warmed.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		if rootflags.Offline() {
			return fmt.Errorf("cannot warm cache in offline mode; remove --offline to proceed")
		}

		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		if err := warmSPDX(); err != nil {
			return err
		}
		return warmIncludes(dir)
	},
}

var cachePurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Delete every cached SPDX text and HTTP include",
	Long: "Remove all SPDX license texts and HTTP includes pf-bridge has cached.\n" +
		"The next render that needs a text or include fetches it fresh. This\n" +
		"does not touch the projectfile or any other file.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		out := cmd.OutOrStdout()

		spdxDir, err := spdx.CacheDir()
		if err != nil {
			return err
		}
		spdxRemoved, err := spdx.Purge()
		if err != nil {
			return fmt.Errorf("purge SPDX: %w", err)
		}
		includesRemoved, err := projectfile.PurgeIncludes()
		if err != nil {
			return fmt.Errorf("purge includes: %w", err)
		}
		fmt.Fprintf(out, "purged %d SPDX text(s) and %d include(s) from %s\n",
			spdxRemoved, includesRemoved, spdxDir)
		return nil
	},
}

func warmSPDX() error {
	genlog.Plain("warming SPDX license cache...")
	emb, cached, fetched, err := spdx.WarmAll()
	if err != nil {
		return fmt.Errorf("warm SPDX: %w", err)
	}
	genlog.Plain(fmt.Sprintf("SPDX: %d embedded, %d cached, %d fetched", emb, cached, fetched))
	return nil
}

func warmIncludes(dir string) error {
	// DetectPath conflates two outcomes this command must keep apart: an empty
	// directory (a legitimate no-op — SPDX is warmed, there are no includes to
	// prefetch) and an AMBIGUOUS one (2+ documents, fail-closed per spec §3.5).
	// Resolving presence first keeps the ambiguity reaching the caller instead
	// of being silently skipped. The glob carries no encoding list of its own,
	// so the accepted extensions stay owned by DetectPath.
	docs, err := filepath.Glob(filepath.Join(dir, projectfile.BaseName+".*"))
	if err != nil {
		return fmt.Errorf("scan %s for a projectfile: %w", dir, err)
	}
	if len(docs) == 0 {
		genlog.Info("no projectfile found; skipping include warm", "dir", dir)
		return nil
	}

	pfPath, err := projectfile.DetectPath(dir)
	if err != nil {
		return fmt.Errorf("detect projectfile in %s: %w", dir, err)
	}
	raw, err := projectfile.ReadRawBaseFromPath(pfPath)
	if err != nil {
		return fmt.Errorf("read projectfile: %w", err)
	}

	// Discover transitive HTTP includes: a remote fragment may itself include
	// another remote fragment, and the cache should prefetch the whole chain so
	// a subsequent --offline read finds every link present.
	includes := projectfile.AllHTTPIncludes(raw, dir, pfPath, projectfile.ReadOptions{})
	if len(includes) == 0 {
		genlog.Plain("includes: no remote includes found in projectfile")
		return nil
	}

	warmed := 0
	for _, ref := range includes {
		if err := projectfile.WarmInclude(ref); err != nil {
			genlog.Warn("include warm failed", "url", ref, "err", err.Error())
			continue
		}
		warmed++
	}
	genlog.Plain(fmt.Sprintf("includes: warmed %d/%d remote includes", warmed, len(includes)))
	return nil
}

func countCachedIncludes() int {
	dir, err := projectfile.IncludesCacheDir()
	if err != nil {
		return 0
	}
	f, err := os.Open(dir) // #nosec G304 -- path derived from XDG
	if err != nil {
		return 0
	}
	defer f.Close()
	names, err := f.Readdirnames(0)
	if err != nil {
		return 0
	}
	return len(names)
}

// Main is the pf-bridge-cache entry point.
func Main(binName string) {
	cacheCmd.Use = binName
	cacheCmd.Version = buildinfo.Version
	cacheCmd.SilenceUsage = true
	cacheCmd.SilenceErrors = true

	cacheCmd.AddCommand(cacheStatusCmd)
	cacheCmd.AddCommand(cacheWarmCmd)
	cacheCmd.AddCommand(cachePurgeCmd)
	rootflags.Bind(cacheCmd)

	if describe.Handled(cacheCmd, os.Args[1:]) {
		return
	}
	if err := cacheCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

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
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/spdx"
	"projectfile.org/projectfile/bridge/internal/buildinfo"
	"projectfile.org/projectfile/bridge/internal/describe"
	"projectfile.org/projectfile/bridge/internal/rootflags"
)

// cacheRoot resolves the shared slot and names where it came from.
func cacheRoot() (string, string) {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "pf"), "XDG_CACHE_HOME"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "XDG_CACHE_HOME|HOME"
	}
	return filepath.Join(home, ".cache", "pf"), "HOME/.cache"
}

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the local SPDX and include cache",
	Long: "Keep a local copy of SPDX license texts and HTTP includes so LICENSE\n" +
		"renders and projectfile reads work offline once warmed.\n" +
		"\n" +
		"Layout under the cache root:\n" +
		"  spdx/      license boilerplate texts (read by the license bridge)\n" +
		"  includes/  HTTP includes resolved while reading a projectfile\n" +
		"\n" +
		"One slot shared by every projectfile tool — a purge here clears it for all.",
}

var cacheStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show what is cached and where",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		out := cmd.OutOrStdout()
		root, src := cacheRoot()
		s := spdx.Status()
		ids := spdx.CachedIDs()
		sort.Strings(ids)
		entries, _ := projectfile.IncludeCacheEntries()
		fmt.Fprintf(out, "Cache: %s (from %s)\n", root, src)
		fmt.Fprintf(out, "SPDX: %d embedded, %d cached\n", s.Embedded, s.Cached)
		fmt.Fprintf(out, "  cached: %s\n", strings.Join(truncateList(ids, 20), ", "))
		fmt.Fprintf(out, "includes: %d cached\n", len(entries))
		for _, e := range entries {
			fmt.Fprintf(out, "  %s  %s  %s\n", freshnessMark(e.Fresh), e.URL, ageString(e.FetchedAt, e.Age))
		}
		return nil
	},
}

func freshnessMark(fresh bool) string {
	if fresh {
		return "fresh"
	}
	return "stale"
}

func ageString(at time.Time, age time.Duration) string {
	if at.IsZero() {
		return "(age unknown)"
	}
	return fmt.Sprintf("(fetched %s, %s ago)", at.Format("2006-01-02 15:04"), shortAge(age))
}

func shortAge(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func truncateList(in []string, limit int) []string {
	if len(in) == 0 {
		return []string{"(none)"}
	}
	if len(in) <= limit {
		return in
	}
	return append(append([]string{}, in[:limit]...), fmt.Sprintf("+%d more", len(in)-limit))
}

var cacheWarmCmd = &cobra.Command{
	Use:   "warm [directory]",
	Short: "Fetch SPDX texts and this project's HTTP includes",
	Long: "Download SPDX license texts plus the HTTP includes declared by the\n" +
		"projectfile in [directory] (default: current directory) so later runs\n" +
		"work offline. Stale entries are refreshed, missing ones fetched.\n" +
		"Requires a network connection.\n" +
		"\n" +
		"Deprecated SPDX ids are skipped: they have no upstream text file, so\n" +
		"fetching them only ever fails. With no projectfile in [directory],\n" +
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
		root, _ := cacheRoot()
		spdxRemoved, err := spdx.Purge()
		if err != nil {
			return fmt.Errorf("purge SPDX: %w", err)
		}
		includesRemoved, err := projectfile.PurgeIncludes()
		if err != nil {
			return fmt.Errorf("purge includes: %w", err)
		}
		fmt.Fprintf(out, "purged %d SPDX text(s) and %d include(s) from %s\n",
			spdxRemoved, includesRemoved, root)
		return nil
	},
}

func warmSPDX() error {
	genlog.Plain("warming SPDX license cache...")
	emb, cached, fetched, err := spdx.WarmAll()
	if err != nil {
		return fmt.Errorf("warm SPDX: %w", err)
	}
	genlog.Success(fmt.Sprintf("SPDX: %d embedded, %d cached, %d fetched", emb, cached, fetched))
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
	genlog.Success(fmt.Sprintf("includes: warmed %d/%d remote includes", warmed, len(includes)))
	return nil
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
		genlog.FlushDebug()
		os.Exit(1)
	}
}

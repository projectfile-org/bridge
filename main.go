// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Command pf-bridge is the git-style front dispatcher for the bridge tools. It
// carries no bridge code itself: `pf-bridge <name> [args]` discovers the
// matching `pf-bridge-<name>` executable on PATH and execs it (like git/kubectl
// plugins). Growth is additive — a new bridge is a new binary on PATH, no
// rebuild here. `pf-bridge all` fans the request out across every installed
// file-bridge binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"

	"projectfile.org/projectfile/bridge/internal/buildinfo"
	"projectfile.org/projectfile/bridge/internal/describe"
	"projectfile.org/projectfile/bridge/internal/warn"
)

const prefix = "pf-bridge-"

const (
	// cmdAll fans a bridge request out across every installed file-bridge binary.
	cmdAll = "all"
	// cmdCheck is the read-only twin of cmdAll: the same fan-out with --check
	// forced on, so "is this repository in sync?" is one command instead of one
	// command per bridge. Built in rather than dispatched, because there is no
	// pf-bridge-check binary to exec — check is a MODE every bridge already has.
	cmdCheck = "check"
	// flagCheck is what cmdCheck forces into every child's argv.
	flagCheck = "--check"
	// flagAll spells cmdCheck's "every bridge" default explicitly.
	flagAll = "--all"
	// dirTo / dirFrom are the direction prepositions of the per-bridge grammar.
	// The dispatcher only has to recognise them: the child re-parses its own.
	dirTo   = "to"
	dirFrom = "from"
)

// toolBinaries are pf-bridge-* that are NOT file bridges, so `all` skips them
// (they take unrelated argument grammars).
var toolBinaries = map[string]bool{"forge": true, "scan": true, "init": true, "cache": true}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage(os.Stdout)
		return
	}

	switch args[0] {
	case "-h", "--help", "help":
		usage(os.Stdout)
		return
	case "-V", "--version", "version":
		fmt.Printf("pf-bridge version %s\n", buildinfo.Version)
		return
	case "--list", "list":
		listBridges(os.Stdout)
		return
	case cmdAll, dirTo, dirFrom:
		if err := runAll(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		return
	case cmdCheck:
		if err := runCheckAll(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		return
	}

	// Git-style dispatch: pf-bridge <name> [args] → exec pf-bridge-<name>.
	name := args[0]
	path, err := exec.LookPath(prefix + name)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Error: unknown command %q — %s%s not found on PATH.\nInstalled: %s\n",
			name, prefix, name, strings.Join(discover(), ", "))
		os.Exit(1)
	}
	// Replace this process so signals/exit codes pass through cleanly.
	argv := append([]string{prefix + name}, args[1:]...)
	if err := syscall.Exec(path, argv, os.Environ()); err != nil { // #nosec G702,G204 -- path resolved via PATH discovery on a fixed prefix; dispatching is this command's job
		fmt.Fprintf(os.Stderr, "Error: exec %s: %s\n", path, err)
		os.Exit(1)
	}
}

// runAll fans a bridge request across every installed file-bridge binary.
// Grammar mirrors the old monolith: `pf-bridge all`, `pf-bridge to all`,
// `pf-bridge from all`. Each sub-binary is driven with the `all` target so
// multi-file bridges (ignore, vulnerabilities) run their whole set too.
//
// Every flag the user typed is forwarded verbatim. Dropping them — which this
// did — turned `pf-bridge all --check` into a silent WRITE sweep: the read-only
// verb the user asked for arrived at each child as a plain `all`, and files were
// overwritten by a command whose whole point was to touch nothing.
func runAll(args []string) error {
	mode := ""
	switch args[0] {
	case dirTo, dirFrom:
		mode = args[0]
		if len(args) < 2 || args[1] != cmdAll {
			return fmt.Errorf("expected %q all", mode)
		}
	case cmdAll:
		// no mode prefix
	}
	_, flags := splitNamesFlags(args)
	return fanout(mode, nil, flags)
}

// runCheckAll implements `pf-bridge check [--all|<name>...] [flags]`: the same
// fan-out with --check forced on. Naming bridges narrows it; naming none (or
// --all) checks every installed one.
func runCheckAll(args []string) error {
	names, flags := splitNamesFlags(args)
	if !slices.Contains(flags, flagCheck) {
		flags = append(flags, flagCheck)
	}
	return fanout("", names, flags)
}

// splitNamesFlags separates positional bridge names from flags, dropping the
// fan-out keywords that address this dispatcher rather than a child. A flag's
// VALUE is never a positional here: every flag the per-bridge binaries take is
// a boolean, so `--flag value` cannot occur.
func splitNamesFlags(args []string) (names, flags []string) {
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "-") && a != flagAll:
			flags = append(flags, a)
		case a == cmdAll, a == flagAll, a == cmdCheck, a == dirTo, a == dirFrom:
			// addressed to this dispatcher; the child gets `all` either way
		default:
			names = append(names, a)
		}
	}
	return names, flags
}

// fanout runs one bridge binary per name (or every installed file bridge when
// names is empty), each driven with the `all` target so multi-file bridges run
// their whole set too.
//
// Children inherit a warnings directory: each hands its ledger back instead of
// printing a summary of its own, so a 20-bridge sweep ends in ONE block naming
// every finding rather than 20 blocks the reader has to reassemble.
func fanout(mode string, names, flags []string) error {
	if len(names) == 0 {
		for _, n := range discover() {
			if !toolBinaries[n] {
				names = append(names, n)
			}
		}
	}
	if len(names) == 0 {
		return errors.New("no pf-bridge-* file-bridge binaries found on PATH")
	}

	dir, collect := warn.StartHandoff()
	defer func() {
		collect()
		warn.Summary(os.Stderr)
	}()

	env := os.Environ()
	if dir != "" {
		env = append(env, warn.EnvDir+"="+dir)
	}

	var ran, failed int
	for _, name := range names {
		if toolBinaries[name] {
			warn.Record("skipped: not a file bridge", "name", name)
			continue
		}
		subArgs := []string{}
		if mode != "" {
			subArgs = append(subArgs, mode)
		}
		subArgs = append(subArgs, cmdAll)
		subArgs = append(subArgs, flags...)

		cmd := exec.Command(prefix+name, subArgs...) // #nosec G702,G204 -- name resolved via PATH discovery on a fixed prefix; dispatching is this command's job
		cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
		cmd.Env = env
		ran++
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "pf-bridge: %s%s failed: %s\n", prefix, name, err)
			failed++
		}
	}
	if ran == 0 {
		return errors.New("no pf-bridge-* file-bridge binaries found on PATH")
	}
	if failed > 0 {
		return fmt.Errorf("%d/%d bridge binaries failed", failed, ran)
	}
	return nil
}

// goosKnown / goarchKnown are Go's platform vocabulary, used only to recognise
// release artifacts (pf-bridge-npm-linux-amd64) so discovery lists commands,
// not their cross-compile copies. A bridge genuinely named "<goos>-<goarch>"
// would be hidden — none is.
var goosKnown = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "illumos": true, "ios": true, "js": true, "linux": true,
	"netbsd": true, "openbsd": true, "plan9": true, "solaris": true,
	"wasip1": true, "windows": true, "zos": true,
}

var goarchKnown = map[string]bool{
	"386": true, "amd64": true, "arm": true, "arm64": true, "arm64be": true,
	"loong64": true, "mips": true, "mipsle": true, "mips64": true,
	"mips64le": true, "ppc64": true, "ppc64le": true, "riscv64": true,
	"s390x": true, "sparc": true, "sparc64": true, "wasm": true,
}

// isPlatformArtifact reports whether name ends in a <goos>-<goarch> pair — the
// suffix every dist/release binary carries from the cross-compile matrix. The
// suffixed dispatcher copy (pf-bridge-linux-amd64) matches too, which matters:
// listed as a "bridge" it would fan out into itself, forever.
func isPlatformArtifact(name string) bool {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return false
	}
	return goosKnown[parts[len(parts)-2]] && goarchKnown[parts[len(parts)-1]]
}

// discover returns the sorted, de-duplicated set of pf-bridge-* suffixes found
// as executables on PATH, minus cross-compile release artifacts.
func discover() []string {
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			n := e.Name()
			if !strings.HasPrefix(n, prefix) || n == "pf-bridge" {
				continue
			}
			suffix := strings.TrimPrefix(n, prefix)
			if suffix != "" && !isPlatformArtifact(suffix) {
				seen[suffix] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// listBridges writes every installed bridge with its self-description.
func listBridges(w *os.File) {
	names := discover()
	if len(names) == 0 {
		fmt.Fprintln(w, "(no pf-bridge-* binaries found on PATH)")
		return
	}
	renderList(w, withDescriptions(names), useColor(w))
}

// withDescriptions probes every child for its one-line self-introduction.
// Children that cannot answer (older install, foreign binary) keep an empty
// description rather than failing the listing; a spent deadline lists the
// rest by name alone.
func withDescriptions(names []string) []bridgeEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	entries := make([]bridgeEntry, len(names))
	for i, name := range names {
		entries[i].name = name
	}
	for i, name := range names {
		if ctx.Err() != nil {
			break
		}
		entries[i].desc = probeDescribe(ctx, name)
	}
	return entries
}

// probeDescribe asks one child who it is; empty when it cannot answer.
func probeDescribe(ctx context.Context, name string) string {
	cmd := exec.CommandContext(ctx, prefix+name, describe.Flag) // #nosec G204 -- probing the named sibling is this command's job
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line)
}

type bridgeEntry struct{ name, desc string }

// renderList writes the aligned name/description table. Names go bold when
// color is on; padding stays outside the escape so columns line up either way.
func renderList(w io.Writer, entries []bridgeEntry, color bool) {
	width := 0
	for _, e := range entries {
		if len(e.name) > width {
			width = len(e.name)
		}
	}
	for _, e := range entries {
		name := e.name
		pad := strings.Repeat(" ", width-len(name))
		if color {
			name = "\x1b[1m" + name + "\x1b[22m"
		}
		fmt.Fprintf(w, "  %s%s  %s\n", name, pad, e.desc)
	}
}

// useColor reports whether w is an interactive terminal that has not opted out
// via NO_COLOR.
func useColor(w *os.File) bool {
	return os.Getenv("NO_COLOR") == "" && isatty.IsTerminal(w.Fd())
}

func usage(w *os.File) {
	fmt.Fprintf(w, "pf-bridge — project the projectfile onto files, forges, and the repo\n\n")
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  pf-bridge <name> [args]   run the pf-bridge-<name> binary (e.g. readme, npm, forge)\n")
	fmt.Fprintf(w, "  pf-bridge all             sync every installed file bridge\n")
	fmt.Fprintf(w, "  pf-bridge check           check every installed file bridge for drift, write nothing\n")
	fmt.Fprintf(w, "  pf-bridge check <name>…   check only the named bridges\n")
	fmt.Fprintf(w, "  pf-bridge to all          write pf → every external file\n")
	fmt.Fprintf(w, "  pf-bridge from all        read every external file → pf\n")
	fmt.Fprintf(w, "  pf-bridge --list          list installed bridges with a one-line description\n\n")
	fmt.Fprintf(w, "Flags after a fan-out verb reach every bridge: `pf-bridge check --fail-on-drift`,\n")
	fmt.Fprintf(w, "`pf-bridge all --dry-run`. Drift warns by default; --fail-on-drift makes it fatal.\n\n")
	if names := discover(); len(names) > 0 {
		fmt.Fprintf(w, "Installed bridges:\n")
		renderList(w, withDescriptions(names), useColor(w))
	} else {
		fmt.Fprintf(w, "Installed: (none found on PATH)\n")
	}
}

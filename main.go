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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/buildinfo"
	"projectfile.org/projectfile/bridge/internal/declared"
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
	// flagAll asks cmdCheck for the PATH sweep instead of the declared set.
	flagAll = "--all"
	// flagOffline is the include-resolution flag this dispatcher reads for its
	// own document read; every other flag it only forwards.
	flagOffline = "--offline"
	// dirTo / dirFrom are the direction prepositions of the per-bridge grammar.
	// The dispatcher only has to recognise them: the child re-parses its own.
	dirTo   = "to"
	dirFrom = "from"
)

// toolBinaries are pf-bridge-* siblings with their own grammar, so `all` skips them.
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
	case "completion":
		printCompletion(os.Stdout, args[1:])
		return
	case cmdAll, dirTo, dirFrom:
		if err := runAll(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			genlog.FlushDebug()
			os.Exit(1)
		}
		return
	case cmdCheck:
		if err := runCheckAll(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			genlog.FlushDebug()
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
		genlog.FlushDebug()
		os.Exit(1)
	}
	// Replace this process so signals/exit codes pass through cleanly.
	argv := append([]string{prefix + name}, args[1:]...)
	if err := syscall.Exec(path, argv, os.Environ()); err != nil { // #nosec G702,G204 -- path resolved via PATH discovery on a fixed prefix; dispatching is this command's job
		fmt.Fprintf(os.Stderr, "Error: exec %s: %s\n", path, err)
		genlog.FlushDebug()
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
// fan-out with --check forced on.
//
// With no names it checks what the PROJECT declares — the `pf-bridge … --check`
// rows of its org.projectfile.ci.tools manifest — not what happens to be
// installed. The two sets are not the same: every consumer inherits ignore
// patterns, a release config and yamllint rules from the shared m6e fragments,
// so a PATH sweep renders `.yamllint`, `.containerignore` and `.releaserc.yaml`
// for projects that maintain none of them and reports each as drift. It also
// costs the CI plane one container per bridge; this way the whole gate is one
// `pf-bridge check`.
//
// Naming bridges narrows it to those; `--all` asks for the PATH sweep
// explicitly, and a project that declares no checks falls back to it.
func runCheckAll(args []string) error {
	names, flags := splitNamesFlags(args)
	if !slices.Contains(flags, flagCheck) {
		flags = append(flags, flagCheck)
	}
	if len(names) == 0 && !slices.Contains(args, flagAll) && !slices.Contains(args, cmdAll) {
		if children := declaredChildren(flags); len(children) > 0 {
			return runChildren(children)
		}
	}
	return fanout("", names, flags)
}

// declaredChildren resolves the project's declared checks into child
// invocations. Returns nil — the caller then sweeps PATH — when the document
// cannot be read or declares no bridge check at all, so the tool keeps working
// outside a projectfile-driven fleet.
//
// A declared row naming a bridge this install does not carry is WARNED and
// skipped, never fatal: a bridge reaches the fleet in two steps (publish the
// tool, rebuild the image every project runs), and between them a project
// legitimately declares a check its image cannot run yet.
func declaredChildren(flags []string) []child {
	runs, err := declared.Checks(".", readOpts(flags))
	if err != nil {
		warn.Record("declared checks unavailable — checking every installed bridge",
			"error", err.Error())
		return nil
	}
	children := make([]child, 0, len(runs))
	for _, r := range runs {
		if toolBinaries[r.Name] || isDispatcherVerb(r.Name) {
			warn.Record("skipped: not a file bridge", "tool", r.Tool, "name", r.Name)
			continue
		}
		if _, err := exec.LookPath(prefix + r.Name); err != nil {
			warn.Record("skipped: declared bridge is not installed",
				"tool", r.Tool, "binary", prefix+r.Name)
			continue
		}
		children = append(children, child{name: r.Name, args: mergeFlags(r.Args, flags)})
	}
	if len(children) > 0 {
		genlog.DebugRow("check_set", strconv.Itoa(len(children))+" declared bridge(s)",
			"org.projectfile.ci.tools", "--all checks every installed bridge instead")
	}
	return children
}

// isDispatcherVerb reports whether name addresses this dispatcher instead of a
// bridge — the guard that stops a manifest row spelled `pf-bridge all --check`
// (or a future aggregate row) from fanning out into this process again.
func isDispatcherVerb(name string) bool {
	switch name {
	case cmdAll, cmdCheck, dirTo, dirFrom:
		return true
	}
	return false
}

// mergeFlags appends the flags typed on the command line to the ones the
// manifest row already carries, dropping duplicates so a child never sees
// `--check --check`.
func mergeFlags(rowArgs, flags []string) []string {
	out := slices.Clone(rowArgs)
	for _, f := range flags {
		if !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// readOpts mirrors the include-resolution flags the children parse with cobra,
// for the ONE read this dispatcher does itself. Only --offline is honoured:
// it is the flag that decides whether a resolution may touch the network, and
// a check sweep on a runner without egress must not hang on an include fetch.
func readOpts(flags []string) projectfile.ReadOptions {
	return projectfile.ReadOptions{
		Offline: slices.Contains(flags, flagOffline),
		FailOn:  projectfile.FailOnError,
	}
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

// child is one resolved invocation: the bridge name (→ pf-bridge-<name>) and
// the complete argv tail it runs with.
type child struct {
	name string
	args []string
}

// fanout runs one bridge binary per name (or every installed file bridge when
// names is empty), each driven with the `all` target so multi-file bridges run
// their whole set too.
func fanout(mode string, names, flags []string) error {
	if len(names) == 0 {
		for _, n := range discover() {
			if !toolBinaries[n] {
				names = append(names, n)
			}
		}
	}

	children := make([]child, 0, len(names))
	for _, name := range names {
		if toolBinaries[name] {
			warn.Record("skipped: not a file bridge", "name", name)
			continue
		}
		args := []string{}
		if mode != "" {
			args = append(args, mode)
		}
		args = append(args, cmdAll)
		args = append(args, flags...)
		children = append(children, child{name: name, args: args})
	}
	return runChildren(children)
}

// runChildren execs each resolved child in order, one process per bridge.
//
// Children inherit a warnings directory: each hands its ledger back instead of
// printing a summary of its own, so a 20-bridge sweep ends in ONE block naming
// every finding rather than 20 blocks the reader has to reassemble.
func runChildren(children []child) error {
	if len(children) == 0 {
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

	var failed int
	for _, c := range children {
		cmd := exec.Command(prefix+c.name, c.args...) // #nosec G702,G204 -- name resolved via PATH discovery on a fixed prefix; dispatching is this command's job
		cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
		cmd.Env = env
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "pf-bridge: %s%s failed: %s\n", prefix, c.name, err)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d/%d bridge binaries failed", failed, len(children))
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

// listBridges writes every installed bridge with its self-description, grouped by category.
func listBridges(w *os.File) {
	names := discover()
	if len(names) == 0 {
		fmt.Fprintln(w, "(no pf-bridge-* binaries found on PATH)")
		return
	}
	renderGrouped(w, withDescriptions(names), useColor(w))
}

// printCompletion writes a shell completion script covering the dispatcher
// verbs plus every installed bridge name, so one command completes the whole tool.
func printCompletion(w io.Writer, args []string) {
	shell := "bash"
	if len(args) > 0 {
		shell = args[0]
	}
	names := append([]string{cmdAll, cmdCheck, dirTo, dirFrom, "forge", "scan", "init", "cache"}, discover()...)
	seen := map[string]bool{}
	uniq := []string{}
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			uniq = append(uniq, n)
		}
	}
	sort.Strings(uniq)
	words := strings.Join(uniq, " ")
	switch shell {
	case "zsh":
		fmt.Fprintf(w, "#compdef pf-bridge\n_pf_bridge() { _arguments '1:command:(%s)' }\ncompdef _pf_bridge pf-bridge\n", words)
	case "fish":
		fmt.Fprintf(w, "complete -c pf-bridge -f -n __fish_use_subcommand -a '%s'\n", words)
	default:
		fmt.Fprintf(w, "_pf_bridge() { local cmds=\"%s\"; COMPREPLY=($(compgen -W \"$cmds\" -- \"${COMP_WORDS[COMP_CWORD]}\")); }\ncomplete -F _pf_bridge pf-bridge\n", words)
	}
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

// Usage listing sections.
const (
	catManifests = "Package manifests"
	catCite      = "Citation and ownership"
	catDocs      = "Documentation"
	catRepo      = "Repository setup"
)

// bridgeCategories groups bridge names for the usage listing; alphabetical within each group.
var bridgeCategories = map[string]string{
	"npm": catManifests, "composer": catManifests, "pyproject": catManifests, "shard": catManifests,
	"cff": catCite, "codeowners": catCite, "funding": catCite, "fundingjson": catCite,
	"readme": catDocs, "contributing": catDocs, "coc": catDocs, "security": catDocs, "support": catDocs, "dei": catDocs, "ai-policy": catDocs, "llm": catDocs, "fragments": catDocs,
	"ignore": catRepo, "gitattributes": catRepo, "yamllint": catRepo, "releaserc": catRepo, "browserslist": catRepo, "vulnerabilities": catRepo, "license": catRepo,
}

// categoryOrder fixes the section order of the usage listing.
var categoryOrder = []string{catManifests, catCite, catDocs, catRepo}

// modeTag classifies a bridge description as two-way ([rw]) or render-only ([ro]).
func modeTag(desc string) string {
	if strings.Contains(desc, "two-way sync") {
		return "[rw]"
	}
	if strings.Contains(desc, "one-way render") || strings.Contains(desc, "sync and render") {
		return "[ro]"
	}
	return "    "
}

// renderEntries writes one aligned name/tag/description table.
func renderEntries(w io.Writer, entries []bridgeEntry, color bool) {
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
		fmt.Fprintf(w, "  %s%s  %s  %s\n", name, pad, modeTag(e.desc), e.desc)
	}
}

// renderGrouped writes tools first, then bridges grouped by category.
func renderGrouped(w io.Writer, entries []bridgeEntry, color bool) {
	var tools, bridges []bridgeEntry
	for _, e := range entries {
		if toolBinaries[e.name] {
			tools = append(tools, e)
		} else {
			bridges = append(bridges, e)
		}
	}
	if len(tools) > 0 {
		fmt.Fprintf(w, "Tools:\n")
		renderEntries(w, tools, color)
	}
	byCat := map[string][]bridgeEntry{}
	var rest []bridgeEntry
	for _, e := range bridges {
		if c, ok := bridgeCategories[e.name]; ok {
			byCat[c] = append(byCat[c], e)
		} else {
			rest = append(rest, e)
		}
	}
	for _, c := range categoryOrder {
		if len(byCat[c]) == 0 {
			continue
		}
		fmt.Fprintf(w, "%s:\n", c)
		renderEntries(w, byCat[c], color)
	}
	if len(rest) > 0 {
		fmt.Fprintf(w, "Other bridges:\n")
		renderEntries(w, rest, color)
	}
}

// renderList writes the aligned name/description table without grouping.
func renderList(w io.Writer, entries []bridgeEntry, color bool) {
	renderEntries(w, entries, color)
}

// useColor reports whether w is an interactive terminal that has not opted out
// via NO_COLOR.
func useColor(w *os.File) bool {
	return os.Getenv("NO_COLOR") == "" && isatty.IsTerminal(w.Fd())
}

func usage(w *os.File) {
	fmt.Fprintf(w, "pf-bridge — keep every project file in sync with your projectfile document\n\n")
	fmt.Fprintf(w, "A projectfile (projectfile.yaml) is the single source of truth for a project:\n")
	fmt.Fprintf(w, "identity, people, links, and tool config. pf-bridge copies that data out\n")
	fmt.Fprintf(w, "into the files each tool already reads — README.md, package.json,\n")
	fmt.Fprintf(w, "CITATION.cff, forge settings — and reads two-way files back.\n")
	fmt.Fprintf(w, "Spec: https://projectfile.org/specification/\n\n")
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  pf-bridge <bridge> [args]   run one bridge (e.g. readme, npm, forge)\n")
	fmt.Fprintf(w, "  pf-bridge all [flags]       update every file from the projectfile [rw]\n")
	fmt.Fprintf(w, "  pf-bridge check [flags]     verify declared files match, write nothing [ro]\n")
	fmt.Fprintf(w, "  pf-bridge to all            write projectfile out to every file [rw]\n")
	fmt.Fprintf(w, "  pf-bridge from all          read every two-way file back in [rw]\n")
	fmt.Fprintf(w, "  pf-bridge --list            list installed bridges\n")
	fmt.Fprintf(w, "  pf-bridge completion SHELL  print completion script (bash|zsh|fish)\n\n")
	fmt.Fprintf(w, "[rw] updates files, [ro] only reads or compares. `check` without names\n")
	fmt.Fprintf(w, "covers the bridges this project declares; `check <name>…` narrows it,\n")
	fmt.Fprintf(w, "`check --all` covers every installed bridge instead.\n\n")
	fmt.Fprintf(w, "Shared flags (also accepted after a bridge name):\n")
	fmt.Fprintf(w, "  --check, --dry-run, --force, --create-all, --fail-on-drift, --offline,\n")
	fmt.Fprintf(w, "  --quiet, --verbose. Each has a PF_BRIDGE_* env equivalent.\n")
	fmt.Fprintf(w, "  Drift warns by default; --fail-on-drift makes it fatal.\n\n")
	if names := discover(); len(names) > 0 {
		renderGrouped(w, withDescriptions(names), useColor(w))
		fmt.Fprintf(w, "\nHelp per bridge: pf-bridge <bridge> --help\n")
	} else {
		fmt.Fprintf(w, "Installed: (none found on PATH)\n")
	}
}

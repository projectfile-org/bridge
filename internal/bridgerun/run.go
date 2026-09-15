// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package bridgerun holds the bridge command's run logic, decoupled from which
// bridges are linked. Every per-bridge binary (pf-bridge-readme, pf-bridge-npm,
// …) blank-imports its own bridge package(s) and calls Main — the command is
// registry-driven, so it operates over exactly the bridges that binary links.
package bridgerun

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/pflock"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/selector"
	"projectfile.org/projectfile/bridge/internal/bridge"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/buildinfo"
	"projectfile.org/projectfile/bridge/internal/derive"
	"projectfile.org/projectfile/bridge/internal/describe"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
	"projectfile.org/projectfile/bridge/internal/rootflags"
	"projectfile.org/projectfile/bridge/internal/warn"
)

const cmdAll = "all"

// readMerged is the single door every bridge run reads the projectfile through:
// the includes-merged document plus the virtual fields derived from it
// (org.projectfile.forge.remotes today). Going through one helper is what keeps
// a template and a `${…}` reference seeing the SAME document no matter which
// entry point — one bridge, `bridge all`, or the stack filter — asked for it.
func readMerged(dir string) (*projectfile.Document, error) {
	pf, _, err := projectfile.ReadWithOptions(dir, rootflags.ReadOpts())
	if err != nil {
		return nil, fmt.Errorf("read projectfile: %w", err)
	}
	derive.AddVirtual(pf)
	return pf, nil
}

var (
	bridgeForce          bool
	bridgeDryRun         bool
	bridgePreview        bool
	bridgeNoCreate       bool
	bridgeCheck          bool
	bridgeNoDiff         bool
	bridgeFailOnDrift    bool
	bridgeList           bool
	bridgeReuseCanonical bool
)

// EnvFailOnDrift restores the hard drift gate without editing a single tool row.
// The knob has to be an ENVIRONMENT one because the run lines that carry
// `--check` live in the m6e library, pinned per project: flipping the fleet back
// to gating through them means a library edit plus 122 pin bumps, while a CI
// plane exports one variable.
const EnvFailOnDrift = "PF_BRIDGE_FAIL_ON_DRIFT"

// failOnDrift resolves the drift severity for this run. Default is WARN: drift
// is reported, diffed and summarised, and the command still exits 0.
//
// Why leniency is the default: a bridge fix reaches the fleet in two steps — the
// tool is published, then the image every project runs is rebuilt. Between them
// every consumer drifts through no fault of its own, and a hard gate at that
// moment blocks the very commits that carry the fix forward. The gate is not
// gone, it is one flag (or one exported variable) away, and the warning summary
// makes the drift louder than a buried failure ever was.
func failOnDrift() bool {
	if bridgeFailOnDrift {
		return true
	}
	v, err := strconv.ParseBool(os.Getenv(EnvFailOnDrift))
	if err != nil {
		return false
	}
	return v
}

// Main is the entry point of a per-bridge binary. binName is the argv[0]-style
// program name (e.g. "pf-bridge-readme"); the binary's blank-imports decide
// which bridge(s) the registry holds.
func Main(binName string) {
	root := &cobra.Command{
		Use:   binName + " [to|from] [filename] [directory]",
		Short: "Bridge projectfile with the external file(s) this binary carries",
		Long: "Two-way bridge between projectfile and the metadata file(s) this\n" +
			"binary provides. The first positional may be the preposition 'to' or\n" +
			"'from' to force direction; otherwise the projectfile is authoritative.\n" +
			"\n" +
			"  " + binName + "                 (sync — pf wins, empty pf fields fill from the file)\n" +
			"  " + binName + " to             (write — pf → external)\n" +
			"  " + binName + " from           (read  — external → pf)\n" +
			"\n" +
			"For derive-only bridges the direction auto-resolves to 'to'. When this\n" +
			"binary carries more than one file, pass the filename (or 'all'); with a\n" +
			"single file the name is implied. Use --list to see the registered set.",
		Version:       buildinfo.Version,
		Args:          cobra.RangeArgs(0, 3),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runBridge,
	}
	root.Flags().BoolVarP(&bridgeForce, "force", "f", false,
		"overwrite even if the file lacks the pf-cli marker; on a two-way bridge, rewrite the file even when every field already agrees")
	root.Flags().BoolVarP(&bridgeDryRun, "dry-run", "n", false,
		"show what would change without writing")
	root.Flags().BoolVar(&bridgePreview, "preview", false,
		"print the rendered file(s) to stdout instead of writing (renderers only; syncers fall back to --dry-run)")
	root.Flags().BoolVar(&bridgeNoCreate, "no-create", false,
		"do not create target file if it does not exist (syncers only)")
	root.Flags().BoolVar(&bridgeCheck, "check", false,
		"report drift instead of writing — the sync gate; implies --dry-run. Warns by default, see --fail-on-drift")
	root.Flags().BoolVar(&bridgeNoDiff, "no-diff", false,
		"under --check, report drift without the unified diff of what changed")
	root.Flags().BoolVar(&bridgeFailOnDrift, "fail-on-drift", false,
		"under --check, exit non-zero on drift instead of warning; also "+EnvFailOnDrift+"=1")
	root.Flags().BoolVar(&bridgeReuseCanonical, "reuse-canonical", false,
		"keep LICENSES/<id>.txt at the canonical SPDX text (no copyright holder/year substitution; license bridge only)")
	root.Flags().BoolVar(&bridgeList, "list", false,
		"print every registered bridge filename and exit")
	rootflags.Bind(root)

	// The dispatcher's listing probe: answer before cobra runs so no config
	// or lock work happens.
	if describe.Requested(os.Args[1:]) {
		fmt.Println(describeLine())
		return
	}

	err := root.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		genlog.FlushDebug()
	}
	// The ledger flushes LAST, after the error line, so the final thing on
	// screen is the set of findings that did not stop the run. A warning that
	// exits 0 is otherwise invisible under a buffering runner.
	warn.Summary(root.ErrOrStderr())
	if err != nil {
		os.Exit(1)
	}
}

func runBridge(cmd *cobra.Command, args []string) error {
	if bridgeList {
		return printList(cmd, bridgeFilenames())
	}

	mode, name, dir, err := parseBridgeArgs(args)
	if err != nil {
		return err
	}

	// A single-file binary implies its one filename; only prompt/list when the
	// binary carries several and none was named.
	if name == "" {
		if names := bridgeFilenames(); len(names) == 1 {
			name = names[0]
		} else if !isatty.IsTerminal(os.Stdout.Fd()) {
			return printList(cmd, names)
		} else {
			picked, err := pickBridgeTarget()
			if err != nil {
				if errors.Is(err, selector.ErrCancelled) {
					return nil
				}
				return err
			}
			name = picked
		}
	}

	if name == cmdAll {
		return runAllBridges(mode, dir, cmd)
	}

	b, ok := bridge.Lookup(name)
	if !ok {
		return fmt.Errorf("unknown bridge filename %q (known: %s; see --list)",
			name, strings.Join(bridgeFilenames(), ", "))
	}

	if bridgeDryRun {
		genlog.Plain("DRY RUN — no files will be written")
	}

	pfPath, err := projectfile.DetectPath(dir)
	if err != nil {
		return fmt.Errorf("detect projectfile: %w", err)
	}

	return pflock.WithLock(pfPath, func() error {
		return runBridgeLocked(b, mode, dir, pfPath, cmd)
	})
}

func runBridgeLocked(b core.Bridge, mode core.Mode, dir, pfPath string, cmd *cobra.Command) error {
	pf, err := readMerged(dir)
	if err != nil {
		return err
	}

	opts := core.Options{
		Dir:    dir,
		PFPath: pfPath,
		Mode:   mode,
		Force:  bridgeForce,
		// --check and --preview never write, so both carry DryRun into every
		// path that gates on it (RunSync's persist step, writeOutput's status
		// lines) even though the renderer preview path returns before reaching it.
		DryRun:         bridgeDryRun || bridgeCheck || bridgePreview,
		NoCreate:       bridgeNoCreate,
		Check:          bridgeCheck,
		Preview:        bridgePreview,
		Diff:           !bridgeNoDiff,
		WarnOnly:       !failOnDrift(),
		Offline:        rootflags.Offline(),
		ReuseCanonical: bridgeReuseCanonical,
		Stderr:         cmd.ErrOrStderr(),
	}

	if rb, ok := b.(core.RequiredFieldsBridge); ok {
		if missing := rb.RequiredFields(pf); len(missing) > 0 && isatty.IsTerminal(os.Stdout.Fd()) {
			if err := fillRequiredFields(b.Filename(), missing, pf, pfPath); err != nil {
				return err
			}
		}
	}

	switch impl := b.(type) {
	case core.Syncer:
		return runBridgeSync(impl, pf, opts)
	case core.Renderer:
		if mode == core.ModeRead {
			return fmt.Errorf("bridge %q is write-only (Renderer cannot read external)", b.Filename())
		}
		genlog.Section(b.Filename() + " (" + pfmodel.DisplayName(pf) + ")")
		return core.RunRender(impl, pf, opts)
	default:
		return fmt.Errorf("bridge %q implements neither Syncer nor Renderer", b.Filename())
	}
}

func runBridgeSync(syn core.Syncer, pf *projectfile.Document, opts core.Options) error {
	if opts.Preview {
		genlog.Warn("--preview is renderer-only; showing dry-run summary instead", "file", syn.Filename())
	}
	res, err := core.RunSync(syn, pf, opts)
	if err != nil {
		return err
	}
	genlog.Success(res.Format())
	// --check on a two-way bridge means the same thing it means on a renderer:
	// report drift, change nothing. RunSync already withheld the writes (Check
	// sets DryRun), so any field it WOULD have moved is drift.
	if opts.Check && (res.ExtChanged || res.PFChanged) {
		warn.Record("drift: file no longer matches the projectfile", "file", syn.Filename())
		warn.Hint(core.DriftHint)
		if !opts.WarnOnly {
			return fmt.Errorf("%s drifted from the projectfile (re-run without --check to sync)", syn.Filename())
		}
	}
	return nil
}

// parseBridgeArgs interprets the positional args. Returns:
//   - mode  — ModeSync by default; ModeWrite when args[0]=="to"; ModeRead when args[0]=="from"
//   - name  — the filename (empty when no positional name was given)
//   - dir   — target directory; "." by default
//
// `to` and `from` are reserved at parse time — no registered bridge has
// either as a filename, so the disambiguation is unambiguous.
func parseBridgeArgs(args []string) (core.Mode, string, string, error) {
	mode := core.ModeSync
	name := ""
	dir := "."
	rest := args

	if len(rest) > 0 {
		switch rest[0] {
		case "to":
			mode = core.ModeWrite
			rest = rest[1:]
		case "from":
			mode = core.ModeRead
			rest = rest[1:]
		}
	}
	if len(rest) > 0 {
		name = rest[0]
		rest = rest[1:]
	}
	if len(rest) > 0 {
		dir = rest[0]
		rest = rest[1:]
	}
	if len(rest) > 0 {
		return "", "", "", fmt.Errorf("too many positional args; usage: [to|from] <filename> [directory]")
	}
	if mode != core.ModeSync && name == "" {
		// A single-file binary still implies its filename under a preposition.
		if names := bridgeFilenames(); len(names) == 1 {
			name = names[0]
		} else {
			return "", "", "", fmt.Errorf("preposition %q requires a filename", string(mode))
		}
	}
	return mode, name, dir, nil
}

func bridgeFilenames() []string {
	out := []string{}
	for _, b := range bridge.List() {
		out = append(out, b.Filename())
	}
	return out
}

// describeLine answers the dispatcher's --describe probe: the file(s) this
// binary bridges and the direction of the exchange, e.g.
// "package.json — two-way sync". Every file of one binary shares the
// direction, so the suffix is stated once. Bridges implementing
// core.Describer supply their own complete line (their Filename says nothing
// on its own).
func describeLine() string {
	parts := make([]string, 0, 4)
	var files []string
	syncers, renderers := 0, 0
	for _, b := range bridge.List() {
		if d, ok := b.(core.Describer); ok {
			parts = append(parts, d.Describe())
			continue
		}
		files = append(files, b.Filename())
		switch b.(type) {
		case core.Syncer:
			syncers++
		case core.Renderer:
			renderers++
		}
	}
	switch {
	case len(files) == 0:
	case syncers > 0 && renderers > 0:
		parts = append(parts, strings.Join(files, ", ")+" — sync and render")
	case syncers > 0:
		parts = append(parts, strings.Join(files, ", ")+" — two-way sync")
	case renderers > 0:
		parts = append(parts, strings.Join(files, ", ")+" — one-way render")
	default:
		parts = append(parts, strings.Join(files, ", "))
	}
	return strings.Join(parts, ", ")
}

func pickBridgeTarget() (string, error) {
	items := bridge.List()
	chosen, err := selector.Run(selector.Choices[core.Bridge]{
		Title: "Pick a file to bridge:",
		Items: items,
		Label: func(b core.Bridge) string { return b.Filename() },
		Detail: func(b core.Bridge) string {
			switch b.(type) {
			case core.Syncer:
				return "syncer — round-trip"
			case core.Renderer:
				return policySummary(b.Policy())
			default:
				return ""
			}
		},
	})
	if err != nil {
		return "", err
	}
	return chosen.Filename(), nil
}

func policySummary(p core.Policy) string {
	if p.Marker {
		return "renderer — managed (overwrites in place with pf-cli marker)"
	}
	return "renderer — overwrite-always"
}

// fillRequiredFields drives the bubbletea fill-mode prompt over the bridge's
// Missing list. Each field's Setter mutates pf in place; after the form is
// accepted, only the filled fields are written to the BASE projectfile so
// include-inherited values are never materialised. Cancellation returns
// "cancelled" so the caller can short-circuit without error.
func fillRequiredFields(filename string, missing []core.Missing, pf *projectfile.Document, pfPath string) error {
	fields := make([]selector.FillField, len(missing))
	for i, m := range missing {
		m := m // capture by value for the closure
		fields[i] = selector.FillField{
			Label:    m.Field,
			Hint:     m.Hint,
			OnSubmit: m.Setter,
		}
	}
	if err := selector.Fill(filename+": fill required fields", fields); err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			return errors.New("cancelled")
		}
		return err
	}
	// Write only the filled fields to the base document, not the merged pf.
	preFill := pf.Clone()
	basePF, err := projectfile.ReadBaseFromPath(pfPath)
	if err != nil {
		return fmt.Errorf("read base after fill: %w", err)
	}
	projectfile.ReconcileBase(basePF, preFill, pf)
	if err := projectfile.Write(basePF, pfPath); err != nil {
		return fmt.Errorf("write projectfile after fill: %w", err)
	}
	return nil
}

// printList writes each name on its own line — used by both --list and the
// piped-no-args fallback.
func printList(cmd *cobra.Command, names []string) error {
	for _, n := range names {
		fmt.Fprintln(cmd.OutOrStdout(), n)
	}
	return nil
}

// runAllBridges runs every registered bridge in Filename-sorted order under a
// single lock. Bridges that declare a StackAware preference are skipped when
// their stack tags do not overlap with pf.Stack. Each bridge gets a fresh
// projectfile read so syncers that mutate pf are visible to subsequent bridges.
// Required-fields prompting is skipped (non-interactive batch mode); bridges
// that need missing fields simply error.
func runAllBridges(mode core.Mode, dir string, cmd *cobra.Command) error {
	all := bridge.List()
	if len(all) == 0 {
		return errors.New("no bridges registered")
	}

	pfPath, err := projectfile.DetectPath(dir)
	if err != nil {
		return fmt.Errorf("detect projectfile: %w", err)
	}

	// Read pf once to resolve stacks for filtering.
	pf, err := readMerged(dir)
	if err != nil {
		return err
	}

	var eligible []core.Bridge
	for _, b := range all {
		if bridge.MatchesStack(b, pf.Stack) {
			eligible = append(eligible, b)
		}
	}

	if bridgeDryRun {
		genlog.Plain("DRY RUN — no files will be written")
	}

	genlog.Section(fmt.Sprintf("bridge all (%d/%d files, stacks: %s)",
		len(eligible), len(all), stackSummary(pf.Stack)))

	return pflock.WithLock(pfPath, func() error {
		var errs []string
		for _, b := range eligible {
			if bridgeDryRun {
				genlog.Debug("bridge all", "file", b.Filename())
			}
			if err := runAllBridgeOne(b, mode, dir, pfPath, cmd); err != nil {
				warn.Record("bridge all: failed",
					"file", b.Filename(),
					"error", err.Error())
				errs = append(errs, b.Filename()+": "+err.Error())
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("bridge all: %d/%d failed:\n  %s",
				len(errs), len(eligible), strings.Join(errs, "\n  "))
		}
		return nil
	})
}

func stackSummary(stacks []string) string {
	if len(stacks) == 0 {
		return "(none declared — running all)"
	}
	return strings.Join(stacks, ", ")
}

// runAllBridgeOne runs a single bridge inside the all-batch. Re-reads the
// projectfile from disk so mutations from earlier syncers are visible.
// Required-fields prompting is skipped (batch mode).
func runAllBridgeOne(b core.Bridge, mode core.Mode, dir, pfPath string, cmd *cobra.Command) error {
	pf, err := readMerged(dir)
	if err != nil {
		return err
	}

	opts := core.Options{
		Dir:    dir,
		PFPath: pfPath,
		Mode:   mode,
		Force:  bridgeForce,
		// --check and --preview never write, so both carry DryRun into every
		// path that gates on it (RunSync's persist step, writeOutput's status
		// lines) even though the renderer preview path returns before reaching it.
		DryRun:         bridgeDryRun || bridgeCheck || bridgePreview,
		NoCreate:       bridgeNoCreate,
		Check:          bridgeCheck,
		Preview:        bridgePreview,
		Diff:           !bridgeNoDiff,
		WarnOnly:       !failOnDrift(),
		Offline:        rootflags.Offline(),
		ReuseCanonical: bridgeReuseCanonical,
		Stderr:         cmd.ErrOrStderr(),
	}

	switch impl := b.(type) {
	case core.Syncer:
		return runBridgeSync(impl, pf, opts)
	case core.Renderer:
		if mode == core.ModeRead {
			return fmt.Errorf("bridge %q is write-only (Renderer cannot read external)", b.Filename())
		}
		genlog.Section(b.Filename() + " (" + pfmodel.DisplayName(pf) + ")")
		return core.RunRender(impl, pf, opts)
	default:
		return fmt.Errorf("bridge %q implements neither Syncer nor Renderer", b.Filename())
	}
}

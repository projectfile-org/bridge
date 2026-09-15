// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/warn"
)

// DriftHint is the remedy the summary block states once, however many files
// drifted. Exported because the syncer check in the CLI states the same one.
const DriftHint = "run the matching generate/sync target (or re-run without --check) to regenerate"

// RunRender drives a single derive-from-pf invocation for a Renderer. It
// computes the Output (one or more files), applies the per-file policy
// gate, emits one status line per file, and refuses with a single
// batch-level error when at least one file was refused.
//
// Lifted out of the old cmd/generate.go so REPL/tests can call the render
// path without re-implementing the policy gate.
func RunRender(r Renderer, pf *projectfile.Document, opts Options) error {
	out, err := r.Render(pf, opts)
	if err != nil {
		return fmt.Errorf("%s: %w", r.Filename(), err)
	}
	if len(out.Files) == 0 {
		genlog.Plain("bridge: " + r.Filename() + " (no-op)")
		return nil
	}
	if opts.Preview {
		return previewOutput(out)
	}
	if opts.Check {
		return checkOutput(opts.Dir, out, opts)
	}
	return writeOutput(opts.Dir, out, r.Policy(), opts)
}

// previewOutput prints every rendered file to stdout instead of writing it —
// the write-gate policy and any existing on-disk copy are irrelevant here,
// since nothing is touched. A single-file render prints bare; a multi-file
// render (e.g. LICENSE's LICENSES/<id>.txt fan-out) prints a path header
// before each file so the files stay distinguishable in the stream.
func previewOutput(out Output) error {
	names := make([]string, 0, len(out.Files))
	for k := range out.Files {
		names = append(names, k)
	}
	sort.Strings(names)

	multi := len(names) > 1
	for _, rel := range names {
		if multi {
			fmt.Printf("--- %s ---\n", rel)
		}
		if _, err := os.Stdout.Write(out.Files[rel]); err != nil {
			return fmt.Errorf("preview %s: %w", rel, err)
		}
	}
	return nil
}

// checkOutput compares each rendered file against the copy on disk and reports
// every difference, writing nothing. This is the drift gate: `--force` answers
// "make it match" by overwriting whatever a human put there, which is useless
// as a CI check because it always succeeds and always destroys the evidence.
//
// A MISSING file counts as drift too — a generated artefact someone deleted is
// exactly the state a sync gate exists to catch.
//
// Every content mismatch is reported WITH the diff that caused it (opts.Diff),
// because the two sides are already in hand here and regenerating to find out
// what moved is exactly what a gate must not require.
//
// Whether the verdict is fatal is opts.WarnOnly's call, not this function's.
// Either way the drift enters the warning ledger, so a lenient run still ends
// with the whole list stated where the reader cannot miss it.
func checkOutput(dir string, out Output, opts Options) error {
	names := make([]string, 0, len(out.Files))
	for name := range out.Files {
		names = append(names, name)
	}
	sort.Strings(names)

	var drifted []string
	for _, rel := range names {
		absPath, err := filepath.Abs(filepath.Join(dir, rel))
		if err != nil {
			return fmt.Errorf("resolve %s path: %w", rel, err)
		}
		existing, readErr := os.ReadFile(absPath) // #nosec G304 -- joined from caller-provided dir + registered filename
		switch {
		case readErr != nil && os.IsNotExist(readErr):
			warn.Record("drift: file is missing", "file", rel)
			drifted = append(drifted, rel)
		case readErr != nil:
			return fmt.Errorf("read existing %s: %w", rel, readErr)
		case !bytes.Equal(existing, out.Files[rel]):
			warn.Record("drift: file no longer matches the projectfile", "file", rel)
			if opts.Diff {
				WriteDiff(diffOut(opts), rel, existing, out.Files[rel])
			}
			drifted = append(drifted, rel)
		default:
			genlog.Success(fmt.Sprintf("bridge: %s (in sync)", rel))
		}
	}
	if len(drifted) == 0 {
		return nil
	}
	warn.Hint(DriftHint)
	if opts.WarnOnly {
		return nil
	}
	return fmt.Errorf("%d file(s) drifted from the projectfile: %s (re-run without --check to regenerate)",
		len(drifted), strings.Join(drifted, ", "))
}

// writeOutput applies the policy gate per file, prints one status line per
// file, and tallies refusals into a single batch-level error.
func writeOutput(dir string, out Output, policy Policy, opts Options) error {
	names := make([]string, 0, len(out.Files))
	for k := range out.Files {
		names = append(names, k)
	}
	sort.Strings(names)

	refusals := 0
	for _, rel := range names {
		absPath, err := filepath.Abs(filepath.Join(dir, rel))
		if err != nil {
			return fmt.Errorf("resolve %s path: %w", rel, err)
		}

		existing, readErr := os.ReadFile(absPath) // #nosec G304 -- joined from caller-provided dir + registered filename
		status := "created"
		switch {
		case readErr != nil && os.IsNotExist(readErr):
			// fall through with status=created
		case readErr != nil:
			return fmt.Errorf("read existing %s: %w", rel, readErr)
		case policy.Marker && !HasMarker(existing) && !opts.Force:
			genlog.Warn("refused (hand-edited)", "file", rel, "hint", "re-run with --force to overwrite")
			refusals++
			continue
		default:
			status = "updated"
		}

		if opts.DryRun {
			genlog.Plain(fmt.Sprintf("bridge: %s (%s) [dry-run]", rel, status))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil { // #nosec G301 -- directories for generated artefacts
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(absPath), err)
		}
		if err := os.WriteFile(absPath, out.Files[rel], 0o644); err != nil { // #nosec G306 -- generated metadata, world-readable by intent
			return fmt.Errorf("write %s: %w", rel, err)
		}
		genlog.Success(fmt.Sprintf("bridge: %s (%s)", rel, status))
	}

	if refusals > 0 {
		return fmt.Errorf("%d file(s) refused; re-run with --force to overwrite", refusals)
	}
	return nil
}

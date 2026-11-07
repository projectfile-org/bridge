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
	"projectfile.org/projectfile/bridge/v4/internal/warn"
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
	if len(out.Files) == 0 && len(out.Remove) == 0 {
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
// before each file so the files stay distinguishable in the stream. Remove
// entries print a status line, or their rewrite body when one exists.
func previewOutput(out Output) error {
	names := make([]string, 0, len(out.Files)+len(out.Remove))
	for k := range out.Files {
		names = append(names, k)
	}
	for k := range out.Remove {
		names = append(names, k)
	}
	sort.Strings(names)

	multi := len(names) > 1
	for _, rel := range names {
		if body, ok := out.Files[rel]; ok {
			if multi {
				fmt.Printf("--- %s ---\n", rel)
			}
			if _, err := os.Stdout.Write(body); err != nil {
				return fmt.Errorf("preview %s: %w", rel, err)
			}
			continue
		}
		if body, ok := out.Remove[rel]; ok && body != nil {
			if multi {
				fmt.Printf("--- %s ---\n", rel)
			}
			if _, err := os.Stdout.Write(body); err != nil {
				return fmt.Errorf("preview %s: %w", rel, err)
			}
			continue
		}
		fmt.Printf("bridge: %s (would remove)\n", rel)
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
			if opts.NoCreate {
				continue // --create-all: missing file is expected, not drift
			}
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
	// Remove entries retire: pf's state is the file's absence (nil body) or its empty-state rewrite (body); a surviving managed copy is drift, a detached one never is.
	for _, rel := range sortedKeys(out.Remove) {
		absPath, err := filepath.Abs(filepath.Join(dir, rel))
		if err != nil {
			return fmt.Errorf("resolve %s path: %w", rel, err)
		}
		rendered := out.Remove[rel]
		existing, readErr := os.ReadFile(absPath) // #nosec G304 -- joined from caller-provided dir + registered filename
		switch {
		case readErr != nil && os.IsNotExist(readErr):
			if rendered != nil && !opts.NoCreate {
				warn.Record("drift: file is missing", "file", rel)
				drifted = append(drifted, rel)
				continue
			}
			if rendered != nil {
				continue // --create-all: missing file is expected, not drift
			}
			genlog.Success(fmt.Sprintf("bridge: %s (in sync)", rel))
		case readErr != nil:
			return fmt.Errorf("read existing %s: %w", rel, readErr)
		case !HasMarker(existing):
			genlog.Plain(fmt.Sprintf("bridge: %s (detached — left alone)", rel))
		case rendered == nil:
			warn.Record("drift: file still holds state the projectfile no longer declares", "file", rel)
			if opts.Diff {
				WriteDiff(diffOut(opts), rel, existing, nil)
			}
			drifted = append(drifted, rel)
		case !bytes.Equal(existing, rendered):
			warn.Record("drift: file no longer matches the projectfile", "file", rel)
			if opts.Diff {
				WriteDiff(diffOut(opts), rel, existing, rendered)
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

// sortedKeys orders the map's file names, so every Output consumer prints deterministically.
func sortedKeys(m map[string][]byte) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// writeOutput applies the policy gate per file, prints one status line per
// file, and tallies refusals into a single batch-level error. Remove entries
// are handled after files under the same gate: a nil body deletes a managed
// copy, a body rewrites it to the empty state, a detached copy is left alone.
func writeOutput(dir string, out Output, policy Policy, opts Options) error {
	names := sortedKeys(out.Files)

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
			if opts.NoCreate {
				genlog.Plain(fmt.Sprintf("bridge: %s (skipped — missing, --create-all not set)", rel))
				continue // --create-all not set: expected, not a refusal
			}
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

	// Retirements succeed even where nothing matches them: a missing delete target is already the state pf declares (absent), and a detached copy stays the human's file — neither refuses nor errors.
	for _, rel := range sortedKeys(out.Remove) {
		absPath, err := filepath.Abs(filepath.Join(dir, rel))
		if err != nil {
			return fmt.Errorf("resolve %s path: %w", rel, err)
		}
		body := out.Remove[rel]
		existing, readErr := os.ReadFile(absPath) // #nosec G304 -- joined from caller-provided dir + registered filename
		status := "updated"
		switch {
		case readErr != nil && os.IsNotExist(readErr):
			if body == nil {
				genlog.Plain(fmt.Sprintf("bridge: %s (absent)", rel))
				continue
			}
			if opts.NoCreate {
				genlog.Plain(fmt.Sprintf("bridge: %s (skipped — missing, --create-all not set)", rel))
				continue
			}
			status = "created"
		case readErr != nil:
			return fmt.Errorf("read existing %s: %w", rel, readErr)
		case policy.Marker && !HasMarker(existing) && !opts.Force:
			genlog.Plain(fmt.Sprintf("bridge: %s (detached — left alone)", rel))
			continue
		}
		if body == nil {
			status = "removed"
		}
		if opts.DryRun {
			genlog.Plain(fmt.Sprintf("bridge: %s (%s) [dry-run]", rel, status))
			continue
		}
		if body == nil {
			if err := os.Remove(absPath); err != nil {
				return fmt.Errorf("remove %s: %w", rel, err)
			}
		} else if err := os.WriteFile(absPath, body, 0o644); err != nil { // #nosec G306 -- generated metadata, world-readable by intent
			return fmt.Errorf("write %s: %w", rel, err)
		}
		genlog.Success(fmt.Sprintf("bridge: %s (%s)", rel, status))
	}

	if refusals > 0 {
		return fmt.Errorf("%d file(s) refused; re-run with --force to overwrite", refusals)
	}
	return nil
}

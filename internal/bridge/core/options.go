// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "io"

// Mode picks the direction of a Syncer run. Renamed from the pre-bridge
// vocabulary:
//   - ModeSync (was ModeBidirectional) — newer side authoritative,
//     other side gap-fills.
//   - ModeWrite (was ModeFromPF) — push pf → external, regardless of mtime.
//   - ModeRead  (was ModeToPF)   — push external → pf, regardless of mtime.
//
// Renderers ignore Mode — they have no read direction.
type Mode string

const (
	ModeSync  Mode = "sync"
	ModeWrite Mode = "write"
	ModeRead  Mode = "read"
)

// Options threads CLI flags into every Bridge invocation. The union of
// fields is small enough that one struct is cleaner than two; fields
// unused by a given bridge kind are simply ignored.
type Options struct {
	Dir      string
	PFPath   string
	Mode     Mode
	Force    bool
	DryRun   bool
	NoCreate bool
	Offline  bool
	// Preview prints each rendered file to stdout instead of writing it —
	// for trying out template output without touching disk. Renderer-only:
	// a Syncer has no in-memory rendered bytes to show, so it falls back to
	// its normal --dry-run summary.
	Preview bool
	// ReuseCanonical keeps LICENSES/<id>.txt at the canonical SPDX text
	// (literal placeholders) instead of substituting the resolved copyright
	// holder/year. License-bridge only; ignored by other bridges.
	ReuseCanonical bool
	// Check turns the run into a DRIFT GATE: nothing is written, and a
	// generated file that no longer matches what the projectfile would produce
	// is reported and fails the command. Without it, --force is the only way to
	// regenerate a marked file and it overwrites hand-edits silently — the
	// difference between "this repository is in sync" and "it is now".
	Check bool
	// Diff prints the unified diff behind every drift report Check produces.
	// Opt-in for library callers, on by default from the CLI (--no-diff turns
	// it off) — a gate that says a file drifted without saying how forces the
	// reader to regenerate, which is the one action that erases the evidence.
	Diff bool
	// WarnOnly downgrades a Check verdict from an error to a warning: drift is
	// still reported, diffed and recorded in the warning ledger, but the run
	// exits 0. The library default is the strict gate; the CLI is what chooses
	// leniency, because the case for it is a FLEET one — a bridge whose fix is
	// published but whose image has not been rebuilt yet makes every consumer
	// drift, and a blocked commit is then the thing standing between the user
	// and the fix.
	WarnOnly bool
	Stderr   io.Writer
}

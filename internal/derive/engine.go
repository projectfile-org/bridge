// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package derive turns the primary repository URL and the detected stack
// into structured field values that would otherwise need to be hand-typed
// in projectfile.toml. Two inference passes today:
//
//   - forges/   — repo URL host → tracker URL pattern (GitHub /issues,
//     GitLab /-/issues, Codeberg/Gitea /issues, sourcehut ~/.../tracker).
//   - registries/ — detected stack + identity.name → package-registry landing
//     page (npmjs.com, pypi.org, packagist.org, crates.io). Output
//     lands as [[links]] entries with type = "package-registry".
//
// A third package, ociregistries/, feeds AddVirtual rather than Apply: it
// composes container-image pull references, which are read at render time and
// never written to disk.
//
// The engine is invoked from internal/bridge/core/runsync.go after
// person-conflict emission and before the Write call, so every sync run keeps
// the derived fields in line with the spec's principle of least astonishment.
// The bare `pf-bridge` (no arguments) form runs ONLY the derivation pass
// — useful when a user has just edited the primary repository URL by hand
// and wants the issue tracker to follow.
//
// Output is communicated through []Change so the caller (sync.go) can fold
// each derived field into the existing Result.Changes log and the [org.projectfile.cli]
// bookkeeping survives idempotent re-runs.
package derive

import (
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive/forges"
	"projectfile.org/projectfile/bridge/internal/derive/ociregistries"
	"projectfile.org/projectfile/bridge/internal/derive/registries"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Change records one field path the engine wrote (or would have written).
// FieldPath is dotted-segment notation matching the spec's vocabulary:
// "links[type=bugs]", "links[type=package-registry]". OldValue
// is "" for new fields. Source identifies the inference rule that produced
// the value (e.g. "forge:github", "registry:npm"). Label is an optional
// human-readable string the engine writes into the link's label field
// (e.g. "Issues on GitHub").
type Change struct {
	FieldPath string
	OldValue  string
	NewValue  string
	Source    string
	Label     string
}

// Options threads runtime knobs through Apply. Today: a single toggle the
// caller flips to opt out without touching the in-pf extension (used by the
// "first run, no extension" path in cmd/sync.go).
type Options struct {
	// Force re-derives every field even when the user has removed its path
	// from CLIExtension.Derived (the standard opt-out). Reserved for an
	// eventual `pf-cli derive --force` flag; today nothing sets it.
	Force bool
}

// Apply runs every enabled inference pass against pf and returns the slice
// of Changes the engine intends to write. Pf is mutated in place: callers
// pass the work-document (the clone in dry-run mode, the live doc otherwise)
// and inspect the return to decide whether to log or persist.
//
// The flow:
//
//  1. Load [org.projectfile.cli] (or default toggles + empty derived).
//  2. For each pass (forges, registries) call the pass's Derive and collect
//     every proposed field write.
//  3. Filter out proposals whose FieldPath isn't already in derived AND was
//     set by hand (non-empty pre-existing value). The user's hand-edit wins;
//     we only re-write paths we own.
//  4. Apply the surviving writes to pf — each link gets Derived=true.
//  5. Rebuild the derived list in memory and SetCLIExtension for the derive
//     toggles (the per-link derived flag persists on disk; the list is kept
//     in memory only for ownership tracking).
func Apply(pf *projectfile.Document, opts Options) ([]Change, error) {
	ext, err := pfmodel.GetCLIExtension(pf)
	if err != nil {
		return nil, err
	}
	if ext == nil {
		// Absent extension means defaults: every pass runs, no opt-outs yet.
		ext = &pfmodel.CLIExtension{
			Derive: pfmodel.CLIDeriveToggles{Forges: true, Registries: true},
		}
	}

	owned := stringSet(ext.Derived)

	var proposals []Change
	if ext.Derive.Forges {
		for _, p := range forges.Derive(pf) {
			proposals = append(proposals, Change{FieldPath: p.FieldPath, NewValue: p.NewValue, Source: p.Source, Label: p.Label})
		}
	}
	if ext.Derive.Registries {
		for _, p := range registries.Derive(pf) {
			proposals = append(proposals, Change{FieldPath: p.FieldPath, NewValue: p.NewValue, Source: p.Source, Label: p.Label})
		}
	}

	applied := make([]Change, 0, len(proposals))
	for _, p := range proposals {
		c := p
		c.OldValue = readField(pf, c.FieldPath)
		// Skip when the user has a hand-written value and we haven't claimed
		// the path yet. Force overrides this guard.
		if c.OldValue != "" && !owned[c.FieldPath] && !opts.Force {
			continue
		}
		// Skip when value would not change.
		if c.OldValue == c.NewValue {
			continue
		}
		if err := writeField(pf, c.FieldPath, c.NewValue, c.Label); err != nil {
			return nil, err
		}
		applied = append(applied, c)
	}

	// Rebuild in-memory derived list — paths we just wrote plus any we
	// previously owned and still have a value for. The list is NOT written
	// to the output document (derived per-link replaces it), but keeping it
	// in memory ensures correct ownership tracking within the same process.
	newDerived := make([]string, 0, len(applied))
	seen := map[string]bool{}
	for _, c := range applied {
		if seen[c.FieldPath] {
			continue
		}
		newDerived = append(newDerived, c.FieldPath)
		seen[c.FieldPath] = true
	}
	for _, path := range ext.Derived {
		if seen[path] {
			continue
		}
		// Carry forward only when the field still has a value the engine
		// could have produced — otherwise the user cleared it and we honour
		// the implied opt-out.
		if readField(pf, path) != "" {
			newDerived = append(newDerived, path)
			seen[path] = true
		}
	}
	ext.Derived = newDerived
	pfmodel.SetCLIExtension(pf, ext)

	return applied, nil
}

// remotesKey is where AddVirtual parks the derived forge coordinates, nested
// under the existing org.projectfile.forge namespace so a forge slug can never
// collide with the hand-written `kinds` / `hosts` maps beside it.
const remotesKey = "remotes"

// AddVirtual computes the derived fields a document carries at INTERPRETATION
// time and never on disk, mutating pf in place. Unlike Apply — which proposes
// changes the caller persists — nothing here is ever written back: every value
// is a pure function of fields already in the document, so persisting it would
// duplicate data and invite drift the moment a mirror URL changes.
//
// Two passes today, each independent of the other so a project missing the
// inputs of one still gets the other:
//
//   - org.projectfile.forge.remotes — the host/owner/repo/kind coordinates of
//     every source-code mirror.
//   - org.projectfile.registries.<slug>.ref — the composed pull reference of
//     every container registry the project publishes to.
//
// Callers run it right after reading the merged document, so both the templates
// (via `pf`) and the ${…} interpolator (via the address grammar) see the derived
// values as ordinary document data — which is what lets a badge, and a pull
// command, be a line of YAML.
//
// Idempotent: re-running overwrites the same subtrees with the same values.
func AddVirtual(pf *projectfile.Document) {
	if pf == nil {
		return
	}
	addForgeRemotes(pf)
	addRegistryRefs(pf)
}

// addForgeRemotes parks the mirror coordinates under the forge namespace.
func addForgeRemotes(pf *projectfile.Document) {
	ext, err := pfmodel.GetForgeExtension(pf)
	if err != nil {
		genlog.Warn("derive: forge namespace unreadable — remotes not derived", "error", err.Error())
		return
	}
	var kinds map[string]string
	if ext != nil {
		kinds = ext.Kinds
	}
	remotes := forges.Remotes(pf, kinds)
	if len(remotes) == 0 {
		return
	}
	forge, _ := projectfile.LookupExtension(pf, pfmodel.ForgeExtensionNS)
	merged, ok := forge.(map[string]any)
	if !ok {
		merged = map[string]any{}
	}
	merged[remotesKey] = remotes
	projectfile.SetExtension(pf, pfmodel.ForgeExtensionNS, merged)
}

// addRegistryRefs replaces the registries namespace with the composed view: the
// same entries, each carrying a concrete `ref` instead of a template. It REPLACES
// rather than merges because every key it writes it also computed — a template
// left beside its own composition is a second spelling of one reference, and the
// two would drift the moment a registry moved.
func addRegistryRefs(pf *projectfile.Document) {
	refs := ociregistries.Refs(pf)
	if len(refs) == 0 {
		return
	}
	projectfile.SetExtension(pf, pfmodel.RegistriesExtensionNS, refs)
}

// stringSet builds a quick lookup map from a string slice. Used for
// owned-path membership testing inside the apply loop.
func stringSet(in []string) map[string]bool {
	out := make(map[string]bool, len(in))
	for _, s := range in {
		out[s] = true
	}
	return out
}

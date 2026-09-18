// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package derive computes the read-time fields of a merged document: forge remotes and composed sink refs.
package derive

import (
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive/forges"
	"projectfile.org/projectfile/bridge/internal/derive/ocisinks"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

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
//   - org.projectfile.sinks.<name>.ref — the composed pull reference of every
//     destination the project publishes its images to.
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
	addSinkRefs(pf)
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

// addSinkRefs replaces the sinks namespace with the composed view: the same
// entries, each carrying a concrete `ref` instead of a template. It REPLACES
// rather than merges because every key it writes it also computed — a template
// left beside its own composition is a second spelling of one reference, and the
// two would drift the moment a sink moved.
func addSinkRefs(pf *projectfile.Document) {
	refs := ocisinks.Refs(pf)
	if len(refs) == 0 {
		return
	}
	projectfile.SetExtension(pf, pfmodel.SinksExtensionNS, refs)
}

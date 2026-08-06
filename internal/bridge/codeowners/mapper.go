// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package codeowners

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// buildMappers wires the single bidirectional FieldMapper for the
// `org.projectfile.codeowners.entries` ↔ CODEOWNERS file mapping.
//
// Identity / equality: two states are "equal" when the ordered (pattern,
// owners) tuples match exactly. Reordering changes semantics — last match
// wins per path — so canonical ordering is not a thing here.
func buildMappers(extDoc *Document, pf *projectfile.Document) core.MapperList {
	if extDoc == nil {
		extDoc = &Document{}
	}
	extDoc.ReuseHeader = core.REUSEHeader(pf, core.StyleHash)
	return core.MapperList{
		{
			ExtKey: "entries",
			PFKey:  "[" + pfmodel.CodeOwnersExtensionNS + "].entries",
			ToPF: func(force bool) string {
				// Read entries from the file's ordered lines.
				incoming := docEntries(extDoc)
				existing := pfEntries(pf)
				if !force && len(existing) > 0 {
					return "" // gap-fill skip — pf already populated
				}
				if entriesEqual(incoming, existing) {
					return ""
				}
				setPFEntries(pf, incoming)
				return summariseEntries(incoming)
			},
			FromPF: func(force bool) string {
				incoming := pfEntries(pf)
				// When no explicit codeowners extension exists, derive a
				// default "all code belongs to author" rule from the
				// projectfile's people and organisations.
				if len(incoming) == 0 {
					incoming = defaultOwnerEntries(pf)
				}
				existing := docEntries(extDoc)
				if !force && len(existing) > 0 {
					return "" // gap-fill skip — file already populated
				}
				if entriesEqual(incoming, existing) {
					return ""
				}
				// Replace the doc's lines wholesale: pf has no slot for
				// comments/blanks, so the to-pf direction has already lost
				// them — re-emitting from pf is purely entries.
				extDoc.Lines = extDoc.Lines[:0]
				for _, e := range incoming {
					extDoc.Lines = append(extDoc.Lines, Line{
						Kind: KindEntry, Pattern: e.Pattern, Owners: append([]string(nil), e.Owners...),
					})
				}
				return summariseEntries(incoming)
			},
		},
	}
}

func ownerTokenPerson(p projectfile.Person) string {
	return pfmodel.ForgePersonHandle(p)
}

func ownerTokenOrg(o projectfile.Organization) string {
	return pfmodel.ForgeOrgHandle(o)
}

func docEntries(doc *Document) []pfmodel.CodeOwnersEntry {
	if doc == nil {
		return nil
	}
	out := make([]pfmodel.CodeOwnersEntry, 0, len(doc.Lines))
	for _, l := range doc.Entries() {
		out = append(out, pfmodel.CodeOwnersEntry{
			Pattern: l.Pattern,
			Owners:  append([]string(nil), l.Owners...),
		})
	}
	return out
}

func pfEntries(pf *projectfile.Document) []pfmodel.CodeOwnersEntry {
	ext, _ := pfmodel.GetCodeOwnersExtension(pf)
	if ext == nil {
		return nil
	}
	out := make([]pfmodel.CodeOwnersEntry, 0, len(ext.Entries))
	for _, e := range ext.Entries {
		out = append(out, pfmodel.CodeOwnersEntry{
			Pattern: e.Pattern,
			Owners:  append([]string(nil), e.Owners...),
		})
	}
	return out
}

// setPFEntries paints the entries list back onto pf.Extensions under the
// codeowners namespace. Uses projectfile.SetExtension so any pre-existing
// nested-table form (the parse result of `[org.projectfile.codeowners]`) is
// pruned first — otherwise the serialiser would emit both the original
// dotted header and the new flat key, doubling the section.
func setPFEntries(pf *projectfile.Document, entries []pfmodel.CodeOwnersEntry) {
	rawEntries := make([]any, 0, len(entries))
	for _, e := range entries {
		owners := make([]any, len(e.Owners))
		for i, o := range e.Owners {
			owners[i] = o
		}
		rawEntries = append(rawEntries, map[string]any{
			"pattern": e.Pattern,
			"owners":  owners,
		})
	}
	projectfile.SetExtension(pf, pfmodel.CodeOwnersExtensionNS, map[string]any{"entries": rawEntries})
}

func entriesEqual(a, b []pfmodel.CodeOwnersEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Pattern != b[i].Pattern {
			return false
		}
		if len(a[i].Owners) != len(b[i].Owners) {
			return false
		}
		for j := range a[i].Owners {
			if a[i].Owners[j] != b[i].Owners[j] {
				return false
			}
		}
	}
	return true
}

// defaultOwnerEntries builds a single catch-all rule `* <owners>` from the
// projectfile's people and organisations with author or maintainer roles.
// Falls back to any person/org with an alias or email when no role match is
// found. Returns nil when no owner tokens can be resolved at all.
func defaultOwnerEntries(pf *projectfile.Document) []pfmodel.CodeOwnersEntry {
	if pf == nil {
		return nil
	}
	var owners []string
	seen := make(map[string]bool)

	addOwner := func(token string) {
		if token == "" || seen[token] {
			return
		}
		seen[token] = true
		owners = append(owners, token)
	}

	// First pass: people with author or maintainer role
	for _, p := range pf.People {
		if hasRole(p.Roles, "author") || hasRole(p.Roles, "maintainer") {
			addOwner(ownerTokenPerson(p))
		}
	}
	// Organisations with author or maintainer role
	for _, o := range pf.Organizations {
		if hasRole(o.Roles, "author") || hasRole(o.Roles, "maintainer") {
			addOwner(ownerTokenOrg(o))
		}
	}
	// Fallback: any person with a forge handle
	if len(owners) == 0 {
		for _, p := range pf.People {
			addOwner(ownerTokenPerson(p))
		}
		for _, o := range pf.Organizations {
			addOwner(ownerTokenOrg(o))
		}
	}
	if len(owners) == 0 {
		return nil
	}
	return []pfmodel.CodeOwnersEntry{
		{Pattern: "*", Owners: owners},
	}
}

func hasRole(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}

func summariseEntries(entries []pfmodel.CodeOwnersEntry) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf("%s → %s", e.Pattern, strings.Join(e.Owners, " ")))
	}
	return core.Trunc(fmt.Sprintf("%d entr(y/ies): %s", len(entries), strings.Join(parts, "; ")))
}

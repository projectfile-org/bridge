// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// GetCLIExtension parses `org.projectfile.cli`. The derive toggles default
// to true: when the extension is absent the engine runs every pass, and when
// the extension is present but a toggle is omitted that pass still runs. The
// only way to disable a pass is to set the toggle to false explicitly.
func GetCLIExtension(doc *projectfile.Document) (*CLIExtension, error) {
	m, present, err := lookupNS(doc, CLIExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		// Absent: every pass runs by default. Engine callers treat a nil
		// extension as "both toggles true, derived is empty".
		return nil, nil
	}
	ext := &CLIExtension{
		// Read from extension map for backward compat with old format.
		Derived: strListVal(m, "derived"),
		Derive: CLIDeriveToggles{
			// Explicit-false is the only "off" — implicit absence stays on.
			Forges:     boolValDefaultTrue(m, "derive", "forges"),
			Registries: boolValDefaultTrue(m, "derive", "registries"),
			Containers: boolValDefaultTrue(m, "derive", "containers"),
		},
	}
	// Also collect paths from links with Derived=true (new per-link format).
	for _, l := range doc.Links {
		if l.Derived {
			ext.Derived = append(ext.Derived, "links[type="+l.Type+",url="+l.URL+"]")
		}
	}
	return ext, nil
}

// SetCLIExtension serialises ext into doc.Extensions under the cli namespace.
// Marshal cost is two map allocations; the engine only calls this once per
// run after Apply() collects every Change.
func SetCLIExtension(doc *projectfile.Document, ext *CLIExtension) {
	if doc == nil || ext == nil {
		return
	}
	m := map[string]any{}
	derive := map[string]any{}
	if !ext.Derive.Forges {
		derive["forges"] = false
	}
	if !ext.Derive.Registries {
		derive["registries"] = false
	}
	if !ext.Derive.Containers {
		derive["containers"] = false
	}
	if len(derive) > 0 {
		m["derive"] = derive
	}
	if len(m) == 0 {
		// Default toggles carry no information: writing `cli: {}` would park an
		// empty namespace in every synced projectfile and make the merged doc
		// differ from its pre-sync snapshot on EVERY run, forcing a reconcile
		// pass that has nothing to reconcile.
		return
	}
	projectfile.SetExtension(doc, CLIExtensionNS, m)
}

// boolValDefaultTrue reads m[outer][inner] as a bool, defaulting to true when
// the path is missing or the leaf is non-bool. The "convention over
// configuration" implication of CLIDeriveToggles: a present extension without
// the toggle still leaves the inference pass on.
func boolValDefaultTrue(m map[string]any, outer, inner string) bool {
	sub, ok := m[outer].(map[string]any)
	if !ok {
		return true
	}
	v, ok := sub[inner]
	if !ok {
		return true
	}
	b, ok := v.(bool)
	if !ok {
		return true
	}
	return b
}

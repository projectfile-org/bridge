// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import "kiota.ch/projectfile/core/v2/pkg/projectfile"

// GetIgnoresExtension parses `org.projectfile.ignores`. Returns (nil, nil)
// when the namespace is absent, which the generator treats as "use defaults".
func GetIgnoresExtension(doc *projectfile.Document) (*IgnoresExtension, error) {
	m, present, err := lookupNS(doc, IgnoresExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	ext := &IgnoresExtension{
		Generate: strListVal(m, "generate"),
		Extra:    strListVal(m, "extra"),
	}
	if v, ok := m["git"]; ok {
		ext.Git = parseIgnoreTargetOverride(v)
	}
	if v, ok := m["docker"]; ok {
		ext.Docker = parseIgnoreTargetOverride(v)
	}
	if v, ok := m["npm"]; ok {
		ext.Npm = parseIgnoreTargetOverride(v)
	}
	if v, ok := m["claude"]; ok {
		ext.Claude = parseIgnoreTargetOverride(v)
	}
	if v, ok := m["container"]; ok {
		ext.Container = parseIgnoreTargetOverride(v)
	}
	if v, ok := m["yamllint"]; ok {
		ext.Yamllint = parseIgnoreTargetOverride(v)
	}
	return ext, nil
}

// GetEditorsExtension parses `org.projectfile.editors`. Returns (nil, nil)
// when absent, which means "auto-detect from filesystem".
func GetEditorsExtension(doc *projectfile.Document) (*EditorsExtension, error) {
	m, present, err := lookupNS(doc, EditorsExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	return &EditorsExtension{
		Use: strListVal(m, "use"),
	}, nil
}

// GetVulnerabilitiesExtension parses `org.projectfile.vulnerabilities` — the
// tool-agnostic source of truth for suppressed vulnerability IDs that fan out
// to every scanner bridge (.trivyignore, .grype.yaml, osv-scanner.toml).
// Returns (nil, nil) when absent, which the bridges treat as "nothing to
// suppress".
func GetVulnerabilitiesExtension(doc *projectfile.Document) (*VulnerabilitiesExtension, error) {
	m, present, err := lookupNS(doc, VulnerabilitiesExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	ext := &VulnerabilitiesExtension{
		Generate: strListVal(m, "generate"),
	}
	if list, ok := m["suppress"].([]any); ok {
		ext.Suppress = parseSuppressList(list)
	}
	return ext, nil
}

// parseSuppressList accepts both the structured form ([{id, reason}]) and the
// shorthand form (a bare list of ID strings) so users can write the common
// case compactly while keeping reason provenance available.
func parseSuppressList(list []any) []VulnerabilitySuppress {
	out := make([]VulnerabilitySuppress, 0, len(list))
	for _, item := range list {
		// Shorthand: a bare string is just the ID.
		if s, ok := item.(string); ok {
			out = append(out, VulnerabilitySuppress{ID: s})
			continue
		}
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		s := VulnerabilitySuppress{
			ID:     strVal(m, "id"),
			Reason: strVal(m, "reason"),
		}
		if s.ID != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseIgnoreTargetOverride(v any) *IgnoreTargetOverride {
	// Shorthand: a bare list is treated as include-only.
	if list, ok := v.([]any); ok {
		return &IgnoreTargetOverride{Include: toStrSlice(list)}
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return &IgnoreTargetOverride{
		Include: strListVal(m, "include"),
		Exclude: strListVal(m, "exclude"),
	}
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pyproject

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"

	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
)

// Write serialises doc to pyproject.toml under dir.
//
// Strategy: the Rest canvas owns top-level key ordering ([project],
// [build-system], [tool.*], [dependency-groups], ...). We marshal the typed
// Project struct, decode it back into a map, then merge those keys ONTO the
// existing `project` sub-table inside Rest. Sub-keys we know about overwrite;
// sub-keys we don't know about (some future PEP-X field) survive intact.
//
// Within-table key ordering inside [project] is alphabetical — a documented
// go-toml/v2 limitation, also called out in internal/rawdoc/toml.go.
func Write(dir string, doc *Document) error {
	path := FullPath(dir)

	canvas := doc.Rest
	if canvas == nil {
		canvas = rawdoc.NewOrderedTOML()
	}

	existing := map[string]any{}
	if v, ok := canvas.Get("project"); ok {
		if m, ok := v.(map[string]any); ok {
			existing = m
		}
	}

	typed, err := typedProjectAsMap(doc.Project)
	if err != nil {
		return fmt.Errorf("marshal pyproject [project] table: %w", err)
	}

	// Paint known keys onto the existing sub-table — typed wins where set,
	// existing keeps any field omitempty dropped. Effect: unknown sub-keys
	// survive; known sub-keys reflect the typed view.
	for k, v := range typed {
		existing[k] = v
	}

	// Remove any known keys that are now empty in the typed view but still
	// present in `existing` from a prior read — otherwise clearing a field
	// in projectfile (via --mode from-pf) would leave a stale value behind.
	// This only deletes keys this package knows about; unknown keys are left
	// untouched.
	for _, k := range knownProjectKeys {
		if _, present := typed[k]; present {
			continue
		}
		delete(existing, k)
	}

	if len(existing) > 0 {
		canvas.Set("project", existing)
	} else {
		canvas.Delete("project")
	}

	data, err := canvas.Marshal()
	if err != nil {
		return fmt.Errorf("marshal pyproject.toml: %w", err)
	}
	return os.WriteFile(path, data, 0o644) // #nosec G306 -- metadata file, not a secret
}

// typedProjectAsMap marshals the typed Project struct via TOML and decodes
// it back into a map[string]any. omitempty fields don't appear in the result,
// so the caller's "paint onto existing" loop won't blank them out.
func typedProjectAsMap(p Project) (map[string]any, error) {
	raw, err := toml.Marshal(p)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	if err := toml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// knownProjectKeys is the full set of [project] sub-keys this package
// models. Listed explicitly so Write can drop stale entries on the canvas
// without touching unknown (future-PEP) keys.
var knownProjectKeys = []string{
	"name",
	"version",
	"description",
	"readme",
	"requires-python",
	"license",
	"license-files",
	"authors",
	"maintainers",
	pyprojectKeywords,
	"classifiers",
	"urls",
	"scripts",
	"gui-scripts",
	"entry-points",
	"dependencies",
	"optional-dependencies",
	"dynamic",
}

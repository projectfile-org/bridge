// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package pyproject models the PEP 621 [project] table inside pyproject.toml.
// The whole pyproject.toml file is *not* owned by this package — sibling
// tables ([build-system], [tool.*], [dependency-groups]) are preserved
// verbatim through the Rest canvas.
package pyproject

import "kiota.ch/projectfile/core/v2/pkg/rawdoc"

var Filename = "pyproject.toml"

// Document is the typed view of pyproject.toml that the sync driver mutates.
// `Project` is the PEP 621 [project] table — the only sub-tree this package
// has opinions about. `Rest` is the order-preserving raw view of the entire
// file (top-level keys: project, build-system, tool, dependency-groups, ...);
// Write paints the typed `project` onto Rest, so foreign tables round-trip
// untouched.
type Document struct {
	Project Project             `toml:"project,omitempty"`
	Rest    *rawdoc.OrderedTOML `toml:"-"`
}

// Project mirrors the PEP 621 [project] table. Field tags are spelled exactly
// as PEP 621 specifies (kebab-case for multi-word keys) — go-toml/v2 will
// emit those tags verbatim. `omitempty` everywhere so absent fields don't
// appear in the rendered TOML.
//
// The two heterogeneous fields (`readme`, `license`) keep `any` because the
// spec allows multiple shapes — string, {file: ...}, {text: ...}.
type Project struct {
	Name                 string                       `toml:"name,omitempty"`
	Version              string                       `toml:"version,omitempty"`
	Description          string                       `toml:"description,omitempty"`
	ReadMe               any                          `toml:"readme,omitempty"`
	RequiresPython       string                       `toml:"requires-python,omitempty"`
	License              any                          `toml:"license,omitempty"`
	LicenseFiles         []string                     `toml:"license-files,omitempty"`
	Authors              []Person                     `toml:"authors,omitempty"`
	Maintainers          []Person                     `toml:"maintainers,omitempty"`
	Keywords             []string                     `toml:"keywords,omitempty"`
	Classifiers          []string                     `toml:"classifiers,omitempty"`
	URLs                 map[string]string            `toml:"urls,omitempty"`
	Scripts              map[string]string            `toml:"scripts,omitempty"`
	GUIScripts           map[string]string            `toml:"gui-scripts,omitempty"`
	EntryPoints          map[string]map[string]string `toml:"entry-points,omitempty"`
	Dependencies         []string                     `toml:"dependencies,omitempty"`
	OptionalDependencies map[string][]string          `toml:"optional-dependencies,omitempty"`
	Dynamic              []string                     `toml:"dynamic,omitempty"`
}

// Person is the PEP 621 author/maintainer entry. Unlike npm's Person there is
// no `url` field — the spec gives you name and email only.
type Person struct {
	Name  string `toml:"name,omitempty"`
	Email string `toml:"email,omitempty"`
}

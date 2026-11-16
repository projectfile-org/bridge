// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package cargo models the package metadata of a Rust Cargo.toml manifest.
package cargo

// Filename is the manifest name Cargo reads.
const Filename = "Cargo.toml"

// Manifest keys this bridge owns.
const (
	keyName          = "name"
	keyVersion       = "version"
	keyDescription   = "description"
	keyAuthors       = "authors"
	keyLicense       = "license"
	keyHomepage      = "homepage"
	keyRepository    = "repository"
	keyDocumentation = "documentation"
	keyKeywords      = "keywords"
	keyRustVersion   = "rust-version"
)

// Document is the typed view of the package fields, read from the table that defines each one.
type Document struct {
	Name          string
	Version       string
	Description   string
	Authors       []string
	License       string
	Homepage      string
	Repository    string
	Documentation string
	Keywords      []string
	RustVersion   string
	// Source is the file as read; Write edits it line by line.
	Source []byte
	// ReuseHeader is gap-filled at the top when the file carries no SPDX tags.
	ReuseHeader string
	// Foreign holds the keys a workspace in another manifest defines for this package.
	Foreign map[string]bool
}

// Owns reports whether this manifest defines key, rather than a parent workspace manifest.
func (d *Document) Owns(key string) bool {
	return !d.Foreign[key]
}

// Clone returns a deep copy; Source is shared because it is never mutated.
func (d *Document) Clone() *Document {
	if d == nil {
		return nil
	}
	cp := *d
	cp.Authors = cloneStrings(d.Authors)
	cp.Keywords = cloneStrings(d.Keywords)
	if d.Foreign != nil {
		cp.Foreign = make(map[string]bool, len(d.Foreign))
		for k, v := range d.Foreign {
			cp.Foreign[k] = v
		}
	}
	return &cp
}

// cloneStrings copies a string slice preserving nil-ness.
func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append(make([]string, 0, len(in)), in...)
}

// stringField returns the scalar field stored under key, nil for a list key.
func (d *Document) stringField(key string) *string {
	switch key {
	case keyName:
		return &d.Name
	case keyVersion:
		return &d.Version
	case keyDescription:
		return &d.Description
	case keyLicense:
		return &d.License
	case keyHomepage:
		return &d.Homepage
	case keyRepository:
		return &d.Repository
	case keyDocumentation:
		return &d.Documentation
	case keyRustVersion:
		return &d.RustVersion
	}
	return nil
}

// listField returns the string-list field stored under key, nil for a scalar key.
func (d *Document) listField(key string) *[]string {
	switch key {
	case keyAuthors:
		return &d.Authors
	case keyKeywords:
		return &d.Keywords
	}
	return nil
}

// ownedKeys lists every manifest key in Cargo's documented order.
var ownedKeys = []string{keyName, keyVersion, keyAuthors, keyRustVersion, keyDescription, keyDocumentation, keyHomepage, keyRepository, keyLicense, keyKeywords}

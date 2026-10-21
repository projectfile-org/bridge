// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	fxCFFName         = "widget"
	fxCFFTitle        = "Widget Toolkit"
	fxCFFAbstract     = "A toolkit for widgets"
	fxCFFVersion      = "3.1.0"
	fxSoftwareKind    = "SoftwareSourceCode"
	fxDatasetKind     = "Dataset"
	fxOrcidBare       = "0000-0002-1825-0097"
	fxGeneratedStack  = "crystal"
	fxLicenseMIT      = "MIT"
	fxLicenseURL      = "https://opensource.org/licenses/MIT"
	fxRepoURL         = "https://github.com/acme/widget.git"
	fxRepoDir         = "packages/widget"
	fxHomeURL         = "https://example.test"
	fxBugsURL         = "https://example.test/issues"
	fxDocsURL         = "https://example.test/docs"
	fxEmailA          = "a@example.test"
	fxEmailB          = "b@example.test"
	fxEmailNpm        = "npm@example.test"
	fxEmailComposer   = "composer@example.test"
	fxGivenAda        = "Ada"
	fxFamilyLovelace  = "Lovelace"
	fxGivenGrace      = "Grace"
	fxFamilyHopper    = "Hopper"
	fxEntityName      = "Acme Org"
	fxEntitySingle    = "Acme"
	fxAffiliation     = "Acme Institute"
	fxOrcidUser       = "0000-0001-5102-6448"
	fxUserURL         = "https://user.example.test"
	fxHomepageCFF     = "https://cff.example.test"
	fxHomepageCompose = "https://composer.example.test"
	fxHomepageShard   = "https://shard.example.test"
	fxNpmScoped       = "@acme/widget"
	fxComposerScoped  = "acme/widget"
	fxNpmDescription  = "A test package"
	fxComposerDesc    = "A composer package"
	fxRoleAuthor      = "author"
	fxRoleMaintain    = "maintainer"
	fxRoleContrib     = "contributor"
	fxKeywordLower    = "test"
	fxKeywordUpper    = "Widget"
	fxStackNode       = "node"
	fxStackYarn       = "yarn"
	fxNameCFF         = "CITATION.cff"
	fxNameNPM         = "package.json"
)

// writeFixture drops a manifest into a fresh dir, so every Extract test sees one real on-disk shape.
func writeFixture(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	writeManifest(t, dir, name, body)
	return dir
}

// writeManifest adds one more manifest to an existing directory.
func writeManifest(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

const (
	cffFixture = `cff-version: 1.2.0
message: "If you use this software, please cite it as below."
type: software
title: Widget Toolkit
abstract: A toolkit for widgets
version: 3.1.0
license: MIT
repository-code: "https://github.com/acme/widget.git"
url: https://cff.example.test
keywords:
  - test
authors:
  - family-names: Lovelace
    given-names: Ada
    email: a@example.test
    orcid: "https://orcid.org/0000-0002-1825-0097"
    website: "https://ada.example.test"
maintainers:
  - family-names: Hopper
    given-names: Grace
    email: b@example.test
`

	npmFixture = `{
  "name": "@acme/widget",
  "version": "1.0.0",
  "description": "A test package",
  "license": "MIT",
  "homepage": "https://example.test",
  "bugs": {"url": "https://example.test/issues"},
  "repository": {"url": "https://github.com/acme/widget.git", "type": "git", "directory": "packages/widget"},
  "keywords": ["test", "Widget"],
  "author": "Ada Lovelace <a@example.test> (https://ada.example.test)",
  "contributors": [{"name": "Grace Hopper", "email": "b@example.test"}],
  "maintainers": [{"name": "Acme Org", "email": "npm@example.test"}],
  "engines": {"node": ">=20", "yarn": ">=4"}
}
`

	composerFixture = `{
  "name": "acme/widget",
  "description": "A composer package",
  "version": "2.0.0",
  "license": "MIT",
  "homepage": "https://composer.example.test",
  "keywords": ["test", "Widget"],
  "authors": [
    {"name": "Ada Lovelace", "email": "a@example.test", "homepage": "https://ada.example.test"},
    {"name": "Acme Org", "email": "composer@example.test"}
  ],
  "support": {"source": "https://github.com/acme/widget", "issues": "https://example.test/issues", "docs": "https://example.test/docs"},
  "require": {"php": ">=8.3"}
}
`

	shardFixture = `name: widget
version: 0.1.0
description: A shard manifest
license: MIT
homepage: https://shard.example.test
repository: https://github.com/acme/widget
documentation: https://example.test/docs
authors:
  - Ada Lovelace <a@example.test>
  - Acme Org <composer@example.test>
  - "<>"
crystal: 1.19.0
`
)

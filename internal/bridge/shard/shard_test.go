// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package shard_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/shard"
)

// fixtureYAML carries every synced field plus preserve-only keys (dependencies,
// targets, executables, scripts, libraries) to prove the canvas keeps them.
const fixtureYAML = `---
name: demo-shard
version: 1.2.3
description: A test shard
authors:
  - Alice Smith <alice@example.com>
crystal: ">= 1.13.0, < 2.0.0"
license: MIT
homepage: https://example.com
repository: https://github.com/acme/demo-shard
documentation: https://docs.example.com/demo-shard
dependencies:
  openssl:
    github: datanoise/openssl.cr
    branch: master
development_dependencies:
  minitest:
    github: ysbaddaden/minitest.cr
    version: "~> 0.1.0"
targets:
  demo:
    main: src/demo.cr
executables:
  - demo
scripts:
  postinstall: make ext
libraries:
  libgit2: ~> 0.24
`

func writeShardYML(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

// preserveKeys are the build-only keys with no projectfile slot in this suite.
var preserveKeys = []string{"dependencies", "targets", "executables", "scripts", "libraries"}

func TestReadTypedFields(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", fixtureYAML)
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "demo-shard", doc.Name)
	assert.Equal(t, "1.2.3", doc.Version)
	assert.Equal(t, "A test shard", doc.Description)
	assert.Equal(t, []string{"Alice Smith <alice@example.com>"}, doc.Authors)
	assert.Equal(t, ">= 1.13.0, < 2.0.0", doc.Crystal)
	assert.Equal(t, "MIT", doc.License)
	assert.Equal(t, "https://example.com", doc.Homepage)
	assert.Equal(t, "https://github.com/acme/demo-shard", doc.Repository)
	assert.Equal(t, "https://docs.example.com/demo-shard", doc.Documentation)
}

func TestReadRestCanvasPresent(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", fixtureYAML)
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, doc.Rest, "Rest canvas must be populated on Read")
	for _, key := range preserveKeys {
		assert.True(t, doc.Rest.Has(key), "preserve-only key %q must be in Rest", key)
	}
}

func TestReadAliasSpelling(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yaml", "---\nname: alias-shard\nversion: 0.1.0\n")
	assert.True(t, (shard.Bridge{}).Exists(dir), "Exists must see the alias spelling")
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "alias-shard", doc.Name)
}

func TestReadMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := shard.Read(dir)
	assert.Error(t, err)
}

func TestWriteRoundTripPreservesUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", fixtureYAML)
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	doc.Version = "2.0.0"
	require.NoError(t, shard.Write(dir, doc))
	got, err := shard.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", got.Version)
	assert.Equal(t, "demo-shard", got.Name, "unmodified field must be unchanged")
	for _, key := range preserveKeys {
		assert.True(t, got.Rest.Has(key), "%q must survive round-trip", key)
	}
}

func TestWriteNewDocNoRest(t *testing.T) {
	dir := t.TempDir()
	doc := &shard.Document{Name: "fresh-shard", Version: "0.1.0", License: "Apache-2.0"}
	require.NoError(t, shard.Write(dir, doc))
	got, err := shard.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "fresh-shard", got.Name)
	assert.Equal(t, "0.1.0", got.Version)
}

func TestWriteOutputIsValidYAML(t *testing.T) {
	dir := t.TempDir()
	doc := &shard.Document{Name: "check-yaml", Version: "1.0.0"}
	require.NoError(t, shard.Write(dir, doc))
	data, err := os.ReadFile(filepath.Join(dir, "shard.yml"))
	require.NoError(t, err)
	var raw map[string]any
	assert.NoError(t, yaml.Unmarshal(data, &raw), "output must be valid YAML")
}

func TestFullSyncFromPF(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", "---\nname: old-name\nversion: 0.0.1\n")
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{}
	pf.Identity.Name = "new-name"
	pf.Identity.Version = "9.9.9"
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.FromPF != nil {
			m.FromPF(true)
		}
	}
	assert.Equal(t, "new-name", doc.Name)
	assert.Equal(t, "9.9.9", doc.Version)
}

func TestFullSyncFromPFIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", fixtureYAML)
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{}
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.ToPF != nil {
			m.ToPF(true)
		}
	}
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.FromPF == nil {
			continue
		}
		assert.Emptyf(t, m.FromPF(true), "%s reported a change on an already-synced document", m.ExtKey)
	}
}

func TestSyncNoForceSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", fixtureYAML)
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{}
	pf.Identity.Name = "new-name"
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.FromPF != nil {
			m.FromPF(false)
		}
	}
	assert.Equal(t, "demo-shard", doc.Name, "force=false must not overwrite existing ext field")
}

func TestAuthorsMergeIntoPeople(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", "---\nname: s\nversion: 1.0.0\nauthors:\n  - Alice Smith <alice@example.com>\n  - Bob\n")
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{}
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.ToPF != nil {
			m.ToPF(true)
		}
	}
	require.Len(t, pf.People, 2)
	assert.Equal(t, "Alice", pf.People[0].GivenNames)
	assert.Equal(t, "Smith", pf.People[0].FamilyNames)
	assert.Equal(t, "alice@example.com", pf.People[0].Email)
	assert.Equal(t, "Bob", pf.People[1].DisplayName)
}

func TestAuthorsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", "---\nname: s\nversion: 1.0.0\n")
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{}
	pf.People = []projectfile.Person{{GivenNames: "Alice", FamilyNames: "Smith", Email: "alice@example.com", Roles: []string{"author"}}}
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.FromPF != nil {
			m.FromPF(true)
		}
	}
	assert.Equal(t, []string{"Alice Smith <alice@example.com>"}, doc.Authors)
}

func TestCrystalConstraintMapsToRequirements(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", "---\nname: s\nversion: 1.0.0\ncrystal: \">= 1.13.0\"\n")
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{}
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.ToPF != nil {
			m.ToPF(true)
		}
	}
	require.NotNil(t, pf.Requirements)
	assert.Equal(t, ">= 1.13.0", pf.Requirements.Runtime["crystal"])
}

func TestLicenseURLIsPreservedNotSynced(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", "---\nname: s\nversion: 1.0.0\nlicense: https://example.com/LICENSE\n")
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{}
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.ToPF != nil {
			m.ToPF(true)
		}
	}
	if pf.License != nil {
		assert.Empty(t, pf.License.Spdx, "license-file URL must not land in license.spdx")
	}
	require.NoError(t, shard.Write(dir, doc))
	got, err := shard.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/LICENSE", got.License, "URL license must survive the sync")
}

func TestDescriptionBlockScalarTrimsCleanly(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", "---\nname: s\nversion: 1.0.0\ndescription: |\n  Linter for ignore files.\n")
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{}
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.ToPF != nil {
			m.ToPF(true)
		}
	}
	assert.Equal(t, "Linter for ignore files.", projectfile.ExtractLocalizedString(pf.Identity.Summary))
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.FromPF == nil {
			continue
		}
		assert.Emptyf(t, m.FromPF(true), "%s churns on a trimmed block scalar", m.ExtKey)
	}
}

func TestPreserveOnlyFieldsSurviveForceSync(t *testing.T) {
	dir := t.TempDir()
	writeShardYML(t, dir, "shard.yml", fixtureYAML)
	doc, err := shard.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "demo-shard"}}
	for _, m := range (shard.Bridge{}).BuildMappers(doc, pf) {
		if m.ToPF != nil {
			m.ToPF(true)
		}
		if m.FromPF != nil {
			m.FromPF(true)
		}
	}
	require.NoError(t, shard.Write(dir, doc))
	got, err := shard.Read(dir)
	require.NoError(t, err)
	for _, key := range []string{"dependencies", "development_dependencies", "targets", "executables", "scripts", "libraries"} {
		assert.True(t, got.Rest.Has(key), "%q must survive a force sync", key)
	}
}

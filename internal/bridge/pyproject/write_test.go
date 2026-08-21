// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pyproject

import (
	"os"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// REUSE-IgnoreStart — the literal SPDX strings below are fixtures, not
// licensing declarations.
const testReuseHeader = "# SPDX-FileCopyrightText: 2026 Test Holder\n" +
	"#\n" +
	"# SPDX-License-Identifier: MIT\n\n"

// fixturePyproject carries every shape the splice must respect: a REUSE
// header, a foreign table before [project], [project] sub-tables, and a
// commented foreign table after — the uwindaji/server layout in miniature.
const fixturePyproject = testReuseHeader +
	"[build-system]\n" +
	"build-backend = 'hatchling.build'\n" +
	"requires = ['hatchling==1.30.1']\n" +
	"\n" +
	"[project]\n" +
	"description = 'test package'\n" +
	"name = 'fixture'\n" +
	"version = '0.1.0'\n" +
	"\n" +
	"[[project.authors]]\n" +
	"name = 'Test Holder'\n" +
	"\n" +
	"[project.urls]\n" +
	"Homepage = 'https://example.test'\n" +
	"\n" +
	"[tool.ruff]\n" +
	"extend-exclude = ['src/gen']\n" +
	"\n" +
	"[tool.ruff.lint.per-file-ignores]\n" +
	"# generated stubs trip BLE001 everywhere — keep the ignore tight\n" +
	"'src/gen/servicers.py' = ['BLE001']\n"

// testNewVersion is the value every case writes into the fixture's 0.1.0.
const testNewVersion = "0.2.0"

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(FullPath(dir), []byte(content), 0o644))
	return dir
}

func readFile(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(FullPath(dir))
	require.NoError(t, err)
	return string(data)
}

// TestWriteSplicesOnlyProjectZone pins the regression this fix answers: a
// sync that changes one [project] key must leave the header, the foreign
// tables, their comments and their layout byte-identical.
func TestWriteSplicesOnlyProjectZone(t *testing.T) {
	dir := writeFixture(t, fixturePyproject)

	doc, err := Read(dir)
	require.NoError(t, err)
	doc.ReuseHeader = testReuseHeader
	doc.Project.Version = testNewVersion
	require.NoError(t, Write(dir, doc))

	got := readFile(t, dir)
	assert.True(t, strings.HasPrefix(got, testReuseHeader), "header must stay:\n%s", got)
	assert.Contains(t, got, "version = '"+testNewVersion+"'")
	assert.Contains(t, got, "[build-system]\nbuild-backend = 'hatchling.build'")
	assert.Contains(t, got, "# generated stubs trip BLE001 everywhere — keep the ignore tight",
		"user comments outside [project] must survive")
	assert.NotContains(t, got, "[tool.ruff.lint]\n", "empty parent tables must not appear")

	// Table order is the user's, not the marshaler's.
	iBuild := strings.Index(got, "[build-system]")
	iProject := strings.Index(got, "[project]\n")
	iRuff := strings.Index(got, "[tool.ruff]")
	require.Greater(t, iProject, iBuild)
	require.Greater(t, iRuff, iProject)
	assert.Contains(t, got, "\n\n[tool.ruff]", "one blank line must separate the zone from the next table")

	// The result must still parse and carry the new value plus the sub-tables.
	var reparsed map[string]any
	require.NoError(t, toml.Unmarshal([]byte(got), &reparsed))
	project := reparsed["project"].(map[string]any)
	assert.Equal(t, testNewVersion, project["version"])
	assert.Equal(t, map[string]any{"Homepage": "https://example.test"}, project["urls"])

	// A second identical write is a no-op — no header stacking, no reflow.
	require.NoError(t, Write(dir, doc))
	assert.Equal(t, got, readFile(t, dir))
}

// TestWriteRemovesEmptiedProjectZone clears every known field: the zone
// disappears whole (sub-tables included) without blank-line seams, and the
// rest of the file survives.
func TestWriteRemovesEmptiedProjectZone(t *testing.T) {
	dir := writeFixture(t, fixturePyproject)

	doc, err := Read(dir)
	require.NoError(t, err)
	doc.ReuseHeader = testReuseHeader
	doc.Project = Project{}
	require.NoError(t, Write(dir, doc))

	got := readFile(t, dir)
	assert.NotContains(t, got, "[project]")
	assert.NotContains(t, got, "https://example.test")
	assert.NotContains(t, got, "\n\n\n", "zone removal must not leave a blank seam")
	assert.Contains(t, got, "[build-system]")
	assert.Contains(t, got, "'src/gen/servicers.py' = ['BLE001']")
}

// TestWriteAppendsProjectZoneWhenAbsent starts from a file with no [project]
// at all — the new zone lands after the last content line, blank-separated.
func TestWriteAppendsProjectZoneWhenAbsent(t *testing.T) {
	source := "[build-system]\nrequires = ['hatchling==1.30.1']\n\n[tool.ruff]\nextend-exclude = ['src/gen']\n"
	dir := writeFixture(t, source)

	doc, err := Read(dir)
	require.NoError(t, err)
	doc.ReuseHeader = testReuseHeader
	doc.Project.Name = "fixture"
	require.NoError(t, Write(dir, doc))

	got := readFile(t, dir)
	assert.True(t, strings.HasPrefix(got, testReuseHeader), "missing header must be gap-filled:\n%s", got)
	require.Contains(t, got, "[project]\nname = 'fixture'")
	assert.Equal(t, "[build-system]\nrequires = ['hatchling==1.30.1']\n\n[tool.ruff]\nextend-exclude = ['src/gen']\n\n[project]\nname = 'fixture'\n",
		strings.SplitN(got, testReuseHeader, 2)[1], "zone appends after the last content line")
}

// TestWriteFreshDocumentHeader covers the create path: no source bytes, so
// the whole canvas marshals and the REUSE header leads the file.
func TestWriteFreshDocumentHeader(t *testing.T) {
	dir := t.TempDir()
	doc := &Document{Project: Project{Name: "fixture", Version: "1.0.0"}, ReuseHeader: testReuseHeader}
	require.NoError(t, Write(dir, doc))

	got := readFile(t, dir)
	assert.True(t, strings.HasPrefix(got, testReuseHeader))
	assert.Contains(t, got, "[project]\nname = 'fixture'")
	assert.Equal(t, 1, strings.Count(got, "SPDX-FileCopyrightText"))
}

// TestWriteKeepsHandWrittenHeaderVariants: a header the user wrote by hand —
// different holder, extra line — is never re-stamped from the projectfile.
func TestWriteKeepsHandWrittenHeaderVariants(t *testing.T) {
	handwritten := "# SPDX-FileCopyrightText: 2019 Someone Else\n#\n# SPDX-License-Identifier: Apache-2.0\n\n"
	dir := writeFixture(t, handwritten+"[project]\nname = 'fixture'\nversion = '0.1.0'\n")

	doc, err := Read(dir)
	require.NoError(t, err)
	doc.ReuseHeader = testReuseHeader
	doc.Project.Version = testNewVersion
	require.NoError(t, Write(dir, doc))

	got := readFile(t, dir)
	assert.True(t, strings.HasPrefix(got, handwritten), "a present header wins untouched:\n%s", got)
	assert.Equal(t, 1, strings.Count(got, "SPDX-FileCopyrightText"))
}

// TestWriteKeepsTrailingBlankLine pins EOF formatting: a file that ends on a
// blank line keeps it, and one without a final newline gains exactly one.
func TestWriteKeepsTrailingBlankLine(t *testing.T) {
	for name, tail := range map[string]string{"blank_line": "\n\n", "no_newline": ""} {
		t.Run(name, func(t *testing.T) {
			dir := writeFixture(t, "[project]\nname = 'fixture'\nversion = '0.1.0'\n"+tail)

			doc, err := Read(dir)
			require.NoError(t, err)
			doc.ReuseHeader = testReuseHeader
			doc.Project.Version = testNewVersion
			require.NoError(t, Write(dir, doc))

			assert.True(t, strings.HasSuffix(readFile(t, dir), "'"+testNewVersion+"'\n"+tail),
				"trailing formatting outside the change must round-trip")
		})
	}
}

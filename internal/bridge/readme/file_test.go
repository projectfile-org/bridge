// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// Fixture path and body shared by the file-group tests.
const (
	fileExample    = ".compose/example.yaml"
	exampleHeader  = "# SPDX-FileCopyrightText: 2026 A <a@b>\n#\n# SPDX-License-Identifier: MIT\n\n"
	exampleCompose = "---\nservices:\n  demo:\n    image: kiota.ch/demo:1.2.3\n"
)

// fileGroupDoc declares one sink and a quick-start group embedding the example file.
func fileGroupDoc(t *testing.T) *projectfile.Document {
	t.Helper()
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		sinksNS: map[string]any{sinkKiota: map[string]any{keyRef: refSinkKiota}},
		readmeNS: map[string]any{
			blockQuickStart: []any{map[string]any{
				keyName:   "compose-example",
				keyPrefix: "Save this as `compose.yaml`:",
				"file":    fileExample,
				"syntax":  "yaml",
				"postfix": "Then run `docker compose up --detach`.",
			}},
		},
	}
	return pf
}

// TestFileGroupEmbedsFileWithoutLicenseHeader: the fence holds the file as-is, minus its SPDX block.
func TestFileGroupEmbedsFileWithoutLicenseHeader(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, fileExample, exampleHeader+exampleCompose)

	out := renderDoc(t, dir, fileGroupDoc(t))

	assert.Contains(t, out, "Save this as `compose.yaml`:\n\n```yaml\n"+exampleCompose+"```\n\nThen run")
	assert.NotContains(t, out, "A <a@b>")
}

// TestFileGroupDropsWhenFileAbsent: no file, no group, and the section goes with it.
func TestFileGroupDropsWhenFileAbsent(t *testing.T) {
	out := renderDoc(t, t.TempDir(), fileGroupDoc(t))

	assert.NotContains(t, out, "compose.yaml")
}

// TestFileGroupFailsOnForeignImage: a file naming no published repository fails the render.
func TestFileGroupFailsOnForeignImage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, fileExample, "services:\n  demo:\n    image: docker.io/other/demo:latest\n")

	_, err := Bridge{}.Render(fileGroupDoc(t), core.Options{Dir: dir, Mode: modeWrite, Force: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "kiota.ch/demo")
}

// TestStripLicenseHeaderKeepsOrdinaryComments: a leading comment without SPDX tags is content.
func TestStripLicenseHeaderKeepsOrdinaryComments(t *testing.T) {
	text := "# Run with docker compose\nservices: {}\n"

	assert.Equal(t, text, stripLicenseHeader(text))
}

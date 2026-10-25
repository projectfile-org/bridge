// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forge_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/scanners/forge"
)

const fragment = `---
org:
  projectfile:
    forge:
      links:
        - type: source-code
          preferred: true
          priority: 10
          tags: [public, badges]
          url: https://kiota.ch/${path}
        - type: bugs
          url: https://kiota.ch/${path}/issues
        - type: source-code
          url: https://github.com/damian-buho/${flatpath}
          label: {en: "${name} mirror on GitHub", es: "Réplica de ${name} en GitHub"}
        - type: source-code
          url: https://codeberg.org/${nope}
      repositories:
        - role: origin
          branch: main
          type: git
          url: ssh://git@kiota.ch/${path}.git
        - role: mirror
          url: git@example.org:${nope}.git
`

const document = `---
identity:
  name: go
  title: B19 / Go
includes:
  - fragment.yaml
org:
  projectfile:
    i18n:
      languages: [en, es]
    forge:
      org: b19
      name: ${identity.name}
      path: ${org}/${name}
      flatpath: ${org}-${name}
`

// write lays the scratch project down: the shared document plus the fragment it includes.
func write(t *testing.T, frag string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fragment.yaml"), []byte(frag), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte(document), 0o600))
	return dir
}

// A template is a URL or nothing: every resolved entry lands concrete, an unresolved one is skipped.
func TestScanMaterializesTemplates(t *testing.T) {
	p, hits, err := forge.Scanner{}.Scan(write(t, fragment))
	require.NoError(t, err)

	require.Len(t, p.Links, 3, "the codeberg template cannot resolve and is skipped")
	assert.Equal(t, "https://kiota.ch/b19/go", p.Links[0].URL)
	assert.True(t, p.Links[0].Preferred)
	assert.Equal(t, 10, p.Links[0].Extra["priority"], "priority rides the extra channel untouched")
	assert.Equal(t, "Repository on kiota.ch", p.Links[0].Label.Langs["en"], "no label → the noun placeholder")
	assert.Equal(t, "https://kiota.ch/b19/go/issues", p.Links[1].URL)
	assert.Equal(t, "Incidencias en kiota.ch", p.Links[1].Label.Langs["es"])
	assert.Equal(t, "go mirror on GitHub", p.Links[2].Label.Langs["en"], "an explicit label is expanded in place")
	assert.Equal(t, "Réplica de go en GitHub", p.Links[2].Label.Langs["es"])

	require.Len(t, p.Repositories, 1)
	assert.Equal(t, projectfile.Repository{Role: "origin", Branch: "main", Type: "git", URL: "ssh://git@kiota.ch/b19/go.git"}, p.Repositories[0])
	assert.Len(t, hits, 4)
}

// A document declaring no templates proposes nothing and errs on nothing.
func TestScanWithoutTemplatesIsEmpty(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte("identity:\n  name: x\n"), 0o600))

	p, hits, err := forge.Scanner{}.Scan(dir)
	require.NoError(t, err)
	assert.Empty(t, p.Links)
	assert.Empty(t, p.Repositories)
	assert.Empty(t, hits)
}

// labelGap declares a label whose Spanish half names a part this document has not, so the whole link is skipped.
const labelGap = `---
org:
  projectfile:
    forge:
      links:
        - type: source-code
          url: https://kiota.ch/${path}
          label: {en: "${name} on kiota.ch", es: "${nope} en kiota.ch"}
`

// nounGap declares a link type the noun table never spells, so the placeholder label answers nil.
const nounGap = `---
org:
  projectfile:
    forge:
      links:
        - type: documentation
          url: https://kiota.ch/${path}/docs
`

// Detect answers by the projectfile's presence in root; an empty directory has none.
func TestDetect(t *testing.T) {
	assert.False(t, forge.Scanner{}.Detect(t.TempDir()))
	assert.True(t, forge.Scanner{}.Detect(write(t, fragment)))
}

// A directory holding no projectfile fails the read rather than proposing nothing.
func TestScanMissingProjectfileFails(t *testing.T) {
	p, hits, err := forge.Scanner{}.Scan(t.TempDir())
	require.Error(t, err)
	assert.Nil(t, p)
	assert.Nil(t, hits)
}

// A label unresolved in any language drops the whole link, never a half-translated one.
func TestScanSkipsUnresolvedLabel(t *testing.T) {
	p, hits, err := forge.Scanner{}.Scan(write(t, labelGap))
	require.NoError(t, err)
	assert.Empty(t, p.Links)
	assert.Empty(t, hits)
}

// A link type with no noun in the table gets a nil placeholder label, not a guess.
func TestScanNilPlaceholderLabelForUnnounsedType(t *testing.T) {
	p, hits, err := forge.Scanner{}.Scan(write(t, nounGap))
	require.NoError(t, err)
	require.Len(t, p.Links, 1)
	assert.Equal(t, "https://kiota.ch/b19/go/docs", p.Links[0].URL)
	assert.Nil(t, p.Links[0].Label)
	assert.Len(t, hits, 1)
}

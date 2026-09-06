// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package vulnerabilities

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	testTrivyIgnore = ".trivyignore"
	testCVE1        = "CVE-2026-1"
	testCVE2        = "CVE-2025-2"
)

// sampleDoc builds a Document carrying the vulnerabilities extension in the
// map form the parser produces (the getter type-asserts map[string]any, not
// the typed struct), mirroring real round-trip behaviour.
func sampleDoc(suppress []pfmodel.VulnerabilitySuppress, generate ...string) *projectfile.Document {
	sup := make([]any, len(suppress))
	for i, e := range suppress {
		entry := map[string]any{"id": e.ID}
		if e.Reason != "" {
			entry["reason"] = e.Reason
		}
		sup[i] = entry
	}
	ns := map[string]any{"suppress": sup}
	if len(generate) > 0 {
		g := make([]any, len(generate))
		for i, x := range generate {
			g[i] = x
		}
		ns["generate"] = g
	}
	return &projectfile.Document{
		Identity:   projectfile.Identity{Name: "p"},
		Extensions: map[string]any{pfmodel.VulnerabilitiesExtensionNS: ns},
	}
}

// Each scanner renders the same IDs to its own format.
func TestRenderAllFormats(t *testing.T) {
	suppress := []pfmodel.VulnerabilitySuppress{
		{ID: "CVE-2026-46320", Reason: "unfixable kernel headers"},
		{ID: "CVE-2025-40190"},
		{ID: "GO-2026-5005"}, // non-CVE/GHSA ids MUST fan out too (Go vuln DB).
	}
	doc := sampleDoc(suppress)

	t.Run(scannerTrivy, func(t *testing.T) {
		b := Bridge{scanner: scannerTrivy, filename: testTrivyIgnore}
		out, err := b.Render(doc, core.Options{})
		assert.NoError(t, err)
		body := string(out.Files[testTrivyIgnore])
		assert.Contains(t, body, core.Marker)
		assert.Contains(t, body, "CVE-2025-40190")
		assert.Contains(t, body, "CVE-2026-46320")
		assert.Contains(t, body, "GO-2026-5005") // GO-* ids are not filtered out.
		// Sorted.
		assert.Less(t, strings.Index(body, "CVE-2025-40190"), strings.Index(body, "CVE-2026-46320"))
		// Plain dialect carries no reason.
		assert.NotContains(t, body, "unfixable kernel headers")
	})

	t.Run(scannerGrype, func(t *testing.T) {
		b := Bridge{scanner: scannerGrype, filename: ".grype.yaml"}
		out, err := b.Render(doc, core.Options{})
		assert.NoError(t, err)
		body := string(out.Files[".grype.yaml"])
		assert.True(t, strings.HasPrefix(body, core.YAMLDocStart))
		assert.Contains(t, body, "ignore:")
		assert.Contains(t, body, "vulnerability: CVE-2026-46320  # unfixable kernel headers")
		assert.Contains(t, body, "vulnerability: CVE-2025-40190")
	})

	t.Run(scannerOSV, func(t *testing.T) {
		b := Bridge{scanner: scannerOSV, filename: "osv-scanner.toml"}
		out, err := b.Render(doc, core.Options{})
		assert.NoError(t, err)
		body := string(out.Files["osv-scanner.toml"])
		assert.Contains(t, body, "[[IgnoredVulns]]")
		assert.Contains(t, body, `id = "CVE-2026-46320"`)
		assert.Contains(t, body, `reason = "unfixable kernel headers"`)
		assert.Contains(t, body, `id = "CVE-2025-40190"`)
		// An entry with no reason emits no reason line.
		assert.NotContains(t, body, `reason = ""`)
	})

	t.Run(scannerAuditCI, func(t *testing.T) {
		b := Bridge{scanner: scannerAuditCI, filename: "audit-ci.jsonc"}
		out, err := b.Render(doc, core.Options{})
		assert.NoError(t, err)
		body := string(out.Files["audit-ci.jsonc"])
		assert.Contains(t, body, core.MarkerSlash)
		assert.Contains(t, body, `"low": true`)
		assert.Contains(t, body, `"CVE-2026-46320"`)
		assert.Contains(t, body, "// unfixable kernel headers")
		assert.Contains(t, body, `"CVE-2025-40190"`)
		// `#` never leaks into the JSONC header; only `//` comments.
		assert.NotContains(t, body, "\n#")
	})
}

// Dedup + sort is stable regardless of input order or duplicates.
func TestDedupSorted(t *testing.T) {
	in := []pfmodel.VulnerabilitySuppress{
		{ID: testCVE1},
		{ID: testCVE2},
		{ID: testCVE1}, // dup
		{ID: ""},
	}
	out := dedupSorted(in)
	assert.Equal(t, []string{testCVE2, testCVE1}, []string{out[0].ID, out[1].ID})
}

// Empty / absent suppress list emits no file for any scanner.
func TestRenderEmptySuppressesNoFile(t *testing.T) {
	cases := []struct {
		name string
		doc  *projectfile.Document
	}{
		{"absent", &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}},
		{"empty", sampleDoc(nil)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, s := range []string{scannerTrivy, scannerGrype, scannerOSV, scannerAuditCI} {
				b := Bridge{scanner: s, filename: s}
				out, err := b.Render(c.doc, core.Options{})
				assert.NoError(t, err)
				assert.Empty(t, out.Files)
			}
		})
	}
}

// `generate` opts a project out of scanners it does not run.
func TestGenerateOptsOut(t *testing.T) {
	suppress := []pfmodel.VulnerabilitySuppress{{ID: testCVE1}}
	doc := sampleDoc(suppress, scannerTrivy) // only trivy
	for _, c := range []struct {
		scanner string
		want    bool
	}{
		{scannerTrivy, true},
		{scannerGrype, false},
		{scannerOSV, false},
	} {
		b := Bridge{scanner: c.scanner, filename: c.scanner}
		out, err := b.Render(doc, core.Options{})
		assert.NoError(t, err)
		if c.want {
			assert.NotEmpty(t, out.Files, c.scanner)
		} else {
			assert.Empty(t, out.Files, c.scanner)
		}
	}
}

// Shorthand bare-string suppress list (no reasons) parses and renders.
func TestShorthandStringList(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			pfmodel.VulnerabilitiesExtensionNS: map[string]any{
				"suppress": []any{testCVE1, testCVE2},
			},
		},
	}
	ext, err := pfmodel.GetVulnerabilitiesExtension(doc)
	assert.NoError(t, err)
	assert.Len(t, ext.Suppress, 2)
	b := Bridge{scanner: scannerTrivy, filename: testTrivyIgnore}
	out, err := b.Render(doc, core.Options{})
	assert.NoError(t, err)
	assert.Contains(t, string(out.Files[testTrivyIgnore]), testCVE1)
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package book

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

const (
	forgeGitHub = "https://github.com/o/tool"
	forgeKiota  = "https://codeberg.org/o/tool"
)

// sourceCode declares the forge pages links resolve to: kiota preferred, GitHub tagged public.
func sourceCode(pf *projectfile.Document, branch string) {
	pf.Links = []projectfile.Link{
		{Type: projectfile.LinkSourceCode, URL: forgeKiota, Preferred: true},
		{Type: projectfile.LinkSourceCode, URL: forgeGitHub, Extra: map[string]any{"tags": []any{"public"}}},
	}
	pf.Repositories = []projectfile.Repository{{URL: "git@codeberg.org:o/tool.git", Branch: branch}}
}

// TestStageRewritesLinks pins every link decision: same book, README as index, another language book, fallback, asset, forge, untouched, missing.
func TestStageRewritesLinks(t *testing.T) {
	in, pf := fixture(t)
	sourceCode(pf, "")
	write(t, in.Dir, readmeMD, "# Tool\n\n[Español](docs/es/README.md) [features](docs/FEATURES.md#list) [license](LICENSE) [docs](docs/) [server](docs/server.md)\n"+
		"[web](https://x.test/) [mail](mailto:a@x.test) [top](#top) [root](/abs.md) [gone](docs/gone.md) [out](../out.md) [spaced](<docs/a b.png>)\n"+
		"```md\n[fenced](CONTRIBUTING.md)\n```\n")
	write(t, in.Dir, "docs/es/README.md", "# Herramienta\n\n[English](../../README.md) [guía](../how-to/beta.md) [imagen](shot.png)\n")
	write(t, in.Dir, "docs/es/FEATURES.md", "# Características\n\n[English](../FEATURES.md) [uso](USAGE.md)\n")
	write(t, in.Dir, "docs/how-to/beta.md", "no heading\n\n[features](../FEATURES.md) [readme](../../README.md)\n")
	write(t, in.Dir, "CONTRIBUTING.md", "# Contributing\n\n[readme](README.md#top)\n")
	write(t, in.Dir, "LICENSE", "MIT")
	write(t, in.Dir, "docs/server.md", "# Server\n")
	write(t, in.Dir, "docs/a b.png", "png")
	write(t, in.Dir, "docs/es/shot.png", "png")
	out := filepath.Join(t.TempDir(), "book")

	_, err := Stage(in.Dir, in.Rendered, out, pf)
	require.NoError(t, err)

	readme := read(t, out, "en/src/README.md")
	assert.Contains(t, readme, "[Español](../es/index.md)", "the language bar crosses into the other book")
	assert.Contains(t, readme, "[features](docs/FEATURES.md#list)", "a page in this book keeps its path and fragment")
	assert.Contains(t, readme, "[license]("+forgeGitHub+"/blob/HEAD/LICENSE)", "an unstaged file goes to the public forge")
	assert.Contains(t, readme, "[docs]("+forgeGitHub+"/blob/HEAD/docs)", "a directory goes to the public forge")
	assert.Contains(t, readme, "[server]("+forgeGitHub+"/blob/HEAD/docs/server.md)", "a page in no part goes to the public forge")
	assert.Contains(t, readme, "[web](https://x.test/) [mail](mailto:a@x.test) [top](#top) [root](/abs.md)", "schemes, anchors and absolute paths are untouched")
	assert.Contains(t, readme, "[gone](docs/gone.md) [out](../out.md)", "a missing or outside target is left as is")
	assert.Contains(t, readme, "[spaced](<docs/a%20b.png>)", "a spaced asset stays one token")
	assert.Equal(t, "png", read(t, out, "en/src/docs/a b.png"))
	assert.Contains(t, readme, "[fenced](CONTRIBUTING.md)", "fenced code is never rewritten")
	assert.Contains(t, read(t, out, "en/src/CONTRIBUTING.md"), "[readme](index.md#top)", "mdBook serves README.md as index.html")

	es := read(t, out, "es/src/README.md")
	assert.Contains(t, es, "[English](../en/index.md)")
	assert.Contains(t, es, "[guía](docs/how-to/beta.md)", "a link rebases from the variant to its canonical place")
	assert.Contains(t, es, "[imagen](docs/es/shot.png)")
	assert.Equal(t, "png", read(t, out, "es/src/docs/es/shot.png"))
	features := read(t, out, "es/src/docs/FEATURES.md")
	assert.Contains(t, features, "[English](../../en/docs/FEATURES.md)", "a translation linking its canonical page crosses books")
	assert.Contains(t, features, "[uso](USAGE.md)", "an untranslated target in this book stays here")
	beta := read(t, out, "es/src/docs/how-to/beta.md")
	assert.Contains(t, beta, "[features](../FEATURES.md)", "a fallback page stays in its own language book")
	assert.Contains(t, beta, "[readme](../../index.md)")
}

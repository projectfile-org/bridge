// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// usageMD is an assembled USAGE.md: top-level help, two examples, a subcommand help, an inherited section.
const usageMD = "# Usage\n\n## Project Usage\n\n" +
	"### `demo --help`\n\n```text\nUsage: demo [PATH]\n# not a heading\n```\n\n" +
	"### Lint a repository\n\n```sh\ndemo .\n```\n\n" +
	"### Fix in place\n\n```sh\ndemo --fix .\n```\n\n" +
	"### `demo lint --help`\n\n```text\nUsage: demo lint\n```\n\n" +
	"## Inherited from parent\n\n### Parent example\n\nbody\n"

// TestUsageExcerptShowsTopHelpAndFirstExample: the README quotes two sections of USAGE.md and links the rest.
func TestUsageExcerptShowsTopHelpAndFirstExample(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, fileUsage, usageMD)

	out := renderDoc(t, dir, minimalDoc(t))

	assert.Contains(t, out, "## Usage\n\n### `demo --help`\n\n```text\nUsage: demo [PATH]\n# not a heading\n```")
	assert.Contains(t, out, "### Lint a repository\n\n```sh\ndemo .\n```")
	assert.NotContains(t, out, "Fix in place")
	assert.NotContains(t, out, "demo lint --help")
	assert.NotContains(t, out, "Parent example")
	assert.Contains(t, out, "[USAGE.md](USAGE.md)")
}

// TestUsageExcerptReadsFlatDocument: a standalone USAGE.md carries its sections at H2 with no Project heading.
func TestUsageExcerptReadsFlatDocument(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, fileUsage, "# Usage\n\n## `demo --help`\n\n```text\nUsage: demo\n## not a heading\n```\n\n"+
		"## Lint a repository\n\n```sh\ndemo .\n```\n\n## `demo lint --help`\n\n```text\nUsage: demo lint\n```\n")

	out := renderDoc(t, dir, minimalDoc(t))

	assert.Contains(t, out, "### `demo --help`\n\n```text\nUsage: demo\n## not a heading\n```")
	assert.Contains(t, out, "### Lint a repository\n\n```sh\ndemo .\n```")
	assert.NotContains(t, out, "demo lint --help")
}

// TestUsageExcerptDropsLintDirectives: a localized USAGE.md’s closing textlint directive stays out of the README.
func TestUsageExcerptDropsLintDirectives(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, fileUsage, "# Usage\n\n<!-- textlint-disable terminology -->\n\n## `demo --help`\n\n```text\nUsage: demo\n```\n\n"+
		"## Lint a repository\n\n```sh\ndemo .\n```\n\n<!-- textlint-enable -->\n")

	out := renderDoc(t, dir, minimalDoc(t))

	assert.Contains(t, out, "### Lint a repository\n\n```sh\ndemo .\n```")
	assert.NotContains(t, out, "textlint-enable")
}

// TestUsageWithoutSectionsFallsBackToLink: a USAGE.md with no level-3 section keeps the plain link.
func TestUsageWithoutSectionsFallsBackToLink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, fileUsage, "# Usage\n\nRun it.\n")

	out := renderDoc(t, dir, minimalDoc(t))

	assert.Contains(t, out, "See [USAGE.md](USAGE.md) for examples.")
}

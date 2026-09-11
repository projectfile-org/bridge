// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package yamllint generates the project `.yamllint` config. Unlike the
// flat-file ignore family (bridge/ignore), yamllint has no standalone ignore
// file: its excludes live as an `ignore:` block INSIDE the YAML config, and
// auto-yamllint uses a project `.yamllint` INSTEAD OF the image default.
//
// The exclude patterns are read from the same namespace as every other
// ignore target — org.projectfile.ignores.yamllint.include, minus anything
// its .exclude drops — so they fan out identically to
// git/docker/npm/claude/container (defaults live in m6e/core/traits/yaml.yaml
// and per-language m6e includes, deep-merged into the project document before
// this bridge runs).
//
// Why this is its own package (not a row in bridge/ignore):
//   - yamllint's `ignore:` REPLACES (not merges) the inherited block on
//     `extends:` (empirically verified). A generated config that sets
//     `ignore:` to the project's excludes alone would silently drop the
//     image-default ignores (vendor, node_modules). So every pattern must be
//     re-emitted here — which is why the language fragments declare their own
//     vendored trees rather than leaning on the image backstop.
//   - bridge/ignore's assembler writes one sorted pattern per line under a `# >>> user-patterns` banner; yamllint needs a YAML block plus rules, so forcing them into one assembler would break the flat-file family's contract.
package yamllint

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	filenameYamllint = ".yamllint"
	extKeyYamllint   = "yamllint"
)

// baseRules is all this bridge asserts on its own: yamllint's `default`
// ruleset and nothing else. Every deviation is DECLARED in the projectfile —
// pf-bridge ships to any consumer and holds no style opinion of its own.
// Kept as a constant (not `extends: /app/...`) so the generated config works
// on the host too, outside the b19 image (where B19_HOME=/app would not
// resolve); yamllint would FileNotFoundError a path-extends otherwise.
const baseRules = "extends: default\n"

// Bridge implements core.Renderer for the project `.yamllint` config.
type Bridge struct {
	filename string
}

func (b Bridge) Name() string      { return "yamllint-config" }
func (b Bridge) Filename() string  { return b.filename }
func (b Bridge) Aliases() []string { return nil }
func (b Bridge) Labels() (string, string) {
	return b.filename, "projectfile"
}

// The generated config carries the marker so the next run overwrites cleanly.
func (Bridge) Policy() core.Policy { return core.Policy{Marker: true} }

func (b Bridge) Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, b.filename))
	return err == nil
}

func (b Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return filepath.Join(dir, b.filename)
}

func (b Bridge) Render(pf *projectfile.Document, _ core.Options) (core.Output, error) {
	ext, err := pfmodel.GetIgnoresExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	// No yamllint sub-namespace → emit nothing. The caller skips the write,
	// so removing every include drops generation; clear a stale file by hand
	// or via `git clean`.
	if ext == nil || ext.Yamllint == nil {
		return core.Output{}, nil
	}
	// `generate` opts a project out of yamllint generation. Unset means
	// "generate" — the default.
	if !ext.Generates(extKeyYamllint) {
		return core.Output{}, nil
	}
	includes := ext.Yamllint.Patterns()
	if len(includes) == 0 {
		return core.Output{}, nil
	}
	// The line width is tool-agnostic and lives with the other cross-cutting
	// conventions, so yamllint carries no number of its own.
	conv, err := pfmodel.GetConventionsExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	lineLength := 0
	if conv != nil {
		lineLength = conv.CodeStyle.LineLength
	}
	body := assemble(pf, includes, lineLength)
	return core.Output{Files: map[string][]byte{b.filename: body}}, nil
}

// assemble builds the byte body of `.yamllint`. Layout:
//
//  1. `---` document-start (yamllint rule document-start wants it; matches
//     projectfile.yaml convention)
//  2. REUSE header (SPDX-FileCopyrightText + SPDX-License-Identifier)
//  3. banner (incl. Marker) pointing at org.projectfile.ignores.yamllint
//  4. base rule block (`extends: default`) plus any DECLARED rule override
//  5. `ignore:|` block: sorted, de-duplicated includes
//
// Every ignore pattern traces back to a user-declared entry in the
// projectfile (or an m6e include that fed it) — nothing is emitted without
// provenance.
func assemble(pf *projectfile.Document, includes []string, lineLength int) []byte {
	entries := sortDedup(includes)

	var buf bytes.Buffer
	buf.WriteString(core.YAMLDocStart)
	buf.WriteString(core.REUSEHeader(pf, core.StyleHash))
	buf.WriteString(yamllintBanner())
	buf.WriteString(baseRules)
	buf.WriteString(writeRules(lineLength))
	buf.WriteString(writeIgnore(entries))
	return buf.Bytes()
}

// writeRules emits the `rules:` block for the declared overrides. Only the
// agnostic line width is expressible today, so an undeclared (0) width emits
// nothing and yamllint's own default stands. `level: warning` keeps a long
// line reported but non-gating — the width is a convention, not a gate.
func writeRules(lineLength int) string {
	if lineLength <= 0 {
		return ""
	}
	return fmt.Sprintf("rules:\n  line-length:\n    level: warning\n    max: %d\n", lineLength)
}

// sortDedup returns the includes sorted and de-duplicated, so output is
// stable regardless of projectfile list order or duplicate entries.
func sortDedup(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, p := range in {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// yamllintBanner is the top-of-file block. It names the source namespace so a
// reader edits the projectfile, not the generated file. Custom (not
// core.Banner) because core.Banner hard-codes the `pf-bridge ignore` command.
func yamllintBanner() string {
	return fmt.Sprintf(`# %s — generated by `+"`pf-bridge yamllint`"+`
# Source: org.projectfile.ignores.yamllint
# Edit projectfile [org.projectfile.ignores.yamllint] include to customise —
# direct edits here are overwritten on the next generation.
%s

`, filenameYamllint, core.Marker)
}

// writeIgnore emits the `ignore:` block. yamllint's `ignore` accepts a single
// multi-line string of newline-separated pathspecs; blank lines and `#`
// comments are NOT stripped by yamllint, so patterns are emitted one per line
// with no commentary inside the block.
func writeIgnore(entries []string) string {
	var b strings.Builder
	b.WriteString("ignore: |\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "  %s\n", e)
	}
	return b.String()
}

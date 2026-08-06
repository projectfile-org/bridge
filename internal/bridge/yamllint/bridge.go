// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package yamllint generates the project `.yamllint` config. Unlike the
// flat-file ignore family (bridge/ignore), yamllint has no standalone ignore
// file: its excludes live as an `ignore:` block INSIDE the YAML config, and
// auto-yamllint uses a project `.yamllint` INSTEAD OF the image default.
//
// The exclude patterns are read from the same namespace as every other
// ignore target — org.projectfile.ignores.yamllint.include — so they fan out
// identically to git/docker/npm/claude/container (defaults live in
// m6e/core/conventions.yaml and per-language m6e includes, deep-merged into
// the project document before this bridge runs).
//
// Why this is its own package (not a row in bridge/ignore):
//   - yamllint's `ignore:` REPLACES (not merges) the inherited block on
//     `extends:` (empirically verified). A generated config that sets
//     `ignore:` to the project's excludes alone would silently drop the
//     image-default ignores (vendor, node_modules). So every pattern must be
//     re-emitted here, and the fleet rule block with it.
//   - bridge/ignore's assembler writes one pattern per line under a
//     `# >>> user-include` banner; yamllint needs a YAML `ignore: |` block
//     plus `extends:`/`rules:`. Forcing them into one assembler would break
//     the flat-file family's contract.
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

// fleetRules mirrors the fleet-wide yamllint policy from
// d9t/python-tools/.container/user/app/.config/yamllint/config. Kept here as
// a constant (not `extends: /app/...`) so the generated config works on the
// host too, outside the b19 image (where B19_HOME=/app would not resolve).
// yamllint's `extends:` would FileNotFoundError a path-extends otherwise.
// Update BOTH places when the fleet default changes.
const fleetRules = `extends: default
rules:
  # Inline-map secrets shorthand: { run: m6e-secret-random }, { value: CHANGEME }.
  # The projectfile convention writes one space inside the braces; the default
  # (0) would reject 35+ projectfiles.
  braces:
    max-spaces-inside: 1
  line-length:
    level: warning
    max: 120
`

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
	if !enabledFor(ext.Generate) {
		return core.Output{}, nil
	}
	includes := ext.Yamllint.Include
	if len(includes) == 0 {
		return core.Output{}, nil
	}
	body := assemble(pf, includes)
	return core.Output{Files: map[string][]byte{b.filename: body}}, nil
}

// enabledFor reports whether yamllint should generate. An explicit `generate`
// list is authoritative: only the listed targets generate. An absent list
// means yamllint generates whenever the yamllint sub-namespace is present.
func enabledFor(generate []string) bool {
	if len(generate) == 0 {
		return true
	}
	for _, g := range generate {
		if g == extKeyYamllint {
			return true
		}
	}
	return false
}

// assemble builds the byte body of `.yamllint`. Layout:
//
//  1. `---` document-start (yamllint rule document-start wants it; matches
//     projectfile.yaml convention)
//  2. REUSE header (SPDX-FileCopyrightText + SPDX-License-Identifier)
//  3. banner (incl. Marker) pointing at org.projectfile.ignores.yamllint
//  4. fleet rule block (mirrors the image default)
//  5. `ignore:|` block: sorted, de-duplicated includes
//
// Every ignore pattern traces back to a user-declared entry in the
// projectfile (or an m6e include that fed it) — nothing is emitted without
// provenance.
func assemble(pf *projectfile.Document, includes []string) []byte {
	entries := sortDedup(includes)

	var buf bytes.Buffer
	buf.WriteString(core.YAMLDocStart)
	buf.WriteString(core.REUSEHeader(pf, core.StyleHash))
	buf.WriteString(yamllintBanner())
	buf.WriteString(fleetRules)
	buf.WriteString(writeIgnore(entries))
	return buf.Bytes()
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

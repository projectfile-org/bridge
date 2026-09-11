// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package gitattributes renders `.gitattributes` from
// `org.projectfile.attributes`. Derive-only (Renderer): pf is authoritative and
// the external file is never read back.
//
// Why a project wants this file generated: a git attribute is the only
// repository-wide way to state "this path IS a shell script" for a file whose
// name carries no extension — a PATH verb in a command directory, a compiler
// shim named after the binary it shadows. Once the fact lives here, every
// consumer asks git for it (`:(attr:shell)` pathspecs, `git check-attr`)
// instead of re-implementing "is this a shell script?" per tool. Two such
// implementations always drift, and when one of them is a linter's cache key
// the drift is silent: the linter greens a file it never read.
//
// Built-ins stay universal. Layout conventions (a project's command/hook
// directories) arrive through the projectfile, so this tool never has to know
// any particular repository's shape.
package gitattributes

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// filename is the canonical on-disk name AND the Output map key.
const filename = ".gitattributes"

// rule is one emitted line: a git pattern and the attribute it sets. A leading
// `-` on attr is the unset form, which git honours as "explicitly not this".
type rule struct {
	pattern string
	attr    string
}

// builtins is the UNIVERSAL vocabulary — rules that hold for any repository
// whatever its layout. Deliberately narrow: exactly the two extensions that
// name a shell script beyond argument. Anything requiring knowledge of a
// project's directory conventions belongs in that project's projectfile, not
// in the reference tooling.
var builtins = []rule{
	{pattern: "*.sh", attr: "shell"},
	{pattern: "*.bash", attr: "shell"},
}

// Bridge renders `.gitattributes` from `org.projectfile.attributes`.
type Bridge struct{}

func (Bridge) Name() string             { return "gitattributes" }
func (Bridge) Filename() string         { return filename }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filename, "projectfile" }

// Policy: marker-managed like the .gitignore family — pure data with no
// user-edit expectation, so the marker lets the next run overwrite cleanly
// while still protecting a genuine hand edit.
func (Bridge) Policy() core.Policy { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, filename))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return filepath.Join(dir, filename)
}

func (Bridge) Render(pf *projectfile.Document, _ core.Options) (core.Output, error) {
	ext, err := pfmodel.GetAttributesExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	body := assemble(pf, ext)
	named := ext.AttributeNames()
	detail := fmt.Sprintf("%d builtin + %d user rule(s)", len(builtins), countUserRules(ext))
	if len(named) > 0 {
		detail += " for " + strings.Join(named, ", ")
	}
	genlog.Decision("attributes", detail, pfmodel.AttributesExtensionNS, "")
	return core.Output{Files: map[string][]byte{filename: body}}, nil
}

// assemble builds the byte body. Layout:
//
//  1. REUSE header + banner (incl. Marker)
//  2. builtin block — the universal vocabulary
//  3. per-attribute user includes  (sets the attribute)
//  4. per-attribute user excludes  (unsets it)
//  5. raw user extra lines
//
// Steps 3 and 4 are separate blocks rather than interleaved because git
// resolves attributes LAST-MATCH-WINS: an exclude emitted before the include
// that it narrows would be silently overridden.
func assemble(pf *projectfile.Document, ext *pfmodel.AttributesExtension) []byte {
	var content bytes.Buffer

	writeBlock(&content, "builtin", builtins)

	for _, name := range ext.AttributeNames() {
		rules := ext.Rules[name]
		writeBlock(&content, name+"-include", rulesFor(rules.Include, name))
	}
	for _, name := range ext.AttributeNames() {
		rules := ext.Rules[name]
		writeBlock(&content, name+"-exclude", rulesFor(rules.Exclude, "-"+name))
	}

	if ext != nil && len(ext.Extra) > 0 {
		content.WriteString("# >>> user-extra\n")
		for _, line := range dedupSorted(ext.Extra) {
			content.WriteString(line)
			content.WriteString("\n")
		}
		content.WriteString("# <<< user-extra\n\n")
	}

	var buf bytes.Buffer
	buf.WriteString(core.REUSEHeader(pf, core.StyleHash))
	buf.WriteString(banner())
	buf.Write(content.Bytes())
	return buf.Bytes()
}

// writeBlock emits one labelled block sorted by pattern and padded to a common column.
func writeBlock(buf *bytes.Buffer, label string, rules []rule) {
	if len(rules) == 0 {
		return
	}
	sorted := make([]rule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].pattern < sorted[j].pattern })
	width := 0
	for _, r := range sorted {
		if len(r.pattern) > width {
			width = len(r.pattern)
		}
	}
	fmt.Fprintf(buf, "# >>> %s\n", label)
	for _, r := range sorted {
		fmt.Fprintf(buf, "%-*s  %s\n", width, r.pattern, r.attr)
	}
	fmt.Fprintf(buf, "# <<< %s\n\n", label)
}

// rulesFor pairs each pattern with attr, sorted and deduplicated so the output
// is byte-stable no matter how the include lists were merged upstream.
func rulesFor(patterns []string, attr string) []rule {
	out := make([]rule, 0, len(patterns))
	for _, p := range dedupSorted(patterns) {
		out = append(out, rule{pattern: p, attr: attr})
	}
	return out
}

func dedupSorted(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func countUserRules(ext *pfmodel.AttributesExtension) int {
	n := 0
	for _, name := range ext.AttributeNames() {
		n += len(ext.Rules[name].Include) + len(ext.Rules[name].Exclude)
	}
	if ext != nil {
		n += len(ext.Extra)
	}
	return n
}

func banner() string {
	return fmt.Sprintf(`# %s — generated by `+"`pf-bridge gitattributes`"+`
# Attributes let tooling ask GIT what a file is, so a name that carries no
# extension is still classified. Query them with `+"`:(attr:NAME)`"+` pathspecs.
# Edit projectfile [%s] include/exclude to customise —
# direct edits here are overwritten on the next generation.
%s

`, filename, pfmodel.AttributesExtensionNS, core.Marker)
}

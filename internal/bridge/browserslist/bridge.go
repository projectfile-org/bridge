// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package browserslist renders `.browserslistrc` from
// `requirements.browsers`. Derive-only (Renderer): pf is authoritative and we
// never read the external file back. The field is a string-or-sequence of
// browserslist queries; each entry becomes one query line. When the list is
// empty/absent the file is suppressed (no-op) so we never emit a meaningless
// empty config that would override a tool's sensible defaults.
//
// Absent is NOT an error: spec §4.8 reads `browsers` absent as "unconstrained",
// and this bridge runs by default in every node-stack project — including
// server-side ones that have no browser matrix to declare. A shape we cannot
// render (the per-environment mapping form) still errors, so a real
// misconfiguration stays loud while an honest opt-out stays quiet.
package browserslist

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// filename is the canonical on-disk name AND the Output map key.
const filename = ".browserslistrc"

// Bridge renders `.browserslistrc` from `requirements.browsers`.
type Bridge struct{}

func (Bridge) Name() string             { return "browserslist" }
func (Bridge) Filename() string         { return filename }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filename, "projectfile" }
func (Bridge) Stacks() []string         { return []string{"node", "javascript", "typescript"} }

// Policy: marker-managed like the .gitignore family / FUNDING.yml — the file is
// pure data with no user-edit expectation, so a marker lets the next run
// overwrite cleanly while still protecting genuine hand edits.
func (Bridge) Policy() core.Policy { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, filename))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return filepath.Join(dir, filename)
}

// No RequiredFields: `browsers` is OPTIONAL per spec §4.8 and this bridge is
// default-on fleet-wide, so prompting would interrogate every TTY run of every
// node project for a field most of them have no business declaring.

func (Bridge) Render(pf *projectfile.Document, _ core.Options) (core.Output, error) {
	list := browsersOf(pf)
	if len(list) == 0 {
		if isEnvMap(pf) {
			return core.Output{}, fmt.Errorf("%s: requirements.browsers is a per-environment mapping — only the flat sequence form is rendered today", filename)
		}
		// Suppress rather than emit an empty file: an empty .browserslistrc
		// reads as "support nothing", clobbering the tool's defaults.
		genlog.DebugRow("browsers", "(none — unconstrained, file suppressed)", "requirements.browsers", "")
		return core.Output{}, nil
	}
	genlog.DebugRow("browsers", fmt.Sprintf("%d quer(y/ies): %s", len(list), strings.Join(list, ", ")),
		"requirements.browsers", "")
	body := assemble(pf, list)
	return core.Output{Files: map[string][]byte{filename: body}}, nil
}

// assemble emits the REUSE header + marker banner + one query per line. Direct
// byte emit (no template) because the body is purely line-oriented, mirroring
// the ignore assembler.
func assemble(pf *projectfile.Document, list queries) []byte {
	var buf bytes.Buffer
	buf.WriteString(core.REUSEHeader(pf, core.StyleHash))
	buf.WriteString(core.Marker)
	buf.WriteString("\n\n")
	for _, q := range list {
		buf.WriteString(q)
		buf.WriteString("\n")
	}
	return buf.Bytes()
}

// browsersOf normalises requirements.browsers (string-or-sequence) into a flat
// query list. Returns nil when the field is absent or empty.
func browsersOf(pf *projectfile.Document) []string {
	if pf == nil || pf.Requirements == nil {
		return nil
	}
	return projectfile.AsStringList(pf.Requirements.Browsers)
}

// isEnvMap reports whether browsers carries the spec's per-environment mapping
// form (env name → queries). AsStringList yields nothing for it, so without
// this check an unimplemented-but-valid shape would look like "unconstrained"
// and silently drop the user's matrix.
func isEnvMap(pf *projectfile.Document) bool {
	if pf == nil || pf.Requirements == nil {
		return false
	}
	switch pf.Requirements.Browsers.(type) {
	case map[string]any, map[string][]string:
		return true
	}
	return false
}

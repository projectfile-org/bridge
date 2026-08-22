// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package ignore is the key-driven bridge family. One Bridge per ignore
// filename (.gitignore, .dockerignore, .npmignore, .claudeignore,
// .containerignore, .textlintignore, .fdignore); each one assembles a body
// purely from the org.projectfile.ignores namespace — per-target include
// lists, minus that target's excludes, plus the top-level extra list. No
// embedded snippet tree, no stack lookups: every pattern lives in the
// projectfile (and the m6e includes that feed it).
//
// Suppressed vulnerability IDs (CVE/GHSA) are NOT handled here — they live in
// the tool-agnostic org.projectfile.vulnerabilities namespace and fan out to
// scanner ignore files (.trivyignore, .grype.yaml, osv-scanner.toml) via
// projectfile.org/projectfile/bridge/internal/bridge/vulnerabilities.
package ignore

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
	filenameGitignore = ".gitignore"
	extKeyClaude      = "claude"
	stackNode         = "node"
	extKeyGit         = "git"
	extKeyNPM         = "npm"
	extKeyDocker      = "docker"
	extKeyContainer   = "container"
	extKeyTextlint    = "textlint"
	extKeyFd          = "fd"
)

// Bridge implements core.Renderer for one ignore filename. One instance
// per row in `targets` (see register.go), distinguished by filename + extKey.
type Bridge struct {
	filename string
	extKey   string
	optIn    bool
}

func (b Bridge) Name() string             { return b.extKey + "-ignore" }
func (b Bridge) Filename() string         { return b.filename }
func (b Bridge) Aliases() []string        { return nil }
func (b Bridge) Labels() (string, string) { return b.filename, "projectfile" }

// Ignore files carry the marker so the next run overwrites cleanly.
func (Bridge) Policy() core.Policy { return core.Policy{Marker: true} }

func (b Bridge) Stacks() []string {
	if b.filename == ".npmignore" {
		return []string{stackNode}
	}
	return nil
}

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
	if !ext.Generates(b.extKey) {
		return core.Output{}, nil
	}
	// An opt-in target stays silent until its sub-namespace exists. A Renderer
	// CREATES files (unlike a Syncer, which has an Exists gate), so a universal
	// new target would drop one carrying nothing but the `extra` fan-out into
	// every project in the fleet and turn every drift gate red at once.
	if b.optIn && overrideFor(b.extKey, ext) == nil {
		return core.Output{}, nil
	}
	body := assemble(pf, b.filename, b.extKey, ext)
	if len(body) == 0 {
		return core.Output{}, nil
	}
	return core.Output{Files: map[string][]byte{b.filename: body}}, nil
}

// assemble builds the byte body for one target. Layout:
//
//  1. REUSE header (SPDX-FileCopyrightText + SPDX-License-Identifier)
//  2. banner (incl. Marker)
//  3. user [org.projectfile.ignores.<extKey>].include
//  4. user top-level extra
//
// Every pattern traces back to a user-declared entry in the projectfile —
// nothing is emitted without provenance.
func assemble(pf *projectfile.Document, filename, extKey string, ext *pfmodel.IgnoresExtension) []byte {
	var content bytes.Buffer

	o := overrideFor(extKey, ext)
	if inc := o.Patterns(); len(inc) > 0 {
		writeBlock(&content, "user-include", []byte(strings.Join(inc, "\n")+"\n"))
	}
	if ext != nil {
		// `extra` fans out to every target, so the target's own exclude is its
		// only way out of an entry that belongs everywhere else.
		if extra := o.Filter(ext.Extra); len(extra) > 0 {
			writeBlock(&content, "user-extra", []byte(strings.Join(extra, "\n")+"\n"))
		}
	}

	if content.Len() == 0 {
		return nil
	}

	var buf bytes.Buffer
	buf.WriteString(core.REUSEHeader(pf, core.StyleHash))
	buf.WriteString(core.Banner(filename, extKey))
	buf.Write(content.Bytes())
	return buf.Bytes()
}

func writeBlock(buf *bytes.Buffer, label string, body []byte) {
	cleaned := sortDedup(body)
	fmt.Fprintf(buf, "# >>> %s\n", label)
	buf.Write(trimTrailingBlanks(cleaned))
	buf.WriteString("\n")
	fmt.Fprintf(buf, "# <<< %s\n\n", label)
}

// sortDedup sorts non-comment, non-blank lines within a block body and
// removes duplicates. Comment lines and blank lines are preserved in their
// original position relative to the content that follows them.
//
// Re-includes (`!pattern`) are held back and emitted after everything else.
// Every consumer of these files resolves LAST match wins, and `!` sorts below
// every letter, so a sorted-in-place re-include lands above the pattern it
// answers and silently does nothing.
func sortDedup(body []byte) []byte {
	lines := strings.Split(string(body), "\n")

	// Parse into segments: trailing-comments → content-lines.
	// We collect content lines, sort+dedup them, then re-attach comments.
	var result []string
	var comments []string
	var content []string
	var negated []string
	seen := make(map[string]bool)

	flush := func() {
		sort.Strings(content)
		for _, line := range content {
			if seen[line] {
				continue
			}
			seen[line] = true
			if strings.HasPrefix(line, "!") {
				negated = append(negated, line)
				continue
			}
			result = append(result, line)
		}
		content = content[:0]
	}

	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			// Hit a comment/blank — flush accumulated content, then
			// accumulate comments until we see content again.
			if len(content) > 0 {
				flush()
			}
			comments = append(comments, line)
		} else {
			if len(comments) > 0 {
				result = append(result, comments...)
				comments = comments[:0]
			}
			content = append(content, t)
		}
	}
	// Flush any remaining content after last comment group.
	if len(comments) > 0 {
		result = append(result, comments...)
	}
	flush()

	if len(negated) > 0 {
		// The body's own trailing newline would otherwise split the block in two.
		for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
			result = result[:len(result)-1]
		}
		sort.Strings(negated)
		result = append(result, negated...)
	}

	return []byte(strings.Join(result, "\n"))
}

func trimTrailingBlanks(body []byte) []byte {
	for len(body) > 0 {
		c := body[len(body)-1]
		if c != '\n' && c != ' ' && c != '\t' {
			break
		}
		body = body[:len(body)-1]
	}
	return body
}

// overrideFor returns the per-target include/exclude overrides. The
// pfmodel.IgnoresExtension struct carries one typed slot per registered
// target (git, docker, npm, claude, container, textlint, fd); explicit named fields are kept
// (over a map) to mirror the rest of the struct and stay greppable.
// Vulnerability IDs are NOT handled here — they live in
// org.projectfile.vulnerabilities (see bridge/vulnerabilities) and fan out to
// scanner ignore files tool-agnostically.
func overrideFor(extKey string, ext *pfmodel.IgnoresExtension) *pfmodel.IgnoreTargetOverride {
	if ext == nil {
		return nil
	}
	switch extKey {
	case extKeyGit:
		return ext.Git
	case extKeyDocker:
		return ext.Docker
	case extKeyNPM:
		return ext.Npm
	case extKeyClaude:
		return ext.Claude
	case extKeyContainer:
		return ext.Container
	case extKeyTextlint:
		return ext.Textlint
	case extKeyFd:
		return ext.Fd
	}
	return nil
}

func includesFor(extKey string, ext *pfmodel.IgnoresExtension) []string {
	return overrideFor(extKey, ext).Patterns()
}

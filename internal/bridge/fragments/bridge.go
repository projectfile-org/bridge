// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package fragments is a derive-only Renderer that assembles Markdown
// documents from docs/<name>.d/*.md fragment directories, plus one cached copy
// per upstream parent. Each document declared under
// org.projectfile.fragments.documents becomes one artefact (e.g.
// docs/features.d → FEATURES.md). It replaces the former Ruby features-md
// CLI, generalised to any number of assembled documents.
//
// Assembly reads local files only. Parents are forge URLs, and reading them is
// a separate opt-in step (--refresh) that writes each parent's published
// document into docs/<name>.d/.inherited/ with the version it was read at. That
// split is what lets the drift gate run offline and deterministically while the
// inherited text still names a real upstream version instead of implying it is
// current.
package fragments

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// templateName is the single structural template every document renders
// through. Fragments are data, not prose, so one template suffices; per
// language variants (fragments.es.md.tmpl) can be added later if needed.
const (
	templateName = "fragments.md.tmpl"
	bridgeName   = "fragments"
)

// Bridge renders every org.projectfile.fragments document. It registers
// one Bridge (Filename "fragments") whose Render returns a multi-file
// Output — one entry per declared document — mirroring how the license
// bridge emits LICENSE plus LICENSES/*.txt from one registration.
type Bridge struct{}

func (Bridge) Name() string             { return bridgeName }
func (Bridge) Filename() string         { return bridgeName }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return bridgeName, "projectfile" }

// Policy is empty: assembled documents are pure generated artefacts with
// no user-edit expectation, so the dispatcher always overwrites.
func (Bridge) Policy() core.Policy { return core.Policy{} }

func (Bridge) Exists(_ string) bool {
	// The bridge has no single canonical file (outputs are config-driven),
	// so existence is reported false; RunRender creates files as needed.
	return false
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return filepath.Join(dir, bridgeName)
}

// Render assembles every declared document into one multi-file Output.
// Documents resolve with this precedence:
//  1. Explicit override: org.projectfile.fragments.documents — used verbatim
//     when present (escape hatch for non-default behaviour).
//  2. Conventions defaults: org.projectfile.conventions.fragments — shells
//     (dir/out/title, supplied by the b19 conventions include) combined with
//     the flat per-project Parents list, applied to every shell.
//
// Returns an empty Output (no-op) when neither yields documents — RunRender
// then logs "bridge: fragments (no-op)".
//
// A document with no own AND no inherited fragments is skipped (no file
// emitted): the markdown linter rejects an empty `# Title` section, so a
// declared-but-unpopulated shell (e.g. roadmap until docs/roadmap.d/ exists)
// must produce nothing rather than a stub.
func (Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	docs, err := resolveDocuments(pf)
	if err != nil {
		return core.Output{}, err
	}
	if len(docs) == 0 {
		return core.Output{}, nil
	}

	reuse := core.REUSEHeader(pf, core.StyleHTML)
	out := core.Output{Files: map[string][]byte{}}
	for _, doc := range docs {
		// Refreshed copies are emitted AND fed straight into the assembly, so one
		// run cannot write a new copy while assembling from the previous one.
		var refreshed map[string]inheritedCopy
		if opts.Refresh {
			refreshed = refreshParents(doc, opts)
			for name, copied := range refreshed {
				rel := path.Join(doc.Dir, inheritedDir, name+".md")
				out.Files[rel] = renderInherited(copied)
				genlog.Plain("bridge: " + rel)
			}
		}

		body, empty, err := assembleDocument(opts.Dir, doc, reuse, refreshed)
		if err != nil {
			return core.Output{}, fmt.Errorf("%s: %w", doc.Out, err)
		}
		if empty {
			genlog.Plain(fmt.Sprintf("bridge: %s (skipped, no fragments)", doc.Out))
			continue
		}
		out.Files[doc.Out] = body
		genlog.Plain(fmt.Sprintf("bridge: %s", doc.Out))
	}
	return out, nil
}

// resolveDocuments applies the override-then-conventions precedence and
// returns the concrete documents to render. Override wins entirely when
// declared — conventions defaults do NOT merge into an override.
func resolveDocuments(pf *projectfile.Document) ([]pfmodel.FragmentDocument, error) {
	// 1. Explicit override.
	if ext, err := pfmodel.GetFragmentsExtension(pf); err != nil {
		return nil, err
	} else if ext != nil && len(ext.Documents) > 0 {
		genlog.Info("fragments: using explicit override", "count", len(ext.Documents))
		return ext.Documents, nil
	}

	// 2. Conventions-driven defaults: shells from the include + flat parents.
	conv, err := pfmodel.GetConventionsFragments(pf)
	if err != nil || conv == nil {
		return nil, err
	}
	out := make([]pfmodel.FragmentDocument, 0, len(conv.Documents))
	for _, shell := range conv.Documents {
		// Each shell carries dir/out/title; the flat Parents list is shared
		// across every document (features and roadmap inherit the same chain).
		shell.Parents = conv.Parents
		out = append(out, shell)
	}
	if len(out) > 0 {
		genlog.Info("fragments: using conventions defaults", "documents", len(out), "parents", len(conv.Parents))
	}
	return out, nil
}

// assembleDocument builds one artefact: own fragments under a Project section,
// then one section per parent, rendered through the template with the SPDX
// header prepended. refreshed carries the copies just read from upstream and
// wins over the copy on disk for the same parent; parents absent from it keep
// what was committed, which is how a failed or offline refresh degrades.
//
// empty is true when the document has neither own nor inherited entries — the
// caller skips emitting such files to avoid empty-section lint failures.
func assembleDocument(projectDir string, doc pfmodel.FragmentDocument, reuse string, refreshed map[string]inheritedCopy) (body []byte, empty bool, err error) {
	own, err := loadFragments(projectDir, doc.Dir)
	if err != nil {
		return nil, false, err
	}
	cached, err := loadInherited(projectDir, doc.Dir)
	if err != nil {
		return nil, false, err
	}
	for name, copied := range refreshed {
		cached[name] = copied
	}
	inherited := orderedCopies(declaredOnly(cached, doc))

	if len(own) == 0 && len(inherited) == 0 {
		return nil, true, nil
	}

	view := fragmentView{
		REUSEHeader:      reuse,
		Title:            doc.Title,
		HasProject:       len(own) > 0,
		ProjectHeading:   "## Project " + doc.Title,
		ProjectFragments: own,
		Inherited:        inherited,
	}
	// core.Render resolves project-local template overrides under projectDir,
	// then the embedded template — same dir the fragments were loaded from.
	out, err := core.Render(projectDir, templateName, view)
	if err != nil {
		return nil, false, err
	}
	// The template's per-fragment range leaves a trailing blank line after
	// every fragment — including each section's last — so section boundaries
	// stack into 3+ newlines. Collapse every run of 3+ newlines to exactly
	// one blank line (markdownlint MD012), then pin a single trailing newline
	// (MD047). Single blank lines between paragraphs/headings are preserved.
	collapsed := multiBlankRE.ReplaceAllString(string(out), "\n\n")
	return []byte(strings.TrimRight(collapsed, "\n") + "\n"), false, nil
}

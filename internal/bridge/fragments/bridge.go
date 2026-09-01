// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package fragments is a derive-only Renderer that assembles Markdown
// documents from docs/<name>.d/*.md fragment directories, plus one section per
// upstream parent. Each document declared under
// org.projectfile.fragments.documents becomes one artefact (e.g.
// docs/features.d → FEATURES.md). It replaces the former Ruby features-md
// CLI, generalised to any number of assembled documents.
//
// Inherited sections carry no cache of their own: a writing run online reads
// the parents' published documents straight off their newest release tag, and
// every other run — offline, check, preview, dry-run, or a generate whose
// fetch failed — reads the sections back out of the committed assembled
// document. That split keeps the drift gate offline and byte-stable; the
// section heading names the parent only, never the version it was read at —
// a version there rewrites every child document on each parent release.
// Regenerating is how upstream changes land.
//
// Documents localize per org.projectfile.i18n: the canonical file assembles
// from docs/<name>.d/ as before, and each other declared language assembles
// from docs/<lang>/<name>.d/ into docs/<lang>/<Out>. A language with no
// translated fragments and no language-specific inherited sections renders
// nothing under a localized name (warned). The inherited sections localize
// from the parent's own docs/<lang>/<Out> and fall back to the canonical copy
// for a parent that publishes no such language, since a child cannot translate
// text it does not own.
package fragments

import (
	"fmt"
	"path/filepath"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// templateName is the single structural template every document and every
// language renders through. Fragments are data, not prose; the strings the
// assembler itself contributes (title, section headings) localize from the
// strings table, so no per-language template exists.
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

// Describe answers the dispatcher probe: "fragments" names no file — the
// outputs are whatever documents org.projectfile.fragments declares.
func (Bridge) Describe() string {
	return "FEATURES.md, ROADMAP.md, … assembled from docs/<name>.d fragments (one-way render)"
}

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
// A document with no own fragments and no inherited sections is skipped (no
// file emitted): the markdown linter rejects an empty `# Title` section, so a
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

	langs := pfmodel.Languages(pf)
	defLang := pfmodel.DefaultLanguage(pf)
	reuse := core.REUSEHeader(pf, core.StyleHTML)
	out := core.Output{Files: map[string][]byte{}}
	for _, doc := range docs {
		own, err := loadFragments(opts.Dir, doc.Dir)
		if err != nil {
			return core.Output{}, fmt.Errorf("%s: %w", doc.Out, err)
		}

		canonical, resolveVariants := sectionsFor(opts.Dir, doc, opts, langs, defLang)
		if len(own) == 0 && len(canonical) == 0 {
			genlog.Plain(fmt.Sprintf("bridge: %s (skipped, no fragments)", doc.Out))
			continue
		}
		variants := resolveVariants()

		body, err := assembleDocument(opts.Dir, defLang, "", defLang, doc, own, canonical, variants, reuse)
		if err != nil {
			return core.Output{}, fmt.Errorf("%s: %w", doc.Out, err)
		}
		out.Files[doc.Out] = body
		genlog.Plain(fmt.Sprintf("bridge: %s", doc.Out))

		for _, v := range variants {
			rel := core.LocalizedFilename(doc.Out, v.Lang)
			body, err := assembleDocument(opts.Dir, v.Lang, v.Lang, defLang, doc, v.Fragments, v.Inherited, variants, reuse)
			if err != nil {
				return core.Output{}, fmt.Errorf("%s: %w", rel, err)
			}
			out.Files[rel] = body
			genlog.Plain(fmt.Sprintf("bridge: %s", rel))
		}
	}
	return out, nil
}

// sectionsFor resolves one document's inherited sections for the canonical
// render, and returns a closure that resolves every variant lazily. A writing
// run online fetches the parents' published documents; every other run
// re-reads the sections verbatim from the committed document, which keeps the
// drift gate offline and byte-stable. A fetch that cannot read every declared
// parent degrades the whole document to its committed sections — a forge
// outage preserves content instead of deleting it, and never mixes fresh and
// stale sections in one file.
//
// The variant resolver is returned unevaluated rather than run here: it is
// the only path that logs "no fragments for language", and a caller must
// know the document actually has canonical content (own fragments or
// inherited sections) before that warning means anything. Evaluating it
// eagerly would warn about missing translations for a document that is not
// declared in any language — Render skips those silently.
func sectionsFor(projectDir string, doc pfmodel.FragmentDocument, opts core.Options, langs []string, defLang string) ([]inheritedEntry, func() []variantFragments) {
	none := func(string) ([]inheritedEntry, bool) { return nil, false }
	if len(doc.Parents) == 0 {
		return nil, func() []variantFragments { return resolveVariantLangs(projectDir, doc, langs, none) }
	}
	if !opts.Offline && !opts.DryRun {
		if copies, localized, ok := fetchParents(doc, langs); ok {
			ordered := orderedCopies(copies)
			return localizedInherited(ordered, defLang), func() []variantFragments {
				return resolveVariantLangs(projectDir, doc, langs, func(lang string) ([]inheritedEntry, bool) {
					merged := overCanonical(ordered, localized[lang], lang, doc.Out)
					return localizedInherited(merged, lang), len(localized[lang]) > 0
				})
			}
		}
	}
	canonical, _ := extractInherited(projectDir, doc.Out, localizedProjectHeading(doc.Title, doc.Out, defLang))
	return canonical, func() []variantFragments {
		return resolveVariantLangs(projectDir, doc, langs, func(lang string) ([]inheritedEntry, bool) {
			sections, existed := extractInherited(projectDir, core.LocalizedFilename(doc.Out, lang), localizedProjectHeading(doc.Title, doc.Out, lang))
			if existed {
				return sections, true
			}
			// No committed variant document: nest the canonical sections under
			// localized headings — the offline edition of the fallback a parent
			// that publishes no such language gets online.
			return localizedFromCanonical(canonical, defLang, lang), false
		})
	}
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

// assembleDocument builds one artefact through the structural template: own
// fragments under a Project section, then one section per parent. strLang
// resolves the assembler's structural strings (the default language for the
// canonical render, the variant's own tag otherwise); barLang is the render
// sentinel the cross-language bar keys off. Variants nest the SAME inherited
// sections as the canonical file — they quote upstream, which publishes one
// language, and a variant that dropped them would understate the project.
func assembleDocument(projectDir, strLang, barLang, defLang string, doc pfmodel.FragmentDocument, own []Fragment, inherited []inheritedEntry, variants []variantFragments, reuse string) ([]byte, error) {
	view := fragmentView{
		REUSEHeader:      reuse,
		Title:            localizedDocTitle(doc.Title, doc.Out, strLang),
		HasProject:       len(own) > 0,
		ProjectHeading:   localizedProjectHeading(doc.Title, doc.Out, strLang),
		ProjectFragments: own,
		Inherited:        inherited,
	}
	// core.Render resolves project-local template overrides under projectDir,
	// then the embedded template — same dir the fragments were loaded from.
	out, err := core.Render(projectDir, templateName, view)
	if err != nil {
		return nil, err
	}
	// Post-processing pipeline, same order every render: collapse the runs the
	// template's per-fragment range leaves (MD012), set the cross-language bar
	// above the H1, wrap the non-English body for the textlint terminology
	// rule (after the REUSE header, which must stay the file's first block),
	// and collapse once more for the seams those steps add.
	body := core.CollapseBlankLines(out)
	body = core.InsertLanguageBar(body, doc.Out, barLang, defLang, variantLangTags(variants))
	body = wrapAfterHeader(body, strLang)
	return core.CollapseBlankLines(body), nil
}

// variantLangTags flattens the resolved variant set for the language bar.
func variantLangTags(variants []variantFragments) []string {
	if len(variants) == 0 {
		return nil
	}
	tags := make([]string, 0, len(variants))
	for _, v := range variants {
		tags = append(tags, v.Lang)
	}
	return tags
}

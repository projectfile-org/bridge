// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// The localization contract shared by every community health file.
//
// A localizable artefact is one canonical file (README.md, CONTRIBUTING.md, …)
// written in the default language (org.projectfile.i18n.default-language,
// "en" by default) at the repo root, plus one variant per other language
// declared in org.projectfile.i18n.languages. Each variant keeps the canonical
// basename and lives under docs/<lang>/ — docs/es/CONTRIBUTING.md — so the
// locale is in the directory, not the filename. This keeps the repo root clean
// when many locales ship. The variant's body comes from a template carrying
// the BCP 47 infix before the extension (CONTRIBUTING.es.md.tmpl); only OUTPUT
// moves under docs/<lang>/, template lookup is unchanged.
//
// When default-language is not "en", the English copies move under docs/en/
// and the root files render in the declared default language.
//
// Translations are prose, so they cannot be derived: a language with no
// template is SKIPPED with a warning rather than emitting the default-language
// body under a localized name. A half-translated file lies to its reader; an
// absent one merely does not exist yet.

// Canonical community health filenames. These bridges cross-reference each
// other's output — SUPPORT.md points at SECURITY.md, CONTRIBUTING.md points
// at SUPPORT.md, README.md links them all — and every such reference has to
// be resolved per language, so the names get one home here rather than being
// re-spelled as literals in each bridge and template.
const (
	FileReadme        = "README.md"
	FileContributing  = "CONTRIBUTING.md"
	FileCodeOfConduct = "CODE_OF_CONDUCT.md"
	FileDEI           = "DEI.md"
	FileSecurity      = "SECURITY.md"
	FileSupport       = "SUPPORT.md"
	FileAIPolicy      = pfmodel.DefaultAIPolicyFile
)

// langLabels maps a BCP 47 tag to the language's own name (endonym) — what a
// speaker of that language recognizes at a glance in the bar. The canonical
// file is labelled with its default language's endonym. A tag without an entry
// falls back to its upper-cased code so an undeclared locale still renders
// something rather than empty; add endonyms here as new locales land.
var langLabels = map[string]string{
	"en": "English",
	"es": "Español",
	"uk": "Українська",
}

// langBarSeparator joins entries in the cross-language bar.
const langBarSeparator = " · "

// LangLink is one entry in the cross-language link bar: where to find this
// same document in another language.
type LangLink struct {
	Code     string // BCP 47 tag; empty for the canonical file
	Label    string // display text — the upper-cased tag, endonym for canonical
	Filename string // repo-relative target, e.g. "docs/es/CONTRIBUTING.md"
}

// LocalizedDir is the directory every non-default-language variant writes to.
// The locale lives in the directory, not the filename, so a repo with many
// locales keeps a clean root: README.es.md became docs/es/README.md.
const LocalizedDir = "docs"

// ResolveLang turns a render-sentinel language tag into a concrete BCP 47 tag.
// The empty string is the canonical root-file render: it resolves to the
// project's default language (org.projectfile.i18n.default-language, "en" by
// default). Any non-empty tag passes through unchanged. Bridges call this at
// the boundary where the render loop's "" sentinel meets a string resolver
// (the message catalog, ExtractLocalizedStringForLang) so a Spanish-first
// project's root file resolves Spanish strings rather than English.
func ResolveLang(lang string, pf *projectfile.Document) string {
	if lang == "" {
		return pfmodel.DefaultLanguage(pf)
	}
	return lang
}

// LocalizedFilename maps a canonical filename and a language to the variant's
// repo-relative path. The empty language is the canonical (default-language)
// file at the root and passes through unchanged. A non-empty language places
// the variant under docs/<lang>/, keeping the canonical basename:
// ("CONTRIBUTING.md", "es") → "docs/es/CONTRIBUTING.md".
func LocalizedFilename(base, lang string) string {
	if lang == "" {
		return base
	}
	return LocalizedDir + "/" + lang + "/" + base
}

// LocalizedTemplateName is the template backing a variant. Template names keep
// the language infix before the extension regardless of the output directory:
// ("CONTRIBUTING.md", "es") → "CONTRIBUTING.es.md.tmpl". The canonical
// (empty-lang) case is the base name + ".tmpl", unchanged from the
// LocalizedFilename passthrough — only OUTPUT relocated under docs/<lang>/,
// template lookup did not.
func LocalizedTemplateName(base, lang string) string {
	return LocalizedTemplateInfix(base, lang) + ".tmpl"
}

// LocalizedTemplateInfix maps a canonical filename and a language to the
// template-name base (without the ".tmpl" suffix): the canonical file for the
// empty language, or the infix form for a variant. ("CONTRIBUTING.md", "")
// → "CONTRIBUTING.md"; ("CONTRIBUTING.md", "es") → "CONTRIBUTING.es.md".
func LocalizedTemplateInfix(base, lang string) string {
	if lang == "" {
		return base
	}
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + "." + lang + ext
}

// HasTemplate reports whether a template is resolvable — as a project-local
// override under dir, or as an embedded template some bridge registered at
// init(). This is the probe RenderLocalized uses to decide whether a declared
// language has a translation; Render itself errors on a miss, which is the
// wrong answer when the miss is expected.
func HasTemplate(dir, name string) bool {
	if dir != "" {
		if _, ok, err := ReadLocalTemplate(dir, filepath.Join(LocalTemplatesDir, name)); err == nil && ok {
			return true
		}
	}
	_, registered := loaders[name]
	return registered
}

// LocalizedSibling resolves a cross-reference to another community health
// file from inside a localized document: docs/es/SUPPORT.md links
// docs/es/SECURITY.md, not the root SECURITY.md, so a reader who arrived in
// their own language stays in it.
//
// It cannot verify the sibling: the §5 binary split means each pf-bridge-*
// links only its own bridge, so SECURITY's templates are invisible from
// inside pf-bridge-support. The declared language set is the contract
// instead — a project that declares `es` is asking for Spanish across every
// health file, and the sibling bridge warns by name about any translation it
// is missing. Callers only pass a lang that survived translatableLangs for
// their own file, so the canonical case is never guessed at.
func LocalizedSibling(base, lang string) string {
	return LocalizedFilename(base, lang)
}

// LanguageLinks builds the cross-language bar for one render: every language
// variant of base EXCEPT the active one, the canonical (default-language) file
// included. defLang is the project's default language; it labels the canonical
// entry with its endonym. Returns nil when no extra languages are configured,
// so a single-language project never grows a bar pointing at itself.
//
// Each entry's target is rebased relative to the active document's own path
// (docs/es/X.md links ../../X.md for the root file and ../uk/X.md for a
// sibling variant), so the bar resolves from whichever directory the reader
// arrived in.
func LanguageLinks(base, active, defLang string, langs []string) []LangLink {
	if len(langs) == 0 {
		return nil
	}
	docPath := LocalizedFilename(base, active)
	all := append([]string{""}, langs...)
	out := make([]LangLink, 0, len(all))
	for _, lang := range all {
		if lang == active {
			continue
		}
		out = append(out, LangLink{
			Code:     lang,
			Label:    langLabel(lang, defLang),
			Filename: RelLink(LocalizedFilename(base, lang), docPath),
		})
	}
	return out
}

// langLabel is the bar's display text for a language tag. The empty tag (the
// canonical root file) is labelled with the default language's endonym, so a
// Spanish-first project's root file reads "Español" rather than "English".
func langLabel(lang, defLang string) string {
	if lang == "" {
		if label, ok := langLabels[defLang]; ok {
			return label
		}
		return strings.ToUpper(defLang)
	}
	if label, ok := langLabels[lang]; ok {
		return label
	}
	return strings.ToUpper(lang)
}

// h1Prefix is the ATX level-1 heading every generated health file opens with.
var h1Prefix = []byte("# ")

// InsertLanguageBar places the cross-language bar immediately above the
// document's first H1 — after any managed marker or HTML comment the template
// emitted, and above the title where a reader looking for their own language
// finds it first. A body with no H1 (degenerate, but never a reason to drop
// the bar) takes it at the very top.
//
// defLang labels the canonical entry with its endonym. Doing this here rather
// than in each template is what keeps the translated templates pure prose: a
// new locale is one file, with no bar markup to forget.
func InsertLanguageBar(body []byte, base, active, defLang string, langs []string) []byte {
	links := LanguageLinks(base, active, defLang, langs)
	if len(links) == 0 {
		return body
	}
	var bar strings.Builder
	for i, l := range links {
		if i > 0 {
			bar.WriteString(langBarSeparator)
		}
		bar.WriteString("[" + l.Label + "](" + l.Filename + ")")
	}
	bar.WriteString("\n\n")

	at := h1Offset(body)
	genlog.Decision("language_bar", strings.TrimSpace(bar.String()), "org.projectfile.i18n.languages", base)
	out := make([]byte, 0, len(body)+bar.Len())
	out = append(out, body[:at]...)
	out = append(out, bar.String()...)
	return append(out, body[at:]...)
}

// h1Offset returns the byte offset of the first line that opens an H1, or 0
// when the body has none.
func h1Offset(body []byte) int {
	if bytes.HasPrefix(body, h1Prefix) {
		return 0
	}
	if i := bytes.Index(body, append([]byte("\n"), h1Prefix...)); i >= 0 {
		return i + 1
	}
	return 0
}

// LocalizedSpec describes one localizable derive-only artefact. Bridges hand
// it to RenderLocalized instead of driving Render themselves, so the naming
// rule, the missing-translation policy, the cross-language bar and the final
// assembly live in exactly one place.
type LocalizedSpec struct {
	// Filename is the on-disk name, e.g. "CONTRIBUTING.md".
	Filename string
	// Template is the canonical name the templates are keyed by, when it
	// differs from Filename. A document that renames its output (the AI
	// policy names its own file) still renders from the one template shipped
	// for it — the name a project chose says nothing about which prose to use.
	// Empty means Filename is also the template name.
	Template string
	// Langs are the extra languages from org.projectfile.i18n.languages.
	Langs []string
	// View returns the template data for one language, called once per
	// rendered language so localized-strings resolve in that language.
	View func(lang string) any
	// SeeAlso returns the project-authored "See also" links for one rendered
	// language (nil, or a nil func, means no section — no orphan heading).
	// Called once per language because a link's label may itself be
	// localized. Typically SeeAlsoFor(pf, <bridge Name()>, lang).
	SeeAlso func(lang string) []SeeAlsoLink
	// HowToLink gates the "Generated from projectfile (learn how)" footer.
	// Off by default — the caller sets it from its own namespace's
	// how-to-link field (e.g. [org.projectfile.contributing].how-to-link).
	HowToLink bool
}

// RenderLocalized renders the canonical file plus one variant per declared
// language. A language whose template is missing is skipped with a warning
// naming the file the project would have to add; the canonical file always
// renders, so a bad locale list can never block regenerating the rest.
//
// The canonical (root) render resolves its strings in the project's default
// language, so a Spanish-first project's root file reads in Spanish.
func RenderLocalized(pf *projectfile.Document, spec LocalizedSpec, opts Options) (Output, error) {
	// Resolve which languages actually render BEFORE rendering any of them:
	// the cross-language bar goes into every variant, so a language dropped
	// for a missing template must be dropped from the bar too — otherwise
	// each file advertises a translation that was never written.
	tmplBase := spec.Template
	if tmplBase == "" {
		tmplBase = spec.Filename
	}
	translated := translatableLangs(tmplBase, spec.Langs, opts.Dir)
	defLang := pfmodel.DefaultLanguage(pf)

	// Every file here is generated, so every file says so. The sentinel tells a
	// human not to edit what the next run overwrites — that warning is needed
	// whatever the write policy is, and the policy gate is a separate concern.
	header := []byte(ManagedREUSEHeader(pf))
	out := Output{Files: map[string][]byte{}}
	for _, lang := range append([]string{""}, translated...) {
		tmpl := LocalizedTemplateName(tmplBase, lang)
		// The View receives the render sentinel ("" for the canonical root
		// render), NOT a resolved tag: path- producing code inside the view
		// (LocalizedSibling output keys) must keep "" so the default language
		// renders at the root, not under docs/<defLang>/. String resolution
		// (catalog, LocalizedString) maps "" → defLang itself via ResolveLang.
		body, err := Render(opts.Dir, tmpl, spec.View(lang))
		if err != nil {
			return Output{}, err
		}
		genlog.Decision("rendered", LocalizedFilename(spec.Filename, lang), tmpl, "lang="+langLabel(lang, defLang))
		body = InsertLanguageBar(body, spec.Filename, lang, defLang, translated)
		// See-also comes before the footer: it is document content (cross-
		// references), the footer is trailing meta-prose about the generator.
		if spec.SeeAlso != nil {
			if block := RenderSeeAlso(spec.SeeAlso(lang)); block != nil {
				body = append(body, []byte("\n\n")...)
				body = append(body, block...)
			}
		}
		// The footer is appended before the textlint wrap so its localized
		// copy sits inside the terminology-disable block for non-English. The
		// trailing \n keeps the single-newline contract (MD047) for the
		// English root render, which skips the wrap that would otherwise add it.
		// The footer's how-to slug follows the CANONICAL name, not the one the
		// project chose: a renamed file is the same document and earns the
		// same how-to, where the output name would only miss the map.
		// Off by default — HowToLink opts in per document type.
		if spec.HowToLink {
			body = append(body, []byte("\n\n"+GeneratedFooter(tmplBase, ResolveLang(lang, pf))+"\n")...)
		}
		// ResolveLang maps the canonical render sentinel ("" ) to the default
		// language, so a Spanish-default project's Spanish root file is wrapped
		// while its docs/en/ variant is not. Wraps after the language bar so the
		// endonym labels (Español, Українська) are protected too.
		body = WrapLocalizedTextlint(body, ResolveLang(lang, pf))
		out.Files[LocalizedFilename(spec.Filename, lang)] = append(append([]byte{}, header...), CollapseBlankLines(body)...)
	}
	return out, nil
}

// translatableLangs filters a declared language list down to the ones that
// have a template for this file, warning once per drop with the exact path
// the project would have to add.
func translatableLangs(filename string, langs []string, dir string) []string {
	out := make([]string, 0, len(langs))
	for _, lang := range langs {
		tmpl := LocalizedTemplateName(filename, lang)
		if !HasTemplate(dir, tmpl) {
			genlog.Warn("no template for language — file skipped",
				"file", LocalizedFilename(filename, lang),
				"lang", lang,
				"hint", "add "+filepath.Join(LocalTemplatesDir, tmpl))
			continue
		}
		out = append(out, lang)
	}
	return out
}

var (
	multiBlankRe   = regexp.MustCompile(`\n{3,}`)
	trailingBlanks = regexp.MustCompile(`\n*\z`)
)

// CollapseBlankLines normalizes generated markdown: runs of blank lines
// collapse to one and the body ends in exactly one newline. Templates full of
// conditional blocks leave ragged whitespace behind; every renderer wants the
// same cleanup, so it lives here rather than once per bridge.
func CollapseBlankLines(body []byte) []byte {
	out := multiBlankRe.ReplaceAllLiteral(body, []byte("\n\n"))
	return trailingBlanks.ReplaceAllLiteral(out, []byte("\n"))
}

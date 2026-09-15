// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"bytes"
	"path"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Localized fragment documents. The default-language fragments stay where
// they always were (docs/features.d/) and each other declared language reads
// its own translations from docs/<lang>/features.d/ — the locale lives in the
// directory, like every other localized artefact. Only OUTPUT relocated; the
// assembled variant keeps the canonical basename under docs/<lang>/.
//
// A language is a whole-document translation, same contract as the health
// files: a locale dir with no fragments renders nothing under a localized
// name, and the warning names the dir to create. Inherited sections localize
// from the parent's own docs/<lang>/<Out> and fall back to the canonical copy
// for a parent that publishes no such language: the child cannot translate
// text it does not own. The readme bridge's features block carries the
// reader-facing fallback (English bullets plus a not-yet-translated note), so
// the two halves together degrade honestly.

// localizedFragDir maps a document's fragment dir to its per-language
// variant: docs/features.d + es → docs/es/features.d. Dirs outside docs/ have
// no locale slot in the layout and are not localized (the caller checks).
func localizedFragDir(dir, lang string) string {
	rest := strings.TrimPrefix(dir, core.LocalizedDir+"/")
	return path.Join(core.LocalizedDir, lang, rest)
}

// docLocalizable reports whether a document's variants have a docs/<lang>/
// slot in the layout: a bare out filename (localized outputs keep the
// canonical basename under docs/<lang>/) and a fragment dir under docs/.
// Custom override documents outside the convention stay single-language by
// decision, not by error.
func docLocalizable(doc pfmodel.FragmentDocument) bool {
	return !strings.Contains(doc.Out, "/") && strings.HasPrefix(doc.Dir, core.LocalizedDir+"/")
}

// variantFragments is one declared language's render set: its own translated
// fragments and the inherited sections that variant nests.
type variantFragments struct {
	Lang      string
	Fragments []Fragment
	Inherited []inheritedEntry
}

// resolveVariantLangs probes every declared language for content it can ship:
// its own translated fragments, or inherited sections specific to that
// language. A language with neither renders nothing under a localized name —
// never faked — and the warning names the dir to add. sectionsFor resolves a
// language's inherited sections and reports whether any of them is specific to
// the language; the source — live fetch or committed document — is the
// caller's choice, existence is decided here and only here.
func resolveVariantLangs(projectDir string, doc pfmodel.FragmentDocument, langs []string, sectionsFor func(lang string) ([]inheritedEntry, bool)) []variantFragments {
	if len(langs) == 0 {
		return nil
	}
	if !docLocalizable(doc) {
		genlog.DebugRow("fragments_localization", doc.Out, doc.Dir, "skipped (dir outside docs/ or nested out)")
		return nil
	}
	out2 := make([]variantFragments, 0, len(langs))
	for _, lang := range langs {
		frags, err := loadFragments(projectDir, localizedFragDir(doc.Dir, lang))
		if err != nil {
			genlog.Warn("unreadable localized fragment dir — language skipped",
				"dir", localizedFragDir(doc.Dir, lang), "error", err.Error())
			continue
		}
		sections, localizedContent := sectionsFor(lang)
		if len(frags) == 0 && !localizedContent {
			genlog.Warn("no fragments for language — file skipped",
				"file", core.LocalizedFilename(doc.Out, lang),
				"lang", lang,
				"hint", "add "+localizedFragDir(doc.Dir, lang)+"/<feature>.md")
			continue
		}
		out2 = append(out2, variantFragments{
			Lang:      lang,
			Fragments: frags,
			Inherited: sections,
		})
	}
	return out2
}

// overCanonical layers one language's fetched localized copies over the
// fetched canonical set: a parent that publishes the language reads
// translated, one that does not falls back to the body it does publish — the
// child cannot translate text it does not own.
func overCanonical(canonical []inheritedCopy, localized map[string]inheritedCopy, lang, document string) []inheritedCopy {
	if len(canonical) == 0 && len(localized) == 0 {
		return nil
	}
	merged := make(map[string]inheritedCopy, len(canonical)+len(localized))
	for _, c := range canonical {
		merged[slug(c.Name)] = c
	}
	for name, copied := range localized {
		merged[name] = copied
	}
	for _, c := range canonical {
		if _, translated := localized[slug(c.Name)]; !translated {
			genlog.Debug("fragments: parent publishes no localized document, nesting the canonical copy",
				"parent", c.Name, "lang", lang, "document", document)
		}
	}
	return orderedCopies(merged)
}

// reuseClose is the boundary marker of the leading REUSE comment the template
// emits first; the localized wrap inserts after it.
var reuseClose = []byte("\n-->")

// wrapAfterHeader applies the textlint terminology wrap to a rendered body
// for non-English renders. The leading REUSE comment must stay the file's
// first block (REUSE compliance), so the disable directive goes after it —
// the same file shape the health-file renders produce.
func wrapAfterHeader(body []byte, lang string) []byte {
	if lang == "en" || len(body) == 0 {
		return body
	}
	head, rest := "", bytes.TrimSpace(body)
	if cut := bytes.Index(body, reuseClose); cut >= 0 {
		end := cut + len(reuseClose)
		head = string(body[:end]) + "\n\n"
		rest = bytes.TrimSpace(body[end:])
	}
	wrapped := core.WrapLocalizedTextlint(rest, lang)
	// WrapLocalizedTextlint joins the disable comment to the content with a
	// single newline; re-open that seam into the blank line every generated
	// localized file carries around the directives.
	if c := bytes.IndexByte(wrapped, '\n'); c >= 0 {
		wrapped = append(wrapped[:c+1:c+1], append([]byte("\n"), wrapped[c+1:]...)...)
	}
	return append([]byte(head), wrapped...)
}

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
)

// Localized fragment documents. The default-language fragments stay where
// they always were (docs/features.d/) and each other declared language reads
// its own translations from docs/<lang>/features.d/ — the locale lives in the
// directory, like every other localized artefact. Only OUTPUT relocated; the
// assembled variant keeps the canonical basename under docs/<lang>/.
//
// A language is a whole-document translation, same contract as the health
// files: a locale dir with no fragments renders nothing under a localized
// name, and the warning names the dir to create. The readme bridge's features
// block carries the reader-facing fallback (English bullets plus a
// not-yet-translated note), so the two halves together degrade honestly.

// localizedFragDir maps a document's fragment dir to its per-language
// variant: docs/features.d + es → docs/es/features.d. Dirs outside docs/ have
// no locale slot in the layout and are not localized (the caller checks).
func localizedFragDir(dir, lang string) string {
	rest := strings.TrimPrefix(dir, core.LocalizedDir+"/")
	return path.Join(core.LocalizedDir, lang, rest)
}

// A document is localizable when its out is a bare filename (localized
// outputs live under docs/<lang>/ keeping the canonical basename) and its
// fragment dir sits under docs/. Custom override documents outside the
// convention stay single-language by decision, not by error.

// variantFragments is one declared language's own translated fragments.
type variantFragments struct {
	Lang      string
	Fragments []Fragment
}

// resolveVariantLangs probes every declared language's fragment dir and keeps
// the ones with at least one own fragment. A missing translation warns with
// the exact dir to add — the same contract the health-file bridges run.
// Documents whose own default-language set is empty (inherited-only, or the
// whole doc skipped) return nil: there is nothing to translate, and a variant
// must never be the only place a feature exists.
func resolveVariantLangs(projectDir, dir, out string, own []Fragment, langs []string) []variantFragments {
	if len(own) == 0 || len(langs) == 0 {
		return nil
	}
	if strings.Contains(out, "/") || !strings.HasPrefix(dir, core.LocalizedDir+"/") {
		genlog.Decision("fragments_localization", out, dir, "skipped (dir outside docs/ or nested out)")
		return nil
	}
	out2 := make([]variantFragments, 0, len(langs))
	for _, lang := range langs {
		frags, err := loadFragments(projectDir, localizedFragDir(dir, lang))
		if err != nil {
			genlog.Warn("unreadable localized fragment dir — language skipped",
				"dir", localizedFragDir(dir, lang), "error", err.Error())
			continue
		}
		if len(frags) == 0 {
			genlog.Warn("no fragments for language — file skipped",
				"file", core.LocalizedFilename(out, lang),
				"lang", lang,
				"hint", "add "+localizedFragDir(dir, lang)+"/<feature>.md")
			continue
		}
		out2 = append(out2, variantFragments{Lang: lang, Fragments: frags})
	}
	return out2
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

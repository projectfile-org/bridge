// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"embed"
	"strconv"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
	"kiota.ch/projectfile/core/v2/pkg/genlog"
)

// The readme's own localization layer.
//
// The four other community health files are prose documents, so each locale
// ships a whole translated template. Readme blocks are not prose: eighteen
// three-line fragments whose only translatable content is a heading and one
// sentence. Shipping a per-language copy of each would mean maintaining 18×N
// near-identical templates and re-applying every structural change N times —
// so the readme localizes its *strings* instead, from one flat catalog per
// language, and keeps one structural template per block.
//
// Templates reach the catalog through the `t` function; Go-side labels (the
// policies link text, the link-group headings, the build-link labels) call
// lookupMessage directly. A block that needs a different *shape* per language
// is still free to ship `<block>.<lang>.tmpl` — see blockCandidates.

//go:embed all:messages
var messagesFS embed.FS

const (
	// messagesDir is the embedded catalog directory; one <lang>.yaml per locale.
	messagesDir = "messages"
	// messagesExt is the catalog file extension, stripped to get the language.
	messagesExt = ".yaml"
	// defaultCatalog backs every key a localized catalog has not translated.
	defaultCatalog = "en"
)

// Catalog key prefixes. The suffix is derived, never hand-written: a link's
// `type`, or a health file's basename. Adding a link type or a health file is
// therefore a catalog edit, not a Go edit.
const (
	keyPrefixLinkType         = "link.type."
	keyPrefixLinkGroup        = "link.group."
	keyPrefixPolicy           = "policy."
	keyPrefixArtifactKind     = "artifact.kind."
	keyPrefixAcknowledgements = "acknowledgements."
)

var (
	catalogsOnce sync.Once
	// catalogs maps language → key → message, loaded once from messagesFS.
	catalogs map[string]map[string]string
)

// loadCatalogs parses every embedded catalog. A malformed or unreadable file
// is warned about and skipped rather than fatal: a broken translation must
// never block generating the readme, it only falls back to English.
func loadCatalogs() {
	catalogs = map[string]map[string]string{}
	entries, err := messagesFS.ReadDir(messagesDir)
	if err != nil {
		genlog.Warn("no message catalogs embedded", "dir", messagesDir, "error", err.Error())
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, messagesExt) {
			genlog.Decision("message_catalog", name, messagesDir, "skipped (not a catalog)")
			continue
		}
		body, err := messagesFS.ReadFile(messagesDir + "/" + name)
		if err != nil {
			genlog.Warn("unreadable message catalog", "file", name, "error", err.Error())
			continue
		}
		var m map[string]string
		if err := yaml.Unmarshal(body, &m); err != nil {
			genlog.Warn("malformed message catalog", "file", name, "error", err.Error())
			continue
		}
		lang := strings.TrimSuffix(name, messagesExt)
		catalogs[lang] = m
		genlog.Decision("message_catalog", lang, messagesDir+"/"+name, strconv.Itoa(len(m))+" keys")
	}
}

// lookupMessage resolves key in lang, falling back to the default catalog.
// The bool separates “no such message” from a message that is legitimately
// empty, so callers carrying their own fallback (a raw link type, a bare
// filename) can keep it.
func lookupMessage(lang, key string) (string, bool) {
	catalogsOnce.Do(loadCatalogs)
	if lang != "" && lang != defaultCatalog {
		if v, ok := catalogs[lang][key]; ok {
			return v, true
		}
		genlog.Decision("message", key, "catalog "+defaultCatalog, "untranslated in "+lang)
	}
	v, ok := catalogs[defaultCatalog][key]
	return v, ok
}

// translate is lookupMessage for callers with no fallback of their own — the
// `t` template function. An unknown key returns the key itself: visible in the
// rendered file and greppable, where a silent empty string would leave a bare
// “## ” heading nobody can trace back.
func translate(lang, key string) string {
	if v, ok := lookupMessage(lang, key); ok {
		return v
	}
	genlog.Warn("no message for key", "key", key, "lang", langOrDefault(lang))
	return key
}

// langOrDefault names the language in logs; the canonical render carries no
// tag of its own.
func langOrDefault(lang string) string {
	if lang == "" {
		return defaultCatalog
	}
	return lang
}

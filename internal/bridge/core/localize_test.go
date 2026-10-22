// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// TestLocalizedFilename pins the naming rule every localized artefact uses:
// the locale lives in the directory (docs/<lang>/), and the canonical file
// (empty lang) stays at the root.
func TestLocalizedFilename(t *testing.T) {
	assert.Equal(t, "CONTRIBUTING.md", core.LocalizedFilename("CONTRIBUTING.md", ""))
	assert.Equal(t, "docs/es/CONTRIBUTING.md", core.LocalizedFilename("CONTRIBUTING.md", "es"))
	assert.Equal(t, "docs/pt-BR/README.md", core.LocalizedFilename("README.md", "pt-BR"))
	assert.Equal(t, "docs/uk/CODE_OF_CONDUCT.md", core.LocalizedFilename(core.FileCodeOfConduct, "uk"))
}

// TestLocalizedTemplateName pins that a variant's template keeps the tag infix
// before the extension regardless of the output directory — template lookup
// did not move, only disk output did.
func TestLocalizedTemplateName(t *testing.T) {
	assert.Equal(t, "SUPPORT.md.tmpl", core.LocalizedTemplateName("SUPPORT.md", ""))
	assert.Equal(t, "SUPPORT.es.md.tmpl", core.LocalizedTemplateName("SUPPORT.md", "es"))
}

// supportVariant is the docs/<lang>/ path the language-links tests expect for
// a SUPPORT variant; declared once so goconst sees a single literal.
const supportVariantUK = "docs/uk/SUPPORT.md"

// labelUkrainian is the Ukrainian endonym used in the language-links tests;
// declared once so goconst sees a single literal.
const labelUkrainian = "Українська"

// TestLanguageLinksExcludesActive verifies the cross-language bar lists every
// other variant including the canonical file, points variants at docs/<lang>/,
// and never links to itself. The canonical entry is labelled with the default
// language's endonym.
func TestLanguageLinksExcludesActive(t *testing.T) {
	langs := []string{"es", "uk"}

	en := core.LanguageLinks(core.FileSupport, "", "en", langs)
	assert.Equal(t, []core.LangLink{
		{Code: "es", Label: "Español", Filename: "docs/es/SUPPORT.md"},
		{Code: "uk", Label: labelUkrainian, Filename: supportVariantUK},
	}, en, "canonical variant lists the translations only")

	es := core.LanguageLinks(core.FileSupport, "es", "en", langs)
	assert.Equal(t, []core.LangLink{
		{Code: "", Label: "English", Filename: "../../SUPPORT.md"},
		{Code: "uk", Label: labelUkrainian, Filename: "../uk/SUPPORT.md"},
	}, es, "a translation links back to the canonical file and the siblings")
}

// TestLanguageLinksDefaultLanguageLabel verifies that when the project's
// default language is not English, the canonical entry is labelled with that
// language's endonym — a Spanish-first project's root file reads "Español".
func TestLanguageLinksDefaultLanguageLabel(t *testing.T) {
	langs := []string{"en", "uk"}
	en := core.LanguageLinks(core.FileSupport, "en", "es", langs)
	assert.Equal(t, []core.LangLink{
		{Code: "", Label: "Español", Filename: "../../SUPPORT.md"},
		{Code: "uk", Label: labelUkrainian, Filename: "../uk/SUPPORT.md"},
	}, en, "canonical entry uses the default-language endonym")
}

// TestLanguageLinksSingleLanguage verifies a project with no declared
// languages grows no bar at all.
func TestLanguageLinksSingleLanguage(t *testing.T) {
	assert.Nil(t, core.LanguageLinks(core.FileSupport, "", "en", nil))
}

// TestInsertLanguageBarAboveH1 verifies the bar lands above the title and
// below whatever the template emitted first (the managed marker), so marker
// detection and the reader's first glance both still work.
func TestInsertLanguageBarAboveH1(t *testing.T) {
	body := []byte("<!-- pf-cli-managed: yes -->\n\n# Getting Support\n\nBody.\n")
	out := string(core.InsertLanguageBar(body, core.FileSupport, "", "en", []string{"es"}))
	assert.Equal(t,
		"<!-- pf-cli-managed: yes -->\n\n[Español](docs/es/SUPPORT.md)\n\n# Getting Support\n\nBody.\n",
		out)
}

// TestInsertLanguageBarNoLanguagesIsNoop guards the single-language path: an
// unlocalized project's files must come out byte-identical to before.
func TestInsertLanguageBarNoLanguagesIsNoop(t *testing.T) {
	body := []byte("# Title\n\nBody.\n")
	assert.Equal(t, body, core.InsertLanguageBar(body, core.FileSupport, "", "en", nil))
}

// TestInsertLanguageBarNoHeading covers the degenerate body with no H1: the
// bar still renders, at the top, rather than being silently dropped. The
// active variant is docs/es/SUPPORT.md, so the canonical entry is rebased to
// ../../SUPPORT.md.
func TestInsertLanguageBarNoHeading(t *testing.T) {
	out := string(core.InsertLanguageBar([]byte("no heading here\n"), core.FileSupport, "es", "en", []string{"es"}))
	assert.Equal(t, "[English](../../SUPPORT.md)\n\nno heading here\n", out)
}

// TestLocalizedSiblingStaysInLanguage verifies a cross-reference from a
// localized document points at the sibling's matching variant, and that the
// canonical document keeps the canonical link.
func TestLocalizedSiblingStaysInLanguage(t *testing.T) {
	assert.Equal(t, "SECURITY.md", core.LocalizedSibling(core.FileSecurity, ""))
	assert.Equal(t, "docs/es/SECURITY.md", core.LocalizedSibling(core.FileSecurity, "es"))
}

// TestCollapseBlankLines pins the shared whitespace contract every renderer
// now goes through.
func TestCollapseBlankLines(t *testing.T) {
	assert.Equal(t, "a\n\nb\n", string(core.CollapseBlankLines([]byte("a\n\n\n\nb\n\n\n"))))
	assert.Equal(t, "a\n", string(core.CollapseBlankLines([]byte("a"))), "a body with zero trailing newlines still gets one")
}

// Template names the RenderLocalized and HasTemplate tests share.
const (
	spdxMIT     = "MIT"
	tmplTLOC    = "TLOC.md"
	tmplTLOCCan = tmplTLOC + ".tmpl"
	tmplTLOCES  = "TLOC.es.md.tmpl"
	tmplBodyEN  = "# Title\n\nEN body\n"
	tmplBodyES  = "# Titulo\n\nES body\n"
)

// tlocOnce keeps the shared registrations idempotent: RegisterTemplates panics on a duplicate name.
var tlocOnce sync.Once

func registerTLOC() {
	tlocOnce.Do(func() {
		core.RegisterTemplates(tmplTLOCCan, func(string) ([]byte, error) { return []byte(tmplBodyEN), nil })
		core.RegisterTemplates(tmplTLOCES, func(string) ([]byte, error) { return []byte(tmplBodyES), nil })
	})
}

// TestResolveLangAnchorsTheRenderSentinel pins the empty sentinel resolving to the default language.
func TestResolveLangAnchorsTheRenderSentinel(t *testing.T) {
	spanish := &projectfile.Document{}
	projectfile.SetExtension(spanish, pfmodel.I18NExtensionNS, map[string]any{"default-language": "es"})
	assert.Equal(t, "es", core.ResolveLang("", spanish))
	assert.Equal(t, "uk", core.ResolveLang("uk", spanish))
	assert.Equal(t, "en", core.ResolveLang("", &projectfile.Document{}))
}

// TestHasTemplateFindsBothTiers pins the probe: embedded, project-local, miss.
func TestHasTemplateFindsBothTiers(t *testing.T) {
	registerTLOC()
	assert.True(t, core.HasTemplate("", tmplTLOCCan))

	dir := t.TempDir()
	local := filepath.Join(dir, core.LocalTemplatesDir)
	require.NoError(t, os.MkdirAll(local, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(local, "OTHER.md.tmpl"), []byte("x"), 0o644))
	assert.True(t, core.HasTemplate(dir, "OTHER.md.tmpl"))
	assert.False(t, core.HasTemplate(dir, "MISSING.md.tmpl"))
	assert.False(t, core.HasTemplate(t.TempDir(), "MISSING.md.tmpl"))
	assert.False(t, core.HasTemplate("", "MISSING.md.tmpl"))
}

// TestRenderLocalizedWritesCanonicalAndVariants pins the header, the bar above the H1, per-language keys.
func TestRenderLocalizedWritesCanonicalAndVariants(t *testing.T) {
	registerTLOC()
	var seen []string

	out, err := core.RenderLocalized(&projectfile.Document{
		License:   &projectfile.License{Spdx: spdxMIT},
		Copyright: &projectfile.Copyright{Year: 2026},
	}, core.LocalizedSpec{
		Filename: tmplTLOC,
		Langs:    []string{"es"},
		View: func(lang string) any {
			seen = append(seen, lang)
			return nil
		},
	}, core.Options{})

	require.NoError(t, err)
	assert.Equal(t, []string{"", "es"}, seen, "the view runs for the sentinel then each variant")
	require.Len(t, out.Files, 2)

	canonical := string(out.Files[tmplTLOC])
	assert.True(t, strings.HasPrefix(canonical, "<!--"), "every generated file carries the managed header")
	assert.Contains(t, canonical, "[Español](docs/es/TLOC.md)\n\n# Title", "the bar sits above the H1")

	variant := string(out.Files["docs/es/TLOC.md"])
	assert.Contains(t, variant, "# Titulo", "the variant renders its own template")
	assert.Contains(t, variant, "[English](../../TLOC.md)", "the variant links home")
}

// TestRenderLocalizedSkipsAMissingTranslation pins the skip rule and the bar drop that follows it.
func TestRenderLocalizedSkipsAMissingTranslation(t *testing.T) {
	registerTLOC()

	out, err := core.RenderLocalized(&projectfile.Document{
		License:   &projectfile.License{Spdx: spdxMIT},
		Copyright: &projectfile.Copyright{Year: 2026},
	}, core.LocalizedSpec{
		Filename: tmplTLOC,
		Langs:    []string{"fr"},
		View:     func(string) any { return nil },
	}, core.Options{})

	require.NoError(t, err)
	require.Len(t, out.Files, 1)
	assert.NotContains(t, string(out.Files[tmplTLOC]), "Français", "a dropped language never enters the bar")
}

// TestRenderLocalizedAppendsFooterAndSeeAlso pins the See also content before the trailing footer.
func TestRenderLocalizedAppendsFooterAndSeeAlso(t *testing.T) {
	registerTLOC()

	out, err := core.RenderLocalized(&projectfile.Document{
		License:   &projectfile.License{Spdx: spdxMIT},
		Copyright: &projectfile.Copyright{Year: 2026},
	}, core.LocalizedSpec{
		Filename:  core.FileSupport,
		Template:  tmplTLOC,
		View:      func(string) any { return nil },
		HowToLink: true,
		SeeAlso:   func(string) []core.SeeAlsoLink { return []core.SeeAlsoLink{{Label: "Guide", URL: "https://g.test"}} },
	}, core.Options{})

	require.NoError(t, err)
	body := string(out.Files[core.FileSupport])
	assert.Contains(t, body, "## See also\n\n- [Guide](https://g.test)")
	assert.Contains(t, body, "Generated from projectfile", "the how-to footer follows the content")
}

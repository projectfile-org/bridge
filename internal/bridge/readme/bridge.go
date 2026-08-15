// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const filenameReadme = core.FileReadme

// readmeDocPath is this readme's own repo-relative path for one render
// language ("" -> root README.md, "es" -> docs/es/README.md), used to rebase
// emitted cross-file links so they resolve from the document's directory.
func readmeDocPath(lang string) string { return core.LocalizedFilename(filenameReadme, lang) }

type Bridge struct{}

func (Bridge) Name() string             { return "readme" }
func (Bridge) Filename() string         { return filenameReadme }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filenameReadme, "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameReadme, filenameReadme))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return core.PathOrDefault(dir, filenameReadme, filenameReadme)
}

// Block-name constants. Each names a template under
// templates/readme.md/<name>.tmpl (or a project-local override). Declared as
// constants so the default list and the test suite share one canonical home
// per name (goconst-clean).
const (
	blockLanguages     = "languages"
	blockLogo          = "logo"
	blockBasics        = "basics"
	blockBadges        = "badges"
	blockScreenshots   = "screenshots"
	blockArtifacts     = "artifacts"
	blockPlatforms     = "platforms"
	blockFeatures      = "features"
	blockBenchmarks    = "benchmarks"
	blockQuickStart    = "quick-start"
	blockRequirements  = "requirements"
	blockInstallation  = "installation"
	blockUsage         = "usage"
	blockConfiguration = "configuration"
	blockBuilding      = "building"
	blockDocumentation = "documentation"
	blockFAQ           = "faq"
	blockRoadmap       = "roadmap"
	blockPolicies      = "policies"
	blockRelated       = "related"
	blockLinks         = "links"
	blockFunding       = "funding"
	blockLicense       = "license"
)

// defaultBlocks is the canonical README composition: every block silently
// drops when its data source is absent, so the list can stay fixed and a bare
// project renders just basics + license while a rich one fills every section.
// blockArtifacts sits directly above the installation/usage pair on purpose: it
// answers "what IS this" (an image, a binary, an npm package) and those two then
// answer "how do I get it" and "how do I call it" for the very same things.
var defaultBlocks = []string{
	blockLanguages, blockLogo, blockBasics, blockBadges, blockRelated, blockScreenshots,
	blockFeatures, blockBenchmarks, blockQuickStart, blockRequirements,
	blockArtifacts, blockPlatforms, blockInstallation, blockUsage, blockConfiguration, blockBuilding,
	blockDocumentation, blockFAQ, blockRoadmap,
	blockPolicies, blockLinks, blockFunding, blockLicense,
}

const readmeBlockDir = "readme.md"

func (b Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	ext, _ := pfmodel.GetReadmeExtension(pf)

	blocks := defaultBlocks
	if ext != nil && len(ext.Blocks) > 0 {
		blocks = ext.Blocks
	}

	// configuredLangs is the document-wide list from org.projectfile.i18n
	// (e.g. [es, uk]) — the same list every community health file honours.
	// The default README.md (lang="") is always rendered too: it carries the
	// canonical content and is the file forges link to. When configuredLangs
	// is empty, only the default README.md is written.
	configuredLangs := pfmodel.Languages(pf)

	// renderSet is the set of langs to render: the default + configured.
	renderSet := append([]string{""}, configuredLangs...)

	out := core.Output{Files: map[string][]byte{}}
	for _, lang := range renderSet {
		rendered, err := b.renderLang(pf, ext, opts, blocks, lang, configuredLangs)
		if err != nil {
			return core.Output{}, err
		}
		out.Files[core.LocalizedFilename(filenameReadme, lang)] = rendered
	}
	return out, nil
}

// renderLang renders the README for a single language. lang == "" is the
// default; otherwise the output is destined for README.<lang>.md and every
// LocalizedString is resolved in that language.
func (Bridge) renderLang(pf *projectfile.Document, ext *pfmodel.ReadmeExtension, opts core.Options, blocks []string, lang string, langs []string) ([]byte, error) {
	data := newReadmeView(pf, lang, langs)

	formatDecisionTrace(opts.Dir, lang, data, ext)

	var parts [][]byte
	for _, blockName := range blocks {
		body, err := renderBlock(opts.Dir, blockName, data, ext, lang, templatesFS)
		if err != nil {
			return nil, err
		}
		if body != nil {
			parts = append(parts, body)
		}
	}

	// Assemble the body first, then bracket it for the terminology rule when
	// the content language is not English. data.StrLang is Lang resolved to the
	// default language, so a Spanish-default project's Spanish root README is
	// wrapped while its docs/en/ variant is not. The header is prepended after,
	// so the disable directive lands directly under the REUSE/marker block.
	var body []byte
	for i, p := range parts {
		body = append(body, p...)
		if i < len(parts)-1 {
			body = append(body, '\n')
		}
	}
	body = append(body, []byte("\n\n"+core.GeneratedFooter(filenameReadme, data.StrLang)+"\n")...)
	body = core.WrapLocalizedTextlint(body, data.StrLang)
	out := append([]byte(core.ManagedREUSEHeader(pf)), body...)
	return core.CollapseBlankLines(out), nil
}

// tmplExt is the template file extension every block template carries.
const tmplExt = ".tmpl"

// Trace labels for the tier a block resolved from.
const (
	sourceLocal    = "project-local"
	sourceEmbedded = "embedded"
)

// blockCandidates lists the template filenames that can back one block in one
// language, most specific first: the language variant (features.es.tmpl), then
// the language-neutral template (features.tmpl). The neutral template
// localizes its own strings through `t`, so a variant is only needed by a
// block a project wants to *restructure* per language — translating one is
// a catalog edit, not a template copy.
//
// Template names keep the infix form (license.es.tmpl) — only disk OUTPUT moved
// under docs/<lang>/, template lookup did not — so this uses the infix, not
// LocalizedFilename.
func blockCandidates(blockName, lang string) []string {
	name := blockName + tmplExt
	if lang == "" {
		return []string{name}
	}
	return []string{core.LocalizedTemplateInfix(name, lang), name}
}

// renderBlock implements the 4-tier block resolution algorithm, each template
// tier tried per-language first (see blockCandidates):
//  1. Project-local template: .projectfile/templates/readme.md/<name>.tmpl
//  2. Embedded built-in: readme.md/<name>.tmpl from templatesFS
//  3. Extras match: find extras[] where .name == blockName (content resolved
//     per-lang when ContentByLang is set)
//  4. Silent skip: no match — omit the block
func renderBlock(dir, blockName string, data readmeView, ext *pfmodel.ReadmeExtension, lang string, fs interface {
	ReadFile(name string) ([]byte, error)
},
) ([]byte, error) {
	for _, name := range blockCandidates(blockName, lang) {
		rel := filepath.Join(core.LocalTemplatesDir, readmeBlockDir, name)
		body, ok, err := core.ReadLocalTemplate(dir, rel)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		return execTracedBlock(blockName+" (local)", body, data, dir, ext, blockName, sourceLocal+" "+name, lang)
	}

	for _, name := range blockCandidates(blockName, lang) {
		body, err := fs.ReadFile("templates/" + readmeBlockDir + "/" + name)
		if err != nil {
			continue
		}
		return execTracedBlock(blockName, body, data, dir, ext, blockName, sourceEmbedded+" "+name, lang)
	}

	if ext != nil {
		for _, extra := range ext.Extras {
			if extra.Name != blockName {
				continue
			}
			content := extraContentForLang(extra, lang)
			if content == "" {
				continue
			}
			genlog.Decision("block", blockName, "extras", "lang="+lang)
			return []byte(content), nil
		}
	}

	genlog.Decision("block", blockName, "no match (skipped)", "lang="+lang)
	return nil, nil
}

// execTracedBlock executes one resolved block template and traces which tier
// and which file it came from. A block rendering to only whitespace is dropped
// (nil, nil) — that is how every probe-driven block disappears when its data
// source is absent.
func execTracedBlock(tmplName string, body []byte, data readmeView, dir string, ext *pfmodel.ReadmeExtension, blockName, source, lang string) ([]byte, error) {
	rendered, err := execBlockTemplate(tmplName, body, data, dir, ext)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(rendered)) == 0 {
		genlog.Decision("block", blockName, source+" (empty, skipped)", "lang="+lang)
		return nil, nil
	}
	genlog.Decision("block", blockName, source, "lang="+lang)
	return rendered, nil
}

// execBlockTemplate parses body and executes it against data. The FuncMap is
// the template's only way to reach values that are functions of (Doc, Lang,
// dir, ext) rather than fields on readmeView — localization, filesystem probes,
// the badges derivation. Each FuncMap calls the same package-level helper the
// decision trace and tests use, so the three stay in lockstep.
//
// Probes run lazily: a block that doesn't call `logo` never stats the logo
// candidates. That's why these live on the FuncMap and not on readmeView.
func execBlockTemplate(name string, body []byte, data readmeView, dir string, ext *pfmodel.ReadmeExtension) ([]byte, error) {
	tpl, err := template.New(name).
		Funcs(template.FuncMap{
			// pf walks Doc.Extensions by dotted reverse-DNS path
			// ("org.acme.todo" or "org.projectfile.readme.languages") and
			// returns the subtree (map/slice/string/number). A miss coerces to
			// "" so direct {{pf "x"}} renders empty (not "<no value>") and
			// {{with pf "x"}} skips cleanly — both forms stay panic-free.
			"pf": func(path string) any {
				if v := pfLookup(data.Doc, path); v != nil {
					return v
				}
				return ""
			},
			// t resolves a catalog message in the active render language,
			// falling back to English. This is what keeps section headings
			// and boilerplate sentences out of the templates themselves, so
			// one structural template serves every language. StrLang maps the
			// canonical render sentinel to the default language so a non-
			// English-default project's root README resolves its own language.
			"t": func(key string) string { return translate(data.StrLang, key) },
			// ls resolves a *LocalizedString in the active render language so
			// templates can call ExtractLocalizedStringForLang inline — that
			// resolution is a function, not a field, so it can't be pre-baked.
			"ls": func(ls *projectfile.LocalizedString) string {
				return projectfile.ExtractLocalizedStringForLang(ls, data.StrLang)
			},
			// projectName wraps DisplayName so templates don't reach into core
			// for the namespace/name fallback logic.
			"projectName": func() string { return pfmodel.DisplayName(data.Doc) },
			// license is nil-safe SPDX lookup; empty when License is unset, so
			// {{with license}}…{{end}} drops the section cleanly.
			"license": func() string { return licenseSPDX(data.Doc) },
			// badgeRows maps ext.Shields to the badge view model grouped into
			// rendered lines: each URL's ${…} references resolved against the
			// document, unresolvable entries dropped, duplicates collapsed
			// last-wins, survivors grouped by their `row`.
			"badgeRows": func() []badgeRow { return buildBadgeRows(data.Doc, ext) },
			// readmeSection resolves a structured command section — its title
			// plus the groups whose commands the document could answer — by
			// block name; nil when the document declares none or none survived,
			// so the template's {{else}} falls back to its companion-file probe.
			"readmeSection": func(name string) *sectionView {
				return buildSection(data.Doc, ext, name, data.StrLang)
			},
			// artifacts lists what the project SHIPS, from
			// org.projectfile.artifacts: one entry per declared artifact with
			// its kind localized and its address resolved.
			"artifacts": func() []artifactView { return buildArtifacts(data.Doc, data.StrLang) },
			// platforms lists the OCI platforms the project builds for, from
			// org.projectfile.operating-system × org.projectfile.architecture
			// (spec §4.8a). Empty when neither extension is declared, so the
			// block drops for a non-shipping project.
			"platforms": func() []string { return buildPlatforms(data.Doc) },
			// linkGroups buckets Doc.Links by category in render order.
			"linkGroups": func() []linkGroup { return buildLinkGroups(data.Doc, data.StrLang) },
			// relatedLinks is the "Related projects" bar: top-level links[]
			// entries tagged `related` (spec §139 — tags round-trips via
			// Link.Extra). Rendered as a headingless bar after the badges.
			// Returns nil when no link carries the tag, so the block drops
			// silently — same self-suppress rule every probe-driven block runs.
			"relatedLinks": func() []linkEntry { return relatedLinks(data.Doc, data.StrLang) },
			// readmeGoals lists the CI goals the building block highlights,
			// preferring goals tagged `readme` and falling back to every goal
			// when none are tagged. Returns nil for a project with no CI DAG.
			"readmeGoals": func() []goalView { return buildReadmeGoals(data.Doc) },
			// hasDevContainer reports whether the CI DAG declares a dev-container
			// node, so the building block can advertise the local dev loop.
			"hasDevContainer": func() bool { return hasDevContainer(data.Doc) },
			// staticLinks probes the community-health files. Path probing uses
			// the render sentinel (Lang) so the default language probes the
			// root; labels resolve in StrLang so a non-English-default project
			// labels them in its own language. docPath rebases every emitted
			// link against this document's own path so a docs/<lang>/ readme
			// links co-located siblings correctly.
			"staticLinks": func() []staticLink {
				return probeHealthFiles(dir, readmeDocPath(data.Lang), data.Lang, data.StrLang)
			},
			// docLink probes one companion file; nil when absent so
			// {{with docLink "FILE" "Label"}} drops the block cleanly.
			"docLink": func(filename, label string) *staticLink {
				return docLink(dir, readmeDocPath(data.Lang), filename, label)
			},
			// logo / screenshots / docLinks / buildLinks probe the filesystem;
			// each returns nil/empty when its directory is absent.
			"logo":        func() []string { return probeLogo(dir) },
			"screenshots": func() []screenshot { return probeScreenshots(dir) },
			"docLinks":    func() []staticLink { return listDocsMarkdown(dir, readmeDocPath(data.Lang)) },
			"buildLinks":  func() []staticLink { return probeBuildLinks(dir, readmeDocPath(data.Lang), data.StrLang) },
			// featureDoc is the features block's data source: which
			// FEATURES.md this language links and scrapes (the localized
			// document when it exists, else the canonical file plus the
			// not-yet-translated note); nil when neither exists.
			"featureDoc": func() *featureDoc { return buildFeatureDoc(dir, data.Lang, data.StrLang) },
		}).
		Option("missingkey=zero").
		Parse(string(body))
	if err != nil {
		return nil, fmt.Errorf("bridge: parse block %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("bridge: exec block %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

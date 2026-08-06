// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/interp"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Link-group identities. These are catalog-key suffixes and trace keys, never
// display text — the heading a reader sees comes from link.group.<key> in the
// active language.
const (
	groupProject   = "project"
	groupCommunity = "community"
	groupSecurity  = "security"
	groupOther     = "other"
)

// keyBlocks is the YAML key for the blocks list inside the readme extension.
// Read by core (parsed into ReadmeExtension.Blocks); the bridge only writes it
// from tests, but it lives here next to keyLanguages so the two extension-map
// keys share one canonical home.
const keyBlocks = "blocks"

// readmeView is the render-state every block template executes against. It
// carries only the three values that are per-render but NOT on Doc itself:
//
//   - Doc — the source; templates reach any field via .Doc.* and pf/ls.
//   - Lang — the active render language; FuncMaps need it but it isn't on Doc.
//   - Languages — the cross-link bar; depends on Lang + the configured langs
//     list, so it's precomputed once per render.
//
// Everything else a template might want (project name, summary, badges, the
// probe-driven links, …) is reached through the FuncMaps registered in
// bridge.go. That keeps the struct tiny and lets new blocks ship as a
// template-only change with no Go edit here.
type readmeView struct {
	Doc       *projectfile.Document
	Lang      string
	Languages []core.LangLink
}

// screenshot pairs a repo-relative image path with its basename for alt text.
type screenshot struct {
	Path string
	Name string
}

type badge struct {
	Alt  string
	Img  string
	Href string
	Row  string
}

// badgeRow is one rendered line of badges. Name is the declared row identity,
// kept for the decision trace; Badges holds that row's members in declaration
// order.
type badgeRow struct {
	Name   string
	Badges []badge
}

// artifactView is one line of the artifacts block: what the project ships, said
// once. Label is the artifact's kind resolved in the active language; Address is
// the single string that identifies it to a user (a pull reference, an executable
// name, a package name); Ports is populated for a service.
type artifactView struct {
	Kind    string
	Label   string
	Address string
	Summary string
	Ports   []string
}

// artifactAddress picks the ONE string that identifies an artifact to a reader.
// The order is a fallback chain, not a preference list: a binary is named by the
// command it installs as, and only failing that by the path it was built to.
//
// It is kind-BLIND on purpose, walking the same fields in the same order for
// every artifact, which is what keeps the kind vocabulary open — declaring
// `kind: helm-chart` produces a correct line today, and only its LABEL needs a
// catalog entry. The declared Key is the last resort so a SERVICE, whose whole
// identity is the ports it answers on, still has something to print.
func artifactAddress(a pfmodel.Artifact) string {
	for _, candidate := range []string{a.Ref, a.Command, a.Name, a.Module, a.URL, a.Path, a.Key} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

// buildArtifacts resolves the declared artifacts into render-ready lines. Every
// `${…}` reference in an address is expanded first, and an artifact whose address
// cannot be resolved at all is dropped — the same rule as a badge or a command:
// the README never publishes a literal `${…}`.
func buildArtifacts(doc *projectfile.Document, lang string) []artifactView {
	declared, err := pfmodel.GetArtifacts(doc)
	if err != nil {
		genlog.Warn("unreadable artifacts namespace", "error", err.Error())
		return nil
	}
	axes := ciMatrixAxes(doc)
	var out []artifactView
	for _, a := range declared {
		expanded, resolved := interp.ExpandFanOut(doc, artifactAddress(a))
		if !resolved {
			genlog.Decision("artifact", a.Key, "unresolved address (dropped)", artifactAddress(a))
			continue
		}
		for _, address := range expandAxes(expanded, axes) {
			out = append(out, artifactView{
				Kind:    a.Kind,
				Label:   artifactKindLabel(a.Kind, lang),
				Address: address,
				Summary: sectionText(a.Summary, a.SummaryByLang, lang),
				Ports:   artifactPorts(a),
			})
		}
	}
	return out
}

// artifactKindLabel is an artifact kind's display text in lang. The catalog key
// is derived from the kind (`artifact.kind.image`) rather than mapped, so a new
// kind needs no Go edit. An unknown — or untranslated — kind falls back to the
// bare kind string, never to an empty label.
func artifactKindLabel(kind, lang string) string {
	if kind == "" {
		return ""
	}
	if label, ok := lookupMessage(lang, keyPrefixArtifactKind+kind); ok {
		return label
	}
	genlog.Decision("artifact_label", kind, "no catalog entry", keyPrefixArtifactKind+kind)
	return kind
}

// artifactPorts renders a service artifact's ports as display strings — "3306",
// or "9090 (metrics)" when the port carries a name. This is what turns a service
// declaration into the "MySQL on 3306" line a reader is looking for.
func artifactPorts(a pfmodel.Artifact) []string {
	var out []string
	for _, p := range a.Ports {
		s := strconv.Itoa(p.Port)
		if p.Name != "" {
			s += " (" + p.Name + ")"
		}
		out = append(out, s)
	}
	return out
}

// linkGroup is one rendered bucket of links. Key is the stable identity (used
// by the catalog and the decision trace); Heading is that key resolved in the
// active language, which is what the template prints.
type linkGroup struct {
	Key     string
	Heading string
	Links   []linkEntry
}

type linkEntry struct {
	Label string
	URL   string
}

// staticLink is a relative in-repo link. Label is the human-readable text;
// Filename is the link target. Zero-value (Filename=="") signals "absent" and
// the block template guards with {{if .Field.Filename}}.
type staticLink struct {
	Filename string
	Label    string
}

// linkCategories buckets a link `type` into a group. An unlisted type falls
// into groupOther. The per-type display label is NOT here — it lives in the
// message catalog under link.type.<type>, so adding a type is a catalog edit.
var linkCategories = map[string]string{
	"homepage":    groupProject,
	"source-code": groupProject,
	// bugs is the issue tracker — a project resource, and the ONE link type
	// this fleet uses that had no mapping, so every README grew an "Other"
	// heading holding nothing but trackers.
	"bugs":               groupProject,
	"documentation":      groupProject,
	"changelog":          groupProject,
	"wiki":               groupProject,
	"faq":                groupProject,
	"package-registry":   groupProject,
	"chat":               groupCommunity,
	"forum":              groupCommunity,
	"contact":            groupCommunity,
	"enforcement":        groupCommunity,
	"first-contribution": groupCommunity,
	"donation":           groupCommunity,
	"translate":          groupCommunity,
	"security-policy":    groupSecurity,
	"security-report":    groupSecurity,
	"bug-bounty":         groupSecurity,
}

var categoryOrder = []string{groupProject, groupCommunity, groupSecurity, groupOther}

// Health-file filenames the policies block auto-discovers at the repo root.
// Declared as constants so the healthFiles list, the healthFileLabels map,
// and the test suite share one canonical home per name (goconst-clean).
const (
	fileContributing  = core.FileContributing
	fileSecurity      = core.FileSecurity
	fileSupport       = core.FileSupport
	fileCodeOfConduct = core.FileCodeOfConduct
)

// healthFiles are the community-health markdown files the policies block
// auto-discovers at the repo root. Order is the rendered link order.
var healthFiles = []string{
	fileContributing,
	fileSecurity,
	fileSupport,
	fileCodeOfConduct,
}

// healthFileLabel is a health file's policies link text in lang. The catalog
// key is derived from the filename (CODE_OF_CONDUCT.md → policy.code-of-conduct)
// rather than mapped, so a new health file needs no Go edit. An untranslated —
// or unknown — file falls back to its bare name, never to an empty link.
func healthFileLabel(file, lang string) string {
	key := keyPrefixPolicy + strings.ReplaceAll(
		strings.ToLower(strings.TrimSuffix(file, filepath.Ext(file))), "_", "-")
	if label, ok := lookupMessage(lang, key); ok {
		return label
	}
	genlog.Decision("policy_label", file, "no catalog entry", key)
	return file
}

// Companion markdown filenames declared as constants so goconst sees one
// canonical home for each. Used in docLinkSpecs, the switch in
// newReadmeView, and the parallel switch in formatDecisionTrace.
const (
	fileFeatures      = "FEATURES.md"
	fileBenchmarks    = "BENCHMARKS.md"
	fileQuickStart    = "QUICKSTART.md"
	fileRequirements  = "REQUIREMENTS.md"
	fileInstall       = "INSTALL.md"
	fileUsage         = "USAGE.md"
	fileFAQ           = "FAQ.md"
	fileFunding       = "FUNDING.md"
	fileRoadmap       = "ROADMAP.md"
	fileConfiguration = "CONFIGURATION.md"
)

// docLinkSpecs lists the uppercase companion files the single-link blocks
// (features, benchmarks, …) auto-discover at the repo root, paired with the
// catalog key holding their label. Key is the one each block template passes
// to docLink, so the decision trace reports the same text the reader sees.
type docLinkSpec struct {
	File string
	Key  string
}

var docLinkSpecs = []docLinkSpec{
	{File: fileFeatures, Key: "features.title"},
	{File: fileBenchmarks, Key: "benchmarks.title"},
	{File: fileQuickStart, Key: "quick-start.title"},
	{File: fileRequirements, Key: "requirements.title"},
	{File: fileInstall, Key: "installation.title"},
	{File: fileUsage, Key: "usage.title"},
	{File: fileFAQ, Key: "faq.title"},
	{File: fileFunding, Key: "funding.title"},
	{File: fileRoadmap, Key: "roadmap.title"},
	{File: fileConfiguration, Key: "configuration.title"},
}

// imageExts are the file extensions recognised as images for the logo and
// screenshots probes. Lowercase, without the leading dot.
var imageExts = map[string]bool{
	"png":  true,
	"jpg":  true,
	"jpeg": true,
	"gif":  true,
	"webp": true,
	"svg":  true,
}

// docMarkdownExcluded lists docs/*.md files that the documentation block
// should NOT surface: meta-docs about the bridge itself, and the generated
// Makefile reference, which the building block already links as the answer to
// "how do I build this" — listing it twice is pure noise.
var docMarkdownExcluded = map[string]bool{
	"readme-generator.md": true,
	"makefile.md":         true,
}

const (
	docsDir           = "docs"
	screenshotsSubdir = "screenshots"
	logoBasename      = "logo"
	mkDirAssets       = "assets"
	makefileDocPath   = "docs/MAKEFILE.md"
	makefileDocKey    = "build.makefile"
	buildDocFile      = "BUILD.md"
	buildDocKey       = "build.build"
)

// newReadmeView builds the per-render view-state. Only Doc/Lang/Languages
// belong here — every other value a template might want is reached through
// the FuncMaps registered in bridge.go, which take dir/ext explicitly so the
// probes run lazily (only for blocks actually in the list).
func newReadmeView(pf *projectfile.Document, lang string, langs []string) readmeView {
	return readmeView{
		Doc:       pf,
		Lang:      lang,
		Languages: core.LanguageLinks(filenameReadme, lang, langs),
	}
}

// extractLSForLang wraps the core lang-aware resolver so templates stay clean.
// A nil LocalizedString resolves to "".
func extractLSForLang(ls *projectfile.LocalizedString, lang string) string {
	return projectfile.ExtractLocalizedStringForLang(ls, lang)
}

// licenseSPDX returns Doc.License.Spdx when present, "" otherwise. Used by the
// license FuncMap and the decision trace so the nil-License path stays a
// one-liner everywhere.
func licenseSPDX(doc *projectfile.Document) string {
	if doc == nil || doc.License == nil {
		return ""
	}
	return doc.License.Spdx
}

// pfLookup is the template-facing escape hatch: given a dotted reverse-DNS
// path (e.g. "org.acme.todo" or "org.projectfile.readme.languages"), it walks
// Doc.Extensions and returns the subtree found there — a map, slice, string,
// number, or nil. The first match wins against both on-disk encodings the core
// supports: the literal dotted key (YAML/JSON, TOML quoted) and the exploded
// dotted-table header (TOML's [org.acme.todo]). Nil at every miss, never panics.
//
// What we are trying to do: let a template reach any custom projectfile field
// without forcing a Go edit on readmeView — the maintainer is no longer the
// gatekeeper for new probe-driven blocks.
func pfLookup(doc *projectfile.Document, path string) any {
	if doc == nil || path == "" || len(doc.Extensions) == 0 {
		return nil
	}
	segments := strings.Split(path, ".")
	// Find the longest namespace prefix that exists, trying both encodings at
	// each split. Longest first so a custom value stored under the full path
	// (literal-dotted "org.acme.todo") wins over a shorter nested prefix — and
	// so a path like "org.acme.status.phase" lands on Extensions's literal
	// "org.acme.status" subtree, then descends the remaining "phase" key.
	for split := len(segments); split >= 1; split-- {
		cur, ok := pfLookupOne(doc.Extensions, segments[:split])
		if !ok {
			continue
		}
		for _, seg := range segments[split:] {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil
			}
			next, ok := m[seg]
			if !ok {
				return nil
			}
			cur = next
		}
		return cur
	}
	return nil
}

// pfLookupOne resolves a dotted prefix against Extensions, honouring both the
// literal-dotted and exploded-table encodings (mirroring LookupExtension's
// semantics for a single namespace).
func pfLookupOne(ext map[string]any, segments []string) (any, bool) {
	joined := strings.Join(segments, ".")
	if v, ok := ext[joined]; ok {
		return v, true
	}
	cur, ok := ext[segments[0]]
	if !ok {
		return nil, false
	}
	for _, seg := range segments[1:] {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[seg]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// extraContentForLang resolves a ReadmeExtra's content for a specific lang.
// Falls back to the default Content (en/first-non-empty) when ContentByLang
// is nil or has no entry for the requested language.
func extraContentForLang(extra pfmodel.ReadmeExtra, lang string) string {
	if lang != "" && extra.ContentByLang != nil {
		if v, ok := extra.ContentByLang[lang]; ok && v != "" {
			return v
		}
	}
	return extra.Content
}

// buildBadges maps declared shields to the template's badge view model, with
// every `${…}` reference resolved against the document (spec §3.8). This is
// what lets ONE badge row in a shared m6e fragment serve the whole fleet: the
// URL names fields, and each consumer answers them from its own projectfile.
//
// Two rules make that fragment safe across heterogeneous projects:
//
//   - A shield whose img or href still carries an unresolved reference is
//     DROPPED. A project with no Codeberg mirror cannot answer
//     ${…forge.remotes.codeberg.url}, and a badge pointing at a literal `${…}`
//     is a broken image in every README that renders it. An href that is simply
//     ABSENT is fine — a pure indicator (project status) links nowhere, and the
//     template renders it unlinked.
//   - Shields are deduplicated by `name`, LAST wins. Includes union sequences
//     with include entries first and the base document last (spec §4.9a), so
//     redeclaring a name in the project's own projectfile replaces the
//     inherited badge — the only way to override one, since includes cannot
//     delete. The first position is kept so overriding never reorders the row.
//
// Alt defaults to Name so a missing alt-text never yields an empty `![ ](...)`.
func buildBadges(doc *projectfile.Document, ext *pfmodel.ReadmeExtension) []badge {
	if ext == nil || len(ext.Shields) == 0 {
		return nil
	}
	out := make([]badge, 0, len(ext.Shields))
	position := make(map[string]int, len(ext.Shields))
	for _, s := range ext.Shields {
		img, href := interp.Expand(doc, s.Img), interp.Expand(doc, s.Href)
		if img == "" || interp.Unresolved(img) || interp.Unresolved(href) {
			genlog.Decision("badge", s.Name, "unresolved reference (dropped)", img)
			continue
		}
		alt := interp.Expand(doc, s.Alt)
		if alt == "" {
			alt = s.Name
		}
		b := badge{Alt: alt, Img: img, Href: href, Row: s.Row}
		if at, seen := position[s.Name]; seen {
			genlog.Decision("badge", s.Name, "redeclared (last wins)", out[at].Img)
			out[at] = b
			continue
		}
		position[s.Name] = len(out)
		out = append(out, b)
	}
	return out
}

// rowUnnamed labels the row a shield joins when it declares no `row`, for the
// decision trace only — the empty string groups like any other row name.
const rowUnnamed = "(unnamed)"

// buildBadgeRows groups the resolved badges into rendered lines by their `row`.
// One README carries badges of three different natures — static facts (licence,
// compliance), dynamic project signals (status, last release), ecosystem
// signals (npm version, dependency freshness) — and a fleet fragment that owns
// only one of them still has to land it beside its own kind.
//
// The row ORDER is the order each row name is first seen, so it needs no
// second key to declare it: includes merge in declared order with the base
// document last (spec §4.9a), which is already the order the rows want to read
// in — core's static row, then dynamic, then whatever a language or publish
// fragment appends. A fragment that invents a row name simply gets a new line
// at the point it enters.
//
// Grouping runs AFTER buildBadges has deduplicated, so redeclaring a name also
// moves that badge to the row the redeclaration names.
func buildBadgeRows(doc *projectfile.Document, ext *pfmodel.ReadmeExtension) []badgeRow {
	var rows []badgeRow
	at := make(map[string]int)
	for _, b := range buildBadges(doc, ext) {
		i, seen := at[b.Row]
		if !seen {
			i = len(rows)
			at[b.Row] = i
			rows = append(rows, badgeRow{Name: b.Row})
			genlog.Decision("badge_row", rowLabel(b.Row), "first appearance", strconv.Itoa(i+1))
		}
		rows[i].Badges = append(rows[i].Badges, b)
	}
	return rows
}

// rowLabel renders a row name for logs, naming the unnamed row.
func rowLabel(row string) string {
	if row == "" {
		return rowUnnamed
	}
	return row
}

func buildLinkGroups(pf *projectfile.Document, lang string) []linkGroup {
	if len(pf.Links) == 0 {
		return nil
	}

	groups := map[string][]linkEntry{}
	for _, l := range pf.Links {
		cat, ok := linkCategories[l.Type]
		if !ok {
			cat = groupOther
		}
		label := extractLSForLang(l.Label, lang)
		if label == "" {
			label, _ = lookupMessage(lang, keyPrefixLinkType+l.Type)
		}
		if label == "" {
			genlog.Decision("link_label", l.Type, "no label and no catalog entry", "links[].label")
			label = l.Type
		}
		groups[cat] = append(groups[cat], linkEntry{Label: label, URL: l.URL})
	}

	var out []linkGroup
	for _, key := range categoryOrder {
		entries, ok := groups[key]
		if !ok || len(entries) == 0 {
			continue
		}
		out = append(out, linkGroup{
			Key:     key,
			Heading: translate(lang, keyPrefixLinkGroup+key),
			Links:   entries,
		})
	}
	return out
}

// docLink is the single-file probe behind the docLink FuncMap: returns a
// populated *staticLink when filename exists at the repo root, or nil
// otherwise. Nil (not a zero-value struct) is what lets a template write
// {{with docLink "FEATURES.md" "Features"}}…{{end}} and have the block drop
// cleanly when the file is absent.
func docLink(dir, filename, label string) *staticLink {
	if !fileExists(dir, filename) {
		return nil
	}
	return &staticLink{Filename: filename, Label: label}
}

// probeHealthFiles walks healthFiles and returns a staticLink per existing
// file, in the declared order. Labels come from healthFileLabels with a
// filename fallback.
//
// In a localized render each link prefers the same-language variant on disk
// (README.es.md → CONTRIBUTING.es.md) and falls back to the canonical file,
// so a reader who arrived in their own language stays in it.
func probeHealthFiles(dir, lang string) []staticLink {
	var out []staticLink
	for _, f := range healthFiles {
		target := f
		if localized := core.LocalizedFilename(f, lang); fileExists(dir, localized) {
			target = localized
		} else if !fileExists(dir, f) {
			continue
		}
		out = append(out, staticLink{Filename: target, Label: healthFileLabel(f, lang)})
	}
	return out
}

// listDocsMarkdown lists every docs/*.md file as a static link, excluding the
// meta-docs in docMarkdownExcluded. Label is the file's first Markdown heading,
// falling back to the humanized filename when the file has no heading. Returns
// nil when docs/ is absent or empty.
func listDocsMarkdown(dir string) []staticLink {
	entries := readDir(dir, docsDir)
	var out []staticLink
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		if docMarkdownExcluded[strings.ToLower(name)] {
			continue
		}
		rel := docsDir + "/" + name
		label := extractFirstHeading(dir, rel)
		if label == "" {
			label = humanizeFilename(name)
		}
		out = append(out, staticLink{Filename: rel, Label: label})
	}
	return out
}

// probeBuildLinks builds the multi-link building block: BUILD.md at the root
// plus the generated Makefile reference at docs/MAKEFILE.md if either exists.
func probeBuildLinks(dir, lang string) []staticLink {
	var out []staticLink
	if link := docLink(dir, buildDocFile, translate(lang, buildDocKey)); link != nil {
		out = append(out, *link)
	}
	if fileExists(dir, makefileDocPath) {
		out = append(out, staticLink{Filename: makefileDocPath, Label: translate(lang, makefileDocKey)})
	}
	return out
}

// probeScreenshots scans docs/screenshots for images and returns each as a
// screenshot (path + basename-for-alt). Returns nil when the directory is
// absent or holds no images.
func probeScreenshots(dir string) []screenshot {
	entries := readDir(dir, filepath.Join(docsDir, screenshotsSubdir))
	var out []screenshot
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isImageFile(name) {
			continue
		}
		out = append(out, screenshot{
			Path: filepath.Join(docsDir, screenshotsSubdir, name),
			Name: name,
		})
	}
	return out
}

// probeLogo looks for a logo image at docs/logo.<ext> then assets/logo.<ext>.
// Returns the matching repo-relative paths (usually zero or one).
func probeLogo(dir string) []string {
	candidates := []string{
		filepath.Join(docsDir, logoBasename),
		filepath.Join(mkDirAssets, logoBasename),
	}
	var out []string
	for _, base := range candidates {
		for ext := range imageExts {
			rel := base + "." + ext
			if fileExists(dir, rel) {
				out = append(out, rel)
			}
		}
	}
	return out
}

// fileExists reports whether a regular file exists at dir/rel. A directory or
// missing entry returns false.
func fileExists(dir, rel string) bool {
	if dir == "" {
		dir = "."
	}
	info, err := os.Stat(filepath.Join(dir, rel))
	return err == nil && !info.IsDir()
}

// readDir lists entries under dir/sub, returning nil on any error (missing
// directory, permission, …) so callers can range safely over a nil slice.
func readDir(dir, sub string) []os.DirEntry {
	if dir == "" {
		dir = "."
	}
	entries, err := os.ReadDir(filepath.Join(dir, sub))
	if err != nil {
		return nil
	}
	return entries
}

// readFile reads the bytes of dir/rel, returning an error when the path is
// missing or unreadable so the heading probes fall back to "" / nil cleanly.
func readFile(dir, rel string) ([]byte, error) {
	if dir == "" {
		dir = "."
	}
	return os.ReadFile(filepath.Join(dir, rel)) // #nosec G304 -- rel is a fixed registered filename (FEATURES.md, docs/*.md), dir is the caller-provided project dir
}

// isImageFile reports whether name has a recognised image extension.
func isImageFile(name string) bool {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	return imageExts[ext]
}

// humanizeFilename turns "MAKEFILE.md" into "Makefile", "api-guide.md" into
// "Api Guide". Strips the extension, replaces - and _ with spaces, and
// title-cases each word.
func humanizeFilename(name string) string {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	stem = strings.ToLower(stem)
	stem = strings.ReplaceAll(stem, "-", " ")
	stem = strings.ReplaceAll(stem, "_", " ")
	words := strings.Fields(stem)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// parseATXHeading parses one line of Markdown as an ATX heading. It returns
// the heading level (1–6) and the trimmed text, or ok=false when the line is
// not a heading. A space or tab must follow the leading #'s (CommonMark); no
// trailing #'s are stripped — none of the repo's documents use closing hashes.
// Shared by the documentation and features blocks.
func parseATXHeading(line string) (level int, text string, ok bool) {
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0, "", false
	}
	// A space or tab must follow the marker (CommonMark 4.2).
	if n >= len(line) || (line[n] != ' ' && line[n] != '\t') {
		return 0, "", false
	}
	return n, strings.TrimSpace(line[n:]), true
}

// extractFirstHeading reads dir/rel and returns the text of the first ATX
// heading of any level, or "" on a read error or when the file has no heading.
// Used by the documentation block to label each docs/*.md link with the
// heading a reader actually sees rather than the bare filename.
func extractFirstHeading(dir, rel string) string {
	body, err := readFile(dir, rel)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(body), "\n") {
		if _, text, ok := parseATXHeading(line); ok {
			return text
		}
	}
	return ""
}

// featureHeadings reads FEATURES.md and returns the text of every level-3
// heading — the per-feature titles features-md emits under the structural
// "# Features" / "## Project features" / "## Inherited features" headings.
// The H1/H2 are skipped so only the feature titles reach the bullet list.
// Returns nil when FEATURES.md is absent or holds no H3.
func featureHeadings(dir string) []string {
	body, err := readFile(dir, fileFeatures)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		level, text, ok := parseATXHeading(line)
		if !ok || level != 3 {
			continue
		}
		out = append(out, text)
	}
	return out
}

// formatDecisionTrace emits one decision-trace line per data source so the
// CLI's -v trace shows what each block rendered from. Probes run here directly
// — same helpers the FuncMaps call at render time — so the trace stays
// accurate without round-tripping through view-model fields. Every log carries
// at least one variable per the workspace's logging rule.
func formatDecisionTrace(dir, lang string, v readmeView, ext *pfmodel.ReadmeExtension) {
	genlog.Decision("lang", core.LocalizedFilename(filenameReadme, lang), "active render language", pfmodel.I18NExtensionNS+".languages")
	for _, l := range v.Languages {
		genlog.Decision("language_link", l.Label+" → "+l.Filename, pfmodel.I18NExtensionNS+".languages", "")
	}
	genlog.Decision("project_name", pfmodel.DisplayName(v.Doc), "identity.title.en or namespace/name", "")
	genlog.Decision("summary", summaryOrUnset(extractLSForLang(v.Doc.Identity.Summary, v.Lang)), "identity.summary", "")
	for _, g := range buildLinkGroups(v.Doc, v.Lang) {
		for _, l := range g.Links {
			genlog.Decision(
				fmt.Sprintf("link[%s]", g.Key),
				l.Label+" → "+l.URL,
				"links[]",
				"",
			)
		}
	}
	for _, s := range probeHealthFiles(dir, lang) {
		genlog.Decision("static_link", s.Label+" → "+s.Filename, "policies probe", "")
	}
	for _, r := range buildBadgeRows(v.Doc, ext) {
		for _, b := range r.Badges {
			genlog.Decision("badge", b.Alt+" → "+b.Img, "readme.shields[]", rowLabel(r.Name))
		}
	}
	for _, l := range probeLogo(dir) {
		genlog.Decision("logo", l, "docs/assets probe", "")
	}
	for _, s := range probeScreenshots(dir) {
		genlog.Decision("screenshot", s.Path, "docs/screenshots probe", "")
	}
	for _, d := range listDocsMarkdown(dir) {
		genlog.Decision("doc_link", d.Label+" → "+d.Filename, "docs/*.md probe", "")
	}
	for _, b := range probeBuildLinks(dir, lang) {
		genlog.Decision("build_link", b.Label+" → "+b.Filename, "BUILD/MAKEFILE probe", "")
	}
	for _, spec := range docLinkSpecs {
		if link := docLink(dir, spec.File, translate(lang, spec.Key)); link != nil {
			genlog.Decision("doc_link", link.Label+" → "+link.Filename, spec.File+" probe", "")
		}
	}
	for _, h := range featureHeadings(dir) {
		genlog.Decision("feature", h, fileFeatures+" H3 probe", "")
	}
	if spdx := licenseSPDX(v.Doc); spdx != "" {
		genlog.Decision("license", spdx, "license.spdx", "")
	}
}

func summaryOrUnset(s string) string {
	if s == "" {
		return "(unset, section omitted)"
	}
	return s
}

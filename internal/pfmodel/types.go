// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import "kiota.ch/projectfile/core/v2/pkg/projectfile"

// The typed shapes of bridge-owned projectfile extension namespaces. Each
// binds one org.projectfile.* namespace to a Go struct the matching bridge
// reads through the accessor in the same file. Moved out of
// core/internal/projectfile/types.go in the core 2.0 cut; the generic
// document model (Document, Identity, Person, ...) stays in core.

type CitationExtension struct {
	DOI       string             `toml:"doi" yaml:"doi" json:"doi"`
	Message   string             `toml:"message" yaml:"message" json:"message"`
	Preferred *PreferredCitation `toml:"preferred" yaml:"preferred" json:"preferred"`
}

type PreferredCitation struct {
	Type    string `toml:"type" yaml:"type" json:"type"`
	Title   string `toml:"title" yaml:"title" json:"title"`
	Journal string `toml:"journal" yaml:"journal" json:"journal"`
	Volume  int    `toml:"volume" yaml:"volume" json:"volume"`
	Issue   int    `toml:"issue" yaml:"issue" json:"issue"`
	Pages   string `toml:"pages" yaml:"pages" json:"pages"`
	Year    int    `toml:"year" yaml:"year" json:"year"`
	DOI     string `toml:"doi" yaml:"doi" json:"doi"`
}

// IgnoresExtension binds `org.projectfile.ignores`. Gitignore-style
// FILE-PATTERN ignores. Suppressed vulnerability IDs live under
// org.projectfile.vulnerabilities instead. yamllint also reads from here:
// its excludes live as an `ignore:` block inside .yamllint (not a standalone
// ignore file), so the bridge/yamllint package emits a YAML config from
// Yamllint.Include rather than a flat pattern list.
type IgnoresExtension struct {
	Generate  []string              `toml:"generate"  yaml:"generate"  json:"generate"`
	Extra     []string              `toml:"extra"     yaml:"extra"     json:"extra"`
	Git       *IgnoreTargetOverride `toml:"git"       yaml:"git"       json:"git"`
	Docker    *IgnoreTargetOverride `toml:"docker"    yaml:"docker"    json:"docker"`
	Npm       *IgnoreTargetOverride `toml:"npm"       yaml:"npm"       json:"npm"`
	Claude    *IgnoreTargetOverride `toml:"claude"    yaml:"claude"    json:"claude"`
	Container *IgnoreTargetOverride `toml:"container" yaml:"container" json:"container"`
	Yamllint  *IgnoreTargetOverride `toml:"yamllint"  yaml:"yamllint"  json:"yamllint"`
}

type IgnoreTargetOverride struct {
	Include []string `toml:"include" yaml:"include" json:"include"`
	Exclude []string `toml:"exclude" yaml:"exclude" json:"exclude"`
}

// AttributesExtension binds `org.projectfile.attributes` — the .gitattributes
// vocabulary. Unlike IgnoresExtension, whose targets are a fixed known list,
// git attribute names are OPEN (shell, linguist-generated, export-ignore, any
// user-coined name), so rules live in a map keyed by attribute name.
type AttributesExtension struct {
	Generate []string                  `toml:"generate" yaml:"generate" json:"generate"`
	Extra    []string                  `toml:"extra"    yaml:"extra"    json:"extra"`
	Rules    map[string]*AttributeRule `toml:"-"        yaml:"-"        json:"-"`
}

// AttributeRule is one attribute's pattern lists. Include SETS the attribute on
// matching paths, Exclude UNSETS it (`-attr`). Git resolves attributes
// last-match-wins, so the generator always emits excludes after includes.
type AttributeRule struct {
	Include []string `toml:"include" yaml:"include" json:"include"`
	Exclude []string `toml:"exclude" yaml:"exclude" json:"exclude"`
}

// VulnerabilitiesExtension binds `org.projectfile.vulnerabilities` — a
// tool-agnostic list of suppressed IDs that fans out to every scanner
// bridge (.trivyignore, .grype.yaml, osv-scanner.toml).
type VulnerabilitiesExtension struct {
	Generate []string                `toml:"generate" yaml:"generate" json:"generate"`
	Suppress []VulnerabilitySuppress `toml:"suppress" yaml:"suppress" json:"suppress"`
}

// VulnerabilitySuppress is one suppressed vulnerability. Reason is OPTIONAL
// and surfaced in the scanner formats that support it (grype, osv); it is
// dropped from the plain-line .trivyignore dialect.
type VulnerabilitySuppress struct {
	ID     string `toml:"id" yaml:"id" json:"id"`
	Reason string `toml:"reason" yaml:"reason" json:"reason"`
}

// EditorsExtension binds `org.projectfile.editors`. Editors are distinct
// from stack — they describe developer tooling preferences, not project
// technology.
type EditorsExtension struct {
	Use []string `toml:"use" yaml:"use" json:"use"`
}

// FundingChannel carries a payment channel for FundingJSON bridges.
// Maps to fundingjson.org channels[] items.
type FundingChannel struct {
	GUID        string `toml:"guid" yaml:"guid" json:"guid"`
	Type        string `toml:"type" yaml:"type" json:"type"`
	Address     string `toml:"address" yaml:"address" json:"address"`
	Description string `toml:"description" yaml:"description" json:"description"`
}

// FundingPlan carries a funding plan for FundingJSON bridges.
// Maps to fundingjson.org plans[] items.
type FundingPlan struct {
	GUID        string   `toml:"guid" yaml:"guid" json:"guid"`
	Status      string   `toml:"status" yaml:"status" json:"status"`
	Name        string   `toml:"name" yaml:"name" json:"name"`
	Description string   `toml:"description" yaml:"description" json:"description"`
	Amount      float64  `toml:"amount" yaml:"amount" json:"amount"`
	Currency    string   `toml:"currency" yaml:"currency" json:"currency"`
	Frequency   string   `toml:"frequency" yaml:"frequency" json:"frequency"`
	Channels    []string `toml:"channels" yaml:"channels" json:"channels"`
}

// FundingHistory carries a yearly funding summary for FundingJSON bridges.
// Maps to fundingjson.org history[] items.
type FundingHistory struct {
	Year        int     `toml:"year" yaml:"year" json:"year"`
	Income      float64 `toml:"income" yaml:"income" json:"income"`
	Expenses    float64 `toml:"expenses" yaml:"expenses" json:"expenses"`
	Taxes       float64 `toml:"taxes" yaml:"taxes" json:"taxes"`
	Currency    string  `toml:"currency" yaml:"currency" json:"currency"`
	Description string  `toml:"description" yaml:"description" json:"description"`
}

// FundingExtension binds `[org.projectfile.funding]`. The set of providers
// follows GitHub's FUNDING.yml schema 1:1; Channels/Plans/History extend it
// for FundingJSON-compatible bridges.
type FundingExtension struct {
	GitHub          []string         `toml:"github"           yaml:"github"           json:"github"`
	Patreon         string           `toml:"patreon"          yaml:"patreon"          json:"patreon"`
	KoFi            string           `toml:"ko-fi"            yaml:"ko-fi"            json:"ko-fi"`
	Liberapay       string           `toml:"liberapay"        yaml:"liberapay"        json:"liberapay"`
	Tidelift        string           `toml:"tidelift"         yaml:"tidelift"         json:"tidelift"`
	CommunityBridge string           `toml:"community-bridge" yaml:"community-bridge" json:"community-bridge"`
	IssueHunt       string           `toml:"issuehunt"        yaml:"issuehunt"        json:"issuehunt"`
	OpenCollective  string           `toml:"open-collective"  yaml:"open-collective"  json:"open-collective"`
	LFXCrowdfunding string           `toml:"lfx-crowdfunding" yaml:"lfx-crowdfunding" json:"lfx-crowdfunding"`
	Polar           string           `toml:"polar"            yaml:"polar"            json:"polar"`
	BuyMeACoffee    string           `toml:"buy-me-a-coffee"  yaml:"buy-me-a-coffee"  json:"buy-me-a-coffee"`
	ThanksDev       string           `toml:"thanks-dev"       yaml:"thanks-dev"       json:"thanks-dev"`
	Custom          []string         `toml:"custom"           yaml:"custom"           json:"custom"`
	Path            string           `toml:"path"             yaml:"path"             json:"path"`
	EntityType      string           `toml:"entity-type"      yaml:"entity-type"      json:"entity-type"`
	EntityRole      string           `toml:"entity-role"      yaml:"entity-role"      json:"entity-role"`
	Channels        []FundingChannel `toml:"channels" yaml:"channels" json:"channels"`
	Plans           []FundingPlan    `toml:"plans" yaml:"plans" json:"plans"`
	History         []FundingHistory `toml:"history" yaml:"history" json:"history"`
}

// SecurityExtension binds `[org.projectfile.security]`. Empty strings/slices
// signal "section absent in output" — the template omits the corresponding
// block rather than emitting an empty stub.
type SecurityExtension struct {
	Contact           string   `toml:"contact"            yaml:"contact"            json:"contact"`
	ReportURL         string   `toml:"report-url"         yaml:"report-url"         json:"report-url"`
	SupportedVersions []string `toml:"supported-versions" yaml:"supported-versions" json:"supported-versions"`
	DisclosureWindow  string   `toml:"disclosure-window"  yaml:"disclosure-window"  json:"disclosure-window"`
	GPGKey            string   `toml:"gpg-key"            yaml:"gpg-key"            json:"gpg-key"`
	BugBountyURL      string   `toml:"bug-bounty-url"     yaml:"bug-bounty-url"     json:"bug-bounty-url"`
}

// CodeOfConductExtension binds `[org.projectfile.code-of-conduct]`.
type CodeOfConductExtension struct {
	Covenant string `toml:"covenant" yaml:"covenant" json:"covenant"`
	Scope    string `toml:"scope"    yaml:"scope"    json:"scope"`
}

// DEIExtension binds `[org.projectfile.dei]` — the CHAOSS / badging DEI.md
// project statement. Enabled gates generation (off by default); Metrics is an
// open map keyed by CHAOSS metric slug (project-access,
// communication-transparency, newcomer-experiences, inclusive-leadership,
// other) whose value is the project's per-metric effort bullets. A metric
// absent from the map renders SAMPLE bullets in the generated DEI.md.
type DEIExtension struct {
	Enabled      bool                `toml:"enabled" yaml:"enabled" json:"enabled"`
	Scope        string              `toml:"scope"   yaml:"scope"   json:"scope"`
	LastReviewed string              `toml:"last-reviewed" yaml:"last-reviewed" json:"last-reviewed"`
	Metrics      map[string][]string `toml:"metrics" yaml:"metrics" json:"metrics"`
}

// ContributingExtension binds `[org.projectfile.contributing]`.
type ContributingExtension struct {
	Sections          []string           `toml:"sections" yaml:"sections" json:"sections"`
	CLAURL            string             `toml:"cla-url"  yaml:"cla-url"  json:"cla-url"`
	ChatURL           string             `toml:"chat-url" yaml:"chat-url" json:"chat-url"`
	CoCURL            string             `toml:"coc-url"  yaml:"coc-url"  json:"coc-url"`
	RecommendToFollow projectfile.Toggle `toml:"recommend-to-follow" yaml:"recommend-to-follow" json:"recommend-to-follow"`
	RecommendToStar   projectfile.Toggle `toml:"recommend-to-star"   yaml:"recommend-to-star"   json:"recommend-to-star"`
}

// SupportExtension binds `[org.projectfile.support]`. Community support URLs
// live in top-level links[]; the extension carries only response-time SLA
// and the end-of-life version table.
type SupportExtension struct {
	ResponseTime string     `toml:"response-time" yaml:"response-time" json:"response-time"`
	EOL          []EOLEntry `toml:"eol"           yaml:"eol"           json:"eol"`
}

// EOLEntry carries an end-of-life date for a version series.
type EOLEntry struct {
	Version string `toml:"version" yaml:"version" json:"version"`
	Date    string `toml:"date"    yaml:"date"    json:"date"`
}

// ReleaseExtension binds `[org.projectfile.release]`. Engine-agnostic release
// INTENT — tag format, changelog style, branch-to-channel map — consumed by
// semantic-release / release-please / similar.
type ReleaseExtension struct {
	TagFormat string          `toml:"tag-format" yaml:"tag-format" json:"tag-format"`
	Changelog string          `toml:"changelog"  yaml:"changelog"  json:"changelog"`
	Branches  []ReleaseBranch `toml:"branches"   yaml:"branches"   json:"branches"`
}

// ReleaseBranch is one entry in the release branch-to-channel map.
type ReleaseBranch struct {
	Pattern    string `toml:"pattern"    yaml:"pattern"    json:"pattern"`
	Channel    string `toml:"channel"    yaml:"channel"    json:"channel"`
	Prerelease bool   `toml:"prerelease" yaml:"prerelease" json:"prerelease"`
}

// ConventionsExtension binds `[org.projectfile.conventions]`. Cross-cutting
// workflow conventions read by multiple generators (contributing, release).
type ConventionsExtension struct {
	CommitStyle   string
	Workflow      string
	StyleGuideURL string
	Languages     map[string]LangConventions
}

// LangConventions holds per-stack-tag convention overrides.
type LangConventions struct {
	StyleGuideURL string
}

// CLIExtension binds `[org.projectfile.cli]`. Derive toggles control
// per-source inference passes (forges, registries). All default to true.
type CLIExtension struct {
	Derived []string         `toml:"-" yaml:"-" json:"-"`
	Derive  CLIDeriveToggles `toml:"derive" yaml:"derive" json:"derive"`
}

// CLIDeriveToggles enables/disables individual inference passes.
type CLIDeriveToggles struct {
	Forges     bool `toml:"forges"     yaml:"forges"     json:"forges"`
	Registries bool `toml:"registries" yaml:"registries" json:"registries"`
}

// ForgeExtension binds `[org.projectfile.forge]`. Push is the global
// kill-switch; Fields toggles per-field push; Hosts/Kinds are per-host.
type ForgeExtension struct {
	Push   bool               `toml:"push"   yaml:"push"   json:"push"`
	Fields ForgeFieldsToggles `toml:"fields" yaml:"fields" json:"fields"`
	Hosts  map[string]bool    `toml:"hosts"  yaml:"hosts"  json:"hosts"`
	Kinds  map[string]string  `toml:"kinds"  yaml:"kinds"  json:"kinds"`
}

// ForgeFieldsToggles enables/disables individual fields the push command
// would otherwise sync.
type ForgeFieldsToggles struct {
	Description bool `toml:"description" yaml:"description" json:"description"`
	Homepage    bool `toml:"homepage"    yaml:"homepage"    json:"homepage"`
	Topics      bool `toml:"topics"      yaml:"topics"      json:"topics"`
}

// CodeOwnersExtension binds `[org.projectfile.codeowners]`. Entries is
// order-significant — CODEOWNERS pattern resolution is "last match wins per
// path", so reordering changes semantics.
type CodeOwnersExtension struct {
	Entries []CodeOwnersEntry `toml:"entries" yaml:"entries" json:"entries"`
}

type CodeOwnersEntry struct {
	Pattern string   `toml:"pattern" yaml:"pattern" json:"pattern"`
	Owners  []string `toml:"owners"  yaml:"owners"  json:"owners"`
}

// FragmentsExtension binds `[org.projectfile.fragments]`. Each document
// assembles one Markdown artefact from a fragment directory, plus one cached
// copy per parent. Documents are independent: each owns its parent list,
// matched by Dir.
type FragmentsExtension struct {
	Documents []FragmentDocument `toml:"documents" yaml:"documents" json:"documents"`
}

type FragmentDocument struct {
	Dir     string           `toml:"dir" yaml:"dir" json:"dir"`
	Out     string           `toml:"out" yaml:"out" json:"out"`
	Title   string           `toml:"title" yaml:"title" json:"title"`
	Parents []FragmentParent `toml:"parents" yaml:"parents" json:"parents"`
}

// FragmentParent names an upstream project whose assembled document this one
// inherits. URL is the parent’s forge repository — never a filesystem path, so
// the parent resolves the same way for a consumer who checked out this project
// alone as for the author who keeps every project in one directory.
//
// Ref floats when empty: the refresh run reads the parent’s newest release tag
// and records which tag it read in the cached copy. That recorded version is
// what the assembled document names, so a heading states “inherited from
// b19/ubuntu 1.0.0” — true forever — instead of claiming to be current.
type FragmentParent struct {
	URL string `toml:"url" yaml:"url" json:"url"`
	Ref string `toml:"ref" yaml:"ref" json:"ref"`
}

// ReadmeExtension binds `[org.projectfile.readme]`. Blocks is the ordered
// list of named template blocks; Extras carries inline content; Shields
// carries badge definitions rendered by the built-in badges block; Sections
// carries the structured command blocks (installation, quick-start, usage,
// building) keyed by block name.
//
// Each section holds SEVERAL groups because a project ships several things, and
// the ways to install them are ALTERNATIVES, not steps: `npm install foo` and
// `docker pull foo` in one fenced block invites a reader to run both. One group
// per artifact keeps each with its own lead-in sentence and its own fence.
type ReadmeExtension struct {
	Blocks   []string
	Extras   []ReadmeExtra
	Shields  []Shield
	Sections map[string][]ReadmeSectionGroup
}

// ReadmeExtra is an inline content block referenced by name in Blocks.
type ReadmeExtra struct {
	Name          string
	Content       string
	ContentByLang map[string]string
}

// ReadmeSectionGroup is one command-oriented group inside a README section:
// optional localized prose (Prefix), a command list rendered as one fenced code
// block, then optional localized prose (Postfix). Commands are code, not
// localized, and carry `${…}` references resolved per project at render time.
// Prefix and Postfix follow the localized-string convention: the default
// resolution plus the raw lang→text map for multi-language renders.
//
// Name is the group's identity for override: includes UNION sequences, so a
// project redeclaring a group name REPLACES the inherited one in place rather
// than appending a second copy. It is never displayed.
//
// What a group buys over a bare command list: it is the unit that DROPS. A
// group whose commands all reference an artifact the project does not declare
// disappears whole — lead-in sentence included — which is what lets one shared
// fragment carry a recipe for every ecosystem and each project render only its
// own.
type ReadmeSectionGroup struct {
	Name          string
	Prefix        string
	PrefixByLang  map[string]string
	Commands      []string
	Postfix       string
	PostfixByLang map[string]string
	// Syntax is the fenced-block info string (sh, dockerfile, python, …).
	// Empty means shell. A base image's usage snippet is a Dockerfile and a
	// library's is source code, so the fence cannot be hardcoded without
	// mislabelling every non-shell shape.
	Syntax string
}

// Shield is a badge image rendered as markdown: [![alt](img)](href). Row is
// the badge's group: the readme renders one line per row, so a fragment that
// only knows about npm can drop its badge next to the other ecosystem badges
// without knowing what else the document carries. Empty is a row like any
// other — the unnamed one.
type Shield struct {
	Name string
	Img  string
	Href string
	Alt  string
	Row  string
}

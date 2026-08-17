// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package contributing

type contribView struct {
	ProjectName string
	Summary     string
	// SupportFile is the SUPPORT.md cross-link resolved for this render
	// language — SUPPORT.es.md from CONTRIBUTING.es.md when that variant is
	// rendered, the canonical SUPPORT.md otherwise.
	SupportFile          string
	RepoURL              string
	SourceCodeURL        string
	DocsURL              string
	BugsURL              string
	NewIssueURL          string
	SecurityContact      string
	LicenseSPDX          string
	Sections             []string
	CLAURL               string
	StyleGuideURL        string
	ChatURL              string
	CoCURL               string
	FirstContributionURL string
	ContactURL           string
	Workflow             string
	DefaultBranch        string
	CommitStyle          string
	AuthorFollows        []followLink
	AuthorSites          []followLink
	ProjectSocials       []followLink
	HasFunding           bool
	// Conventions is the data-driven rows of the Conventions section: workflow,
	// commits, versioning, and one row per declared stack style-guide-url.
	// Each Detail is pre-rendered markdown; the template just loops, so a user
	// overriding the template only restyles the loop.
	Conventions []conventionItem
	// ForgeStars holds one entry per links[type=source-code] forge so the
	// "Star the project" appreciation block surfaces every mirror. Each
	// Label is a pre-built markdown link "[host](url)".
	ForgeStars []followLink
	// StarsEnabled gates the whole star block: when false (recommend-to-star
	// unset or false) no star line renders at all. When true with an empty
	// ForgeStars the template falls back to a bare "Star the project".
	StarsEnabled bool
	// LLMPolicyFile is the LLM.md cross-link for this render language, or ""
	// when the project declared no org.projectfile.llm namespace. A pointer
	// only — the policy itself is stated once, by the llm bridge.
	LLMPolicyFile string
}

// conventionItem is one row of the Conventions section. Label is the bold
// lead ("Workflow", "Commits", "Versioning", or a stack tag like "js");
// Detail is the pre-resolved markdown that follows the colon. Resolved in Go
// so the template is a uniform range — any unknown enum value degrades to a
// raw-value bullet rather than vanishing.
type conventionItem struct {
	Label  string
	Detail string
}

type followLink struct {
	// Key is the identifier a Toggle filters on: a link host for forge stars
	// (e.g. "codeberg.org") or a handle platform / "site" / "funding" for the
	// recommend-to-follow block. Empty means "unfiltered" (always passes).
	Key string
	// Platform identifies the follow link kind so the template can pick its URL
	// pattern: a handle platform ("mastodon", "github", …) or "site".
	Platform string
	// Handle is the raw handle/username (follows) or the host domain (sites).
	// The template composes the URL from it — no URL building happens in Go.
	Handle string
	// Site is the language-neutral brand name of the platform (Mastodon, …),
	// shown next to the handle so each template localizes only the prose.
	Site string
	// URL is the resolved link target for sites (author websites). Follow links
	// leave it empty and build the URL in the template.
	URL string
	// Label is the pre-built markdown used by forge-star and project-social
	// links (host text / project name), which carry no translatable prose.
	Label string
}

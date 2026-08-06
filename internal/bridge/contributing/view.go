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
	HasStyleGuideContent bool
	// ForgeStars holds one entry per links[type=source-code] forge so the
	// "Star the project" appreciation block surfaces every mirror. Each
	// Label is a pre-built markdown link "[host](url)".
	ForgeStars []followLink
	// StarsEnabled gates the whole star block: when false (recommend-to-star
	// unset or false) no star line renders at all. When true with an empty
	// ForgeStars the template falls back to a bare "Star the project".
	StarsEnabled bool
}

type followLink struct {
	// Key is the identifier a Toggle filters on: a link host for forge stars
	// (e.g. "codeberg.org") or a handle platform / "site" / "funding" for the
	// recommend-to-follow block. Empty means "unfiltered" (always passes).
	Key   string
	Label string
	URL   string
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package contributing

import (
	"fmt"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	filenameContributing = core.FileContributing
	socialBluesky        = "bluesky"
	socialMastodon       = "mastodon"
)

// Bridge renders CONTRIBUTING.md as a scaffold-once artefact: the file is the
// user's after creation, so no run overwrites it without --force. It still
// carries the pf-cli-managed sentinel every generated file carries — that line
// warns a reader before they edit, which --force is exactly what defeats.
type Bridge struct{}

func (Bridge) Name() string             { return "contributing" }
func (Bridge) Filename() string         { return filenameContributing }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filenameContributing, "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{ScaffoldOnce: true} }

// defaultSections matches the spec's documented default list.
var defaultSections = []string{
	"question", "legal", "bugs", "enhancements",
	"conventions", "docs",
	"join",
}

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameContributing, filenameContributing))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return core.PathOrDefault(dir, filenameContributing, filenameContributing)
}

func (Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	ext, err := pfmodel.GetContributingExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	if ext == nil {
		ext = &pfmodel.ContributingExtension{}
	}
	core.ApplyUserContributingFallback(ext)
	sections := ext.Sections
	sectionsSrc := "[org.projectfile.contributing].sections"
	if len(sections) == 0 {
		sections = defaultSections
		sectionsSrc = "default"
	}

	// Resolve conventions (commit-style, workflow, versioning, style-guide-url).
	conv, _ := pfmodel.GetConventionsExtension(pf)
	if conv == nil {
		conv = &pfmodel.ConventionsExtension{}
	}
	core.ApplyUserConventionsFallback(conv)
	commitStyle := conv.CommitStyle
	workflow := conv.Workflow
	versioning := conv.Versioning
	styleGuideURL := pfmodel.ConventionsStyleGuideURL(conv, pf.Stack)

	// Resolve URLs: links[] first, extension fields override.
	docsURL := pfmodel.LinkURL(pf, "documentation")
	bugsURL := pfmodel.LinkURL(pf, "bugs")
	newIssueURL := deriveNewIssueURL(bugsURL)
	sourceCodeURL := pfmodel.LinkURL(pf, "source-code")
	chatURL := resolveURL(ext.ChatURL, pf, "chat")
	cocURL := resolveURL(ext.CoCURL, pf, "enforcement")
	if cocURL == "" {
		cocURL = pfmodel.LinkURL(pf, "conduct-full-text")
	}
	claURL := resolveURL(ext.CLAURL, pf, "cla")
	firstContributionURL := pfmodel.LinkURL(pf, "first-contribution")
	contactURL := pfmodel.LinkURL(pf, "contact")
	securityContact, _ := pfmodel.ContactEmail(pf, projectfile.RoleSecurity)
	if securityContact == "" {
		securityContact = pfmodel.LinkURL(pf, "security-report")
	}

	// Resolve default branch from primary repository.
	defaultBranch := "main"
	repo := pfmodel.PrimaryRepository(pf)
	if repo != nil && repo.Branch != "" {
		defaultBranch = repo.Branch
	}

	// LLM.md is a separate bridge's file; contributing only points at it —
	// restating the policy here would be the drift this suite exists to kill
	// (llm-generator.md, Integration). Gated on the namespace, not a file-exists
	// probe: every other cross-link in this bridge already assumes the fleet
	// convention that a declared sibling renders, and a stat call would just
	// duplicate that assumption with a race against pf-bridge-llm's own run.
	hasLLMPolicy := pfmodel.HasExtension(pf, pfmodel.LLMExtensionNS)

	authorFollows := resolveAuthorFollows(pf)
	authorSites := resolveAuthorSites(pf)
	projectSocials := resolveProjectSocials(pf)
	hasFunding := pfmodel.HasExtension(pf, pfmodel.FundingExtensionNS)

	// Every links[type=source-code] entry becomes its own "Star the project
	// on <host>" line — projects commonly mirror across multiple forges.
	// recommend-to-star defaults OFF (no recommendation unless asked for),
	// mirroring recommend-to-follow. Its map form is a per-host allowlist
	// keyed by link host (github.com, codeberg.org, ...).
	forgeStars := filterFollows(resolveForgeStars(pf), ext.RecommendToStar, false)
	starsEnabled := ext.RecommendToStar.Enabled()

	// recommend-to-follow defaults OFF (no author self-promotion unless asked
	// for); its map form is a per-contact allowlist keyed by handle platform
	// (mastodon, github, ...), plus "site" for author websites and "funding".
	follow := ext.RecommendToFollow
	authorFollows = filterFollows(authorFollows, follow, false)
	authorSites = filterFollows(authorSites, follow, false)
	projectSocials = filterFollows(projectSocials, follow, false)
	hasFunding = follow.Allow("funding", false) && hasFunding

	// Build the data-driven Conventions section rows once: workflow, commits,
	// versioning, then one row per declared stack style-guide-url. Resolved in
	// Go so the template is a uniform range and any unknown enum value degrades
	// to a raw-value bullet instead of vanishing.
	conventionRows := buildConventions(workflow, commitStyle, versioning, conv, pf)

	emitDecisionTrace(pf, ext, sections, sectionsSrc, docsURL, bugsURL,
		chatURL, cocURL, claURL, securityContact, commitStyle, workflow, versioning, styleGuideURL,
		authorFollows, authorSites, projectSocials, forgeStars, hasFunding)
	genlog.Decision("llm_policy_pointer", fmt.Sprintf("%v", hasLLMPolicy), "org.projectfile.llm present", "")

	// Only three things vary per language here: the project's own display
	// name, its summary (both localized-strings), and the SUPPORT.md
	// cross-link, which must point at the same-language variant. Everything
	// else — URLs, contacts, the follow/star blocks — is language
	// independent and resolved once above.
	return core.RenderLocalized(pf, core.LocalizedSpec{
		Filename: filenameContributing,
		Langs:    pfmodel.Languages(pf),
		View: func(lang string) any {
			// strLang resolves the render sentinel to the default language for
			// string resolution; the raw sentinel is kept for the sibling-file
			// path so the default language renders at the root. SupportFile is
			// rebased relative to CONTRIBUTING's own path so the link resolves
			// from the root and from docs/<lang>/ alike.
			strLang := core.ResolveLang(lang, pf)
			return contribView{
				ProjectName:          pfmodel.DisplayNameForLang(pf, strLang),
				Summary:              projectfile.ExtractLocalizedStringForLang(pf.Identity.Summary, strLang),
				SupportFile:          core.RelLinkSibling(core.FileSupport, lang, core.FileContributing),
				RepoURL:              repoURL(pf),
				SourceCodeURL:        sourceCodeURL,
				DocsURL:              docsURL,
				BugsURL:              bugsURL,
				NewIssueURL:          newIssueURL,
				SecurityContact:      securityContact,
				LicenseSPDX:          licenseSPDX(pf),
				Sections:             sections,
				CLAURL:               claURL,
				StyleGuideURL:        styleGuideURL,
				ChatURL:              chatURL,
				CoCURL:               cocURL,
				FirstContributionURL: firstContributionURL,
				ContactURL:           contactURL,
				Workflow:             workflow,
				DefaultBranch:        defaultBranch,
				CommitStyle:          commitStyle,
				AuthorFollows:        authorFollows,
				AuthorSites:          authorSites,
				ProjectSocials:       projectSocials,
				HasFunding:           hasFunding,
				Conventions:          conventionRows,
				ForgeStars:           forgeStars,
				StarsEnabled:         starsEnabled,
				LLMPolicyFile:        llmPolicyFile(hasLLMPolicy, lang),
			}
		},
	}, opts)
}

// resolveURL returns the extension value when non-empty, otherwise the
// links[type=linkType] URL. This lets extension fields override links[]
// for backwards compatibility.
func resolveURL(extVal string, pf *projectfile.Document, linkType string) string {
	if extVal != "" {
		return extVal
	}
	return pfmodel.LinkURL(pf, linkType)
}

// llmPolicyFile resolves the LLM.md cross-link for this render language, or
// "" when the project declared no org.projectfile.llm namespace — the
// template's {{with}} then drops the pointer line entirely.
func llmPolicyFile(declared bool, lang string) string {
	if !declared {
		return ""
	}
	return core.RelLinkSibling(core.FileLLM, lang, core.FileContributing)
}

// deriveNewIssueURL appends /new to the bugs URL when it looks like an
// issue tracker URL. Returns "" when bugsURL is empty.
func deriveNewIssueURL(bugsURL string) string {
	if bugsURL == "" {
		return ""
	}
	if strings.HasSuffix(bugsURL, "/issues") ||
		strings.HasSuffix(bugsURL, "/issues/") {
		return strings.TrimRight(bugsURL, "/") + "/new"
	}
	return bugsURL
}

func repoURL(pf *projectfile.Document) string {
	repo := pfmodel.PrimaryRepository(pf)
	if repo == nil {
		return ""
	}
	return repo.URL
}

// buildConventions assembles the data-driven rows of the Conventions section:
// workflow, commits, versioning, then one row per declared stack
// style-guide-url. Each row's Detail is pre-rendered markdown. Unknown enum
// values degrade to a raw-value bullet rather than vanishing, so arbitrary
// projectfile values stay visible. Empty workflow/versioning produce no row;
// empty commit-style falls back to Conventional Commits (matching the prior
// default-when-unset behaviour).
func buildConventions(workflow, commitStyle, versioning string,
	conv *pfmodel.ConventionsExtension, pf *projectfile.Document,
) []conventionItem {
	var rows []conventionItem
	if d := workflowDetail(workflow); d != "" {
		rows = append(rows, conventionItem{Label: "Workflow", Detail: d})
	}
	rows = append(rows, conventionItem{Label: "Commits", Detail: commitDetail(commitStyle)})
	if d := versioningDetail(versioning); d != "" {
		rows = append(rows, conventionItem{Label: "Versioning", Detail: d})
	}
	rows = append(rows, languageStyleRows(conv, pf)...)
	return rows
}

// workflowDetail renders the raw workflow value; the descriptive sentences were
// dropped as dogmatic. Returns "" so an unset workflow yields no row.
func workflowDetail(workflow string) string {
	if workflow == "" {
		return ""
	}
	return workflow
}

// commitDetail maps a commit-style value to its rendered text. Empty defaults
// to Conventional Commits, preserving the prior default-when-unset behaviour;
// unknown values degrade to the raw value.
func commitDetail(commitStyle string) string {
	switch commitStyle {
	case "", "conventional":
		return "[Conventional Commits](https://www.conventionalcommits.org/)"
	case "gitmoji":
		return "[Gitmoji](https://gitmoji.dev/)"
	case "free":
		return "no strict convention — write clear, descriptive messages."
	default:
		return commitStyle
	}
}

// versioningDetail maps a versioning value to its rendered text. Known values
// get a link or label; unknown values degrade to the raw value. Returns "" so
// an unset versioning yields no row.
func versioningDetail(versioning string) string {
	switch versioning {
	case "":
		return ""
	case "semantic":
		return "[Semantic Versioning](https://semver.org/)"
	case "calver":
		return "[Calendar Versioning](https://calver.org/)"
	case "rolling":
		return "rolling — no discrete releases."
	case "none":
		return "none."
	default:
		return versioning
	}
}

// languageStyleRows emits one row per declared stack tag that carries a
// non-empty per-language style-guide-url. Stack tags listed in pf.Stack come
// first (in declared order); any remaining convention-languages follow in
// deterministic sorted order. The label is the raw stack tag — no embedded
// display-name table — so arbitrary tags stay honest.
func languageStyleRows(conv *pfmodel.ConventionsExtension, pf *projectfile.Document) []conventionItem {
	if conv == nil || len(conv.Languages) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var rows []conventionItem
	add := func(tag, url string) {
		if url == "" || seen[tag] {
			return
		}
		seen[tag] = true
		rows = append(rows, conventionItem{Label: tag + " style", Detail: "<" + url + ">"})
	}
	for _, tag := range pf.Stack {
		if lang, ok := conv.Languages[tag]; ok {
			add(tag, lang.StyleGuideURL)
		}
	}
	remainder := make([]string, 0, len(conv.Languages))
	for tag := range conv.Languages {
		if !seen[tag] {
			remainder = append(remainder, tag)
		}
	}
	sort.Strings(remainder)
	for _, tag := range remainder {
		add(tag, conv.Languages[tag].StyleGuideURL)
	}
	return rows
}

func licenseSPDX(pf *projectfile.Document) string {
	if pf == nil || pf.License == nil {
		return ""
	}
	return pf.License.Spdx
}

func emitDecisionTrace(pf *projectfile.Document, ext *pfmodel.ContributingExtension,
	sections []string, sectionsSrc, docsURL, bugsURL, chatURL, cocURL,
	claURL, securityContact, commitStyle, workflow, versioning, styleGuideURL string,
	authorFollows, authorSites, projectSocials, forgeStars []followLink,
	hasFunding bool,
) {
	genlog.Decision("project_name", pfmodel.DisplayName(pf), "identity.title.en or namespace/name", "")
	genlog.Decision("sections", strings.Join(sections, ", "), sectionsSrc, "[org.projectfile.contributing].sections")
	genlog.Decision("docs_url", valOrUnset(docsURL), "links[type=documentation]", "")
	genlog.Decision("bugs_url", valOrUnset(bugsURL), "links[type=bugs]", "")
	genlog.Decision("chat_url", valOrUnset(chatURL), "links[type=chat] or ext.chat-url", "")
	genlog.Decision("coc_url", valOrUnset(cocURL), "links[type=enforcement] or ext.coc-url", "")
	genlog.Decision("cla_url", valOrUnset(claURL), "links[type=cla] or ext.cla-url", "")
	genlog.Decision("security_contact", valOrUnset(securityContact), "people[role=security].email or links[type=security-report]", "")
	genlog.Decision("commit_style", valOrDefault(commitStyle, "conventional"), "org.projectfile.conventions.commit-style", "")
	genlog.Decision("workflow", valOrDefault(workflow, "(unset, no workflow row)"), "org.projectfile.conventions.workflow", "")
	genlog.Decision("versioning", valOrDefault(versioning, "(unset, no versioning row)"), "org.projectfile.conventions.versioning", "")
	genlog.Decision("style_guide_url", valOrUnset(styleGuideURL), "org.projectfile.conventions.style-guide-url", "")
	genlog.Decision("recommend_to_star", ext.RecommendToStar.Mode(), "org.projectfile.contributing.recommend-to-star", "default: none (bool|map host->bool)")
	genlog.Decision("recommend_to_follow", ext.RecommendToFollow.Mode(), "org.projectfile.contributing.recommend-to-follow", "default: none (bool|map platform->bool)")
	for _, f := range forgeStars {
		genlog.Decision("forge_star", f.URL, "links[type=source-code]", f.URL)
	}
	for _, f := range authorFollows {
		genlog.Decision("author_follow", f.Handle, "people[role=author].handles", f.Site)
	}
	for _, s := range authorSites {
		genlog.Decision("author_site", s.URL, "people[role=author].url", s.Handle)
	}
	for _, s := range projectSocials {
		genlog.Decision("project_social", s.Label, "links[].handles or links[type=social]", s.URL)
	}
	genlog.Decision("has_funding", fmt.Sprintf("%v", hasFunding), "org.projectfile.funding present", "")
}

func valOrUnset(s string) string {
	if s == "" {
		return "(unset, section adapts)"
	}
	return s
}

func valOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// socialHandleOrder defines the preferred display order for social handles
// in the "follow the author" block. Platforms not in this list are ignored.
// Twitter/X is deliberately excluded.
var socialHandleOrder = []string{socialMastodon, socialBluesky, "github", "codeberg", "linkedin", "rss"}

// socialSiteNames maps handle platforms to their language-neutral brand names,
// shown next to the handle so each template localizes only the connective prose.
var socialSiteNames = map[string]string{
	socialMastodon: "Mastodon",
	socialBluesky:  "Bluesky",
	"github":       "GitHub",
	"codeberg":     "Codeberg",
	"linkedin":     "LinkedIn",
	"rss":          "RSS",
}

// resolveAuthorFollows collects one follow link per (author, platform) handle.
// It carries only data (platform, raw handle, brand) — the template composes the
// URL and the localized prose, so no English label is baked in and each language
// renders natively. A handle value may be a string, an array, or a map (spec
// §4.5.1); every string in any shape becomes its own follow link.
func resolveAuthorFollows(pf *projectfile.Document) []followLink {
	if pf == nil {
		return nil
	}
	var out []followLink
	for _, p := range pf.People {
		if !hasRole(p.Roles, "author") || len(p.Handles) == 0 {
			continue
		}
		for _, platform := range socialHandleOrder {
			raw, ok := p.Handles[platform]
			if !ok {
				continue
			}
			for _, handle := range handleStrings(raw) {
				out = append(out, followLink{
					Key:      platform,
					Platform: platform,
					Handle:   handle,
					Site:     socialSiteNames[platform],
				})
			}
		}
	}
	return out
}

// handleStrings flattens a handles value (string | array of strings | map of
// strings, per spec §4.5.1) into the slice of strings it carries. Empty
// strings are dropped. Non-string leaves and unknown shapes yield nothing.
func handleStrings(v any) []string {
	switch val := v.(type) {
	case string:
		if val != "" {
			return []string{val}
		}
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(val))
		for _, s := range val {
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// resolveAuthorSites collects website links from people with role "author" who
// have a url field set. The template shows the host domain next to the link; a
// template cannot parse a URL, so the host is read here as data.
func resolveAuthorSites(pf *projectfile.Document) []followLink {
	if pf == nil {
		return nil
	}
	var out []followLink
	for _, p := range pf.People {
		if !hasRole(p.Roles, "author") || p.URL == "" {
			continue
		}
		host := p.URL
		if u, err := url.Parse(p.URL); err == nil && u.Host != "" {
			host = strings.TrimPrefix(u.Host, "www.")
		}
		out = append(out, followLink{
			Key:      "site",
			Platform: "site",
			URL:      p.URL,
			Handle:   host,
		})
	}
	return out
}

// projectSocialHandleOrder defines the platforms to check for project social
// links. Forges (github, codeberg, gitlab, sourcehut, forgejo) are excluded —
// they are code hosts, not social platforms.
var projectSocialHandleOrder = []string{socialMastodon, socialBluesky}

// resolveProjectSocials collects followable social links for the project from
// links[] entries that carry handles, or from links with type matching a
// social platform.
func resolveProjectSocials(pf *projectfile.Document) []followLink {
	if pf == nil {
		return nil
	}
	var out []followLink
	projectName := pfmodel.DisplayName(pf)
	seen := map[string]bool{}
	for _, platform := range projectSocialHandleOrder {
		// Collect this platform's links, then order by priority (higher first)
		// so a pinned social account surfaces first. The platform ladder stays
		// the outer ordering — priority only reorders peers within a platform.
		var matches []projectfile.Link
		for _, l := range pf.Links {
			if l.Type == platform {
				matches = append(matches, l)
			}
		}
		slices.SortStableFunc(matches, func(a, b projectfile.Link) int {
			return pfmodel.ByPriorityDesc(pfmodel.LinkPriority(a), pfmodel.LinkPriority(b))
		})
		for _, l := range matches {
			if seen[l.URL] {
				continue
			}
			seen[l.URL] = true
			label := fmt.Sprintf("Follow [%s on %s](%s)", projectName, platform, l.URL)
			out = append(out, followLink{Key: platform, Label: label, URL: l.URL})
		}
	}
	return out
}

func hasRole(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}

// filterFollows applies a Toggle to a slice of follow links. When the Toggle
// is unset, defaultAllow governs (true keeps everything, false drops all) so
// each caller owns its resting state. In allowlist mode only links whose Key
// is mapped to true survive; bool true keeps all, bool false drops all.
func filterFollows(in []followLink, tog projectfile.Toggle, defaultAllow bool) []followLink {
	if !tog.Set() {
		if defaultAllow {
			return in
		}
		return nil
	}
	if len(in) == 0 {
		return nil
	}
	out := make([]followLink, 0, len(in))
	for _, l := range in {
		if tog.Allow(l.Key, defaultAllow) {
			out = append(out, l)
		}
	}
	return out
}

// forgeHostFromURL extracts the host portion of a URL for display labels.
// Returns "" for unparseable or empty URLs.
func forgeHostFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// resolveForgeStars collects every links[type=source-code] entry so the
// "Star the project" appreciation block lists each forge the project is
// mirrored to. Projects commonly publish across Codeberg, GitHub, and a
// self-hosted Forgejo instance; surfacing only the primary URL hid the
// others. Returns nil when no source-code links exist — the template then
// falls back to a bare "Star the project" line.
//
// Duplicate URLs (frequent when several includes merge in the same
// repository) collapse to a single entry so each forge appears at most once.
func resolveForgeStars(pf *projectfile.Document) []followLink {
	if pf == nil {
		return nil
	}
	links := pfmodel.LinksByType(pf, "source-code")
	if len(links) == 0 {
		return nil
	}
	// Priority orders the mirrors (higher first), stable so the keep-first URL
	// dedup below is deterministic: a higher-priority entry wins the slot for a
	// duplicate URL, and ties keep document order.
	slices.SortStableFunc(links, func(a, b projectfile.Link) int {
		return pfmodel.ByPriorityDesc(pfmodel.LinkPriority(a), pfmodel.LinkPriority(b))
	})
	out := make([]followLink, 0, len(links))
	seen := map[string]bool{}
	for _, l := range links {
		if l.URL == "" || seen[l.URL] {
			continue
		}
		seen[l.URL] = true
		text := forgeHostFromURL(l.URL)
		if text == "" {
			text = l.URL
		}
		out = append(out, followLink{
			Key:   text,
			Label: fmt.Sprintf("[%s](%s)", text, l.URL),
			URL:   l.URL,
		})
	}
	return out
}

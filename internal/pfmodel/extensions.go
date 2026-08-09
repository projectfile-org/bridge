// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// keyCommitStyle mirrors the core constant of the same name — the on-disk key
// the conventions accessor reads. Declared locally so pfmodel does not reach
// into core internals.
const keyCommitStyle = "commit-style"

// GetCodeOfConductExtension parses `org.projectfile.code-of-conduct`. Returns
// (nil, nil) when absent — the CoC bridge defaults to Contributor Covenant 2.1.
func GetCodeOfConductExtension(doc *projectfile.Document) (*CodeOfConductExtension, error) {
	m, present, err := lookupNS(doc, CodeOfConductExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	return &CodeOfConductExtension{
		Covenant: strVal(m, "covenant"),
		Scope:    strVal(m, "scope"),
	}, nil
}

// GetDEIExtension parses `org.projectfile.dei`. Returns (nil, nil) when the
// namespace is absent. A present-but-disabled namespace (enabled != true)
// returns a zero-value extension with Enabled=false so the DEI bridge can
// gate on it without distinguishing the two cases — DEI.md is opt-in.
func GetDEIExtension(doc *projectfile.Document) (*DEIExtension, error) {
	m, present, err := lookupNS(doc, DEIExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	ext := &DEIExtension{
		Enabled:      boolVal(m, "enabled"),
		Scope:        strVal(m, "scope"),
		LastReviewed: strVal(m, "last-reviewed"),
	}
	// metrics is an open map<string, []string>. Unknown keys are accepted
	// (peaceful cohabitation); the bridge renders any metric it recognizes.
	if raw, ok := m["metrics"].(map[string]any); ok {
		ext.Metrics = make(map[string][]string, len(raw))
		for k, v := range raw {
			ext.Metrics[k] = toStrSlice(v)
		}
	}
	return ext, nil
}

// GetContributingExtension parses `org.projectfile.contributing`. Returns
// (nil, nil) when absent; the CONTRIBUTING.md generator applies its default
// section list at render time.
func GetContributingExtension(doc *projectfile.Document) (*ContributingExtension, error) {
	m, present, err := lookupNS(doc, ContributingExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	return &ContributingExtension{
		Sections:          strListVal(m, "sections"),
		CLAURL:            strVal(m, "cla-url"),
		ChatURL:           strVal(m, "chat-url"),
		CoCURL:            strVal(m, "coc-url"),
		RecommendToFollow: projectfile.ParseToggle(m["recommend-to-follow"]),
		RecommendToStar:   projectfile.ParseToggle(m["recommend-to-star"]),
	}, nil
}

// GetSupportExtension parses `org.projectfile.support`. Returns (nil, nil)
// when absent; the SUPPORT.md generator applies its defaults at render time.
func GetSupportExtension(doc *projectfile.Document) (*SupportExtension, error) {
	m, present, err := lookupNS(doc, SupportExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	ext := &SupportExtension{
		ResponseTime: strVal(m, "response-time"),
	}
	if items, ok := m["eol"].([]any); ok {
		for _, item := range items {
			em, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ext.EOL = append(ext.EOL, EOLEntry{
				Version: strVal(em, "version"),
				Date:    strVal(em, "date"),
			})
		}
	}
	return ext, nil
}

// GetSecurityExtension parses `org.projectfile.security`. Returns (nil, nil)
// when absent — the SECURITY.md template falls back to first-maintainer email.
func GetSecurityExtension(doc *projectfile.Document) (*SecurityExtension, error) {
	m, present, err := lookupNS(doc, SecurityExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	return &SecurityExtension{
		Contact:           strVal(m, "contact"),
		ReportURL:         strVal(m, "report-url"),
		SupportedVersions: strListVal(m, "supported-versions"),
		DisclosureWindow:  strVal(m, "disclosure-window"),
		GPGKey:            strVal(m, "gpg-key"),
		GPGFingerprint:    strVal(m, "gpg-fingerprint"),
		BugBountyURL:      strVal(m, "bug-bounty-url"),
	}, nil
}

// GetReleaseExtension parses `org.projectfile.release`. Returns (nil, nil)
// when absent — the `.releaserc.yaml` renderer refuses rather than emit a
// release config for a project that declared no release intent.
func GetReleaseExtension(doc *projectfile.Document) (*ReleaseExtension, error) {
	m, present, err := lookupNS(doc, ReleaseExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	return &ReleaseExtension{
		TagFormat: strVal(m, "tag-format"),
		Changelog: strVal(m, "changelog"),
		Branches:  parseReleaseBranches(m["branches"]),
	}, nil
}

func parseReleaseBranches(raw any) []ReleaseBranch {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]ReleaseBranch, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, ReleaseBranch{
			Pattern:    strVal(m, "pattern"),
			Channel:    strVal(m, "channel"),
			Prerelease: boolVal(m, "prerelease"),
		})
	}
	return out
}

// GetConventionsExtension parses `org.projectfile.conventions`. Returns
// (nil, nil) when absent. Per-language sub-maps (keys matching stack tags)
// are parsed into Languages; scalar keys are parsed directly.
func GetConventionsExtension(doc *projectfile.Document) (*ConventionsExtension, error) {
	m, present, err := lookupNS(doc, ConventionsExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	ext := &ConventionsExtension{
		CommitStyle:   strVal(m, keyCommitStyle),
		Workflow:      strVal(m, "workflow"),
		Versioning:    strVal(m, "versioning"),
		StyleGuideURL: strVal(m, "style-guide-url"),
	}
	for k, v := range m {
		if k == keyCommitStyle || k == "workflow" || k == "versioning" || k == "style-guide-url" {
			continue
		}
		sub, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if ext.Languages == nil {
			ext.Languages = map[string]LangConventions{}
		}
		ext.Languages[k] = LangConventions{
			StyleGuideURL: strVal(sub, "style-guide-url"),
		}
	}
	return ext, nil
}

// ConventionsStyleGuideURL resolves the effective style-guide-url for a
// project: top-level conventions.style-guide-url wins; otherwise the first
// per-language entry whose key matches a stack tag. Returns "" when no URL
// is found.
func ConventionsStyleGuideURL(conv *ConventionsExtension, stack []string) string {
	if conv == nil {
		return ""
	}
	if conv.StyleGuideURL != "" {
		return conv.StyleGuideURL
	}
	for _, tag := range stack {
		if lang, ok := conv.Languages[tag]; ok && lang.StyleGuideURL != "" {
			return lang.StyleGuideURL
		}
	}
	return ""
}

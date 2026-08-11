// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package coc

import (
	"fmt"
	"os"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Bridge renders CODE_OF_CONDUCT.md. Covenant + scope come from
// [org.projectfile.code-of-conduct] (defaulting to Contributor Covenant 2.1
// when the namespace is absent); the contact email is resolved from
// [org.projectfile.security].contact or a person with the 'community' role.
type Bridge struct{}

const filenameCOC = core.FileCodeOfConduct

// Covenant values the bridge can render. Only Contributor Covenant 2.1 ships a
// template today; a project adopting another covenant SHOULD point a
// links[type=conduct-full-text] entry at its own text instead of setting an
// unsupported covenant here.
const covenantContributorCovenant = "contributor-covenant-2.1"

func (Bridge) Name() string             { return "coc" }
func (Bridge) Filename() string         { return filenameCOC }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filenameCOC, "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameCOC, filenameCOC))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return core.PathOrDefault(dir, filenameCOC, filenameCOC)
}

func (Bridge) RequiredFields(pf *projectfile.Document) []core.Missing {
	if email, _ := contactEmail(pf); email != "" {
		return nil
	}
	return []core.Missing{{
		Field: "[org.projectfile.security].contact",
		Hint:  "moderator email address (e.g. conduct@example.org)",
		Setter: func(v string) error {
			if v == "" {
				return fmt.Errorf("contact email is required")
			}
			ext, _ := pfmodel.GetSecurityExtension(pf)
			if ext == nil {
				ext = &pfmodel.SecurityExtension{}
			}
			ext.Contact = v
			projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, securityExtensionToMap(ext))
			return nil
		},
	}}
}

func (b Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	email, emailSrc := contactEmail(pf)
	if email == "" {
		return core.Output{}, fmt.Errorf("CODE_OF_CONDUCT.md: no contact email available — add a [[people]] entry with role 'community' (or 'maintainer') and email, or set [org.projectfile.security].contact")
	}
	// Covenant + scope come from [org.projectfile.code-of-conduct]; absent
	// defaults to Contributor Covenant 2.1 (the only shipped template). An
	// unsupported covenant fails loudly rather than silently rendering CC 2.1
	// against the project's declared intent.
	covenant := covenantContributorCovenant
	scope := "project-and-spaces"
	if ext, _ := pfmodel.GetCodeOfConductExtension(pf); ext != nil {
		if ext.Covenant != "" {
			covenant = ext.Covenant
		}
		if ext.Scope != "" {
			scope = ext.Scope
		}
	}
	if covenant != covenantContributorCovenant {
		return core.Output{}, fmt.Errorf("CODE_OF_CONDUCT.md: unsupported covenant %q — only %q ships a template; for other covenants point a links[type=conduct-full-text] entry at your own text", covenant, covenantContributorCovenant)
	}
	genlog.Decision("project_name", pfmodel.DisplayName(pf), "identity.title.en or namespace/name", "")
	genlog.Decision("contact_email", email, emailSrc, "[org.projectfile.security].contact")
	genlog.Decision("covenant", covenant, "[org.projectfile.code-of-conduct].covenant", "default")
	genlog.Decision("scope", scope, "[org.projectfile.code-of-conduct].scope", "default")
	return core.RenderLocalized(pf, core.LocalizedSpec{
		Filename: filenameCOC,
		Policy:   b.Policy(),
		Langs:    pfmodel.Languages(pf),
		View: func(lang string) any {
			strLang := core.ResolveLang(lang, pf)
			return cocView{
				ProjectName:  pfmodel.DisplayNameForLang(pf, strLang),
				ContactEmail: email,
				Covenant:     covenant,
				Scope:        scope,
			}
		},
	}, opts)
}

func contactEmail(pf *projectfile.Document) (string, string) {
	sec, _ := pfmodel.GetSecurityExtension(pf)
	if sec != nil && sec.Contact != "" {
		return sec.Contact, "[org.projectfile.security].contact"
	}
	return pfmodel.ContactEmail(pf, projectfile.RoleCommunity)
}

// securityExtensionToMap mirrors the helper in the security bridge — kept
// local so each bridge package stays standalone.
func securityExtensionToMap(ext *pfmodel.SecurityExtension) map[string]any {
	m := map[string]any{}
	if ext.Contact != "" {
		m["contact"] = ext.Contact
	}
	if ext.ReportURL != "" {
		m["report-url"] = ext.ReportURL
	}
	if len(ext.SupportedVersions) > 0 {
		out := make([]any, len(ext.SupportedVersions))
		for i, v := range ext.SupportedVersions {
			out[i] = v
		}
		m["supported-versions"] = out
	}
	if ext.DisclosureWindow != "" {
		m["disclosure-window"] = ext.DisclosureWindow
	}
	if ext.GPGKey != "" {
		m["gpg-key"] = ext.GPGKey
	}
	if ext.BugBountyURL != "" {
		m["bug-bounty-url"] = ext.BugBountyURL
	}
	return m
}

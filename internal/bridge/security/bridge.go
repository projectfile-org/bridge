// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package security

import (
	"fmt"
	"os"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const filenameSecurity = core.FileSecurity

// Bridge renders SECURITY.md from [org.projectfile.security].
type Bridge struct{}

func (Bridge) Name() string             { return "security" }
func (Bridge) Filename() string         { return filenameSecurity }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filenameSecurity, "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameSecurity, filenameSecurity))
	return err == nil
}

// FullPath honours $XDG_CONFIG_HOME/projectfile/cli.toml
// [generate.defaults."SECURITY.md"].path when set; otherwise writes to root
// (GitHub auto-discovers SECURITY.md in `.`, `docs/`, or `.github/`).
func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return core.PathOrDefault(dir, filenameSecurity, filenameSecurity)
}

func (Bridge) RequiredFields(pf *projectfile.Document) []core.Missing {
	if contact, _ := contactEmail(pf, projectfile.RoleSecurity); contact != "" {
		return nil
	}
	if userconfigEmail := userconfig.Load().Identity.Email; userconfigEmail != "" {
		return nil
	}
	return []core.Missing{{
		Field: "[org.projectfile.security].contact",
		Hint:  "email address for security reports (e.g. security@example.org)",
		Setter: func(v string) error {
			if v == "" {
				return fmt.Errorf("security contact is required")
			}
			ext, _ := pfmodel.GetSecurityExtension(pf)
			if ext == nil {
				ext = &pfmodel.SecurityExtension{}
			}
			ext.Contact = v
			projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, extensionToMap(ext))
			return nil
		},
	}}
}

func (Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	ext, err := pfmodel.GetSecurityExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	if ext == nil {
		ext = &pfmodel.SecurityExtension{}
	}
	core.ApplyUserSecurityFallback(ext)
	contact, contactSrc := contactEmail(pf, projectfile.RoleSecurity)
	if contact == "" && ext.Contact != "" {
		contact, contactSrc = ext.Contact, "user-config [identity].email"
	}
	if contact == "" && ext.ReportURL == "" {
		return core.Output{}, fmt.Errorf("%s: no contact channel — set [org.projectfile.security].contact, .report-url, or add a [[people]] entry with role 'security' (or 'maintainer') and email", filenameSecurity)
	}
	// The disclosure window's fallback is prose, so the template owns it:
	// passing the field through empty lets each localized template word its
	// own default instead of dropping an English "30 days" into SECURITY.es.md.
	window := ext.DisclosureWindow
	windowSrc := "[org.projectfile.security].disclosure-window"
	if window == "" {
		windowSrc = "default (per-language, from template)"
	}

	emitDecisionTrace(contact, contactSrc, ext, window, windowSrc)

	return core.RenderLocalized(pf, core.LocalizedSpec{
		Filename: filenameSecurity,
		Langs:    pfmodel.Languages(pf),
		View: func(lang string) any {
			return securityView{
				Marker:            core.MarkerHTML,
				ProjectName:       pfmodel.DisplayNameForLang(pf, lang),
				Contact:           contact,
				ReportURL:         ext.ReportURL,
				SupportedVersions: ext.SupportedVersions,
				DisclosureWindow:  window,
				GPGKey:            ext.GPGKey,
				BugBountyURL:      ext.BugBountyURL,
			}
		},
	}, opts)
}

func contactEmail(pf *projectfile.Document, primaryRole string) (string, string) {
	sec, _ := pfmodel.GetSecurityExtension(pf)
	if sec != nil && sec.Contact != "" {
		return sec.Contact, "[org.projectfile.security].contact"
	}
	return pfmodel.ContactEmail(pf, primaryRole)
}

// extensionToMap inverts GetSecurityExtension. Used by Required setters to
// write back through the standard extension surface so the dotted-vs-flat
// round-trip invariant holds.
func extensionToMap(ext *pfmodel.SecurityExtension) map[string]any {
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

func emitDecisionTrace(contact, contactSrc string, ext *pfmodel.SecurityExtension, window, windowSrc string) {
	genlog.Decision("contact", valueOrEmpty(contact), contactSrc, "[org.projectfile.security].contact")
	genlog.Decision("report-url", valueOrEmpty(ext.ReportURL), "[org.projectfile.security].report-url", "")
	genlog.Decision("disclosure-window", window, windowSrc, "[org.projectfile.security].disclosure-window")
	if len(ext.SupportedVersions) == 0 {
		genlog.Decision("supported-versions", "(unset, section omitted)", "[org.projectfile.security].supported-versions", "")
	} else {
		genlog.Decision("supported-versions", fmt.Sprintf("%v", ext.SupportedVersions), "[org.projectfile.security].supported-versions", "")
	}
	if ext.GPGKey != "" {
		genlog.Decision("gpg-key", ext.GPGKey, "[org.projectfile.security].gpg-key", "")
	}
	if ext.BugBountyURL != "" {
		genlog.Decision("bug-bounty-url", ext.BugBountyURL, "[org.projectfile.security].bug-bounty-url", "")
	}
}

func valueOrEmpty(s string) string {
	if s == "" {
		return "(unset, omitted)"
	}
	return s
}

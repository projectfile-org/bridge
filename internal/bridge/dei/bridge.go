// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package dei

import (
	"fmt"
	"os"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Bridge renders DEI.md — the CHAOSS / badging DEI Project Statement. It is a
// marker community-health file: generated once from the fixed CHAOSS
// boilerplate plus the project's per-metric effort bullets, then owned and
// edited by the project. Generation is opt-in: [org.projectfile.dei].enabled
// must be true, otherwise the bridge is a no-op (DEI.md never deletes).
type Bridge struct{}

const filenameDEI = core.FileDEI

func (Bridge) Name() string             { return "dei" }
func (Bridge) Filename() string         { return filenameDEI }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filenameDEI, "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameDEI, filenameDEI))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return core.PathOrDefault(dir, filenameDEI, filenameDEI)
}

// RequiredFields is nil: DEI.md is a marker doc with no hard-required field.
// The reporting contact resolves gracefully — when none is found, the template
// emits a placeholder prompt rather than failing generation (unlike CoC's hard
// fail, a DEI file with an unfilled reporting slot is still a usable draft).
func (Bridge) RequiredFields(_ *projectfile.Document) []core.Missing { return nil }

func (Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	ext, err := pfmodel.GetDEIExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	// enabled is the hard gate: a namespace present without enabled:true (or
	// absent entirely) means the project has not opted in, so we emit nothing.
	// We never delete an existing DEI.md — the marker policy handles that.
	if ext == nil || !ext.Enabled {
		genlog.Decision("enabled", "false", "[org.projectfile.dei].enabled", "skipped — DEI.md is opt-in")
		return core.Output{Files: map[string][]byte{}}, nil
	}

	contact, contactSrc := pfmodel.ContactEmail(pf, projectfile.RoleCommunity)

	emitDecisionTrace(ext, contact, contactSrc)

	return core.RenderLocalized(pf, core.LocalizedSpec{
		Filename:  filenameDEI,
		Langs:     pfmodel.Languages(pf),
		HowToLink: ext.HowToLink,
		SeeAlso:   func(lang string) []core.SeeAlsoLink { return core.SeeAlsoFor(pf, "dei", lang) },
		View: func(lang string) any {
			strLang := core.ResolveLang(lang, pf)
			return deiView{
				ProjectName:  pfmodel.DisplayNameForLang(pf, strLang),
				Scope:        ext.Scope,
				LastReviewed: ext.LastReviewed,
				ContactEmail: contact,

				ProjectAccess:       metric(ext, "project-access"),
				CommTransparency:    metric(ext, "communication-transparency"),
				NewcomerExperiences: metric(ext, "newcomer-experiences"),
				InclusiveLeadership: metric(ext, "inclusive-leadership"),
				OtherEfforts:        metric(ext, "other"),
				HasOther:            hasMetric(ext, "other"),
			}
		},
	}, opts)
}

// metric returns the bullets for a CHAOSS metric slug, or nil when the project
// did not declare it. A nil slice is the template's signal to render the
// per-language placeholder bullets (so enabled:true alone yields a complete,
// editable DEI.md across all four required metrics).
func metric(ext *pfmodel.DEIExtension, slug string) []string {
	if ext == nil || ext.Metrics == nil {
		return nil
	}
	return ext.Metrics[slug]
}

// hasMetric reports whether a metric was explicitly declared. Used only for
// the optional `other` section, which is omitted entirely when absent (unlike
// the four CHAOSS metrics, which always render with placeholders).
func hasMetric(ext *pfmodel.DEIExtension, slug string) bool {
	if ext == nil || ext.Metrics == nil {
		return false
	}
	_, ok := ext.Metrics[slug]
	return ok
}

func emitDecisionTrace(ext *pfmodel.DEIExtension, contact, contactSrc string) {
	genlog.Decision("enabled", fmt.Sprintf("%v", ext.Enabled), "[org.projectfile.dei].enabled", "")
	scopeSrc := "[org.projectfile.dei].scope"
	if ext.Scope == "" {
		scopeSrc = "default (from template)"
	}
	genlog.Decision("scope", valueOrEmpty(ext.Scope), scopeSrc, "[org.projectfile.dei].scope")
	reviewedSrc := "[org.projectfile.dei].last-reviewed"
	if ext.LastReviewed == "" {
		reviewedSrc = "default ([Enter Date] placeholder)"
	}
	genlog.Decision("last-reviewed", valueOrEmpty(ext.LastReviewed), reviewedSrc, "[org.projectfile.dei].last-reviewed")
	genlog.Decision("contact", valueOrEmpty(contact), contactSrc, "people[roles=community]")
}

func valueOrEmpty(s string) string {
	if s == "" {
		return "(unset, placeholder)"
	}
	return s
}

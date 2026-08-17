// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package aipolicy

import (
	"fmt"
	"os"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// filenameAIPolicy is the DEFAULT on-disk name and the name the templates are
// keyed by. A document renaming its own policy file (org.projectfile.ai.
// filename) changes the output name only — the prose it renders is the same.
const filenameAIPolicy = core.FileAIPolicy

// aiPolicyLinkType is the links[].type value for a project's external AI
// governance page — the escape hatch a project whose real policy is a
// governance page uses instead of (or alongside) the declared fields.
const aiPolicyLinkType = "ai-policy"

// Bridge renders the AI policy file — the project's stance on AI/LLM use —
// from org.projectfile.ai. Absence of the namespace means NO DECLARED
// POLICY: the bridge emits nothing and never invents a permissive default.
type Bridge struct{}

func (Bridge) Name() string { return "ai-policy" }

// Filename is the registry/CLI identity, so it is the DEFAULT name rather
// than any one document's choice: the dispatcher resolves a bridge before it
// has read a projectfile. Aliases cover the other spellings in the wild, so
// `pf-bridge AI.md` finds this bridge whatever the project calls its file.
func (Bridge) Filename() string         { return filenameAIPolicy }
func (Bridge) Aliases() []string        { return []string{"AI.md", "AI-POLICY.md", "LLM.md"} }
func (Bridge) Labels() (string, string) { return filenameAIPolicy, "projectfile" }

// Policy is Marker, not ScaffoldOnce: the policy file is a projection of
// declared fields, so flipping `attitude` MUST change the file on the next
// run. A policy that became the maintainer's own document (ScaffoldOnce)
// would be exactly the drift this bridge exists to kill.
func (Bridge) Policy() core.Policy { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameAIPolicy, filenameAIPolicy))
	return err == nil
}

func (Bridge) FullPath(dir string, pf *projectfile.Document) string {
	name := pfmodel.AIPolicyFilename(pf)
	if name == "" {
		name = filenameAIPolicy
	}
	return core.PathOrDefault(dir, filenameAIPolicy, name)
}

// RequiredFields is nil: an absent namespace is a valid, meaningful state
// (no declared policy), never a gap the dispatcher should prompt to fill.
func (Bridge) RequiredFields(_ *projectfile.Document) []core.Missing { return nil }

func (Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	ext, err := pfmodel.GetAIExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	if ext == nil {
		genlog.Decision("namespace", "absent", "org.projectfile.ai",
			"skipped — absence is not permission, never rendered as one")
		return core.Output{Files: map[string][]byte{}}, nil
	}
	warnUnknownStances(ext)

	outName := pfmodel.AIPolicyFilename(pf)
	rows := buildActivityRows(ext)
	useRows := buildProjectUseRows(ext)
	signals := dedupSignals(ext.ContentSignals)
	obligations := dedupSignals(ext.Obligations)
	details := dedupSignals(ext.DiscloseDetails)
	enforcement := dedupSignals(ext.Enforcement)
	policyURL := pfmodel.LinkURL(pf, aiPolicyLinkType)
	contact, contactSrc := pfmodel.ContactEmail(pf, projectfile.RoleCommunity)

	emitDecisionTrace(ext, outName, rows, useRows, signals, enforcement, policyURL, contact, contactSrc)

	return core.RenderLocalized(pf, core.LocalizedSpec{
		Filename: outName,
		Template: filenameAIPolicy,
		Langs:    pfmodel.Languages(pf),
		View: func(lang string) any {
			strLang := core.ResolveLang(lang, pf)
			return policyView{
				ProjectName:      pfmodel.DisplayNameForLang(pf, strLang),
				Statement:        projectfile.ExtractLocalizedStringForLang(ext.Statement, strLang),
				PolicyURL:        policyURL,
				Attitude:         ext.Attitude,
				Autonomy:         ext.Autonomy,
				AppliesTo:        ext.AppliesTo,
				DiscloseRequired: ext.DiscloseRequired,
				DiscloseTrailer:  ext.DiscloseTrailer,
				DiscloseDetails:  details,
				Obligations:      obligations,
				IssueRequired:    ext.IssueRequired,
				ExcludedLabels:   ext.ExcludedLabels,
				Enforcement:      enforcement,
				Rows:             rows,
				ProjectUseRows:   useRows,
				ContentSignals:   signals,
				ContactEmail:     contact,
				SupportFile:      core.RelLinkSibling(core.FileSupport, lang, outName),
			}
		},
	}, opts)
}

func emitDecisionTrace(ext *pfmodel.AIExtension, outName string, rows []activityRow, useRows []projectUseRow, signals, enforcement []string, policyURL, contact, contactSrc string) {
	genlog.Decision("filename", outName, "[org.projectfile.ai].filename", "default: "+filenameAIPolicy)
	genlog.Decision("attitude", ext.Attitude, "[org.projectfile.ai].attitude", "")
	genlog.Decision("autonomy", ext.Autonomy, "[org.projectfile.ai].autonomy", "default: any")
	genlog.Decision("applies_to", ext.AppliesTo, "[org.projectfile.ai].applies-to", "default: everyone")
	genlog.Decision("disclose_required", fmt.Sprintf("%v", ext.DiscloseRequired), "[org.projectfile.ai].disclose-required", "")
	for _, r := range rows {
		genlog.Decision("activity_override", r.Activity+" -> "+r.Stance, "[org.projectfile.ai].activities."+r.Activity, "differs from attitude")
	}
	for _, r := range useRows {
		genlog.Decision("project_use", r.Activity+" -> "+r.Autonomy, "[org.projectfile.ai].project-use."+r.Activity, "internal direction")
	}
	if len(enforcement) == 0 {
		genlog.Decision("enforcement", "(unset, no consequence stated)", "[org.projectfile.ai].enforcement", "")
	} else {
		genlog.Decision("enforcement", strings.Join(enforcement, " → "), "[org.projectfile.ai].enforcement", "declared order is the escalation order")
	}
	if len(signals) == 0 {
		genlog.Decision("content_signals", "(unset, section states the absence)", "[org.projectfile.ai].content-signals", "")
	} else {
		genlog.Decision("content_signals", strings.Join(signals, ", "), "[org.projectfile.ai].content-signals", "")
	}
	genlog.Decision("policy_url", valOrUnset(policyURL), "links[type=ai-policy]", "")
	genlog.Decision("contact", valOrUnset(contact), contactSrc, "people[roles=community]")
}

func valOrUnset(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}

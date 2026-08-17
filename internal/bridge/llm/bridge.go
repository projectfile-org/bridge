// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package llm

import (
	"fmt"
	"os"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const filenameLLM = core.FileLLM

// aiPolicyLinkType is the links[].type value for a project's external AI
// governance page — the escape hatch a project whose real policy is a
// governance page uses instead of (or alongside) the declared fields.
const aiPolicyLinkType = "ai-policy"

// Bridge renders LLM.md — the project's stance on AI/LLM use — from
// org.projectfile.llm. Absence of the namespace means NO DECLARED POLICY:
// the bridge emits nothing and never invents a permissive default.
type Bridge struct{}

func (Bridge) Name() string             { return "llm" }
func (Bridge) Filename() string         { return filenameLLM }
func (Bridge) Aliases() []string        { return []string{"AI.md"} }
func (Bridge) Labels() (string, string) { return filenameLLM, "projectfile" }

// Policy is Marker, not ScaffoldOnce: LLM.md is a projection of declared
// fields, so flipping `attitude` MUST change the file on the next run. A
// policy that became the maintainer's own document (ScaffoldOnce) would be
// exactly the drift this bridge exists to kill.
func (Bridge) Policy() core.Policy { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameLLM, filenameLLM))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return core.PathOrDefault(dir, filenameLLM, filenameLLM)
}

// RequiredFields is nil: an absent namespace is a valid, meaningful state
// (no declared policy), never a gap the dispatcher should prompt to fill.
func (Bridge) RequiredFields(_ *projectfile.Document) []core.Missing { return nil }

func (Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	ext, err := pfmodel.GetLLMExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	if ext == nil {
		genlog.Decision("namespace", "absent", "org.projectfile.llm",
			"skipped — absence is not permission, never rendered as one")
		return core.Output{Files: map[string][]byte{}}, nil
	}
	warnUnknownStances(ext)

	rows := buildActivityRows(ext)
	signals := dedupSignals(ext.ContentSignals)
	policyURL := pfmodel.LinkURL(pf, aiPolicyLinkType)
	contact, contactSrc := pfmodel.ContactEmail(pf, projectfile.RoleCommunity)

	emitDecisionTrace(ext, rows, signals, policyURL, contact, contactSrc)

	return core.RenderLocalized(pf, core.LocalizedSpec{
		Filename: filenameLLM,
		Langs:    pfmodel.Languages(pf),
		View: func(lang string) any {
			strLang := core.ResolveLang(lang, pf)
			return llmView{
				ProjectName:      pfmodel.DisplayNameForLang(pf, strLang),
				Statement:        projectfile.ExtractLocalizedStringForLang(ext.Statement, strLang),
				PolicyURL:        policyURL,
				Attitude:         ext.Attitude,
				Autonomy:         ext.Autonomy,
				DiscloseRequired: ext.DiscloseRequired,
				DiscloseTrailer:  ext.DiscloseTrailer,
				Rows:             rows,
				ContentSignals:   signals,
				ContactEmail:     contact,
				SupportFile:      core.RelLinkSibling(core.FileSupport, lang, filenameLLM),
			}
		},
	}, opts)
}

func emitDecisionTrace(ext *pfmodel.LLMExtension, rows []activityRow, signals []string, policyURL, contact, contactSrc string) {
	genlog.Decision("attitude", ext.Attitude, "[org.projectfile.llm].attitude", "")
	genlog.Decision("autonomy", ext.Autonomy, "[org.projectfile.llm].autonomy", "default: any")
	genlog.Decision("disclose_required", fmt.Sprintf("%v", ext.DiscloseRequired), "[org.projectfile.llm].disclose-required", "")
	for _, r := range rows {
		genlog.Decision("activity_override", r.Activity+" -> "+r.Stance, "[org.projectfile.llm]."+r.Activity, "differs from attitude")
	}
	if len(signals) == 0 {
		genlog.Decision("content_signals", "(unset, section states the absence)", "[org.projectfile.llm].content-signals", "")
	} else {
		genlog.Decision("content_signals", strings.Join(signals, ", "), "[org.projectfile.llm].content-signals", "")
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

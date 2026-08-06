// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package funding

import (
	"fmt"
	"os"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	fundingKofi            = "ko-fi"
	fundingCommunityBridge = "community-bridge"
	fundingIssueHunt       = "issuehunt"
	fundingLFXCrowd        = "lfx-crowdfunding"
	fundingBuyMeACoffee    = "buy-me-a-coffee"
	fundingLiberapay       = "liberapay"
	fundingPatreon         = "patreon"
	fundingTidelift        = "tidelift"
	fundingPolar           = "polar"
	fundingThanksDev       = "thanks-dev"
)

// Bridge renders `.github/FUNDING.yml` (or the user's override path) from
// [org.projectfile.funding].
type Bridge struct{}

func (Bridge) Name() string             { return "funding" }
func (Bridge) Filename() string         { return "FUNDING.yml" }
func (Bridge) Aliases() []string        { return []string{"FUNDING.yaml"} }
func (Bridge) Labels() (string, string) { return "FUNDING.yml", "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(resolvePath(dir, nil))
	return err == nil
}

// FullPath resolves the on-disk write location. Precedence: in-pf
// [org.projectfile.funding].path → user config → built-in `.github/FUNDING.yml`.
func (Bridge) FullPath(dir string, pf *projectfile.Document) string {
	return resolvePath(dir, pf)
}

func (Bridge) RequiredFields(pf *projectfile.Document) []core.Missing {
	ext, _ := pfmodel.GetFundingExtension(pf)
	if ext == nil {
		ext = &pfmodel.FundingExtension{}
	}
	core.ApplyUserFundingFallback(ext)
	if hasProviders(ext) {
		return nil
	}
	return []core.Missing{
		{
			Field: "[org.projectfile.funding].github (handle)",
			Hint:  "GitHub Sponsors handle (e.g. octocat) — leave blank if none",
			Setter: func(v string) error {
				if v != "" {
					ext.GitHub = []string{v}
				}
				return saveFundingExtension(pf, ext)
			},
		},
		{
			Field: "[org.projectfile.funding].open-collective",
			Hint:  "Open Collective slug (e.g. my-project) — leave blank if none",
			Setter: func(v string) error {
				if v != "" {
					ext.OpenCollective = v
				}
				return saveFundingExtension(pf, ext)
			},
		},
		{
			Field: "[org.projectfile.funding].other (slug=value; slug=value)",
			Hint: "Other providers (semicolon-separated): patreon, ko-fi, liberapay, " +
				"tidelift, community-bridge, issuehunt, lfx-crowdfunding, polar, " +
				"buy-me-a-coffee, thanks-dev, custom — leave blank if none",
			Setter: func(v string) error {
				if err := parseOthers(v, ext); err != nil {
					return err
				}
				if !hasProviders(ext) {
					return fmt.Errorf("no funding provider provided — fill at least one of github, open-collective, or the other-providers field")
				}
				return saveFundingExtension(pf, ext)
			},
		},
	}
}

func (Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	ext, err := pfmodel.GetFundingExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	if ext == nil {
		ext = &pfmodel.FundingExtension{}
	}
	core.ApplyUserFundingFallback(ext)
	if !hasProviders(ext) {
		// Empty FUNDING.yml is worse than no file: it advertises "this
		// project accepts no funding" while looking like a real config.
		return core.Output{}, fmt.Errorf(`FUNDING.yml: no funding providers configured.
  Add [org.projectfile.funding] to projectfile.toml, e.g.
    ["org.projectfile.funding"]
    github = ["your-handle"]
  Or use custom URLs for non-GitHub platforms:
    ["org.projectfile.funding"]
    custom = ["https://buymeacoffee.com/..."]
  Spec reference: https://projectfile.org/spec/v1#funding`)
	}
	genlog.Decision("providers", summary(ext), "[org.projectfile.funding]", "")
	body, err := core.Render(opts.Dir, "FUNDING.yml.tmpl", fundingView{
		Marker:          core.Marker,
		GitHub:          ext.GitHub,
		Patreon:         ext.Patreon,
		KoFi:            ext.KoFi,
		Liberapay:       ext.Liberapay,
		Tidelift:        ext.Tidelift,
		CommunityBridge: ext.CommunityBridge,
		IssueHunt:       ext.IssueHunt,
		OpenCollective:  ext.OpenCollective,
		LFXCrowdfunding: ext.LFXCrowdfunding,
		Polar:           ext.Polar,
		BuyMeACoffee:    ext.BuyMeACoffee,
		ThanksDev:       ext.ThanksDev,
		Custom:          ext.Custom,
	})
	if err != nil {
		return core.Output{}, err
	}
	out := append([]byte(core.YAMLDocStart+core.REUSEHeader(pf, core.StyleHash)), body...)
	rel := relPath(pf)
	return core.Output{Files: map[string][]byte{rel: out}}, nil
}

// resolvePath returns the absolute path; relPath returns the same path
// relative to dir for use as the Output key.
func resolvePath(dir string, pf *projectfile.Document) string {
	rel := ".github/FUNDING.yml"
	if pf != nil {
		if ext, _ := pfmodel.GetFundingExtension(pf); ext != nil && ext.Path != "" {
			rel = ext.Path
		}
	}
	return core.PathOrDefault(dir, "FUNDING.yml", rel)
}

func relPath(pf *projectfile.Document) string {
	if pf != nil {
		if ext, _ := pfmodel.GetFundingExtension(pf); ext != nil && ext.Path != "" {
			return ext.Path
		}
	}
	return ".github/FUNDING.yml"
}

// saveFundingExtension reflects FundingExtension back into pf.Extensions via
// the shared SetExtension helper so the dotted-vs-flat round-trip invariant
// holds.
func saveFundingExtension(pf *projectfile.Document, ext *pfmodel.FundingExtension) error {
	m := map[string]any{}
	if len(ext.GitHub) > 0 {
		m["github"] = stringsToAnys(ext.GitHub)
	}
	for k, v := range map[string]string{
		fundingPatreon: ext.Patreon, fundingKofi: ext.KoFi, fundingLiberapay: ext.Liberapay,
		fundingTidelift: ext.Tidelift, fundingCommunityBridge: ext.CommunityBridge,
		fundingIssueHunt: ext.IssueHunt, "open-collective": ext.OpenCollective,
		fundingLFXCrowd: ext.LFXCrowdfunding, fundingPolar: ext.Polar,
		fundingBuyMeACoffee: ext.BuyMeACoffee, fundingThanksDev: ext.ThanksDev,
		"path": ext.Path,
	} {
		if v != "" {
			m[k] = v
		}
	}
	if len(ext.Custom) > 0 {
		m["custom"] = stringsToAnys(ext.Custom)
	}
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, m)
	return nil
}

// secondarySlugs maps FUNDING.yml platform identifiers to a mutator that
// assigns the captured value onto the FundingExtension. Used by parseOthers
// so unknown slugs error out instead of being silently dropped.
var secondarySlugs = map[string]func(*pfmodel.FundingExtension, string){
	fundingPatreon:         func(e *pfmodel.FundingExtension, v string) { e.Patreon = v },
	fundingKofi:            func(e *pfmodel.FundingExtension, v string) { e.KoFi = v },
	fundingLiberapay:       func(e *pfmodel.FundingExtension, v string) { e.Liberapay = v },
	fundingTidelift:        func(e *pfmodel.FundingExtension, v string) { e.Tidelift = v },
	fundingCommunityBridge: func(e *pfmodel.FundingExtension, v string) { e.CommunityBridge = v },
	fundingIssueHunt:       func(e *pfmodel.FundingExtension, v string) { e.IssueHunt = v },
	fundingLFXCrowd:        func(e *pfmodel.FundingExtension, v string) { e.LFXCrowdfunding = v },
	fundingPolar:           func(e *pfmodel.FundingExtension, v string) { e.Polar = v },
	fundingBuyMeACoffee:    func(e *pfmodel.FundingExtension, v string) { e.BuyMeACoffee = v },
	fundingThanksDev:       func(e *pfmodel.FundingExtension, v string) { e.ThanksDev = v },
	"custom": func(e *pfmodel.FundingExtension, v string) {
		urls := strings.Split(v, ",")
		out := make([]string, 0, len(urls))
		for _, u := range urls {
			if u = strings.TrimSpace(u); u != "" {
				out = append(out, u)
			}
		}
		e.Custom = out
	},
}

func parseOthers(input string, ext *pfmodel.FundingExtension) error {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}
	for _, raw := range strings.Split(trimmed, ";") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			return fmt.Errorf("expected slug=value, got %q", entry)
		}
		slug := strings.TrimSpace(key)
		val := strings.TrimSpace(value)
		apply, known := secondarySlugs[slug]
		if !known {
			return fmt.Errorf("unknown funding slug %q (allowed: patreon, ko-fi, liberapay, tidelift, community-bridge, issuehunt, lfx-crowdfunding, polar, buy-me-a-coffee, thanks-dev, custom)", slug)
		}
		if val == "" {
			return fmt.Errorf("empty value for slug %q", slug)
		}
		genlog.Decision("funding-other", slug, "user-input", val)
		apply(ext, val)
	}
	return nil
}

func hasProviders(ext *pfmodel.FundingExtension) bool {
	if ext == nil {
		return false
	}
	if len(ext.GitHub) > 0 || len(ext.Custom) > 0 {
		return true
	}
	for _, s := range []string{
		ext.Patreon, ext.KoFi, ext.Liberapay, ext.Tidelift,
		ext.CommunityBridge, ext.IssueHunt, ext.OpenCollective,
		ext.LFXCrowdfunding, ext.Polar, ext.BuyMeACoffee, ext.ThanksDev,
	} {
		if s != "" {
			return true
		}
	}
	return false
}

func summary(ext *pfmodel.FundingExtension) string {
	parts := []string{}
	if n := len(ext.GitHub); n > 0 {
		parts = append(parts, fmt.Sprintf("github(%d)", n))
	}
	for label, val := range map[string]string{
		fundingPatreon:         ext.Patreon,
		fundingKofi:            ext.KoFi,
		fundingLiberapay:       ext.Liberapay,
		fundingTidelift:        ext.Tidelift,
		fundingCommunityBridge: ext.CommunityBridge,
		fundingIssueHunt:       ext.IssueHunt,
		"open-collective":      ext.OpenCollective,
		fundingLFXCrowd:        ext.LFXCrowdfunding,
		fundingPolar:           ext.Polar,
		fundingBuyMeACoffee:    ext.BuyMeACoffee,
		fundingThanksDev:       ext.ThanksDev,
	} {
		if val != "" {
			parts = append(parts, label)
		}
	}
	if n := len(ext.Custom); n > 0 {
		parts = append(parts, fmt.Sprintf("custom(%d)", n))
	}
	return strings.Join(parts, ", ")
}

func stringsToAnys(in []string) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

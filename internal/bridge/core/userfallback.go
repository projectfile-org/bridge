// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Per-target user-config fallbacks. Each helper gap-fills empty fields on
// an in-pf extension from the per-user defaults at
// $XDG_CONFIG_HOME/projectfile/cli.<ext>. Precedence is mechanical:
// in-pf value (non-empty) > user-config value > renderer's hard-coded
// default. The helpers never overwrite a populated field.

// ApplyUserFundingFallback fills empty fields on ext from [funding] in the
// user config. Mutates ext in place. Safe on nil.
func ApplyUserFundingFallback(ext *pfmodel.FundingExtension) {
	if ext == nil {
		return
	}
	f := userconfig.Load().Funding
	if len(ext.GitHub) == 0 {
		ext.GitHub = f.GitHub
	}
	if ext.Patreon == "" {
		ext.Patreon = f.Patreon
	}
	if ext.KoFi == "" {
		ext.KoFi = f.KoFi
	}
	if ext.Liberapay == "" {
		ext.Liberapay = f.Liberapay
	}
	if ext.Tidelift == "" {
		ext.Tidelift = f.Tidelift
	}
	if ext.CommunityBridge == "" {
		ext.CommunityBridge = f.CommunityBridge
	}
	if ext.IssueHunt == "" {
		ext.IssueHunt = f.IssueHunt
	}
	if ext.OpenCollective == "" {
		ext.OpenCollective = f.OpenCollective
	}
	if ext.LFXCrowdfunding == "" {
		ext.LFXCrowdfunding = f.LFXCrowdfunding
	}
	if ext.Polar == "" {
		ext.Polar = f.Polar
	}
	if ext.BuyMeACoffee == "" {
		ext.BuyMeACoffee = f.BuyMeACoffee
	}
	if ext.ThanksDev == "" {
		ext.ThanksDev = f.ThanksDev
	}
	if len(ext.Custom) == 0 {
		ext.Custom = f.Custom
	}
}

// ApplyUserSecurityFallback fills empty fields on ext from [security] and
// [identity] in the user config. Cross-section gap-fills:
//   - ext.Contact falls back to [identity].email
//   - ext.GPGKey falls back to [identity].gpg-key
//
// Section-level Security fields fill from [security] directly.
func ApplyUserSecurityFallback(ext *pfmodel.SecurityExtension) {
	if ext == nil {
		return
	}
	cfg := userconfig.Load()
	if ext.Contact == "" {
		ext.Contact = cfg.Identity.Email
	}
	if ext.GPGKey == "" {
		ext.GPGKey = cfg.Identity.GPGKey
	}
	if ext.ReportURL == "" {
		ext.ReportURL = cfg.Security.ReportURL
	}
	if ext.DisclosureWindow == "" {
		ext.DisclosureWindow = cfg.Security.DisclosureWindow
	}
	if ext.BugBountyURL == "" {
		ext.BugBountyURL = cfg.Security.BugBountyURL
	}
}

// ApplyUserContributingFallback fills empty fields on ext from
// [contributing] in the user config. Sections list is per-project so the
// user-config has no Sections field.
func ApplyUserContributingFallback(ext *pfmodel.ContributingExtension) {
	if ext == nil {
		return
	}
	c := userconfig.Load().Contributing
	if ext.CLAURL == "" {
		ext.CLAURL = c.CLAURL
	}
	if ext.ChatURL == "" {
		ext.ChatURL = c.ChatURL
	}
}

// ApplyUserConventionsFallback fills empty fields on conv from [conventions]
// in the user config. Per-language overrides are per-project so the user-config
// has no Languages field.
func ApplyUserConventionsFallback(conv *pfmodel.ConventionsExtension) {
	if conv == nil {
		return
	}
	c := userconfig.Load().Conventions
	if conv.CommitStyle == "" {
		conv.CommitStyle = c.CommitStyle
	}
	if conv.Workflow == "" {
		conv.Workflow = c.Workflow
	}
	if conv.StyleGuideURL == "" {
		conv.StyleGuideURL = c.StyleGuideURL
	}
}

// ApplyUserSupportFallback fills empty fields on ext from [support] in the
// user config. EOL table is per-project so the user-config has no EOL field.
func ApplyUserSupportFallback(ext *pfmodel.SupportExtension) {
	if ext == nil {
		return
	}
	s := userconfig.Load().Support
	if ext.ResponseTime == "" {
		ext.ResponseTime = s.ResponseTime
	}
}

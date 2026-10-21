// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fundingjson

import (
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// Fixture literals the funding.json suite shares; goconst counts every repeat.
const (
	fxName       = "widget"
	fxNS         = "d9t"
	fxTitle      = "Widget"
	fxSummary    = "A widget."
	fxAltDesc    = "Fallback description."
	fxEmail      = "ada@example.org"
	fxOrgName    = "Lovelace Collective"
	fxOrgEmail   = "collective@example.org"
	fxHomepage   = "https://example.org"
	fxRepo       = "https://github.com/d9t/widget"
	fxSpdx       = "MIT"
	fxGiven      = "Ada"
	fxFamily     = "Lovelace"
	fxMaintainer = "maintainer"
	fxOwner      = "owner"
	fxSteward    = "steward"
	fxContrib    = "contributor"
)

// Funding.json namespace keys more than one fixture or assertion repeats.
const (
	keyChannels = "channels"
	keyPlans    = "plans"
	keyHistory  = "history"
	keyGUID     = "guid"
	keyAddress  = "address"
	keyType     = "type"
	keyCurrency = "currency"
)

// Fixture values, shared so a fixture and the assertion reading it cannot drift.
const (
	valStripe          = "stripe"
	valSponsorGUID     = "sponsor"
	valActive          = "active"
	valMonthly         = "monthly"
	valUSD             = "USD"
	valPaymentProvider = "payment-provider"
	valChannelName     = "Sponsor"
	valChannelDesc     = "Card payments"
	valPayURL          = "https://pay.example.org"
	valHistoryDesc     = "First year"
	valHistoryYear     = 2025
)

// fullDoc renders a schema-valid funding.json: person entity, one project with
// homepage/repo/SPDX licence and filtered keywords, plus a channel, plan, history.
func fullDoc() *projectfile.Document {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{
			Namespace: fxNS,
			Name:      fxName,
			Title:     &projectfile.LocalizedString{Bare: fxTitle},
			Summary:   &projectfile.LocalizedString{Bare: fxSummary},
		},
		License:      &projectfile.License{Spdx: fxSpdx},
		People:       []projectfile.Person{{GivenNames: fxGiven, FamilyNames: fxFamily, Email: fxEmail, Roles: []string{fxMaintainer}}},
		Keywords:     []string{"go", "UPPER", "two words", "tooling", "build", "lint", "test", "ci", "dev", "ops", "extra"},
		Links:        []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
		Repositories: []projectfile.Repository{{URL: fxRepo}},
	}
	return withFunding(pf, map[string]any{
		keyChannels: []any{map[string]any{keyGUID: valStripe, keyType: valPaymentProvider, keyAddress: valPayURL, "description": valChannelDesc}},
		keyPlans:    []any{map[string]any{keyGUID: valSponsorGUID, "status": valActive, "name": valChannelName, "amount": 10.0, keyCurrency: valUSD, "frequency": valMonthly, keyChannels: []any{valStripe}}},
		keyHistory:  []any{map[string]any{"year": valHistoryYear, "income": 100.0, "expenses": 40.0, "taxes": 5.0, keyCurrency: valUSD, "description": valHistoryDesc}},
	})
}

// fullExt is the minimal namespace that passes the schema: one channel, one plan.
func fullExt() map[string]any {
	return map[string]any{keyChannels: []any{oneChannel()}, keyPlans: []any{onePlan()}}
}

// fundingOverrides extends fullExt, so a test tweaks one knob without restating the rest.
func fundingOverrides(extra map[string]any) map[string]any {
	out := fullExt()
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// noProjectDoc has an eligible person but no identity name or title, so no project resolves.
func noProjectDoc() *projectfile.Document {
	pf := barePersonDoc()
	pf.Identity = projectfile.Identity{}
	return pf
}

// barePersonDoc carries no funding namespace, only a homepage link and an eligible person.
func barePersonDoc() *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: fxName},
		People:   []projectfile.Person{{GivenNames: fxGiven, FamilyNames: fxFamily, Email: fxEmail}},
		Links:    []projectfile.Link{{Type: projectfile.LinkHomepage, URL: fxHomepage}},
	}
}

// withFunding attaches [org.projectfile.funding], the namespace every decision reads.
func withFunding(pf *projectfile.Document, ext map[string]any) *projectfile.Document {
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, ext)
	return pf
}

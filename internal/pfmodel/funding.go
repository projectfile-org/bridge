// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import "kiota.ch/projectfile/core/v2/pkg/projectfile"

// GetFundingExtension parses `org.projectfile.funding`. Returns (nil, nil)
// when the namespace is absent so callers can fall through to defaults.
func GetFundingExtension(doc *projectfile.Document) (*FundingExtension, error) {
	m, present, err := lookupNS(doc, FundingExtensionNS)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	return &FundingExtension{
		GitHub:          strListVal(m, "github"),
		Patreon:         strVal(m, "patreon"),
		KoFi:            strVal(m, "ko-fi"),
		Liberapay:       strVal(m, "liberapay"),
		Tidelift:        strVal(m, "tidelift"),
		CommunityBridge: strVal(m, "community-bridge"),
		IssueHunt:       strVal(m, "issuehunt"),
		OpenCollective:  strVal(m, "open-collective"),
		LFXCrowdfunding: strVal(m, "lfx-crowdfunding"),
		Polar:           strVal(m, "polar"),
		BuyMeACoffee:    strVal(m, "buy-me-a-coffee"),
		ThanksDev:       strVal(m, "thanks-dev"),
		Custom:          strListVal(m, "custom"),
		Path:            strVal(m, "path"),
		EntityType:      strVal(m, "entity-type"),
		EntityRole:      strVal(m, "entity-role"),
		Channels:        parseFundingChannels(m["channels"]),
		Plans:           parseFundingPlans(m["plans"]),
		History:         parseFundingHistory(m["history"]),
	}, nil
}

func parseFundingChannels(raw any) []FundingChannel {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]FundingChannel, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, FundingChannel{
			GUID:        strVal(m, "guid"),
			Type:        strVal(m, "type"),
			Address:     strVal(m, "address"),
			Description: strVal(m, "description"),
		})
	}
	return out
}

func parseFundingPlans(raw any) []FundingPlan {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]FundingPlan, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, FundingPlan{
			GUID:        strVal(m, "guid"),
			Status:      strVal(m, "status"),
			Name:        strVal(m, "name"),
			Description: strVal(m, "description"),
			Amount:      floatVal(m, "amount"),
			Currency:    strVal(m, "currency"),
			Frequency:   strVal(m, "frequency"),
			Channels:    strListVal(m, "channels"),
		})
	}
	return out
}

func parseFundingHistory(raw any) []FundingHistory {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]FundingHistory, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, FundingHistory{
			Year:        intVal(m, "year"),
			Income:      floatVal(m, "income"),
			Expenses:    floatVal(m, "expenses"),
			Taxes:       floatVal(m, "taxes"),
			Currency:    strVal(m, "currency"),
			Description: strVal(m, "description"),
		})
	}
	return out
}

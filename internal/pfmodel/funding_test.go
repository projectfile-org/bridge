// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// TestFundingReadsTheGitHubSchemaOneToOne pins the provider set FUNDING.yml declares, so the bridge can never drift from it.
func TestFundingReadsTheGitHubSchemaOneToOne(t *testing.T) {
	ext, err := GetFundingExtension(extDoc(FundingExtensionNS, map[string]any{
		forgeGitHub:        []any{"a", "b"},
		"patreon":          "pat",
		"ko-fi":            "kofi",
		"liberapay":        "lib",
		"tidelift":         "tid",
		"community-bridge": "cb",
		"issuehunt":        "ih",
		"open-collective":  "oc",
		"lfx-crowdfunding": "lfx",
		"polar":            "pol",
		"buy-me-a-coffee":  "bmc",
		"thanks-dev":       "td",
		"custom":           []any{urlXTest},
		"path":             ".github/FUNDING.yml",
		"entity-type":      "Organization",
		"entity-role":      "Maintainer",
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, []string{"a", "b"}, ext.GitHub)
	assert.Equal(t, "pat", ext.Patreon)
	assert.Equal(t, "kofi", ext.KoFi)
	assert.Equal(t, "lib", ext.Liberapay)
	assert.Equal(t, "tid", ext.Tidelift)
	assert.Equal(t, "cb", ext.CommunityBridge)
	assert.Equal(t, "ih", ext.IssueHunt)
	assert.Equal(t, "oc", ext.OpenCollective)
	assert.Equal(t, "lfx", ext.LFXCrowdfunding)
	assert.Equal(t, "pol", ext.Polar)
	assert.Equal(t, "bmc", ext.BuyMeACoffee)
	assert.Equal(t, "td", ext.ThanksDev)
	assert.Equal(t, []string{urlXTest}, ext.Custom)
	assert.Equal(t, ".github/FUNDING.yml", ext.Path)
	assert.Equal(t, "Organization", ext.EntityType)
	assert.Equal(t, "Maintainer", ext.EntityRole)
}

// TestFundingParsesChannelsPlansHistory pins the three lists that extend the GitHub schema for FundingJSON bridges, and the drop of a malformed row.
func TestFundingParsesChannelsPlansHistory(t *testing.T) {
	ext, err := GetFundingExtension(extDoc(FundingExtensionNS, map[string]any{
		keyChannels: []any{
			map[string]any{"guid": "g1", "type": "payment", "address": "https://p.test", keyDescription: "card"},
			junkRow,
		},
		"plans": []any{
			map[string]any{
				"guid": "g2", "status": "active", keyName: "Bronze", keyDescription: "d",
				"amount": 5, "currency": currencyUSD, "frequency": "monthly", keyChannels: []any{"g1"},
			},
			junkRow,
		},
		"history": []any{
			map[string]any{
				keyYear: 2025, "income": 10.5, "expenses": 3.25, "taxes": 1.5,
				"currency": currencyUSD, keyDescription: "first",
			},
			junkRow,
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)

	require.Len(t, ext.Channels, 1)
	assert.Equal(t, FundingChannel{GUID: "g1", Type: "payment", Address: "https://p.test", Description: "card"}, ext.Channels[0])

	require.Len(t, ext.Plans, 1)
	assert.Equal(t, FundingPlan{
		GUID: "g2", Status: "active", Name: "Bronze", Description: "d",
		Amount: 5, Currency: currencyUSD, Frequency: "monthly", Channels: []string{"g1"},
	}, ext.Plans[0])

	require.Len(t, ext.History, 1)
	assert.Equal(t, FundingHistory{
		Year: 2025, Income: 10.5, Expenses: 3.25, Taxes: 1.5,
		Currency: currencyUSD, Description: "first",
	}, ext.History[0])
}

// TestFundingListsThatAreNotListsCarryNothing: a malformed list degrades to empty rather than failing the whole extension parse.
func TestFundingListsThatAreNotListsCarryNothing(t *testing.T) {
	ext, err := GetFundingExtension(extDoc(FundingExtensionNS, map[string]any{
		keyChannels: notAMapping, "plans": 5, "history": map[string]any{keyYear: 2025},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Empty(t, ext.Channels)
	assert.Empty(t, ext.Plans)
	assert.Empty(t, ext.History)
}

// TestFundingAbsentIsNil.
func TestFundingAbsentIsNil(t *testing.T) {
	ext, err := GetFundingExtension(&projectfile.Document{})
	require.NoError(t, err)
	assert.Nil(t, ext)
}

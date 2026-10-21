// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fundingjson

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// Every channel field crosses verbatim: the schema carries no derived field.
func TestBuildChannelsCopiesEveryField(t *testing.T) {
	in := []pfmodel.FundingChannel{{
		GUID:        valStripe,
		Type:        valPaymentProvider,
		Address:     "https://pay.example.org",
		Description: "Card payments",
	}}
	assert.Equal(t, []fundingChannel{{
		GUID:        valStripe,
		Type:        valPaymentProvider,
		Address:     "https://pay.example.org",
		Description: "Card payments",
	}}, buildChannels(in))
}

// An absent channel list renders as an empty array, which the schema requires.
func TestBuildChannelsEmpty(t *testing.T) {
	assert.Empty(t, buildChannels(nil))
	assert.NotNil(t, buildChannels(nil))
}

// Every plan field crosses verbatim, including the channel list it accepts.
func TestBuildPlansCopiesEveryField(t *testing.T) {
	in := []pfmodel.FundingPlan{{
		GUID:        valSponsorGUID,
		Status:      valActive,
		Name:        valChannelName,
		Description: "Monthly support",
		Amount:      10.5,
		Currency:    valUSD,
		Frequency:   valMonthly,
		Channels:    []string{valStripe, "wire"},
	}}
	assert.Equal(t, []fundingPlan{{
		GUID:        valSponsorGUID,
		Status:      valActive,
		Name:        valChannelName,
		Description: "Monthly support",
		Amount:      10.5,
		Currency:    valUSD,
		Frequency:   valMonthly,
		Channels:    []string{valStripe, "wire"},
	}}, buildPlans(in))
}

func TestBuildPlansEmpty(t *testing.T) {
	assert.Empty(t, buildPlans(nil))
	assert.NotNil(t, buildPlans(nil))
}

// History rides verbatim, zeroes included; omitempty hides a year's zero income.
func TestBuildHistoryCopiesEveryField(t *testing.T) {
	in := []pfmodel.FundingHistory{{
		Year:        2025,
		Income:      100,
		Expenses:    40,
		Taxes:       5,
		Currency:    valUSD,
		Description: "First year",
	}}
	assert.Equal(t, []fundingHistoryEntry{{
		Year:        2025,
		Income:      100,
		Expenses:    40,
		Taxes:       5,
		Currency:    valUSD,
		Description: "First year",
	}}, buildHistory(in))
}

// No history means no key at all, not an empty array.
func TestBuildHistoryEmptyIsNil(t *testing.T) {
	assert.Nil(t, buildHistory(nil))
	assert.Nil(t, buildHistory([]pfmodel.FundingHistory{}))
}

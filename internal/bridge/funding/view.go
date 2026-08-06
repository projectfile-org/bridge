// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package funding

type fundingView struct {
	Marker          string
	GitHub          []string
	Patreon         string
	KoFi            string
	Liberapay       string
	Tidelift        string
	CommunityBridge string
	IssueHunt       string
	OpenCollective  string
	LFXCrowdfunding string
	Polar           string
	BuyMeACoffee    string
	ThanksDev       string
	Custom          []string
}

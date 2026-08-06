// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package dei

// deiView is the DEI.md template data. The four CHAOSS metric slices carry the
// project's effort bullets; a nil slice signals "render the per-template
// placeholder" so each language variant words its own prompt. OtherEfforts
// renders only when the project declared an `other` metric — unlike the four
// CHAOSS metrics it is optional and omitted entirely when absent.
type deiView struct {
	Marker       string
	ProjectName  string
	Scope        string
	LastReviewed string
	ContactEmail string

	ProjectAccess       []string
	CommTransparency    []string
	NewcomerExperiences []string
	InclusiveLeadership []string
	OtherEfforts        []string
	HasOther            bool
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package security

type securityView struct {
	Marker            string
	ProjectName       string
	Contact           string
	ReportURL         string
	SupportedVersions []string
	DisclosureWindow  string
	GPGKey            string
	BugBountyURL      string
}

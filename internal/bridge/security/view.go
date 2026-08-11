// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package security

type securityView struct {
	ProjectName       string
	Contact           string
	ReportURL         string
	SupportedVersions []string
	DisclosureWindow  string
	GPGKey            string
	GPGFingerprint    string
	GPGKeyURL         string
	BugBountyURL      string
	Acknowledged      []ackView
}

// ackView is one reviewed-and-suppressed finding surfaced for disclosure. It
// reads from org.projectfile.vulnerabilities.suppress — the same list the
// scanner bridges consume — so a finding that no longer scans is the finding
// SECURITY.md names.
type ackView struct {
	ID     string
	Reason string
}

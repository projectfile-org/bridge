// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package git

import (
	"projectfile.org/projectfile/bridge/internal/scanners/core"
	"projectfile.org/projectfile/bridge/internal/source"
)

const scannerGitDates = "git-dates"

// datesScanner extracts project lifecycle dates from git commits:
// identity.created from the root commit, identity.modified from HEAD.
// Both use gap-fill semantics — a curated value is never overwritten.
type datesScanner struct{}

func (datesScanner) Name() string { return scannerGitDates }

func (datesScanner) Detect(root string) bool { return detectGit(root) }

func (datesScanner) Scan(root string) (*source.Partial, []core.Hit, error) {
	p := &source.Partial{}
	var hits []core.Hit

	if d := firstCommitDate(root); d != "" {
		p.Created = source.StringPtr(d)
		hits = append(hits, core.Hit{Source: scannerGitDates, Field: "identity.created"})
	}
	if d := lastCommitDate(root); d != "" {
		p.Modified = source.StringPtr(d)
		hits = append(hits, core.Hit{Source: scannerGitDates, Field: "identity.modified"})
	}

	return p, hits, nil
}

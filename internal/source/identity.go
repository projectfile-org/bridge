// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
)

// ApplyUserIdentity splices fields from the per-user [identity] section into
// any PersonEntry whose Email matches [identity].email (case-insensitive).
// Only empty fields are overwritten — a person who already has an ORCID
// from CITATION.cff keeps it. The goal is "stop hand-typing my ORCID into
// every project", not "force my ORCID over whatever the project says".
//
// GPGKey is intentionally NOT propagated to the person — projectfile's
// Person has no gpg-key slot; that field is consumed by the
// SECURITY.md bridge instead (see bridge/core.ApplyUserSecurityFallback).
func ApplyUserIdentity(p *Partial) {
	if p == nil || len(p.People) == 0 {
		return
	}
	cfg := userconfig.Load()
	wantEmail := strings.ToLower(strings.TrimSpace(cfg.Identity.Email))
	if wantEmail == "" {
		return
	}
	for i := range p.People {
		if strings.ToLower(strings.TrimSpace(p.People[i].Email)) != wantEmail {
			continue
		}
		// Email match: gap-fill personal fields. Each guard preserves any
		// value the source/scanner pass already supplied.
		filled := false
		// Name shape — fill the matching discriminator only. A scanner that
		// already produced family-names keeps them; an entity entry (no
		// FamilyNames, with Name) gets the user's Name when the user is
		// configured as an organization. Affiliation is person-only per
		// spec §5.5 and is suppressed on the entity branch.
		if cfg.Identity.IsEntity {
			if p.People[i].Name == "" && cfg.Identity.Name != "" {
				p.People[i].Name = cfg.Identity.Name
				filled = true
			}
		} else {
			if p.People[i].FamilyNames == "" && cfg.Identity.FamilyNames != "" {
				p.People[i].FamilyNames = cfg.Identity.FamilyNames
				filled = true
			}
			if p.People[i].GivenNames == "" && cfg.Identity.GivenNames != "" {
				p.People[i].GivenNames = cfg.Identity.GivenNames
				filled = true
			}
			if p.People[i].Affiliation == "" && cfg.Identity.Affiliation != "" {
				p.People[i].Affiliation = cfg.Identity.Affiliation
				filled = true
			}
		}
		if p.People[i].Orcid == "" && cfg.Identity.Orcid != "" {
			p.People[i].Orcid = cfg.Identity.Orcid
			filled = true
		}
		if p.People[i].URL == "" && cfg.Identity.URL != "" {
			p.People[i].URL = cfg.Identity.URL
			filled = true
		}
		if filled {
			genlog.Info("user identity attached", "email", p.People[i].Email)
		}
	}
}

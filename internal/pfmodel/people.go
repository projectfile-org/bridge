// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"fmt"
	"strings"
	"time"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// RoleCopyright mirrors projectfile.RoleCopyright — the [[people]].roles
// value the copyright-line builder filters on.
const RoleCopyright = projectfile.RoleCopyright

// RoleMaintainer mirrors projectfile.RoleMaintainer — the generic fallback
// role ContactEmail uses when no artefact-specific contact matches.
const RoleMaintainer = projectfile.RoleMaintainer

// PersonOrEntityCompat is the common alias/email surface ForgeHandle accepts.
// Persons and organisations both lower to it so one forge-handle resolver
// serves CODEOWNERS and similar sinks.
type PersonOrEntityCompat struct {
	Alias string
	Email string
}

// ForgePersonHandle resolves the owner token a forge expects for a person,
// for sinks like CODEOWNERS. Per spec §4.5.1 `alias` is a "short handle,
// nickname, or username (e.g. a GitHub login)", so it is preferred over email:
//
//   - alias present → emit `@alias`, or verbatim when the user already wrote
//     a leading `@`.
//   - alias absent  → fall back to the email address.
//   - neither set   → empty string (caller decides how to handle).
func ForgePersonHandle(p projectfile.Person) string {
	if alias := strings.TrimSpace(p.Alias); alias != "" {
		if strings.HasPrefix(alias, "@") {
			return alias
		}
		return "@" + alias
	}
	return strings.TrimSpace(p.Email)
}

// ForgeOrgHandle is the organisation analogue of ForgePersonHandle.
func ForgeOrgHandle(o projectfile.Organization) string {
	if alias := strings.TrimSpace(o.Alias); alias != "" {
		if strings.HasPrefix(alias, "@") {
			return alias
		}
		return "@" + alias
	}
	return strings.TrimSpace(o.Email)
}

// ForgeHandle resolves a forge handle from the common alias/email surface.
func ForgeHandle(p PersonOrEntityCompat) string {
	if p.Alias != "" {
		return ForgePersonHandle(projectfile.Person{Alias: p.Alias, Email: p.Email})
	}
	return strings.TrimSpace(p.Email)
}

// FlatPersonName flattens a Person to the single-name string consumers emit
// to formats with no structured name slot. Implements the synthesis rule of
// spec §5.5.3 for persons.
func FlatPersonName(p projectfile.Person) string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	parts := make([]string, 0, 4)
	if p.GivenNames != "" {
		parts = append(parts, p.GivenNames)
	}
	if p.NameParticle != "" {
		parts = append(parts, p.NameParticle)
	}
	if p.FamilyNames != "" {
		parts = append(parts, p.FamilyNames)
	}
	flat := strings.Join(parts, " ")
	if p.NameSuffix != "" {
		if flat != "" {
			flat = flat + ", " + p.NameSuffix
		} else {
			flat = p.NameSuffix
		}
	}
	return flat
}

// FlatOrgName is the org analogue of FlatPersonName — organisations already
// carry a single name, so this is the identity with the trimming done.
func FlatOrgName(o projectfile.Organization) string {
	return o.Name
}

// DisplayName returns the best human-readable label for the project:
// identity.title.en → namespace/name → name → "this project". Used by
// every template generator so projects with a localized title don't get
// rendered as their lowercase DNS-label name in CoC / CONTRIBUTING / SECURITY.
func DisplayName(doc *projectfile.Document) string {
	return DisplayNameForLang(doc, "")
}

// DisplayNameForLang is DisplayName resolved in a specific render language,
// so CONTRIBUTING.es.md names the project by its Spanish title when
// identity.title carries one. An empty lang behaves like DisplayName; an
// untranslated title falls back through en to the first non-empty entry.
func DisplayNameForLang(doc *projectfile.Document, lang string) string {
	if doc == nil {
		return "this project"
	}
	if t := projectfile.ExtractLocalizedStringForLang(doc.Identity.Title, lang); t != "" {
		return t
	}
	if doc.Identity.Namespace != "" && doc.Identity.Name != "" {
		return doc.Identity.Namespace + "/" + doc.Identity.Name
	}
	if doc.Identity.Name != "" {
		return doc.Identity.Name
	}
	return "this project"
}

// CopyrightHolders assembles SPDX-style copyright lines from the
// `copyright`-role entries under [[people]]. Order matches the people array.
// Per-person `from` supplies the start year; absent, the top-level
// [copyright].year is used; absent both, the current year is used. Per-person
// `to` becomes the upper bound; absent → open-ended ("YYYY"). The output is
// the deterministic ordered slice the LICENSE generator and SPDX substitution
// previously read from License.Holders — single source of truth.
func CopyrightHolders(doc *projectfile.Document) []string {
	if doc == nil {
		return nil
	}
	defaultYear := 0
	if doc.Copyright != nil && doc.Copyright.Year != 0 {
		defaultYear = doc.Copyright.Year
	}
	if defaultYear == 0 {
		defaultYear = time.Now().Year()
	}
	var out []string
	for _, p := range doc.People {
		if !roleContains(p.Roles, RoleCopyright) {
			continue
		}
		line := copyrightLine(p.From, p.To, defaultYear, FlatPersonName(p), p.Email)
		if line != "" {
			out = append(out, line)
		}
	}
	for _, o := range doc.Organizations {
		if !roleContains(o.Roles, RoleCopyright) {
			continue
		}
		line := copyrightLine(o.From, o.To, defaultYear, o.Name, o.Email)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func copyrightLine(fromStr, toStr string, defaultYear int, name, email string) string {
	fromYear := yearOf(fromStr)
	if fromYear == 0 {
		fromYear = defaultYear
	}
	toYear := yearOf(toStr)
	years := fmt.Sprintf("%d", fromYear)
	if toYear != 0 && toYear != fromYear {
		years = fmt.Sprintf("%d-%d", fromYear, toYear)
	}
	if name == "" {
		return ""
	}
	line := fmt.Sprintf("Copyright %s %s", years, name)
	if email != "" {
		line = fmt.Sprintf("%s <%s>", line, email)
	}
	return line
}

// CopyrightHolderNames returns just the holder name(s) behind the
// `copyright`-role entries — no "Copyright YYYY" prefix and only the holder
// name(s) go into the SPDX [fullname] placeholder substitution.
func CopyrightHolderNames(doc *projectfile.Document) []string {
	if doc == nil {
		return nil
	}
	var out []string
	for _, p := range doc.People {
		if !roleContains(p.Roles, RoleCopyright) {
			continue
		}
		if name := FlatPersonName(p); name != "" {
			out = append(out, name)
		}
	}
	for _, o := range doc.Organizations {
		if !roleContains(o.Roles, RoleCopyright) {
			continue
		}
		if o.Name != "" {
			out = append(out, o.Name)
		}
	}
	return out
}

func roleContains(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}

// yearOf extracts the leading YYYY out of an ISO-8601 date string. Returns 0
// when the input is empty or malformed (the schema enforces ISO format, but
// the helper is defensive for hand-edited / partial documents).
func yearOf(iso string) int {
	if len(iso) < 4 {
		return 0
	}
	y := 0
	for i := range 4 {
		c := iso[i]
		if c < '0' || c > '9' {
			return 0
		}
		y = y*10 + int(c-'0')
	}
	return y
}

// ContactEmail picks the most appropriate contact email from pf.People for a
// single-contact artefact (SECURITY.md, CODE_OF_CONDUCT.md, future siblings).
// Walk order, first non-empty wins:
//
//  1. People whose roles contain primaryRole — the artefact-specific role
//     (RoleSecurity, RoleCommunity, ...). Spec v1 §5.5.4 SHOULD-filter rule.
//  2. People whose roles contain RoleMaintainer — generic fallback when no
//     artefact-specific contact is registered.
//  3. Any person with an email — last-resort so a project with [[people]] but
//     no curated roles still produces a usable file.
//
// Returns ("", "") when no person carries an email so the caller can surface
// an actionable "no contact available" error. The string returned is the
// email; the source label names which step matched (used by the genlog
// decision trace so the reader can see why a particular address was chosen).
// Pass primaryRole = "" or RoleMaintainer to skip step 1.
func ContactEmail(doc *projectfile.Document, primaryRole string) (email, source string) {
	if doc == nil {
		return "", ""
	}
	if primaryRole != "" && primaryRole != RoleMaintainer {
		if e := firstEmailByRole(doc, primaryRole); e != "" {
			return e, fmt.Sprintf("first [[people]] or [[organizations]] with role %q", primaryRole)
		}
	}
	if e := firstEmailByRole(doc, RoleMaintainer); e != "" {
		return e, "first maintainer in [[people]] or [[organizations]]"
	}
	for _, p := range doc.People {
		if p.Email != "" {
			return p.Email, "first [[people]] entry with email"
		}
	}
	for _, o := range doc.Organizations {
		if o.Email != "" {
			return o.Email, "first [[organizations]] entry with email"
		}
	}
	return "", ""
}

func firstEmailByRole(doc *projectfile.Document, role string) string {
	for _, p := range doc.People {
		if p.Email != "" && roleContains(p.Roles, role) {
			return p.Email
		}
	}
	for _, o := range doc.Organizations {
		if o.Email != "" && roleContains(o.Roles, role) {
			return o.Email
		}
	}
	return ""
}

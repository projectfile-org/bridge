// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"fmt"
	"strings"
	"time"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// CommentStyle selects how REUSEHeader wraps its SPDX lines. The SPDX text
// itself is identical across styles; only the comment leader changes so the
// header parses cleanly in the destination file dialect.
type CommentStyle int

const (
	// StyleHash wraps each line with `#` — for .gitignore / .dockerignore /
	// .npmignore / .trivyignore and YAML (FUNDING.yml).
	StyleHash CommentStyle = iota

	// StyleHTML emits a single multi-line HTML comment — for Markdown
	// documents where `#` would render as an H1 heading.
	StyleHTML

	// StyleSlash wraps each line with `//` — for JSONC (audit-ci.jsonc),
	// whose grammar has no `#` comment.
	StyleSlash
)

// defaultSPDX is the fallback SPDX expression when pf.License.Spdx is empty.
const defaultSPDX = "MIT"

// REUSE-IgnoreStart

// REUSEHeader returns the REUSE (https://reuse.software) compliance block
// pf-cli prepends to every overwriteable generated artefact.
func REUSEHeader(pf *projectfile.Document, style CommentStyle) string {
	copyrights := ReuseCopyrightLines(pf)
	spdx := reuseLicenseID(pf)

	var b strings.Builder
	switch style {
	case StyleHTML:
		b.WriteString("<!--\n")
		for _, c := range copyrights {
			fmt.Fprintf(&b, "SPDX-FileCopyrightText: %s\n", c)
		}
		fmt.Fprintf(&b, "SPDX-License-Identifier: %s\n", spdx)
		b.WriteString("-->\n")
	case StyleSlash:
		for _, c := range copyrights {
			fmt.Fprintf(&b, "// SPDX-FileCopyrightText: %s\n", c)
		}
		b.WriteString("//\n")
		fmt.Fprintf(&b, "// SPDX-License-Identifier: %s\n", spdx)
	default:
		for _, c := range copyrights {
			fmt.Fprintf(&b, "# SPDX-FileCopyrightText: %s\n", c)
		}
		// Bare comment leader between the two tag groups — `reuse annotate`
		// writes this separator for every line-comment dialect, so a generated
		// file and a hand-annotated source file carry the identical header.
		b.WriteString("#\n")
		fmt.Fprintf(&b, "# SPDX-License-Identifier: %s\n", spdx)
	}
	b.WriteString("\n")
	return b.String()
}

// REUSE-IgnoreEnd

// ManagedREUSEHeader is the Markdown header for a marker-managed artefact: the
// StyleHTML REUSE block with the pf-cli-managed sentinel folded in as its last
// line, so the file opens with ONE comment instead of a licence comment and a
// marker comment separated by a blank line.
//
// REUSE-compliant: the marker line carries no SPDX-* tag, so it neither
// declares a licence nor trips the REUSE-Ignore rules. The sentinel keeps its
// own line inside the comment, which HasMarker recognises via MarkerInner.
func ManagedREUSEHeader(pf *projectfile.Document) string {
	header := REUSEHeader(pf, StyleHTML) // <!--\n…SPDX…\n-->\n\n
	closeIdx := strings.Index(header, "-->")
	if closeIdx < 0 {
		// Defensive: a header without a closing comment is malformed; fall back
		// to the two-block form rather than emitting a broken header.
		return header + MarkerHTML + "\n"
	}
	return header[:closeIdx] + MarkerInner + "\n" + header[closeIdx:]
}

// ReuseCopyrightLines resolves the copyright lines every REUSE-compliant
// artefact must carry, in priority order: explicit `copyright`-role holders →
// author/maintainer people → the project DisplayName (last resort). Exported so
// the LICENSE bridge reuses the same chain as the per-file SPDX headers — the
// root LICENSE and the headers MUST agree on the rights-holder.
func ReuseCopyrightLines(pf *projectfile.Document) []string {
	if holders := pfmodel.CopyrightHolders(pf); len(holders) > 0 {
		out := make([]string, len(holders))
		for i, h := range holders {
			out[i] = strings.TrimPrefix(h, "Copyright ")
		}
		return out
	}
	year := resolveCopyrightYear(pf)
	if name, email, ok := contributorHolder(pf); ok {
		if email != "" {
			return []string{fmt.Sprintf("%d %s <%s>", year, name, email)}
		}
		return []string{fmt.Sprintf("%d %s", year, name)}
	}
	return []string{fmt.Sprintf("%d %s", year, pfmodel.DisplayName(pf))}
}

// ReuseCopyrightHolderNames is the name-only projection of
// ReuseCopyrightLines — same priority chain (copyright-role holders →
// author/maintainer people → DisplayName) but returns bare names, for SPDX
// boilerplate whose `<copyright holders>` placeholder is filled separately
// from `<year>` (e.g. the MIT LICENSE text).
func ReuseCopyrightHolderNames(pf *projectfile.Document) []string {
	if names := pfmodel.CopyrightHolderNames(pf); len(names) > 0 {
		return names
	}
	if name, _, ok := contributorHolder(pf); ok {
		return []string{name}
	}
	return []string{pfmodel.DisplayName(pf)}
}

// contributorHolder returns the first author/maintainer person's flat name and
// email, the fallback tier between explicit copyright-role holders and the
// DisplayName. Returns ok=false when no such person exists.
func contributorHolder(pf *projectfile.Document) (name, email string, ok bool) {
	if pf == nil {
		return "", "", false
	}
	for _, p := range pf.People {
		if !hasContributorRole(p.Roles) {
			continue
		}
		name := pfmodel.FlatPersonName(p)
		if name == "" {
			continue
		}
		return name, p.Email, true
	}
	return "", "", false
}

func reuseLicenseID(pf *projectfile.Document) string {
	if pf != nil && pf.License != nil {
		if id := strings.TrimSpace(pf.License.Spdx); id != "" {
			return id
		}
	}
	return defaultSPDX
}

func resolveCopyrightYear(pf *projectfile.Document) int {
	if pf != nil && pf.Copyright != nil && pf.Copyright.Year != 0 {
		return pf.Copyright.Year
	}
	return time.Now().Year()
}

func hasContributorRole(roles []string) bool {
	for _, r := range roles {
		if r == "author" || r == "maintainer" {
			return true
		}
	}
	return false
}

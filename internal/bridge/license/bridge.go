// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package license

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/spdx"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const filenameLicense = "LICENSE"

// Bridge renders LICENSE from pf.License.Spdx plus copyright lines.
// Single-term expressions are substituted SPDX boilerplate; compound
// (X OR Y / X AND Y) expressions fan out into a top-level overview body
// plus per-term files in the same Output. Every term also emits
// LICENSES/<id>.txt so `reuse lint` resolves the expression. By default the
// holder/year are substituted into LICENSES/<id>.txt too (more useful than
// stale placeholders, still REUSE-compliant); --reuse-canonical keeps the
// literal placeholders, matching `reuse download`. WITH exception clauses
// are stripped before Text lookup (the exception has no standalone
// boilerplate; projects SHOULD append it by hand).
type Bridge struct{}

func (Bridge) Name() string             { return "license" }
func (Bridge) Filename() string         { return filenameLicense }
func (Bridge) Aliases() []string        { return []string{"LICENSE.md", "LICENSE.txt"} }
func (Bridge) Labels() (string, string) { return filenameLicense, "projectfile" }

// Describe answers the dispatcher probe: the licence bridge writes a tree,
// not the one file its Filename names.
func (Bridge) Describe() string {
	return filenameLicense + " + LICENSES/<spdx>.txt (one-way render, always overwrite)"
}

func (Bridge) Policy() core.Policy {
	// Legal artefact carries no comment syntax to host the pf-cli marker, so
	// it gets no write-gate: every run regenerates it from license.spdx.
	return core.Policy{}
}

func (Bridge) Exists(dir string) bool {
	_, err := stat(filepath.Join(dir, primaryRel(nil)))
	return err == nil
}

func (Bridge) FullPath(dir string, pf *projectfile.Document) string {
	return filepath.Join(dir, primaryRel(pf))
}

// primaryRel is the headline on-disk name for display / Exists. It honours the
// first declared license.file path (when valid) so `bridge LICENSE` reports
// the real write target; falls back to `LICENSE`. Validation errors fall back
// silently — Render surfaces them as a hard error on the actual write path.
func primaryRel(pf *projectfile.Document) string {
	if declared, err := declaredFiles(pf); err == nil && len(declared) > 0 {
		return declared[0]
	}
	return filenameLicense
}

func (Bridge) RequiredFields(pf *projectfile.Document) []core.Missing {
	if pf != nil && pf.License != nil && pf.License.Spdx != "" {
		return nil
	}
	return []core.Missing{{
		Field: "license.spdx",
		Hint:  "SPDX expression (e.g. MIT, Apache-2.0, MIT OR Apache-2.0)",
		Setter: func(v string) error {
			if v == "" {
				return fmt.Errorf("license.spdx is required")
			}
			if pf.License == nil {
				pf.License = &projectfile.License{}
			}
			pf.License.Spdx = v
			return nil
		},
	}}
}

func (Bridge) Render(pf *projectfile.Document, opts core.Options) (core.Output, error) {
	expr := licenseExpr(pf)
	if expr == "" {
		return core.Output{}, fmt.Errorf("LICENSE: pf has no [license].spdx — set one before generating")
	}
	vars := licenseVars(pf)
	emitDecisionTrace(pf, expr, vars)

	// license.covers: `additions` is a signal — the SPDX expression is NOT
	// authoritative for the full tree (bundled upstream retains its own
	// licensing). Per spec §4.4 v1 mandates no NOTICE/THIRD-PARTY file here;
	// we only emit a decision-trace line so the boundary is visible.
	emitCoversTrace(pf, expr)

	// Resolve and validate the declared output path(s). When license.file is
	// absent, declared is nil and we fall back to the synthesised names.
	declared, err := declaredFiles(pf)
	if err != nil {
		return core.Output{}, err
	}

	files := map[string][]byte{}
	terms, conj := spdx.SplitCompound(expr)
	if len(terms) <= 1 {
		// Single term: may still carry a WITH exception clause — strip it
		// so Text gets a resolvable base license id.
		baseID := spdx.StripException(expr)
		text, err := spdx.Text(baseID, spdx.Options{Offline: opts.Offline})
		if err != nil {
			return core.Output{}, err
		}
		// Single term writes to the first declared path, else `LICENSE`.
		key := filenameLicense
		if len(declared) > 0 {
			key = declared[0]
			genlog.DebugRow("output", key, "license.file", "")
		}
		files[key] = []byte(spdx.Substitute(text, vars))
		emitReuseFile(files, baseID, text, vars, !opts.ReuseCanonical)
		genlog.DebugRow("reuse",
			"LICENSES/"+baseID+".txt ("+reuseMode(opts.ReuseCanonical)+")", "spdx", baseID)
		return core.Output{Files: files}, nil
	}

	// Compound: each term gets a canonical LICENSES/<id>.txt (REUSE) plus, when
	// the user declared a path sequence, a substituted copy at the declared
	// path. The overview LICENSE carries the conjunction summary unless the
	// user declared one path per term (a pure fan-out with no room for it).
	termKeys := make([]string, len(terms))
	for i := range terms {
		if i < len(declared) {
			termKeys[i] = declared[i]
		} else {
			termKeys[i] = ""
		}
	}
	if len(declared) > 0 {
		genlog.DebugRow("output",
			fmt.Sprintf("%d declared file(s): %s", len(declared), strings.Join(declared, ", ")),
			"license.file", "")
	}
	body, err := core.Render(opts.Dir, "LICENSE.tmpl", compoundView{
		Expression:  expr,
		Conjunction: conj,
		Terms:       terms,
		Holders:     vars.Holders,
		Year:        vars.Year,
	})
	if err != nil {
		return core.Output{}, err
	}
	// The overview LICENSE is suppressed only when the user declared exactly
	// one path per term (a pure REUSE fan-out with no room for an overview);
	// otherwise the top-level LICENSE carries the conjunction summary.
	if len(declared) < len(terms) {
		files[filenameLicense] = body
	}
	for i, term := range terms {
		baseID := spdx.StripException(term)
		text, err := spdx.Text(baseID, spdx.Options{Offline: opts.Offline})
		if err != nil {
			return core.Output{}, fmt.Errorf("LICENSE term %s: %w", term, err)
		}
		// LICENSES/<id>.txt is always emitted so `reuse lint` resolves the
		// expression regardless of declared paths. Default: substituted.
		emitReuseFile(files, baseID, text, vars, !opts.ReuseCanonical)
		genlog.DebugRow("reuse",
			"LICENSES/"+baseID+".txt ("+reuseMode(opts.ReuseCanonical)+")", "spdx", baseID)
		// Declared per-term paths carry the substituted copy (legacy fan-out).
		if key := termKeys[i]; key != "" {
			files[key] = []byte(spdx.Substitute(text, vars))
		}
	}
	return core.Output{Files: files}, nil
}

// emitReuseFile writes LICENSES/<id>.txt so `reuse lint` resolves the SPDX
// expression. By default the resolved copyright holder/year are substituted
// into the boilerplate (more useful to humans than stale placeholders, and
// still REUSE-compliant — lint resolves copyright from per-file headers).
// Pass substitute=false (--reuse-canonical) to keep the literal placeholders,
// matching what `reuse download` emits.
func emitReuseFile(files map[string][]byte, id, text string, vars spdx.Vars, substitute bool) {
	if substitute {
		text = spdx.Substitute(text, vars)
	}
	files["LICENSES/"+id+".txt"] = []byte(text)
}

// reuseMode labels the decision trace: "substituted" (default) or "canonical"
// (--reuse-canonical).
func reuseMode(canonical bool) string {
	if canonical {
		return "canonical"
	}
	return "substituted"
}

// emitCoversTrace logs the license.covers signal. `additions` means the SPDX
// expression covers only this repository's original work; bundled upstream is
// not relicensed. v1 mandates no generated NOTICE/THIRD-PARTY file (spec §4.4),
// so this is a trace-only consumer of the field.
func emitCoversTrace(pf *projectfile.Document, expr string) {
	if pf == nil || pf.License == nil {
		return
	}
	covers := strings.ToLower(strings.TrimSpace(pf.License.Covers))
	switch covers {
	case "", "project":
		genlog.DebugRow("covers", "project (full source tree)", "license.covers", "")
	case "additions":
		genlog.DebugRow("covers",
			"additions — "+expr+" is NOT authoritative for the full tree; bundled upstream retains its own licensing",
			"license.covers", "")
	default:
		genlog.DebugRow("covers", covers+" (unknown — treated as project)", "license.covers", "")
	}
}

func licenseExpr(pf *projectfile.Document) string {
	if pf == nil || pf.License == nil {
		return ""
	}
	return strings.TrimSpace(pf.License.Spdx)
}

// licenseVars resolves SPDX-substitution variables. User-config year strategy
// "current" overrides any in-pf year (the strategy choice IS opting in to that
// override); other values fall through to pf-wins-then-now semantics.
//
// Holders reuse core.ReuseCopyrightHolderNames — the SAME priority chain the
// per-file REUSE headers use (copyright-role holders → author/maintainer →
// DisplayName) — so the root LICENSE and the headers always agree on the
// rights-holder and never ship a literal "<copyright holders>" placeholder.
func licenseVars(pf *projectfile.Document) spdx.Vars {
	v := spdx.Vars{Year: time.Now().Year()}
	if pf != nil && pf.Copyright != nil && pf.Copyright.Year != 0 {
		v.Year = pf.Copyright.Year
	}
	if strings.EqualFold(userconfig.Load().Copyright.YearStrategy, "current") {
		v.Year = time.Now().Year()
	}
	v.Holders = core.ReuseCopyrightHolderNames(pf)
	return v
}

func emitDecisionTrace(pf *projectfile.Document, expr string, vars spdx.Vars) {
	yearSrc := "default (current year)"
	if pf != nil && pf.Copyright != nil && pf.Copyright.Year != 0 {
		yearSrc = "[copyright].year"
	}
	if strings.EqualFold(userconfig.Load().Copyright.YearStrategy, "current") {
		yearSrc = "user-config [copyright].year_strategy=current"
	}
	genlog.DebugRow("spdx", expr, "[license].spdx", "")
	genlog.DebugRow("year", fmt.Sprintf("%d", vars.Year), yearSrc, "[copyright].year")
	// Holder source mirrors core.ReuseCopyrightHolderNames: copyright-role
	// wins, then author/maintainer, then DisplayName last resort.
	holderSrc := "identity (DisplayName fallback)"
	if len(pfmodel.CopyrightHolderNames(pf)) > 0 {
		holderSrc = "[[people]]/[[organizations]] roles=copyright"
	} else if hasAuthorMaintainer(pf) {
		holderSrc = "[[people]] roles=author/maintainer"
	}
	genlog.DebugRow("holders",
		fmt.Sprintf("%d entr(y/ies): %s", len(vars.Holders), strings.Join(vars.Holders, ", ")),
		holderSrc, "[[people]] roles")
}

// hasAuthorMaintainer reports whether any person carries an author or
// maintainer role — the middle tier of the copyright-holder chain (mirrors
// core.hasContributorRole; neither role has an exported constant).
func hasAuthorMaintainer(pf *projectfile.Document) bool {
	if pf == nil {
		return false
	}
	for _, p := range pf.People {
		for _, r := range p.Roles {
			if r == "author" || r == "maintainer" {
				return true
			}
		}
	}
	return false
}

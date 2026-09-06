// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package vulnerabilities is the tool-agnostic suppress-list bridge family.
// One source — org.projectfile.vulnerabilities.suppress — fans the SAME set of
// vulnerability IDs out to every scanner ignore file the project runs:
//
//   - .trivyignore       (trivy: one vulnerability ID per line; any format)
//   - .grype.yaml        (grype: ignore[].vulnerability; auto-discovered from cwd)
//   - osv-scanner.toml   (osv: [[IgnoredVulns]]; auto-discovered from cwd)
//   - audit-ci.jsonc     (audit-ci: allowlist[]; passed via --config)
//
// "Specify once, fan out everywhere": a known-unfixable vuln (e.g. kernel
// headers shipped by a transitive distro package) is declared a single time in
// the projectfile and dropped from every scanner's report, so trivy, grype and
// osv can never disagree on whether it gates publish. This is intentionally
// separate from bridge/ignore (gitignore-style FILE-PATTERN ignores): the data
// model and per-scanner formats are unrelated to stack snippet-stitching.
package vulnerabilities

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	scannerGrype   = "grype"
	scannerTrivy   = "trivy"
	scannerOSV     = "osv"
	scannerAuditCI = "auditci"
)

// Bridge implements core.Renderer for one scanner ignore file. One instance
// per row in `targets` (see register.go), distinguished by scanner extKey +
// on-disk filename.
type Bridge struct {
	scanner  string // extKey: "trivy" | "grype" | "osv" | "auditci"
	filename string // canonical on-disk name
}

func (b Bridge) Name() string      { return b.scanner + "-vulnignore" }
func (b Bridge) Filename() string  { return b.filename }
func (b Bridge) Aliases() []string { return nil }
func (b Bridge) Labels() (string, string) {
	return b.filename, "projectfile"
}

// Scanner ignore files carry the marker so the next run overwrites cleanly.
func (Bridge) Policy() core.Policy { return core.Policy{Marker: true} }

func (b Bridge) Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, b.filename))
	return err == nil
}

func (b Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return filepath.Join(dir, b.filename)
}

func (b Bridge) Render(pf *projectfile.Document, _ core.Options) (core.Output, error) {
	ext, err := pfmodel.GetVulnerabilitiesExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	// Nothing to suppress (or namespace absent) → emit nothing. The caller
	// skips the write, so removing every suppression drops generation; clear
	// any stale generated file by hand or via `git clean`.
	if ext == nil || len(ext.Suppress) == 0 {
		return core.Output{}, nil
	}
	// `generate` opts a project out of scanners it does not run. Unset means
	// "fan out to every scanner" — the specify-once default.
	if !enabledFor(ext, b.scanner) {
		return core.Output{}, nil
	}
	body := b.assemble(pf, ext.Suppress)
	if len(body) == 0 {
		return core.Output{}, nil
	}
	return core.Output{Files: map[string][]byte{b.filename: body}}, nil
}

// enabledFor reports whether this scanner should receive a file. An explicit
// `generate` list is authoritative; an absent list means every scanner.
func enabledFor(ext *pfmodel.VulnerabilitiesExtension, scanner string) bool {
	if ext == nil || len(ext.Generate) == 0 {
		return true
	}
	for _, s := range ext.Generate {
		if s == scanner {
			return true
		}
	}
	return false
}

// assemble builds the byte body for one scanner: REUSE header + banner + the
// scanner-specific suppress block. The banner points at the single
// org.projectfile.vulnerabilities source so a reader always knows where to edit.
func (b Bridge) assemble(pf *projectfile.Document, suppress []pfmodel.VulnerabilitySuppress) []byte {
	entries := dedupSorted(suppress)

	var buf bytes.Buffer
	// grype's file is YAML: yamllint (rule document-start) wants a `---`
	// before the SPDX comment header, matching projectfile.yaml convention.
	if b.scanner == scannerGrype {
		buf.WriteString(core.YAMLDocStart)
	}
	style, marker := core.StyleHash, core.Marker
	// audit-ci.jsonc has no `#` comment, so its header + banner use `//`.
	if b.scanner == scannerAuditCI {
		style, marker = core.StyleSlash, core.MarkerSlash
	}
	buf.WriteString(core.REUSEHeader(pf, style))
	buf.WriteString(vulnBanner(b.filename, len(entries), marker))

	switch b.scanner {
	case scannerTrivy:
		writeTrivy(&buf, entries)
	case scannerGrype:
		writeGrype(&buf, entries)
	case scannerOSV:
		writeOsv(&buf, entries)
	case scannerAuditCI:
		writeAuditCI(&buf, entries)
	}
	return buf.Bytes()
}

// dedupSorted returns the suppress entries de-duplicated by ID and sorted, so
// output is stable regardless of projectfile list order or duplicate entries.
func dedupSorted(in []pfmodel.VulnerabilitySuppress) []pfmodel.VulnerabilitySuppress {
	byID := make(map[string]string, len(in)) // id → reason (first reason wins)
	order := make([]string, 0, len(in))
	for _, e := range in {
		if e.ID == "" {
			continue
		}
		if _, seen := byID[e.ID]; !seen {
			byID[e.ID] = e.Reason
			order = append(order, e.ID)
		}
	}
	sort.Strings(order)
	out := make([]pfmodel.VulnerabilitySuppress, 0, len(order))
	for _, id := range order {
		out = append(out, pfmodel.VulnerabilitySuppress{ID: id, Reason: byID[id]})
	}
	return out
}

// vulnBanner is the top-of-file block for vulnerability ignore files. It names
// the single tool-agnostic source (org.projectfile.vulnerabilities) so a reader
// edits the projectfile, not the generated file. marker selects the dialect's
// comment leader ("#" everywhere but audit-ci.jsonc's core.MarkerSlash, "//").
func vulnBanner(filename string, count int, marker string) string {
	noun := "ID"
	if count != 1 {
		noun = "IDs"
	}
	leader := "#"
	if marker == core.MarkerSlash {
		leader = "//"
	}
	return fmt.Sprintf(`%[1]s %[2]s — generated by `+"`pf-bridge vulnerabilities`"+`
%[1]s Source: org.projectfile.vulnerabilities (%[3]d suppressed %[4]s)
%[1]s Edit projectfile [org.projectfile.vulnerabilities] suppress to customise —
%[1]s direct edits here are overwritten on the next generation.
%[5]s

`, leader, filename, count, noun, marker)
}

// writeTrivy emits one vulnerability ID per line (any format: CVE/GHSA/GO-*).
// The plain-line .trivyignore dialect has no slot for a reason, so reason
// provenance is dropped here (it still surfaces in the grype/osv formats and
// the projectfile itself).
func writeTrivy(buf *bytes.Buffer, entries []pfmodel.VulnerabilitySuppress) {
	for _, e := range entries {
		fmt.Fprintf(buf, "%s\n", e.ID)
	}
}

// writeGrype emits grype's `ignore:` list. grype's ignore rule has no `reason`
// field, so a reason is carried as a trailing line comment (grype ignores it,
// humans keep the provenance). grype auto-discovers .grype.yaml from the cwd.
func writeGrype(buf *bytes.Buffer, entries []pfmodel.VulnerabilitySuppress) {
	buf.WriteString("ignore:\n")
	for _, e := range entries {
		if e.Reason != "" {
			fmt.Fprintf(buf, "  - vulnerability: %s  # %s\n", e.ID, e.Reason)
		} else {
			fmt.Fprintf(buf, "  - vulnerability: %s\n", e.ID)
		}
	}
}

// writeOsv emits osv-scanner's [[IgnoredVulns]] array (id + optional reason).
// osv-scanner auto-discovers osv-scanner.toml from the scanned directory.
func writeOsv(buf *bytes.Buffer, entries []pfmodel.VulnerabilitySuppress) {
	for _, e := range entries {
		buf.WriteString("[[IgnoredVulns]]\n")
		fmt.Fprintf(buf, "id = %s\n", strconv.Quote(e.ID))
		if e.Reason != "" {
			fmt.Fprintf(buf, "reason = %s\n", strconv.Quote(e.Reason))
		}
		buf.WriteByte('\n')
	}
}

// writeAuditCI emits audit-ci's `allowlist` config. `low: true` is the
// severity FLOOR, and audit-ci treats it as cumulative (low fails low and
// everything above it) — the only setting that reproduces a bare `npm audit`,
// which fails on any finding regardless of severity. audit-ci has no reason
// field on an allowlist entry, so a reason is carried as a preceding line
// comment (audit-ci's jju parser accepts it, same as .grype.yaml's trailer).
func writeAuditCI(buf *bytes.Buffer, entries []pfmodel.VulnerabilitySuppress) {
	buf.WriteString("{\n")
	buf.WriteString("  \"$schema\": \"https://github.com/IBM/audit-ci/raw/main/docs/schema.json\",\n")
	buf.WriteString("  \"low\": true,\n")
	buf.WriteString("  \"allowlist\": [\n")
	for i, e := range entries {
		if e.Reason != "" {
			fmt.Fprintf(buf, "    // %s\n", e.Reason)
		}
		comma := ","
		if i == len(entries)-1 {
			comma = ""
		}
		fmt.Fprintf(buf, "    %s%s\n", strconv.Quote(e.ID), comma)
	}
	buf.WriteString("  ]\n}\n")
}

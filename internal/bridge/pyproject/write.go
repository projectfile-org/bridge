// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pyproject

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
)

// Write serialises doc to pyproject.toml under dir.
//
// Strategy: the Rest canvas owns top-level key ordering ([project],
// [build-system], [tool.*], [dependency-groups], ...). We marshal the typed
// Project struct, decode it back into a map, then merge those keys ONTO the
// existing `project` sub-table inside Rest. Sub-keys we know about overwrite;
// sub-keys we don't know about (some future PEP-X field) survive intact.
//
// Rendering: go-toml/v2 drops comments and alphabetises tables, so a full
// canvas marshal eats the user's file. Instead, when Read captured the source
// bytes we re-render ONLY the [project] zone and splice it into the original
// text — the REUSE header, foreign tables, their comments and their layout
// survive byte for byte. A fresh document (no source) still marshals the whole
// canvas, and the REUSE header is gap-filled at the top whenever the leading
// comment block carries no SPDX tags.
//
// Within-table key ordering inside [project] is alphabetical — a documented
// go-toml/v2 limitation, also called out in internal/rawdoc/toml.go.
func Write(dir string, doc *Document) error {
	path := FullPath(dir)

	canvas := doc.Rest
	if canvas == nil {
		canvas = rawdoc.NewOrderedTOML()
	}

	existing := map[string]any{}
	if v, ok := canvas.Get("project"); ok {
		if m, ok := v.(map[string]any); ok {
			existing = m
		}
	}

	typed, err := typedProjectAsMap(doc.Project)
	if err != nil {
		return fmt.Errorf("marshal pyproject [project] table: %w", err)
	}

	// Paint known keys onto the existing sub-table — typed wins where set,
	// existing keeps any field omitempty dropped. Effect: unknown sub-keys
	// survive; known sub-keys reflect the typed view.
	for k, v := range typed {
		existing[k] = v
	}

	// Remove any known keys that are now empty in the typed view but still
	// present in `existing` from a prior read — otherwise clearing a field
	// in projectfile (via --mode from-pf) would leave a stale value behind.
	// This only deletes keys this package knows about; unknown keys are left
	// untouched.
	for _, k := range knownProjectKeys {
		if _, present := typed[k]; present {
			continue
		}
		delete(existing, k)
	}

	if len(existing) > 0 {
		canvas.Set("project", existing)
	} else {
		canvas.Delete("project")
	}

	data, err := render(doc, canvas, existing)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644) // #nosec G306 -- metadata file, not a secret
}

// render produces the file bytes: a [project]-zone splice into the captured
// source when there is one, a whole-canvas marshal otherwise. The REUSE header
// is gap-filled in both paths.
func render(doc *Document, canvas *rawdoc.OrderedTOML, project map[string]any) ([]byte, error) {
	var zone []byte
	if len(project) > 0 {
		z, err := toml.Marshal(map[string]any{"project": project})
		if err != nil {
			return nil, fmt.Errorf("marshal pyproject [project] table: %w", err)
		}
		zone = z
	}

	if doc.Source == nil {
		data, err := canvas.Marshal()
		if err != nil {
			return nil, fmt.Errorf("marshal pyproject.toml: %w", err)
		}
		return ensureREUSEHeader(data, doc.ReuseHeader), nil
	}
	return ensureREUSEHeader(spliceProjectZone(doc.Source, zone), doc.ReuseHeader), nil
}

// spliceProjectZone replaces the [project] table (with its [project.*]
// sub-tables) in src with zone — or removes it when zone is empty, or appends
// it after the last content line when src has none. Every other byte of src,
// comments included, is preserved.
func spliceProjectZone(src, zone []byte) []byte {
	lines := strings.Split(string(src), "\n")
	start, end := projectZoneBounds(lines)
	block := toLines(zone)

	var out []string
	switch {
	case start < 0 && len(block) == 0:
		out = lines
	case start < 0:
		out = appendAfterLastContent(lines, block)
	case len(block) == 0:
		out = collapseZoneSeam(append(append([]string{}, lines[:start]...), lines[end:]...), start)
	default:
		out = append([]string{}, lines[:start]...)
		out = append(out, block...)
		if end >= len(lines) {
			// Zone at EOF: its own trailing blank lines are file formatting,
			// so the re-rendered block closes with the same number of them.
			for i := end - 1; i > start && strings.TrimSpace(lines[i]) == ""; i-- {
				out = append(out, "")
			}
		} else if strings.TrimSpace(lines[end]) != "" {
			// Keep one blank line between the zone and the table that followed
			// it — the separator the old zone consumed was part of the splice.
			out = append(out, "")
		}
		out = append(out, lines[end:]...)
	}

	joined := strings.Join(out, "\n")
	if !strings.HasSuffix(joined, "\n") {
		joined += "\n"
	}
	return []byte(joined)
}

// projectZoneBounds locates the [project] table: start is its header line,
// end the first line of the next FOREIGN table ([project.*] sub-tables belong
// to the zone) or len(lines) at EOF. Returns start -1 when no such header
// exists. A column-0 "[" line inside a multi-line value inside [project]
// would read as a zone end; pyproject practice never writes one, and the
// duplicate keys a misdetection leaves behind fail the next parse loudly.
func projectZoneBounds(lines []string) (start, end int) {
	start = -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "[project]" {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, -1
	}
	end = len(lines)
	for i := start + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "[") && tomlTableRoot(t) != "project" {
			end = i
			break
		}
	}
	return start, end
}

// tomlTableRoot returns the root table name of a header line: "tool" for
// "[tool.hatch]", "project" for "[[project.authors]]".
func tomlTableRoot(header string) string {
	h := strings.TrimPrefix(strings.TrimPrefix(header, "[["), "[")
	if i := strings.IndexAny(h, ".]"); i >= 0 {
		h = h[:i]
	}
	return strings.TrimSpace(h)
}

// appendAfterLastContent inserts block after the last non-blank line, one
// blank line ahead of it; trailing blanks after that point are dropped.
func appendAfterLastContent(lines, block []string) []string {
	last := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			last = i
			break
		}
	}
	if last < 0 {
		return block
	}
	out := append([]string{}, lines[:last+1]...)
	out = append(out, "")
	return append(out, block...)
}

// collapseZoneSeam removes the double blank line removing a zone can leave
// where its separator lines used to sit on both sides of `at`.
func collapseZoneSeam(lines []string, at int) []string {
	if at > 0 && at < len(lines) &&
		strings.TrimSpace(lines[at-1]) == "" && strings.TrimSpace(lines[at]) == "" {
		return append(append([]string{}, lines[:at]...), lines[at+1:]...)
	}
	return lines
}

// ensureREUSEHeader prepends header when the leading comment block of data
// carries no SPDX tags — a header that is already there, hand-written or
// stamped, always wins untouched.
func ensureREUSEHeader(data []byte, header string) []byte {
	if header == "" || leadingBlockHasSPDX(data) {
		return data
	}
	return append([]byte(header), data...)
}

// leadingBlockHasSPDX reports whether the comment and blank lines ahead of
// the first data line contain an SPDX tag.
func leadingBlockHasSPDX(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
		case strings.HasPrefix(t, "#"):
			if strings.Contains(t, "SPDX-FileCopyrightText:") || strings.Contains(t, "SPDX-License-Identifier:") {
				return true
			}
		default:
			return false
		}
	}
	return false
}

// toLines splits a marshalled block into lines without its trailing newline.
func toLines(block []byte) []string {
	if len(block) == 0 {
		return nil
	}
	return strings.Split(strings.TrimRight(string(block), "\n"), "\n")
}

// typedProjectAsMap marshals the typed Project struct via TOML and decodes
// it back into a map[string]any. omitempty fields don't appear in the result,
// so the caller's "paint onto existing" loop won't blank them out.
func typedProjectAsMap(p Project) (map[string]any, error) {
	raw, err := toml.Marshal(p)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	if err := toml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// knownProjectKeys is the full set of [project] sub-keys this package
// models. Listed explicitly so Write can drop stale entries on the canvas
// without touching unknown (future-PEP) keys.
var knownProjectKeys = []string{
	"name",
	"version",
	"description",
	"readme",
	"requires-python",
	"license",
	"license-files",
	"authors",
	"maintainers",
	pyprojectKeywords,
	"classifiers",
	"urls",
	"scripts",
	"gui-scripts",
	"entry-points",
	"dependencies",
	"optional-dependencies",
	"dynamic",
}

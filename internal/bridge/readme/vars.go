// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"maps"
	"slices"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/interp"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// ciExtensionNS is the CI namespace read for the matrix axes. pfmodel exports no
// constant for it (CI is owned by the resolver, not a bridge), so the bridge
// names it locally.
const ciExtensionNS = "org.projectfile.ci"

// sectionGroupView is the render-ready shape of one command group handed to the
// installation/quick-start/usage/building templates: prose already localized and
// interpolated, commands already expanded and fanned out.
type sectionGroupView struct {
	Name     string
	Prefix   string
	Commands []string
	Postfix  string
	Syntax   string
}

// sectionView is a whole section: its localized title plus the groups that
// survived. Nil signals "no structured section" so the template falls back to
// its companion-file link probe.
type sectionView struct {
	Title  string
	Groups []sectionGroupView
}

// syntaxDefault is the fenced-block info string for a group that declares none.
// Shell, because that is what a command group is unless the shape says otherwise
// (a base image's FROM line, a library's import snippet).
const syntaxDefault = "sh"

// buildSection resolves the structured section named `name` for the active
// language. Each declared group's commands are expanded against the document and
// fanned out; a group whose commands ALL failed to resolve is dropped, and a
// section left with no group returns nil.
//
// The drop rule is what makes an artifact-driven README work, and it is the same
// rule the badges block already runs on shields: a shared fragment may declare a
// recipe for every ecosystem the fleet knows, unconditionally, because each
// command names the artifact it installs and only the projects that DECLARE that
// artifact can answer the reference. No `when:`, no per-namespace fragment
// wiring, no Go table of package managers — a project's install instructions
// follow from what it says it ships.
func buildSection(doc *projectfile.Document, ext *pfmodel.ReadmeExtension, name, lang string) *sectionView {
	if ext == nil {
		return nil
	}
	declared := ext.Sections[name]
	if len(declared) == 0 {
		return nil
	}
	axes := ciMatrixAxes(doc)
	source := pfmodel.ReadmeExtensionNS + "." + name
	var groups []sectionGroupView
	for _, group := range declared {
		view, ok := buildSectionGroup(doc, group, axes, source, lang)
		if !ok {
			continue
		}
		groups = append(groups, view)
	}
	if len(groups) == 0 {
		genlog.Decision("readme_section", name, "no group resolved (section dropped)", source)
		return nil
	}
	return &sectionView{Title: translate(lang, name+".title"), Groups: groups}
}

// buildSectionGroup renders one group. ok is false when the group declared
// commands but none survived expansion — the prose exists to introduce those
// commands, so keeping it alone would leave a lead-in pointing at nothing. A
// group that declared NO commands is pure prose and is kept as authored.
func buildSectionGroup(doc *projectfile.Document, group pfmodel.ReadmeSectionGroup, axes map[string][]string, source, lang string) (sectionGroupView, bool) {
	label := groupLabel(group.Name)
	var commands []string
	for _, command := range group.Commands {
		expanded, resolved := interp.ExpandFanOut(doc, command)
		if !resolved {
			genlog.Decision("readme_command", command, "unresolved reference (dropped)", label)
			continue
		}
		for _, line := range expandAxes(expanded, axes) {
			genlog.Decision("readme_command", line, source, "group="+label+" lang="+lang)
			commands = append(commands, line)
		}
	}
	if len(group.Commands) > 0 && len(commands) == 0 {
		genlog.Decision("readme_group", label, "no command resolved (group dropped)", source)
		return sectionGroupView{}, false
	}
	syntax := group.Syntax
	if syntax == "" {
		syntax = syntaxDefault
	}
	return sectionGroupView{
		Name:     group.Name,
		Prefix:   expandProse(doc, sectionText(group.Prefix, group.PrefixByLang, lang), label),
		Commands: commands,
		Postfix:  expandProse(doc, sectionText(group.Postfix, group.PostfixByLang, lang), label),
		Syntax:   syntax,
	}, true
}

// groupUnnamed labels a group that declares no name, for the decision trace only.
const groupUnnamed = "(unnamed)"

func groupLabel(name string) string {
	if name == "" {
		return groupUnnamed
	}
	return name
}

// expandProse interpolates a lead-in / lead-out sentence. Prose cannot fan out —
// a sentence has no per-value form — so a reference that resolves to several
// values is left verbatim by interp and the whole sentence is dropped rather
// than published with a literal `${…}` in it.
func expandProse(doc *projectfile.Document, text, label string) string {
	expanded, resolved := interp.ExpandChecked(doc, text)
	if !resolved {
		genlog.Decision("readme_prose", expanded, "unresolved reference (dropped)", label)
		return ""
	}
	return expanded
}

// sectionText resolves a group's localized prose: the per-language variant when
// a multi-language render requests one, else the default resolution.
func sectionText(def string, byLang map[string]string, lang string) string {
	if lang != "" && byLang != nil {
		if v, ok := byLang[lang]; ok && v != "" {
			return v
		}
	}
	return def
}

// expandAxes substitutes the `{AXIS}` matrix placeholders m6e and ci-resolver
// both replace per cell, fanning each line out to one per cell. A base image
// built once per Ubuntu series carries `{B19_UBUNTU_SERIES}` inside its
// published image path, and a reader choosing a series needs to see every one.
//
// Only axes the document DECLARES are substituted, and an unmatched brace is
// left alone: that is interp's cohabitation clause applied to the second
// placeholder syntax — `docker inspect --format '{{.Id}}'` is a shell brace, not
// an axis, and a command is not the bridge's to rewrite. Axes are walked in
// sorted key order and their values in declared order, so a two-axis image
// (`{B19_JAVA_DISTRO}-{B19_JAVA_SERIES}`) produces a stable cross product.
func expandAxes(lines []string, axes map[string][]string) []string {
	if len(axes) == 0 {
		return lines
	}
	for _, axis := range slices.Sorted(maps.Keys(axes)) {
		values := axes[axis]
		if len(values) == 0 {
			continue
		}
		token := "{" + axis + "}"
		expanded := make([]string, 0, len(lines))
		for _, line := range lines {
			if !strings.Contains(line, token) {
				expanded = append(expanded, line)
				continue
			}
			for _, value := range values {
				expanded = append(expanded, strings.ReplaceAll(line, token, value))
			}
		}
		lines = expanded
	}
	return lines
}

// ciMatrixAxes reads org.projectfile.ci.matrix.axes as axis name → declared
// values. Nil when the project has no matrix, which makes expandAxes a no-op for
// the ~130 single-image projects.
func ciMatrixAxes(doc *projectfile.Document) map[string][]string {
	matrix, ok := ciSubtree(doc)["matrix"].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := matrix["axes"].(map[string]any)
	if !ok {
		return nil
	}
	axes := make(map[string][]string, len(raw))
	for axis, list := range raw {
		items, ok := list.([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			if value, ok := item.(string); ok {
				axes[axis] = append(axes[axis], value)
			}
		}
	}
	return axes
}

// ciSubtree returns the org.projectfile.ci mapping, or nil. Nil maps index
// safely in Go, so every caller reads a key without a presence dance.
func ciSubtree(doc *projectfile.Document) map[string]any {
	raw, ok := projectfile.LookupExtension(doc, ciExtensionNS)
	if !ok {
		return nil
	}
	m, _ := raw.(map[string]any)
	return m
}

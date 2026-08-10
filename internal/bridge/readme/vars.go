// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
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
//
// Variants is a humanised summary of the matrix axes that fanned the commands out
// (e.g. "SAPI: cli, fpm · series: 8.5, 8.4, 8.3"). Empty for a single-image
// project — no matrix, no variant note. The template uses it to introduce the
// fan-out block so a reader can see every cell the project publishes without
// scanning the command lines themselves.
type sectionGroupView struct {
	Name     string
	Prefix   string
	Commands []string
	Postfix  string
	Variants string
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
	// Rank the groups before rendering. The slice arrives in MERGE order, which
	// puts every include-provided group ahead of the project's own (includes
	// concatenate loser-first), so declaration order would make a shared
	// fragment's fallback recipe lead the section. Stable, so equal ranks — the
	// state of every document that sets no priority — keep that merge order.
	declared = slices.Clone(declared)
	slices.SortStableFunc(declared, func(a, b pfmodel.ReadmeSectionGroup) int {
		return pfmodel.ByPriorityDesc(pfmodel.RankOf(a.Priority), pfmodel.RankOf(b.Priority))
	})
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
		Variants: matrixSummary(axes),
		Syntax:   syntax,
	}, true
}

// matrixSummary renders a project's matrix axes as one humanised line, in sorted
// axis order so a two-axis image reads deterministically. Empty when there is no
// matrix or the matrix carries no values — the caller treats empty as "no variant
// note", so a single-image project renders no fan-out block at all.
func matrixSummary(axes map[string][]string) string {
	if len(axes) == 0 {
		return ""
	}
	var parts []string
	for _, axis := range slices.Sorted(maps.Keys(axes)) {
		values := axes[axis]
		if len(values) == 0 {
			continue
		}
		parts = append(parts, axis+": "+strings.Join(values, ", "))
	}
	return strings.Join(parts, " · ")
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
//
// YAML scalar values are coerced to their STRING form: a matrix declared as
// `B19_LLVM_SERIES: [22, 21]` carries INTEGER items, and ci-resolver/m6e
// substitute them as plain tokens — so the README must do the same to fill the
// matching `{B19_LLVM_SERIES}` placeholder. Without this the integer axes were
// silently dropped and the placeholder survived into the published README.
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
			if value := scalarToString(item); value != "" {
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

// scalarToString renders a YAML scalar (the shape an untyped decoder yields) as
// the plain token m6e/ci-resolver substitute per matrix cell. Strings pass
// through; numbers and bools take their natural form (22, 8.5, true); anything
// composite or nil is not a matrix value and returns "" so the caller drops it.
func scalarToString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return fmt.Sprintf("%v", x)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", x)
	default:
		return ""
	}
}

// goalTag is the advisory tags[] value that opts a CI node into the README's
// "Pipeline entry points" list. A node tagged with it is highlighted; nodes
// without it are shown only when NO node carries the tag (the fallback), so a
// project that does not opt in keeps the full list it always had.
const goalTag = "readme"

// goalView is one pipeline entry point handed to the building template: the
// `make <name>` target plus its advisory description.
type goalView struct {
	Name        string
	Description string
}

// buildReadmeGoals lists the CI entry points the building block should
// highlight. A node joins the list when it is a forge goal (flagged `goal:
// true`) OR it opts in via the `readme` tag — the tag is the ONLY path for a
// non-goal such as ready-to-publish, a local pseudo-CI target that emits no
// forge workflow. The README prefers tagged nodes, falling back to ALL goals
// when none are tagged — so a project that never opts in keeps the full goal
// list, and one that tags a subset narrows to exactly that subset.
//
// Iteration is over sorted node names so the rendered list is stable regardless
// of map iteration order. A NULL node (the shape a bare `ci:` override leaves)
// is skipped, as is an entry point with no description (the template would
// otherwise print a literal <no value>).
func buildReadmeGoals(doc *projectfile.Document) []goalView {
	nodes, ok := ciSubtree(doc)["nodes"].(map[string]any)
	if !ok {
		return nil
	}
	var tagged, all []goalView
	for _, name := range slices.Sorted(maps.Keys(nodes)) {
		node, ok := nodes[name].(map[string]any)
		if !ok || node == nil {
			continue
		}
		isGoal, _ := node["goal"].(bool)
		taggedNode := hasTag(node["tags"], goalTag)
		// A non-goal without the tag (e.g. dev-container) is surfaced by its own
		// dev-loop line, not here.
		if !isGoal && !taggedNode {
			continue
		}
		description, _ := node["description"].(string)
		if description == "" {
			continue
		}
		entry := goalView{Name: name, Description: description}
		all = append(all, entry)
		if taggedNode {
			tagged = append(tagged, entry)
		}
	}
	if len(tagged) > 0 {
		genlog.Decision("readme_goals", "tagged", "filtered to readme-tagged nodes", strconv.Itoa(len(tagged)))
		return tagged
	}
	genlog.Decision("readme_goals", "all", "no readme tag; falling back to every goal", strconv.Itoa(len(all)))
	return all
}

// hasTag reports whether the advisory tags[] list contains tag. Extraction is
// pfmodel.TagsFrom so this reader and the forge-alias deriver cannot disagree
// on what a malformed tag list means.
func hasTag(tags any, tag string) bool {
	for _, s := range pfmodel.TagsFrom(tags) {
		if s == tag {
			return true
		}
	}
	return false
}

// hasDevContainer reports whether the CI DAG declares a dev-container node,
// which the building block advertises as the local dev loop. A dev-container is
// a selectable (non-goal) node contributed by the container plane
// (m6e/container/goals/publish.yaml); its presence is the signal a project
// supports `make ci-dag M6E_CI_TARGETS=dev`.
func hasDevContainer(doc *projectfile.Document) bool {
	nodes, ok := ciSubtree(doc)["nodes"].(map[string]any)
	if !ok {
		return false
	}
	_, present := nodes["dev-container"]
	return present
}

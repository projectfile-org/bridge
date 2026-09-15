// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/interp"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
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
// Subgroups is the per-destination split of those commands, set when they fan
// out over several sinks: each sink's lines render under their own heading with
// every matrix cell of that destination joined in ONE fence. Nil otherwise, and
// Commands carries the lines instead — the two are mutually exclusive.
type sectionGroupView struct {
	Name      string
	Prefix    string
	Commands  []string
	Postfix   string
	Syntax    string
	Subgroups []sectionSubgroupView
}

// sectionSubgroupView is one sink's slice of a group: the label its heading
// shows and the commands destined for that sink, matrix cells joined.
type sectionSubgroupView struct {
	Label    string
	Commands []string
}

// sinkView is one sink as the readme names it: the composed pull reference and
// the label a per-destination subsection heading shows.
type sinkView struct {
	Name  string
	Label string
	Ref   string
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
	axes := pfmodel.MatrixAxes(doc, ciExtensionNS)
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
		genlog.DebugRow("readme_section", name, "no group resolved (section dropped)", source)
		return nil
	}
	return &sectionView{Title: translate(lang, name+".title"), Groups: groups}
}

// buildSectionGroup renders one group. ok is false when the group declared
// commands but none survived expansion — the prose exists to introduce those
// commands, so keeping it alone would leave a lead-in pointing at nothing. A
// group that declared NO commands is pure prose and is kept as authored.
//
// A group whose commands ALL name a sink's composed reference splits into one
// subgroup per destination (buckets, sink priority order); anything else keeps
// the single-fence shape, so an npm recipe or a hand-written host never gains a
// destination heading nobody declared for it.
func buildSectionGroup(doc *projectfile.Document, group pfmodel.ReadmeSectionGroup, axes map[string][]string, source, lang string) (sectionGroupView, bool) {
	label := groupLabel(group.Name)
	var expanded []string
	for _, command := range group.Commands {
		lines, resolved := interp.ExpandFanOut(doc, command)
		if !resolved {
			genlog.DebugRow("readme_command", command, "unresolved reference (dropped)", label)
			continue
		}
		expanded = append(expanded, lines...)
	}
	if len(group.Commands) > 0 && len(expanded) == 0 {
		genlog.DebugRow("readme_group", label, "no command resolved (group dropped)", source)
		return sectionGroupView{}, false
	}
	syntax := group.Syntax
	if syntax == "" {
		syntax = syntaxDefault
	}
	view := sectionGroupView{
		Name:    group.Name,
		Prefix:  expandProse(doc, sectionText(group.Prefix, group.PrefixByLang, lang), label),
		Postfix: expandProse(doc, sectionText(group.Postfix, group.PostfixByLang, lang), label),
		Syntax:  syntax,
	}
	// Bucketing runs on lines still carrying {AXIS}: the composed ref is a
	// literal substring of its own pull line at that point, and axis expansion
	// would erase the match.
	sinks := readmeSinks(doc)
	subgroups, bucketed := bucketBySink(sinks, expanded)
	if bucketed && len(sinks) > 1 {
		for i := range subgroups {
			subgroups[i].Commands = pfmodel.ExpandAxes(subgroups[i].Commands, axes)
		}
		view.Subgroups = subgroups
		genlog.DebugRow("readme_group", label, "commands grouped by sink", source+" sinks="+strconv.Itoa(len(subgroups)))
		for _, sg := range subgroups {
			for _, line := range sg.Commands {
				genlog.DebugRow("readme_command", line, source, "group="+label+" sink="+sg.Label+" lang="+lang)
			}
		}
		return view, true
	}
	view.Commands = pfmodel.ExpandAxes(expanded, axes)
	for _, line := range view.Commands {
		genlog.DebugRow("readme_command", line, source, "group="+label+" lang="+lang)
	}
	return view, true
}

// readmeSinks lists the sinks a document carries, for naming per-destination
// subsections: each entry's composed `ref` (written by the ocisinks derive pass
// before render) and the label its heading shows — the declared `label`, else
// the sink name. Sorted longest-ref-first so a ref that is a prefix of another
// (`x/y:1` inside `x/y:12`) cannot steal its line during matching.
func readmeSinks(doc *projectfile.Document) []sinkView {
	raw, ok := projectfile.LookupExtension(doc, pfmodel.SinksExtensionNS)
	if !ok {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	out := make([]sinkView, 0, len(m))
	for name, v := range m {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		ref, _ := entry[pfmodel.SinkRefKey].(string)
		if ref == "" {
			continue
		}
		label, _ := entry[pfmodel.SinkLabelKey].(string)
		if label == "" {
			label = name
		}
		out = append(out, sinkView{Name: name, Label: label, Ref: ref})
	}
	slices.SortFunc(out, func(a, b sinkView) int { return len(b.Ref) - len(a.Ref) })
	return out
}

// bucketBySink partitions expanded command lines by the sink whose composed
// reference the line carries — a pull command names its destination's ref, so
// the ref is a literal substring of the line. ok is false when any line names
// no sink: a group referencing something else (an npm artifact, a hand-written
// host) renders as one fence rather than guessing a destination. Bucket order
// is first appearance, which is the priority-ordered fan-out order.
func bucketBySink(sinks []sinkView, lines []string) ([]sectionSubgroupView, bool) {
	if len(sinks) == 0 || len(lines) == 0 {
		return nil, false
	}
	var buckets []sectionSubgroupView
	at := make(map[string]int, len(sinks))
	for _, line := range lines {
		var owner *sinkView
		for i := range sinks {
			if strings.Contains(line, sinks[i].Ref) {
				owner = &sinks[i]
				break
			}
		}
		if owner == nil {
			return nil, false
		}
		if pos, seen := at[owner.Name]; seen {
			buckets[pos].Commands = append(buckets[pos].Commands, line)
			continue
		}
		at[owner.Name] = len(buckets)
		buckets = append(buckets, sectionSubgroupView{Label: owner.Label, Commands: []string{line}})
	}
	return buckets, true
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
		genlog.DebugRow("readme_prose", expanded, "unresolved reference (dropped)", label)
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
		genlog.DebugRow("readme_goals", "tagged", "filtered to readme-tagged nodes", strconv.Itoa(len(tagged)))
		return tagged
	}
	genlog.DebugRow("readme_goals", "all", "no readme tag; falling back to every goal", strconv.Itoa(len(all)))
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

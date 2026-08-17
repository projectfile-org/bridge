// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Repeated fixture keys, named so goconst sees one home per string.
const (
	keyImage       = "image"
	keyCommands    = "commands"
	keyName        = "name"
	keyPrefix      = "prefix"
	keyKind        = "kind"
	keyAxes        = "axes"
	keyRef         = "ref"
	keyMatrix      = "matrix"
	keyLabel       = "label"
	keyURL         = "url"
	keyDescription = "description"
	keyGoal        = "goal"
	// CI node-name and field fixtures shared across the goal-filter tests.
	nodePublished      = "published"
	nodeAnalyze        = "analyze"
	nodeDevContainer   = "dev-container"
	nodeReadyToPublish = "ready-to-publish"
	keyTags            = "tags"
	// devLoopCmd is the command the dev-loop line advertises — the node launcher
	// m6e derives for the dev-container node (core/ci/010-select.mk), so the line
	// stays in sync with the catalog (messages/<lang>.yaml building.intro.dev).
	devLoopCmd = "make " + nodeDevContainer
	// keyPriority is the §139 advisory priority key a link carries via Extra.
	keyPriority = "priority"
	// pathCLI stands in for a build output in the address-chain cases.
	pathCLI     = "dist/pf-cli"
	artifactsNS = "org.projectfile.artifacts"
	sinksNS     = "org.projectfile.sinks"
	// Sink-fixture names and the shared series axis, named so goconst sees one
	// home per string across the grouping tests.
	sinkGhcr   = "ghcr"
	sinkKiota  = "kiota"
	axisSeries = "B19_UBUNTU_SERIES"
	// Sink-fixture refs and series values, same goconst rule.
	refSinkGhcr    = "ghcr.io/o/demo:latest"
	refSinkKiota   = "kiota.ch/demo:latest"
	seriesResolute = "resolute"
	seriesNoble    = "noble"
	// refImage is the address every recipe below installs from — the ONE thing
	// a project has to declare for the docker-pull group to render.
	refImage = "docker pull ${org.projectfile.artifacts{kind=image}.ref}"
	// Fixture strings shared across the matrix, goal and link tests.
	descPublish    = "Publish"
	descAnalyze    = "Analyze"
	descDevLoop    = "Dev loop"
	urlExampleRepo = "https://example.com/repo"
	urlExampleX    = "https://x"
)

// imageArtifact is the shape a container project declares (or inherits from a
// shared fragment): one artifact, one pull reference.
func imageArtifact(ref string) map[string]any {
	return map[string]any{
		"primary": map[string]any{keyKind: pfmodel.ArtifactKindImage, keyRef: ref},
	}
}

// group builds one section group as it appears on disk: a list entry.
func group(name, prefix string, commands ...any) map[string]any {
	return map[string]any{
		keyName:     name,
		keyPrefix:   prefix,
		keyCommands: commands,
	}
}

// A structured installation group renders inline: heading, prefix prose, a fenced
// command block with the artifact's reference substituted, and postfix prose.
func TestInstallationGroupRendersInline(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		artifactsNS: imageArtifact("kiota.ch/d9t/dind:latest"),
		readmeNS: map[string]any{
			blockInstallation: []any{map[string]any{
				keyName:     keyImage,
				keyPrefix:   "Pull the published image:",
				keyCommands: []any{refImage},
				"postfix":   "Tags follow the release train.",
			}},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "## Installation")
	assert.Contains(t, out, "Pull the published image:")
	assert.Contains(t, out, "```sh\ndocker pull kiota.ch/d9t/dind:latest\n```")
	assert.Contains(t, out, "Tags follow the release train.")
}

// With no structured section but an INSTALL.md on disk, the block keeps its
// file-link behavior — the {{else}} fallback.
func TestInstallationFallsBackToFileLink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "INSTALL.md", "# how to install\n")
	pf := minimalDoc(t)

	out := renderDoc(t, dir, pf)

	assert.Contains(t, out, "## Installation")
	assert.Contains(t, out, "(INSTALL.md)")
	assert.NotContains(t, out, "```sh")
}

// TestUndeclaredArtifactDropsGroup is the rule the whole artifact-driven model
// rests on: a shared fragment may carry a recipe for an ecosystem this project
// has nothing to do with, and it disappears — lead-in sentence included — rather
// than publishing a command with a literal `${…}` in it. Because it is the LAST
// surviving group that decides, the section heading goes too.
func TestUndeclaredArtifactDropsGroup(t *testing.T) {
	pf := minimalDoc(t) // declares no artifacts at all
	pf.Extensions = map[string]any{
		readmeNS: map[string]any{
			blockInstallation: []any{
				group("npm", "Install the package:", "npm install ${org.projectfile.artifacts{kind=package}.name}"),
			},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.NotContains(t, out, "## Installation")
	assert.NotContains(t, out, "Install the package:")
	assert.NotContains(t, out, "${", "a README must never publish an unresolved reference")
}

// TestGroupsAreAlternativesNotSteps: two artifacts of different kinds render as
// two groups, each with its OWN lead-in and its OWN fence. Collapsing them into
// one fenced block would read as a sequence — a reader would run both.
func TestGroupsAreAlternativesNotSteps(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		artifactsNS: map[string]any{
			"primary": map[string]any{keyKind: pfmodel.ArtifactKindImage, keyRef: "kiota.ch/x/foo:latest"},
			"lib":     map[string]any{keyKind: pfmodel.ArtifactKindPackage, "registry": "npm", keyName: "foo"},
		},
		readmeNS: map[string]any{
			blockInstallation: []any{
				group("npm", "Install the package:", "npm install ${org.projectfile.artifacts{kind=package}.name}"),
				group(keyImage, "Or pull the image:", refImage),
			},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "Install the package:")
	assert.Contains(t, out, "```sh\nnpm install foo\n```")
	assert.Contains(t, out, "Or pull the image:")
	assert.Contains(t, out, "```sh\ndocker pull kiota.ch/x/foo:latest\n```")
	// Two fences, not one shared block holding both commands.
	assert.NotContains(t, out, "npm install foo\ndocker pull")
}

// TestGroupRedeclarationReplacesInPlace: includes union sequences, so overriding
// an inherited group means redeclaring its NAME. Last wins, and the position is
// kept so an override never reorders the section.
func TestGroupRedeclarationReplacesInPlace(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		readmeNS: map[string]any{
			blockUsage: []any{
				group("run", "Inherited lead-in:", "inherited-command"),
				group("second", "Second group:", "second-command"),
				group("run", "Overridden lead-in:", "overridden-command"),
			},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "Overridden lead-in:")
	assert.NotContains(t, out, "Inherited lead-in:")
	assert.NotContains(t, out, "inherited-command")
	assert.Less(t, index(out, "Overridden lead-in:"), index(out, "Second group:"),
		"an override must keep the position it inherited")
}

// index is strings.Index as an int for ordering assertions.
func index(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// TestSeriesImageFansOutFromAxisList: a base image built once per Ubuntu series
// declares ONE artifact whose reference addresses the matrix axis as a LIST, and
// the command renders one line per series. No axis-placeholder machinery is
// involved — the generic list fan-out does it.
func TestSeriesImageFansOutFromAxisList(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		ciExtensionNS: map[string]any{
			keyMatrix: map[string]any{keyAxes: map[string]any{
				axisSeries: []any{seriesResolute, seriesNoble},
			}},
		},
		artifactsNS: imageArtifact(
			"kiota.ch/b19/ubuntu/${org.projectfile.ci.matrix.axes.B19_UBUNTU_SERIES[]}:latest",
		),
		readmeNS: map[string]any{
			blockInstallation: []any{group(keyImage, "Pull one:", refImage)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "docker pull kiota.ch/b19/ubuntu/resolute:latest")
	assert.Contains(t, out, "docker pull kiota.ch/b19/ubuntu/noble:latest")
	assert.NotContains(t, out, "docker pull kiota.ch/b19/ubuntu:latest")
}

// TestSeriesImageFansOutFromAxisPlaceholder: the same series project may instead
// carry the `{AXIS}` placeholder m6e and ci-resolver substitute per cell — the
// spelling `ci.image` already uses. Both routes must produce the same lines.
func TestSeriesImageFansOutFromAxisPlaceholder(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		ciExtensionNS: map[string]any{
			keyImage: "b19/ubuntu/{B19_UBUNTU_SERIES}",
			keyMatrix: map[string]any{keyAxes: map[string]any{
				axisSeries: []any{seriesResolute, seriesNoble},
			}},
		},
		artifactsNS: imageArtifact("kiota.ch/${org.projectfile.ci.image}:latest"),
		readmeNS: map[string]any{
			blockInstallation: []any{group(keyImage, "Pull one:", refImage)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "docker pull kiota.ch/b19/ubuntu/resolute:latest")
	assert.Contains(t, out, "docker pull kiota.ch/b19/ubuntu/noble:latest")
}

// TestUndeclaredBraceSurvives: only axes the document DECLARES are substituted.
// A shell brace in a command is not the bridge's to rewrite — the old derivation
// nil'd out any line still carrying one, which would delete this command.
func TestUndeclaredBraceSurvives(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		ciExtensionNS: map[string]any{
			keyMatrix: map[string]any{keyAxes: map[string]any{"GOARCH": []any{archAMD64}}},
		},
		readmeNS: map[string]any{
			blockUsage: []any{group("inspect", "Inspect it:", `docker inspect --format '{{.Id}}' x`)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, `docker inspect --format '{{.Id}}' x`)
}

// TestIntegerAxisValuesAreCoerced: YAML decodes a bare `B19_LLVM_SERIES: [22, 21]`
// as INTEGER items, but ci-resolver/m6e substitute the plain token "22"/"21" per
// cell. The README must coerce the same way or the {AXIS} placeholder survives
// into the published pull line — which is exactly the bug that left b19/llvm's
// README showing `llvm-{B19_LLVM_SERIES}` while b19/php (quoted strings) resolved.
func TestIntegerAxisValuesAreCoerced(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		ciExtensionNS: map[string]any{
			keyImage: "b19/llvm/{B19_LLVM_SERIES}",
			keyMatrix: map[string]any{keyAxes: map[string]any{
				"B19_LLVM_SERIES": []any{22, 21}, // integers, not quoted strings
			}},
		},
		artifactsNS: imageArtifact("kiota.ch/${org.projectfile.ci.image}:latest"),
		readmeNS: map[string]any{
			blockInstallation: []any{group(keyImage, "Pull one:", refImage)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "docker pull kiota.ch/b19/llvm/22:latest")
	assert.Contains(t, out, "docker pull kiota.ch/b19/llvm/21:latest")
	assert.NotContains(t, out, "{B19_LLVM_SERIES}", "an integer axis must substitute, not survive")
}

// TestScalarToStringCoercesAxisShapes pins the matrix-cell rendering for every
// YAML scalar shape a projectfile may carry.
func TestScalarToStringCoercesAxisShapes(t *testing.T) {
	assert.Equal(t, "22", scalarToString(22))
	assert.Equal(t, "8.5", scalarToString(8.5))
	assert.Equal(t, "cli", scalarToString("cli"))
	assert.Equal(t, "true", scalarToString(true))
	assert.Empty(t, scalarToString(nil), "nil is not a matrix value")
	assert.Empty(t, scalarToString([]any{"x"}), "a composite is not a matrix value")
}

// TestMatrixSectionRendersJoinedBlock: a matrix install group renders every
// cell in ONE fenced block. The old layout — first cell showcased alone, a
// variants note, then the remaining cells — is retired: the joined fence
// already lists every cell, and a note would restate what the lines show.
func TestMatrixSectionRendersJoinedBlock(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		ciExtensionNS: map[string]any{
			keyImage: "b19/llvm/{B19_LLVM_SERIES}",
			keyMatrix: map[string]any{keyAxes: map[string]any{
				"B19_LLVM_SERIES": []any{22, 21},
			}},
		},
		artifactsNS: imageArtifact("kiota.ch/${org.projectfile.ci.image}:latest"),
		readmeNS: map[string]any{
			blockInstallation: []any{group(keyImage, "Pull the published image:", refImage)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "```sh\ndocker pull kiota.ch/b19/llvm/22:latest\ndocker pull kiota.ch/b19/llvm/21:latest\n```")
	assert.NotContains(t, out, "Available variants")
}

// TestMultiSinkGroupRendersPerSinkSubsections: when a group's commands fan out
// over several sinks, each destination renders its own "<verb> <label>"
// subsection with one joined fence, ordered by the fan-out — which is sink
// priority, descending. A sink declaring no label is named by its sink name.
// The usage block pairs with the install block so the test also pins the
// per-block verb: a shared word repeats the heading in both blocks (MD024).
func TestMultiSinkGroupRendersPerSinkSubsections(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		sinksNS: map[string]any{
			sinkKiota: map[string]any{keyRef: refSinkKiota, pfmodel.SinkRoleKey: pfmodel.SinkRolePrimary, keyPriority: 10},
			sinkGhcr:  map[string]any{keyRef: refSinkGhcr, pfmodel.SinkRoleKey: pfmodel.SinkRolePrimary, keyPriority: 90, pfmodel.SinkLabelKey: "GHCR"},
		},
		artifactsNS: imageArtifact("${org.projectfile.sinks{role=primary}.ref}"),
		readmeNS: map[string]any{
			blockInstallation: []any{group(keyImage, "Pull the published image:", refImage)},
			blockUsage:        []any{group(keyImage, "Build on it:", "FROM ${org.projectfile.sinks{role=primary}.ref}")},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "### Pull from GHCR")
	assert.Contains(t, out, "### Pull from kiota")
	assert.Less(t, index(out, "### Pull from GHCR"), index(out, "### Pull from kiota"),
		"the fan-out order (priority desc) orders the subsections")
	assert.Contains(t, out, "```sh\ndocker pull ghcr.io/o/demo:latest\n```")
	assert.Contains(t, out, "```sh\ndocker pull kiota.ch/demo:latest\n```")
	assert.Contains(t, out, "### From GHCR", "usage keeps the bare verb")
	assert.Equal(t, 1, strings.Count(out, "### From GHCR"),
		"install and usage must not repeat a heading")
}

// TestSinkSubsectionsJoinMatrixCells: each destination's fence lists EVERY
// matrix cell of that sink, joined — one block per registry, never one per
// cell.
func TestSinkSubsectionsJoinMatrixCells(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		ciExtensionNS: map[string]any{
			keyMatrix: map[string]any{keyAxes: map[string]any{
				axisSeries: []any{seriesResolute, seriesNoble},
			}},
		},
		sinksNS: map[string]any{
			sinkGhcr:  map[string]any{keyRef: "ghcr.io/o/demo/{B19_UBUNTU_SERIES}:latest", pfmodel.SinkRoleKey: pfmodel.SinkRolePrimary, keyPriority: 90, pfmodel.SinkLabelKey: "GHCR"},
			sinkKiota: map[string]any{keyRef: "kiota.ch/demo/{B19_UBUNTU_SERIES}:latest", pfmodel.SinkRoleKey: pfmodel.SinkRolePrimary, keyPriority: 10},
		},
		artifactsNS: imageArtifact("${org.projectfile.sinks{role=primary}.ref}"),
		readmeNS: map[string]any{
			blockInstallation: []any{group(keyImage, "Pull the published image:", refImage)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "### Pull from GHCR\n\n```sh\ndocker pull ghcr.io/o/demo/resolute:latest\ndocker pull ghcr.io/o/demo/noble:latest\n```")
	assert.Contains(t, out, "### Pull from kiota\n\n```sh\ndocker pull kiota.ch/demo/resolute:latest\ndocker pull kiota.ch/demo/noble:latest\n```")
}

// TestSingleSinkGroupRendersPlain: a document carrying ONE sink (the legacy
// single-registry projects) keeps the plain prefix + fence shape — a sole
// destination needs no subsection heading naming it.
func TestSingleSinkGroupRendersPlain(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		sinksNS: map[string]any{
			sinkKiota: map[string]any{keyRef: refSinkKiota, pfmodel.SinkRoleKey: pfmodel.SinkRolePrimary},
		},
		artifactsNS: imageArtifact("${org.projectfile.sinks{role=primary}.ref}"),
		readmeNS: map[string]any{
			blockInstallation: []any{group(keyImage, "Pull the published image:", refImage)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "```sh\ndocker pull kiota.ch/demo:latest\n```")
	assert.NotContains(t, out, "### Pull from", "one sink needs no destination heading")
}

// TestNonSinkCommandsStayPlain: a group whose commands reference something no
// sink declares renders one fence even in a multi-sink document — bucketing
// must not guess a destination nobody named.
func TestNonSinkCommandsStayPlain(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		sinksNS: map[string]any{
			sinkGhcr:  map[string]any{keyRef: refSinkGhcr, pfmodel.SinkRoleKey: pfmodel.SinkRolePrimary, keyPriority: 90},
			sinkKiota: map[string]any{keyRef: refSinkKiota, pfmodel.SinkRoleKey: pfmodel.SinkRolePrimary, keyPriority: 10},
		},
		artifactsNS: imageArtifact("registry.example/other/demo:latest"),
		readmeNS: map[string]any{
			blockInstallation: []any{group(keyImage, "Pull the published image:", refImage)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "```sh\ndocker pull registry.example/other/demo:latest\n```")
	assert.NotContains(t, out, "### Pull from")
}

// TestFallbackSinkGroupKeepsProseAndHeading: the shared fragment's origin group
// references the {role=fallback} sink alone; in a multi-sink document it keeps
// its warning prose and its one bucket is named like any other destination.
func TestFallbackSinkGroupKeepsProseAndHeading(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		sinksNS: map[string]any{
			sinkGhcr:  map[string]any{keyRef: refSinkGhcr, pfmodel.SinkRoleKey: pfmodel.SinkRolePrimary, keyPriority: 90},
			sinkKiota: map[string]any{keyRef: refSinkKiota, pfmodel.SinkRoleKey: "fallback", keyPriority: 10},
		},
		artifactsNS: imageArtifact("${org.projectfile.sinks{role=primary}.ref}"),
		readmeNS: map[string]any{
			blockInstallation: []any{
				group(keyImage, "Pull the published image:", refImage),
				group("image-fallback", "If the registries above are unreachable, pull from the origin instead:",
					"docker pull ${org.projectfile.sinks{role=fallback}.ref}"),
			},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "If the registries above are unreachable")
	assert.Contains(t, out, "### Pull from ghcr")
	assert.Contains(t, out, "### Pull from kiota")
	assert.Less(t, index(out, "### Pull from ghcr"), index(out, "If the registries above"),
		"the fallback group renders after the primary one")
}

// TestNonMatrixSectionRendersSingleBlock: a single-image project (no matrix)
// renders exactly one fenced block with no variant note — the matrix reshape
// must not add a redundant "variants" line for the ~130 single-image projects.
func TestNonMatrixSectionRendersSingleBlock(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		artifactsNS: imageArtifact("kiota.ch/d9t/dind:latest"),
		readmeNS: map[string]any{
			blockInstallation: []any{group(keyImage, "Pull the published image:", refImage)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "```sh\ndocker pull kiota.ch/d9t/dind:latest\n```")
	assert.NotContains(t, out, "Available variants")
}

// ciNodes builds an org.projectfile.ci.nodes extension for the goal-filter
// tests: each entry is name → {goal, description, tags}.
func ciNodes(entries ...map[string]any) map[string]any {
	nodes := map[string]any{}
	for _, e := range entries {
		name, _ := e[keyName].(string)
		if name == "" {
			continue
		}
		entry := map[string]any{}
		for k, v := range e {
			if k != keyName {
				entry[k] = v
			}
		}
		nodes[name] = entry
	}
	return map[string]any{ciExtensionNS: map[string]any{"nodes": nodes}}
}

// TestReadmeGoalsPrefersTagged: a goal tagged `readme` is the only one
// highlighted, even when other goals exist. This is how a project narrows the
// README's "Pipeline entry points" to the headline target.
func TestReadmeGoalsPrefersTagged(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = ciNodes(
		map[string]any{keyName: nodePublished, keyGoal: true, keyDescription: descPublish, keyTags: []any{goalTag}},
		map[string]any{keyName: nodeAnalyze, keyGoal: true, keyDescription: descAnalyze},
	)
	got := buildReadmeGoals(pf)
	require.Len(t, got, 1)
	assert.Equal(t, nodePublished, got[0].Name)
}

// TestReadmeGoalsFallbackAllWhenNoneTagged: a project that tags no goal keeps
// every goal in the list — the opt-in never removes information a project that
// never heard of the tag was showing.
func TestReadmeGoalsFallbackAllWhenNoneTagged(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = ciNodes(
		map[string]any{keyName: nodePublished, keyGoal: true, keyDescription: descPublish},
		map[string]any{keyName: nodeAnalyze, keyGoal: true, keyDescription: descAnalyze},
	)
	got := buildReadmeGoals(pf)
	require.Len(t, got, 2)
}

// TestReadmeGoalsSkipsNonGoalsAndDescriptionless: a non-goal WITHOUT the tag
// (dev-container) and a goal lacking a description are dropped, so the list
// never prints a literal <no value>.
func TestReadmeGoalsSkipsNonGoalsAndDescriptionless(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = ciNodes(
		// not a goal, not tagged
		map[string]any{keyName: nodeDevContainer, keyDescription: descDevLoop},
		// no description
		map[string]any{keyName: "tagless", keyGoal: true},
	)
	assert.Empty(t, buildReadmeGoals(pf))
}

// TestReadmeGoalsAdmitsTaggedNonGoal: a node that is NOT a forge goal but opts
// in via `tags: [readme]` joins the highlighted list — the path for
// ready-to-publish, a local pseudo-CI target that emits no forge workflow yet
// still headlines the README's entry points. No-tag goals fall away because a
// tagged node exists (the prefer-tagged rule).
func TestReadmeGoalsAdmitsTaggedNonGoal(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = ciNodes(
		map[string]any{keyName: nodeReadyToPublish, keyDescription: "Run the pseudo-CI pipeline", keyTags: []any{goalTag}}, // not goal:true
		map[string]any{keyName: nodeAnalyze, keyGoal: true, keyDescription: descAnalyze},                                   // goal, no tag
	)
	got := buildReadmeGoals(pf)
	require.Len(t, got, 1)
	assert.Equal(t, nodeReadyToPublish, got[0].Name)
}

// TestBuildingBlockAdvertisesDevContainer: when the DAG declares a
// dev-container node, the building block renders the dev-loop line — answering
// the question "how do I run this locally" the README previously left open.
func TestBuildingBlockAdvertisesDevContainer(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = ciNodes(
		map[string]any{keyName: nodePublished, keyGoal: true, keyDescription: descPublish, keyTags: []any{goalTag}},
		map[string]any{keyName: nodeDevContainer, keyDescription: descDevLoop},
	)
	out := renderDoc(t, t.TempDir(), pf)
	assert.Contains(t, out, devLoopCmd)
	assert.Contains(t, out, "make` with no arguments")
}

// TestBuildingBlockIntrosLeadTheSection: the make/dev-loop paragraphs open the
// Building section and the pipeline entry-point list follows them — the reader
// gets "how do I build this" before the pipeline map.
func TestBuildingBlockIntrosLeadTheSection(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = ciNodes(
		map[string]any{keyName: nodePublished, keyGoal: true, keyDescription: descPublish, keyTags: []any{goalTag}},
		map[string]any{keyName: nodeDevContainer, keyDescription: descDevLoop},
	)
	out := renderDoc(t, t.TempDir(), pf)
	introIdx := strings.Index(out, "make` with no arguments")
	devIdx := strings.Index(out, devLoopCmd)
	goalsIdx := strings.Index(out, "Pipeline entry points")
	require.NotEqual(t, -1, introIdx)
	require.NotEqual(t, -1, devIdx)
	require.NotEqual(t, -1, goalsIdx)
	assert.Less(t, introIdx, devIdx, "the make paragraph opens the section")
	assert.Less(t, devIdx, goalsIdx, "the dev-container paragraph comes second")
}

// TestBuildingBlockOmitsDevLoopWhenNoDevContainer: a project with no
// dev-container node renders the make-intro but not the dev-loop line.
func TestBuildingBlockOmitsDevLoopWhenNoDevContainer(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = ciNodes(
		map[string]any{keyName: nodePublished, keyGoal: true, keyDescription: descPublish, keyTags: []any{goalTag}},
	)
	out := renderDoc(t, t.TempDir(), pf)
	assert.Contains(t, out, "make` with no arguments")
	assert.NotContains(t, out, devLoopCmd)
}

// TestLinksSingleGroupDropsSubheading: a project whose links all fall in one
// group (the near-universal case — source-code + bugs) renders `## Links` with
// NO `###` subheading. The heading would only repeat what the section title
// already says.
func TestLinksSingleGroupDropsSubheading(t *testing.T) {
	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{
		readmeLink(linkTypeSourceCode, urlExampleRepo),
		readmeLink("bugs", "https://example.com/issues"),
	}
	out := renderDoc(t, t.TempDir(), pf)
	assert.Contains(t, out, "## Links")
	assert.Contains(t, out, "[Source Code](https://example.com/repo)")
	assert.Contains(t, out, "[Issue tracker](https://example.com/issues)")
	assert.NotContains(t, out, "### Project", "single group renders no subheading")
}

// TestLinksMultipleGroupsKeepSubheadings: a project mixing project and community
// links keeps the per-group subheadings, because they now carry information the
// section title does not.
func TestLinksMultipleGroupsKeepSubheadings(t *testing.T) {
	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{
		readmeLink(linkTypeSourceCode, urlExampleRepo),
		readmeLink("chat", "https://example.com/chat"),
	}
	out := renderDoc(t, t.TempDir(), pf)
	assert.Contains(t, out, "### Project")
	assert.Contains(t, out, "### Community")
}

// TestLinksRequireReadmeTag: the Links section is curated — a link renders
// there only when tagged `readme`, and a project whose links carry no tag
// renders no section at all rather than an uncurated dump.
func TestLinksRequireReadmeTag(t *testing.T) {
	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{
		{Type: linkTypeSourceCode, URL: urlExampleRepo},
		{Type: "bugs", URL: "https://example.com/issues"},
	}
	out := renderDoc(t, t.TempDir(), pf)
	assert.NotContains(t, out, "## Links")
	assert.NotContains(t, out, "https://example.com/repo")
}

// readmeLink builds one link fixture tagged for the readme's Links section —
// the shape a YAML-decoded `tags: [readme]` entry takes after the spec §139
// round-trip stashes tags into Link.Extra.
func readmeLink(linkType, url string) projectfile.Link {
	return projectfile.Link{
		Type:  linkType,
		URL:   url,
		Extra: map[string]any{keyTags: []any{linksTagReadme}},
	}
}

// relatedLink builds one top-level link fixture tagged `related` — the form a
// YAML-decoded `tags: [related]` entry takes after the spec §139 round-trip
// stashes tags into Link.Extra. links... lets a caller stack several siblings.
func relatedLink(label, url string) projectfile.Link {
	return projectfile.Link{
		Type:  linkTypeSourceCode,
		URL:   url,
		Label: &projectfile.LocalizedString{Bare: label},
		Extra: map[string]any{keyTags: []any{relatedTag}},
	}
}

// TestRelatedBarRendersAfterBadges: a link tagged `related` renders as a
// pipe-separated bar immediately after the badges block — the navigational
// position, before the project's own content. The bar is headingless.
func TestRelatedBarRendersAfterBadges(t *testing.T) {
	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{
		relatedLink("F5M/I2P", "https://kiota.ch/f5m/i2p"),
		relatedLink("F5M/Tor", "https://kiota.ch/f5m/tor"),
	}
	out := renderDoc(t, t.TempDir(), pf)
	assert.Contains(t, out, "[F5M/I2P](https://kiota.ch/f5m/i2p) | [F5M/Tor](https://kiota.ch/f5m/tor)")
	// The bar lands before the license section (the navigational slot).
	barIdx := index(out, "[F5M/I2P]")
	licenseIdx := index(out, "## License")
	assert.Less(t, barIdx, licenseIdx, "related bar renders above the license section")
}

// TestRelatedBarSkippedWhenNoTaggedLinks: a project whose links carry no
// `related` tag renders no bar — the block self-suppresses, not an empty line.
func TestRelatedBarSkippedWhenNoTaggedLinks(t *testing.T) {
	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{{Type: linkTypeSourceCode, URL: urlExampleRepo}}
	out := renderDoc(t, t.TempDir(), pf)
	assert.NotContains(t, out, "F5M/I2P")
}

// TestRelatedLinkExcludedFromLinksBlock: a link tagged `related` renders in the
// bar and NOT also in the regular Links section — surfacing a sibling twice is
// pure noise. A readme-tagged source-code link still appears under Links.
func TestRelatedLinkExcludedFromLinksBlock(t *testing.T) {
	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{
		relatedLink("Sibling", urlExampleX),
		readmeLink(linkTypeSourceCode, urlExampleRepo),
	}
	out := renderDoc(t, t.TempDir(), pf)
	// The tagged sibling is in the bar …
	assert.Contains(t, out, "[Sibling]("+urlExampleX+")")
	// … and not duplicated under the Links section.
	assert.NotContains(t, out, "[Sibling](https://example.com/repo)")
	// The readme-tagged link still renders under Links.
	assert.Contains(t, out, "[Source Code]("+urlExampleRepo+")")
}

// TestRelatedLabelFromLinkLabel: the bar text comes from the link's own label,
// not its `type`. A sibling tagged `related` keeps its real type (source-code)
// yet reads by its human label in the bar.
func TestRelatedLabelFromLinkLabel(t *testing.T) {
	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{
		relatedLink("Projectfile CLI", urlExampleX),
	}
	out := renderDoc(t, t.TempDir(), pf)
	assert.Contains(t, out, "[Projectfile CLI]("+urlExampleX+")")
	assert.NotContains(t, out, "[Source Code]("+urlExampleX+")", "bar uses link.label, not the type label")
}

// relatedLinkPRIORITY builds a related-tagged link carrying a §139 `priority`
// key, the shape a YAML-decoded `priority: 300` entry takes after round-trip
// stashes it into Link.Extra. label/url identify the fixture; pri is the
// advisory priority.
func relatedLinkPriority(label, url string, pri int) projectfile.Link {
	l := relatedLink(label, url)
	l.Extra[keyPriority] = pri
	return l
}

// TestRelatedLinksPriorityOrdersBar: priority orders the related-projects bar
// (higher first), stable so equal priorities keep document order. A pinned
// sibling rises to the head of the bar.
func TestRelatedLinksPriorityOrdersBar(t *testing.T) {
	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{
		relatedLink("First", "https://example.test/first"),
		relatedLinkPriority("Pinned", "https://example.test/pinned", 300),
		relatedLink("Third", "https://example.test/third"),
	}
	out := renderDoc(t, t.TempDir(), pf)
	barIdx := index(out, "[Pinned]")
	firstIdx := index(out, "[First]")
	thirdIdx := index(out, "[Third]")
	assert.Less(t, barIdx, firstIdx, "pinned sibling renders before the default-priority ones")
	assert.Less(t, firstIdx, thirdIdx, "equal-priority siblings keep declaration order")
}

// TestLinkGroupsPriorityOrdersWithinBucket: priority orders links WITHIN a
// category bucket (higher first); the category order itself is unchanged.
func TestLinkGroupsPriorityOrdersWithinBucket(t *testing.T) {
	home := readmeLink("homepage", "https://example.test/home")
	home.Label = &projectfile.LocalizedString{Bare: "Home"}
	source := readmeLink(linkTypeSourceCode, "https://example.test/repo")
	source.Label = &projectfile.LocalizedString{Bare: "Source"}
	source.Extra[keyPriority] = 300
	pf := minimalDoc(t)
	pf.Links = []projectfile.Link{home, source}
	groups := buildLinkGroups(pf, "")
	require.Len(t, groups, 1, "both links fall in the project bucket")
	require.Len(t, groups[0].Links, 2)
	assert.Equal(t, "https://example.test/repo", groups[0].Links[0].URL,
		"higher-priority source-code link renders first within the project bucket")
	assert.Equal(t, "https://example.test/home", groups[0].Links[1].URL)
}

// Generic interpolation: any field address resolves, anything that is not one
// survives verbatim, and `$$` escapes to a literal `$`.
func TestCommandInterpolation(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		readmeNS: map[string]any{
			blockUsage: []any{map[string]any{
				keyCommands: []any{"run ${identity.name} --license ${license.spdx} $HOME $${literal}"},
			}},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "run "+pf.Identity.Name+" --license "+pf.License.Spdx+" $HOME ${literal}")
}

// Per-language prefix/postfix resolve from the lang map in a localized render.
func TestSectionTextLocalized(t *testing.T) {
	byLang := map[string]string{"es": "Descarga la imagen:"}
	assert.Equal(t, "Descarga la imagen:", sectionText("Pull the image:", byLang, "es"))
	assert.Equal(t, "Pull the image:", sectionText("Pull the image:", byLang, "uk"))
	assert.Equal(t, "Pull the image:", sectionText("Pull the image:", byLang, ""))
}

// parseReadmeSection drops empty and malformed entries so the block falls through
// to its file probe rather than rendering a bare heading.
func TestParseReadmeSectionDropsEmpty(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		readmeNS: map[string]any{
			blockInstallation: []any{map[string]any{}}, // an entry with no content
			blockUsage:        "not-a-list",
			blockBuilding:     []any{"not-a-map"},
		},
	}
	ext, err := pfmodel.GetReadmeExtension(pf)
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Empty(t, ext.Sections[blockInstallation], "content-free group is dropped")
	assert.Empty(t, ext.Sections[blockUsage], "non-list section is dropped")
	assert.Empty(t, ext.Sections[blockBuilding], "non-map group is dropped")
}

// TestProseOnlyGroupSurvives: a group that declares NO commands is intentional
// prose and must be kept — the drop rule only fires on a group whose declared
// commands all failed to resolve.
func TestProseOnlyGroupSurvives(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		readmeNS: map[string]any{
			blockUsage: []any{map[string]any{
				keyName:   "note",
				keyPrefix: "This library has no CLI; import it instead.",
			}},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "## Usage")
	assert.Contains(t, out, "This library has no CLI; import it instead.")
}

// TestSectionGroupPriorityOutranksMergeOrder is the ordering primitive the
// multi-registry recipes rest on. The slice below arrives in the order a MERGE
// produces: includes concatenate loser-first, so the shared fragment's fallback
// group sits FIRST even though it must render last. Only a declared priority can
// invert that — a project cannot delete an inherited group, and reordering by
// declaration would need the fragment to know what consumes it.
func TestSectionGroupPriorityOutranksMergeOrder(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		readmeNS: map[string]any{
			blockInstallation: []any{
				map[string]any{ // as if contributed by an include
					keyName:     "fallback",
					keyPrefix:   "Last resort:",
					keyCommands: []any{"docker pull kiota.ch/x:latest"},
					keyPriority: 10,
				},
				map[string]any{ // the project's own
					keyName:     "preferred",
					keyPrefix:   "Recommended:",
					keyCommands: []any{"docker pull ghcr.io/o/x:latest"},
					keyPriority: 90,
				},
			},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Less(t, strings.Index(out, "Recommended:"), strings.Index(out, "Last resort:"),
		"a higher priority must render above a group the merge placed first")
}

// TestSectionGroupOrderStableWithoutPriority pins the compatibility half: a
// document that declares no priority renders in the order it always did, so the
// ~130 projectfiles carrying no priority key produce a byte-identical README.
func TestSectionGroupOrderStableWithoutPriority(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		readmeNS: map[string]any{
			blockInstallation: []any{
				map[string]any{keyName: "alpha", keyPrefix: "Alpha:", keyCommands: []any{"echo a"}},
				map[string]any{keyName: "beta", keyPrefix: "Beta:", keyCommands: []any{"echo b"}},
			},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Less(t, strings.Index(out, "Alpha:"), strings.Index(out, "Beta:"),
		"equal (unset) priorities keep declaration order")
}

// TestUnresolvedProseIsDropped: a lead-in whose reference nothing answers is
// emptied rather than published with a literal `${…}`; the commands that DID
// resolve still render.
func TestUnresolvedProseIsDropped(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		readmeNS: map[string]any{
			blockUsage: []any{map[string]any{
				keyName:     "run",
				keyPrefix:   "Run ${org.projectfile.nope.value}:",
				keyCommands: []any{"make dc-up"},
			}},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "make dc-up")
	assert.NotContains(t, out, "${org.projectfile.nope.value}")
}

// TestArtifactsBlockLists is the inventory: what the project ships, said once,
// with a service's ports spelled out — the "MySQL on 3306" line.
func TestArtifactsBlockLists(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		artifactsNS: map[string]any{
			"db": map[string]any{
				keyKind:   pfmodel.ArtifactKindService,
				"summary": "MySQL-compatible database",
				"ports":   []any{3306, map[string]any{"port": 9104, keyName: "metrics"}},
			},
			"cli": map[string]any{
				keyKind:   pfmodel.ArtifactKindBinary,
				"command": "pf-bridge",
			},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, "## What this provides")
	assert.Contains(t, out, "**Executable** `pf-bridge`")
	assert.Contains(t, out, "**Service**")
	assert.Contains(t, out, "`3306`")
	assert.Contains(t, out, "`9104 (metrics)`")
	assert.Contains(t, out, "MySQL-compatible database")
}

// TestArtifactsBlockSkippedWhenNoneDeclared: the block drops silently, like every
// other probe-driven block.
func TestArtifactsBlockSkippedWhenNoneDeclared(t *testing.T) {
	out := renderDoc(t, t.TempDir(), minimalDoc(t))
	assert.NotContains(t, out, "## What this provides")
}

// TestPlatformsBlockRendersCartesianProduct: operating-system × architecture
// (spec §4.8a) renders as the OCI platform set, sorted, one bullet per
// platform. A reader scanning a b19 image README sees `linux/amd64`,
// `linux/arm64` … exactly what `docker pull --platform` takes.
func TestPlatformsBlockRendersCartesianProduct(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		osExtensionNS:   []any{"linux"},
		archExtensionNS: []any{"arm64", archAMD64},
	}
	out := renderDoc(t, t.TempDir(), pf)
	assert.Contains(t, out, "## Supported platforms")
	assert.Contains(t, out, "- `linux/amd64`\n- `linux/arm64`")
}

// TestPlatformsBlockDefaultsOSForArchesOnly: a project that declares only
// architectures (the common b19 case) still ships — the OS defaults to linux,
// the spec's stated assumption for container builds.
func TestPlatformsBlockDefaultsOSForArchesOnly(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		archExtensionNS: []any{archAMD64},
	}
	out := renderDoc(t, t.TempDir(), pf)
	assert.Contains(t, out, "`linux/amd64`")
}

// TestPlatformsBlockSkippedWhenNeitherDeclared: a project that targets nothing
// publishable drops the block — the platforms section is a build-target
// statement, not a property every project has.
func TestPlatformsBlockSkippedWhenNeitherDeclared(t *testing.T) {
	out := renderDoc(t, t.TempDir(), minimalDoc(t))
	assert.NotContains(t, out, "## Supported platforms")
}

// TestArtifactAddressPicksByKind pins the per-kind address chain: the ONE string
// that identifies each artifact to a reader.
func TestArtifactAddressPicksByKind(t *testing.T) {
	cases := []struct {
		name string
		art  pfmodel.Artifact
		want string
	}{
		{"image by ref", pfmodel.Artifact{Kind: pfmodel.ArtifactKindImage, Ref: "r/i:1"}, "r/i:1"},
		{"binary by command", pfmodel.Artifact{Kind: pfmodel.ArtifactKindBinary, Command: "pf-cli", Path: pathCLI}, "pf-cli"},
		{"binary falls back to path", pfmodel.Artifact{Kind: pfmodel.ArtifactKindBinary, Path: pathCLI}, pathCLI},
		{"package by name", pfmodel.Artifact{Kind: pfmodel.ArtifactKindPackage, Name: "@scope/x"}, "@scope/x"},
		{"module by path", pfmodel.Artifact{Kind: pfmodel.ArtifactKindModule, Module: "kiota.ch/x/v2"}, "kiota.ch/x/v2"},
		{"website by url", pfmodel.Artifact{Kind: pfmodel.ArtifactKindWebsite, URL: "https://x"}, "https://x"},
		{"unknown kind still resolves", pfmodel.Artifact{Kind: "helm-chart", Path: "charts/x"}, "charts/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, artifactAddress(tc.art))
		})
	}
}

// TestGetArtifactsSortedByKey: the parse order must match the resolver's
// `{kind=…}` fan-out order, or a README's artifact list and its rendered commands
// disagree about which image is which.
func TestGetArtifactsSortedByKey(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		artifactsNS: map[string]any{
			"zeta":  map[string]any{keyKind: pfmodel.ArtifactKindImage, keyRef: "z"},
			"alpha": map[string]any{keyKind: pfmodel.ArtifactKindImage, keyRef: "a"},
		},
	}
	got, err := pfmodel.GetArtifacts(pf)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "alpha", got[0].Key)
	assert.Equal(t, "zeta", got[1].Key)
}

// TestArtifactPortsRejectNonNumeric: port 0 is not a port, so a malformed entry
// is dropped rather than rendered as "listens on 0".
func TestArtifactPortsRejectNonNumeric(t *testing.T) {
	pf := minimalDoc(t)
	pf.Extensions = map[string]any{
		artifactsNS: map[string]any{
			"db": map[string]any{
				keyKind: pfmodel.ArtifactKindService,
				"ports": []any{"not-a-port", map[string]any{keyName: "nameless"}, 5432},
			},
		},
	}
	got, err := pfmodel.GetArtifacts(pf)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got[0].Ports, 1)
	assert.Equal(t, 5432, got[0].Ports[0].Port)
}

// A document with no artifacts namespace is a valid state, not an error.
func TestGetArtifactsAbsent(t *testing.T) {
	got, err := pfmodel.GetArtifacts(minimalDoc(t))
	require.NoError(t, err)
	assert.Nil(t, got)
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Repeated fixture keys, named so goconst sees one home per string.
const (
	keyImage    = "image"
	keyCommands = "commands"
	keyName     = "name"
	keyPrefix   = "prefix"
	keyKind     = "kind"
	keyAxes     = "axes"
	keyRef      = "ref"
	keyMatrix   = "matrix"
	// pathCLI stands in for a build output in the address-chain cases.
	pathCLI     = "dist/pf-cli"
	artifactsNS = "org.projectfile.artifacts"
	// refImage is the address every recipe below installs from — the ONE thing
	// a project has to declare for the docker-pull group to render.
	refImage = "docker pull ${org.projectfile.artifacts{kind=image}.ref}"
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
				"B19_UBUNTU_SERIES": []any{"resolute", "noble"},
			}},
		},
		artifactsNS: imageArtifact(
			"kiota.ch/b19/ubuntu/${org.projectfile.ci.matrix.axes.B19_UBUNTU_SERIES[]}:latest"),
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
				"B19_UBUNTU_SERIES": []any{"resolute", "noble"},
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
			keyMatrix: map[string]any{keyAxes: map[string]any{"GOARCH": []any{"amd64"}}},
		},
		readmeNS: map[string]any{
			blockUsage: []any{group("inspect", "Inspect it:", `docker inspect --format '{{.Id}}' x`)},
		},
	}

	out := renderDoc(t, t.TempDir(), pf)

	assert.Contains(t, out, `docker inspect --format '{{.Id}}' x`)
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

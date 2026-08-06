// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package interp_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/interp"
)

// doc is the fixture every case resolves against: a typed field, a list the
// selector grammar must reach, and an extension namespace in the NESTED form
// (the on-disk shape every projectfile in this fleet uses).
func doc() *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: "ubuntu"},
		License:  &projectfile.License{Spdx: "MIT"},
		Links: []projectfile.Link{
			{Type: projectfile.LinkSourceCode, URL: "https://codeberg.org/b19/ubuntu"},
			{Type: projectfile.LinkSourceCode, URL: "https://github.com/damian-buho/b19-ubuntu"},
		},
		Extensions: map[string]any{
			"org": map[string]any{
				"projectfile": map[string]any{
					"status": "maintained",
					"readme": map[string]any{"shields-base": "https://img.shields.io"},
					"ci":     map[string]any{"matrix": map[string]any{"axes": map[string]any{"S": []any{"a", "b"}}}},
				},
			},
		},
	}
}

func TestExpand(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"typed field", "${identity.name}", "ubuntu"},
		{"nested field", "${license.spdx}", "MIT"},
		{"extension namespace", "${org.projectfile.status}", "maintained"},
		{"list selector takes the first match", "${links[type=source-code].url}", "https://codeberg.org/b19/ubuntu"},
		{"list index", "${links[1].url}", "https://github.com/damian-buho/b19-ubuntu"},
		{"several references in one string", "${identity.name}-${license.spdx}", "ubuntu-MIT"},
		{
			"composed URL", "${org.projectfile.readme.shields-base}/badge/license-${license.spdx}-blue",
			"https://img.shields.io/badge/license-MIT-blue",
		},

		// Everything below must survive untouched — this is the clause that
		// lets make variables, shell variables and matrix placeholders share a
		// document with field references (spec §3.8).
		{"make variable", "${B19_DOCKER_REGISTRY}/b19/fd", "${B19_DOCKER_REGISTRY}/b19/fd"},
		{"shell variable", "docker run -v $PWD:/w", "docker run -v $PWD:/w"},
		{"matrix placeholder", "b19/ubuntu/{S}", "b19/ubuntu/{S}"},
		{"missing field", "${identity.nope}", "${identity.nope}"},
		{"non-scalar value", "${links}", "${links}"},
		{"unterminated reference", "${identity.name", "${identity.name"},
		{"escape", "$${identity.name}", "${identity.name}"},
		{"lone dollar", "100$ and $", "100$ and $"},
		{"no dollar at all", "plain text", "plain text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, interp.Expand(doc(), tc.in))
		})
	}
}

// A nil document interpolates nothing but must never panic — the readme bridge
// renders badge fixtures with no document at all.
func TestExpandNilDocument(t *testing.T) {
	assert.Equal(t, "${identity.name}", interp.Expand(nil, "${identity.name}"))
}

// A resolved value that is itself a reference expands too, so a fragment can
// point at a field a second fragment defines. Depth is bounded, not unlimited.
func TestExpandRecursesIntoResolvedValues(t *testing.T) {
	d := doc()
	d.Identity.Title = &projectfile.LocalizedString{Bare: "${license.spdx} image"}
	assert.Equal(t, "MIT image", interp.Expand(d, "${identity.title}"))
}

func TestUnresolved(t *testing.T) {
	assert.True(t, interp.Unresolved("https://x/${a.b}"))
	assert.False(t, interp.Unresolved("https://x/resolved"))
}

// Repeated fixture keys, named so goconst sees one home per string.
const (
	artifactsNS  = "org.projectfile.artifacts"
	kindImage    = "image"
	keyKindField = "kind"
	keyRefField  = "ref"
)

// fanOutDoc holds the artifacts shape the README recipes address: two images and
// one binary under org.projectfile.artifacts, keyed by name.
func fanOutDoc() *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: "demo"},
		Extensions: map[string]any{
			artifactsNS: map[string]any{
				"noble-image":    map[string]any{keyKindField: kindImage, keyRefField: "kiota.ch/b19/ubuntu/noble:latest"},
				"resolute-image": map[string]any{keyKindField: kindImage, keyRefField: "kiota.ch/b19/ubuntu/resolute:latest"},
				"cli":            map[string]any{"kind": "binary", "command": "pf-bridge"},
			},
		},
	}
}

// TestExpandFanOutMultipleValues is the series-image case: one declared command
// in a shared fragment, one rendered line per image the project publishes.
func TestExpandFanOutMultipleValues(t *testing.T) {
	got, _ := interp.ExpandFanOut(fanOutDoc(), "docker pull ${org.projectfile.artifacts{kind=image}.ref}")
	want := []string{
		"docker pull kiota.ch/b19/ubuntu/noble:latest",
		"docker pull kiota.ch/b19/ubuntu/resolute:latest",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
}

// TestExpandFanOutSingleValueMatchesExpand pins the invariant that keeps the
// common case cheap and identical: one match must render exactly what Expand
// renders, as a single line.
func TestExpandFanOutSingleValueMatchesExpand(t *testing.T) {
	doc := fanOutDoc()
	const s = "${org.projectfile.artifacts{kind=binary}.command} --help"
	got, _ := interp.ExpandFanOut(doc, s)
	if len(got) != 1 {
		t.Fatalf("lines = %#v, want exactly one", got)
	}
	if got[0] != interp.Expand(doc, s) {
		t.Fatalf("fan-out = %q, Expand = %q — a single match must agree", got[0], interp.Expand(doc, s))
	}
	if got[0] != "pf-bridge --help" {
		t.Fatalf("line = %q, want %q", got[0], "pf-bridge --help")
	}
}

// TestExpandFanOutUnresolvedStaysVerbatim: interp never invents a value. An
// address nothing answers survives as a literal `${…}` on one line, which is the
// signal the readme bridge tests to DROP the command rather than publish it.
func TestExpandFanOutUnresolvedStaysVerbatim(t *testing.T) {
	got, resolved := interp.ExpandFanOut(fanOutDoc(), "npm install ${org.projectfile.artifacts{kind=package}.name}")
	if len(got) != 1 || resolved {
		t.Fatalf("lines = %#v resolved = %v, want one unresolved line", got, resolved)
	}
	if !interp.Unresolved(got[0]) {
		t.Fatalf("line %q must still carry the reference verbatim", got[0])
	}
}

// TestExpandFanOutCrossProduct: two fan-out references in one string multiply
// out, which is what recursing on the substituted result buys instead of a
// dedicated case. Contrived — but a silent wrong answer here would be worse.
func TestExpandFanOutCrossProduct(t *testing.T) {
	doc := &projectfile.Document{
		Keywords: []string{"a", "b"},
		Extensions: map[string]any{
			artifactsNS: map[string]any{
				"one": map[string]any{keyKindField: kindImage, keyRefField: "x"},
				"two": map[string]any{keyKindField: kindImage, keyRefField: "y"},
			},
		},
	}
	got, _ := interp.ExpandFanOut(doc, "${keywords[]} ${org.projectfile.artifacts{kind=image}.ref}")
	want := []string{"a x", "a y", "b x", "b y"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
}

// TestExpandFanOutNoReferences: a plain command line is returned untouched as a
// single line, with no document walk at all.
func TestExpandFanOutNoReferences(t *testing.T) {
	got, _ := interp.ExpandFanOut(fanOutDoc(), "make dc-up")
	if !reflect.DeepEqual(got, []string{"make dc-up"}) {
		t.Fatalf("lines = %#v, want [make dc-up]", got)
	}
}

// TestExpandBraceBalancedReference is the regression for the reference reader
// ending at the FIRST `}`: with map forms in the address grammar, that brace
// belongs to the selector, and the truncated `artifacts{kind=image` resolves to
// nothing — silently leaving every artifact-driven command literal.
func TestExpandBraceBalancedReference(t *testing.T) {
	doc := fanOutDoc()
	got := interp.Expand(doc, "${org.projectfile.artifacts{kind=binary}.command} --version")
	assert.Equal(t, "pf-bridge --version", got)
	// Text after the reference must survive, and a following plain reference
	// must still resolve — proof `next` lands past the right brace.
	got = interp.Expand(doc, "run ${org.projectfile.artifacts{kind=binary}.command} in ${identity.name}")
	assert.Equal(t, "run pf-bridge in demo", got)
	// An unbalanced opener is not a reference: emitted verbatim, never a panic.
	assert.Equal(t, "${broken{", interp.Expand(doc, "${broken{"))
}

// TestExpandFanOutEscapeIsNotUnresolved is why resolution is REPORTED rather than
// scanned for: `$${literal}` expands to a literal `${literal}`, and a caller that
// dropped every line containing `${` would throw away a perfectly good command.
func TestExpandFanOutEscapeIsNotUnresolved(t *testing.T) {
	got, resolved := interp.ExpandFanOut(fanOutDoc(), "echo $${literal}")
	assert.Equal(t, []string{"echo ${literal}"}, got)
	assert.True(t, resolved, "an escaped $ is not an unresolved reference")
}

// TestExpandRefusesMultiValue: the single-value entry point leaves a fan-out
// reference verbatim instead of silently rendering the first of several values.
func TestExpandRefusesMultiValue(t *testing.T) {
	const s = "docker pull ${org.projectfile.artifacts{kind=image}.ref}"
	got, resolved := interp.ExpandChecked(fanOutDoc(), s)
	assert.Equal(t, s, got)
	assert.False(t, resolved)
}

// TestExpandFanOutNestedReference is the series-image chain end to end: the
// artifact's ref is itself written in terms of a matrix-axis LIST, so the
// fan-out has to be found one level down in the substituted value.
func TestExpandFanOutNestedReference(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			"org.projectfile.ci": map[string]any{
				"matrix": map[string]any{"axes": map[string]any{
					"B19_UBUNTU_SERIES": []any{"resolute", "noble"},
				}},
			},
			artifactsNS: map[string]any{
				"image": map[string]any{
					"kind":      "image",
					keyRefField: "kiota.ch/b19/ubuntu/${org.projectfile.ci.matrix.axes.B19_UBUNTU_SERIES[]}:latest",
				},
			},
		},
	}
	got, resolved := interp.ExpandFanOut(doc, "docker pull ${org.projectfile.artifacts{kind=image}.ref}")
	assert.True(t, resolved)
	assert.Equal(t, []string{
		"docker pull kiota.ch/b19/ubuntu/resolute:latest",
		"docker pull kiota.ch/b19/ubuntu/noble:latest",
	}, got)
}

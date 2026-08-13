// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "testing"

// Fixtures declared once so goconst sees a single literal per repeated path.
const (
	probeFeatures      = "FEATURES.md"
	probeDocsArch      = "docs/architecture.md"
	docsEsReadme       = "docs/es/README.md"
	docsEsContributing = "docs/es/CONTRIBUTING.md"
	docsEsSupport      = "docs/es/SUPPORT.md"
	docsUkReadme       = "docs/uk/README.md"
	rootFeaturesRel    = "../../FEATURES.md"
	rootReadmeRel      = "../../README.md"
	archFromEsRel      = "../architecture.md"
	ukSiblingFromEsRel = "../uk/README.md"
)

func TestRelLink(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		docPath string
		want    string
	}{{
		name: "root doc keeps root target", target: probeFeatures, docPath: FileReadme, want: probeFeatures,
	}, {
		name: "localized doc rebases root target", target: probeFeatures, docPath: docsEsReadme, want: rootFeaturesRel,
	}, {
		name: "co-located sibling collapses to basename", target: docsEsSupport, docPath: docsEsContributing, want: FileSupport,
	}, {
		name: "root doc keeps docs-prefixed target", target: probeDocsArch, docPath: FileReadme, want: probeDocsArch,
	}, {
		name: "localized doc rebases docs-prefixed target", target: probeDocsArch, docPath: docsEsReadme, want: archFromEsRel,
	}, {
		name: "localized doc rebases other-language sibling", target: docsUkReadme, docPath: docsEsReadme, want: ukSiblingFromEsRel,
	}, {
		name: "language bar: localized doc rebases root readme", target: FileReadme, docPath: docsEsReadme, want: rootReadmeRel,
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RelLink(tc.target, tc.docPath); got != tc.want {
				t.Errorf("RelLink(%q, %q) = %q, want %q", tc.target, tc.docPath, got, tc.want)
			}
		})
	}
}

func TestRelLinkSibling(t *testing.T) {
	cases := []struct {
		name                       string
		siblingBase, lang, docBase string
		want                       string
	}{{
		name: "root CONTRIBUTING -> root SUPPORT", siblingBase: FileSupport, lang: "", docBase: FileContributing, want: FileSupport,
	}, {
		name: "es CONTRIBUTING -> es SUPPORT (co-located)", siblingBase: FileSupport, lang: "es", docBase: FileContributing, want: FileSupport,
	}, {
		name: "es CODE_OF_CONDUCT -> es README (co-located)", siblingBase: FileReadme, lang: "es", docBase: FileCodeOfConduct, want: FileReadme,
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RelLinkSibling(tc.siblingBase, tc.lang, tc.docBase); got != tc.want {
				t.Errorf("RelLinkSibling(%q, %q, %q) = %q, want %q",
					tc.siblingBase, tc.lang, tc.docBase, got, tc.want)
			}
		})
	}
}

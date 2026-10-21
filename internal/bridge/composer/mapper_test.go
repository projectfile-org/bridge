// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package composer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

const (
	testSourceURL = "https://example.test/source.git"
	testIssuesURL = "https://example.test/issues"
	testDocsURL   = "https://example.test/docs"
	testChatURL   = "https://example.test/chat"
	testForumURL  = "https://example.test/forum"
)

func mapperByExtKey(t *testing.T, mappers core.MapperList, extKey string) core.FieldMapper {
	t.Helper()
	for _, m := range mappers {
		if m.ExtKey == extKey {
			return m
		}
	}
	t.Fatalf("no mapper for %s", extKey)
	return core.FieldMapper{}
}

// ── name conversion ──────────────────────────────────────────────────────────

func TestComposerNameToPFName(t *testing.T) {
	cases := []struct{ in, namespace, name string }{
		{in: testComposerName, namespace: testComposerNS, name: testVendorWidget},
		{in: testVendorWidget, namespace: "", name: testVendorWidget},
		{in: "acme/deep/widget", namespace: testComposerNS, name: "deep/widget"},
	}
	for _, tc := range cases {
		namespace, name := composerNameToPFName(tc.in)
		assert.Equal(t, tc.namespace, namespace, tc.in)
		assert.Equal(t, tc.name, name, tc.in)
	}
}

func TestPFNameToComposer(t *testing.T) {
	assert.Equal(t, testComposerName, pfNameToComposer(testComposerNS, testVendorWidget))
	assert.Equal(t, "services/widget", pfNameToComposer("com.acme.services", testVendorWidget), "a foreign namespace keeps only its last segment")
	assert.Empty(t, pfNameToComposer("", testVendorWidget))
	assert.Empty(t, pfNameToComposer(testComposerNS, ""))
	assert.Empty(t, namespaceLastSegment(""))
	assert.Equal(t, testVendorAcme, namespaceLastSegment("com.acme"))
}

// ── per-field mapper closures ────────────────────────────────────────────────

func TestMapName(t *testing.T) {
	t.Run("to_pf_fills_both_slots", func(t *testing.T) {
		doc, pf := &Document{Name: testComposerName}, &projectfile.Document{}
		assert.Equal(t, testComposerName, mapperByExtKey(t, buildMappers(doc, pf), "name").ToPF(false))
		assert.Equal(t, testComposerNS, pf.Identity.Namespace)
		assert.Equal(t, testVendorWidget, pf.Identity.Name)
	})

	t.Run("to_pf_holds_a_declared_slot_without_force", func(t *testing.T) {
		doc, pf := &Document{Name: testComposerName}, &projectfile.Document{Identity: projectfile.Identity{Namespace: "com.other", Name: testVendorWidget}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "name").ToPF(false))
		assert.Equal(t, "com.other", pf.Identity.Namespace)
		assert.Equal(t, testComposerName, mapperByExtKey(t, buildMappers(doc, pf), "name").ToPF(true))
		assert.Equal(t, testComposerNS, pf.Identity.Namespace)
	})

	t.Run("to_pf_reports_no_change_when_values_agree", func(t *testing.T) {
		doc, pf := &Document{Name: testComposerName}, &projectfile.Document{Identity: projectfile.Identity{Namespace: testComposerNS, Name: testVendorWidget}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "name").ToPF(true))
	})

	t.Run("to_pf_ignores_an_empty_name", func(t *testing.T) {
		doc, pf := &Document{}, &projectfile.Document{}
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "name").ToPF(true))
	})

	t.Run("from_pf_writes_a_vendor_prefixed_name", func(t *testing.T) {
		doc, pf := &Document{Name: "other/pkg"}, &projectfile.Document{Identity: projectfile.Identity{Namespace: testComposerNS, Name: testVendorWidget}}
		assert.Equal(t, testComposerName, mapperByExtKey(t, buildMappers(doc, pf), "name").FromPF(false))
		assert.Equal(t, testComposerName, doc.Name)
	})

	t.Run("from_pf_keeps_an_agreed_name", func(t *testing.T) {
		doc, pf := &Document{Name: testComposerName}, &projectfile.Document{Identity: projectfile.Identity{Namespace: testComposerNS, Name: testVendorWidget}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "name").FromPF(true))
	})

	t.Run("from_pf_says_nothing_without_an_identity", func(t *testing.T) {
		doc, pf := &Document{Name: testComposerName}, &projectfile.Document{} // no namespace, so no composer name composes
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "name").FromPF(true))
		assert.Equal(t, testComposerName, doc.Name)
	})
}

func TestMapVersion(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		empty := &Document{}
		assert.Empty(t, mapperByExtKey(t, buildMappers(empty, &projectfile.Document{}), "version").ToPF(true))

		agreed, pf := &Document{Version: testVersionOne}, &projectfile.Document{Identity: projectfile.Identity{Version: testVersionOne}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, pf), "version").ToPF(true))

		held, heldPF := &Document{Version: testVersionTwo}, &projectfile.Document{Identity: projectfile.Identity{Version: testVersionOne}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "version").ToPF(false))
		assert.Equal(t, testVersionOne, heldPF.Identity.Version)

		fresh, freshPF := &Document{Version: testVersionTwo}, &projectfile.Document{Identity: projectfile.Identity{Version: testVersionOne}}
		assert.Equal(t, testVersionTwo, mapperByExtKey(t, buildMappers(fresh, freshPF), "version").ToPF(true))
		assert.Equal(t, testVersionTwo, freshPF.Identity.Version)
	})

	t.Run("from_pf", func(t *testing.T) {
		blank, blankPF := &Document{Version: testVersionOne}, &projectfile.Document{}
		assert.Empty(t, mapperByExtKey(t, buildMappers(blank, blankPF), "version").FromPF(true))
		assert.Equal(t, testVersionOne, blank.Version)

		agreed, pf := &Document{Version: testVersionOne}, &projectfile.Document{Identity: projectfile.Identity{Version: testVersionOne}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, pf), "version").FromPF(true))

		pushed, pushPF := &Document{Version: testVersionOne}, &projectfile.Document{Identity: projectfile.Identity{Version: testVersionTwo}}
		assert.Equal(t, testVersionTwo, mapperByExtKey(t, buildMappers(pushed, pushPF), "version").FromPF(false))
		assert.Equal(t, testVersionTwo, pushed.Version)
	})
}

func TestMapDescription(t *testing.T) {
	assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "description").ToPF(true))

	agreed := &Document{Description: testDescription}
	pf := &projectfile.Document{Identity: projectfile.Identity{Summary: localizedEN(testDescription)}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, pf), "description").ToPF(true))

	held, heldPF := &Document{Description: testDescription}, &projectfile.Document{Identity: projectfile.Identity{Summary: localizedEN("Old")}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "description").ToPF(false))
	assert.Equal(t, "Old", projectfile.ExtractLocalizedString(heldPF.Identity.Summary))
	assert.NotEmpty(t, mapperByExtKey(t, buildMappers(held, heldPF), "description").ToPF(true))
	assert.Equal(t, testDescription, projectfile.ExtractLocalizedString(heldPF.Identity.Summary))

	t.Run("from_pf", func(t *testing.T) {
		blank, blankPF := &Document{Description: testDescription}, &projectfile.Document{}
		assert.Empty(t, mapperByExtKey(t, buildMappers(blank, blankPF), "description").FromPF(false))
		assert.Equal(t, testDescription, blank.Description, "a default sync never destroys")
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(blank, blankPF), "description").FromPF(true))
		assert.Empty(t, blank.Description)

		agreed := &Document{Description: testDescription}
		pf := &projectfile.Document{Identity: projectfile.Identity{Summary: localizedEN(testDescription)}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, pf), "description").FromPF(true))

		pushed, pushPF := &Document{Description: "Old"}, &projectfile.Document{Identity: projectfile.Identity{Summary: localizedEN(testDescription)}}
		assert.Equal(t, testDescription, mapperByExtKey(t, buildMappers(pushed, pushPF), "description").FromPF(false))
		assert.Equal(t, testDescription, pushed.Description)
	})
}

func TestMapTime(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "time").ToPF(true))

		held, heldPF := &Document{Time: testTimeFeb}, &projectfile.Document{Identity: projectfile.Identity{Released: testTimeJan}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "time").ToPF(false))
		assert.Equal(t, testTimeFeb, mapperByExtKey(t, buildMappers(held, heldPF), "time").ToPF(true))
		assert.Equal(t, testTimeFeb, heldPF.Identity.Released)
	})

	t.Run("from_pf", func(t *testing.T) {
		blank, blankPF := &Document{Time: testTimeJan}, &projectfile.Document{}
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(blank, blankPF), "time").FromPF(true))
		assert.Empty(t, blank.Time)

		held, heldPF := &Document{Time: testTimeJan}, &projectfile.Document{Identity: projectfile.Identity{Released: testTimeJan}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "time").FromPF(true), "an agreed release date is a no-op")

		pushed, pushPF := &Document{Time: testTimeJan}, &projectfile.Document{Identity: projectfile.Identity{Released: testTimeFeb}}
		assert.Equal(t, testTimeFeb, mapperByExtKey(t, buildMappers(pushed, pushPF), "time").FromPF(false))
		assert.Equal(t, testTimeFeb, pushed.Time)
	})
}

func TestMapLicense(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "license").ToPF(true))

		held, heldPF := &Document{License: []any{testSPDXMIT, testSPDXApache}}, &projectfile.Document{License: &projectfile.License{Spdx: testSPDXGPL}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "license").ToPF(false))
		assert.Equal(t, "MIT OR Apache-2.0", mapperByExtKey(t, buildMappers(held, heldPF), "license").ToPF(true))
		assert.Equal(t, "MIT OR Apache-2.0", heldPF.License.Spdx)

		fresh := &Document{License: []any{testSPDXMIT, testSPDXApache}}
		freshPF := &projectfile.Document{}
		assert.Equal(t, "MIT OR Apache-2.0", mapperByExtKey(t, buildMappers(fresh, freshPF), "license").ToPF(false))
		require.NotNil(t, freshPF.License, "the ensure step allocates the licence slot")

		agreed := &Document{License: testSPDXMIT}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, &projectfile.Document{License: &projectfile.License{Spdx: testSPDXMIT}}), "license").ToPF(true))
	})

	t.Run("from_pf", func(t *testing.T) {
		blank, blankPF := &Document{License: testSPDXMIT}, &projectfile.Document{}
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(blank, blankPF), "license").FromPF(true))
		assert.Nil(t, blank.License)

		agreed := &Document{License: testSPDXMIT}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, &projectfile.Document{License: &projectfile.License{Spdx: testSPDXMIT}}), "license").FromPF(true))

		widened := &Document{License: testSPDXMIT}
		widenedPF := &projectfile.Document{License: &projectfile.License{Spdx: "MIT OR Apache-2.0"}}
		assert.Equal(t, "MIT OR Apache-2.0", mapperByExtKey(t, buildMappers(widened, widenedPF), "license").FromPF(false))
		assert.Equal(t, []string{testSPDXMIT, testSPDXApache}, widened.License)

		narrowed := &Document{License: []any{testSPDXMIT, testSPDXApache}}
		assert.Equal(t, testSPDXMIT, mapperByExtKey(t, buildMappers(narrowed, &projectfile.Document{License: &projectfile.License{Spdx: testSPDXMIT}}), "license").FromPF(false))
		assert.Equal(t, testSPDXMIT, narrowed.License)
	})
}

// supportMappers drives the five URL mappers sharing mapSupportURL's shape.
func supportMappers(pkg *Document, pf *projectfile.Document) core.MapperList {
	return buildMappers(pkg, pf)
}

func TestMapSupportURLGapFillAndForce(t *testing.T) {
	cases := []string{
		"homepage", testKeySupportIssues, testKeySupportChat, testKeySupportForum, "support.docs",
	}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			empty := supportMappers(&Document{}, &projectfile.Document{})
			assert.Empty(t, mapperByExtKey(t, empty, key).ToPF(false), "no file value says nothing")
			assert.Empty(t, mapperByExtKey(t, empty, key).FromPF(true), "no pf value and no file value is a no-op")
		})
	}
}

func TestMapSupportURLRemovalUnderForce(t *testing.T) {
	doc := &Document{Homepage: "https://home.test", Support: &Support{
		Issues: testIssuesURL, Chat: testChatURL, Forum: testForumURL, Docs: testDocsURL,
	}}
	pf := &projectfile.Document{} // nothing declared on the projectfile side

	for _, tc := range []struct {
		key   string
		clear func()
	}{
		{"homepage", func() { doc.Homepage = "" }},
		{testKeySupportIssues, func() { doc.Support.Issues = "" }},
		{testKeySupportChat, func() { doc.Support.Chat = "" }},
		{testKeySupportForum, func() { doc.Support.Forum = "" }},
		{"support.docs", func() { doc.Support.Docs = "" }},
	} {
		m := mapperByExtKey(t, buildMappers(doc, pf), tc.key)
		assert.Equal(t, core.Removed, m.FromPF(true), tc.key+" clears under force")
		tc.clear()
		assert.Empty(t, m.FromPF(false), tc.key+" is a no-op once the file value is gone")
	}
}

func TestMapSupportSourceAgreement(t *testing.T) {
	nominal := &Document{Support: &Support{Source: testSourceURL}}
	assert.NotEmpty(t, mapperByExtKey(t, buildMappers(nominal, &projectfile.Document{}), "support.source").ToPF(false), "an empty repository slot fills")

	agreed := &Document{Support: &Support{Source: testSourceURL}}
	pf := &projectfile.Document{Repositories: []projectfile.Repository{{URL: testSourceURL, Role: pfmodel.RepositoryRoleOrigin}}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, pf), "support.source").ToPF(false), "an agreeing URL is a no-op")

	sourceless := &Document{Support: &Support{Issues: testIssuesURL}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(sourceless, &projectfile.Document{}), "support.source").ToPF(true), "an empty file value says nothing")

	absent := &Document{}
	assert.Empty(t, mapperByExtKey(t, buildMappers(absent, &projectfile.Document{}), "support.source").ToPF(true), "no support object at all says nothing")

	urlless := &Document{Support: &Support{Source: testSourceURL}}
	pfURLess := &projectfile.Document{Repositories: []projectfile.Repository{{Role: pfmodel.RepositoryRoleOrigin}}}
	m := mapperByExtKey(t, buildMappers(urlless, pfURLess), "support.source")
	assert.NotEmpty(t, m.ToPF(false), "an empty repository URL is the gap the mapper fills")
	assert.Equal(t, testSourceURL, pfmodel.PackagesRepository(pfURLess).URL)
	assert.Equal(t, "git", pfmodel.PackagesRepository(pfURLess).Type)
	assert.Empty(t, m.ToPF(true), "a filled URL then agrees")
}

func TestMapFundingGapFill(t *testing.T) {
	// An extension namespace present but carrying nothing is still the gap.
	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, map[string]any{})
	doc := &Document{Funding: []FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}}}
	assert.Equal(t, "1 entr(y/ies)", mapperByExtKey(t, buildMappers(doc, pf), "funding").ToPF(false))
	ext, err := pfmodel.GetFundingExtension(pf)
	require.NoError(t, err)
	assert.Equal(t, []string{testFundFixtureURL}, ext.Custom)
}

func TestMapHomepage(t *testing.T) {
	t.Run("to_pf_ignores_an_agreed_url", func(t *testing.T) {
		doc := &Document{Homepage: testIssuesURL}
		pf := &projectfile.Document{Links: []projectfile.Link{{Type: projectfile.LinkHomepage, URL: testIssuesURL}}}
		assert.Empty(t, mapperByExtKey(t, supportMappers(doc, pf), "homepage").ToPF(true))
	})

	t.Run("to_pf", func(t *testing.T) {
		held, heldPF := &Document{Homepage: testURLNew}, &projectfile.Document{Links: []projectfile.Link{{Type: projectfile.LinkHomepage, URL: testURLOld}}}
		assert.Empty(t, mapperByExtKey(t, supportMappers(held, heldPF), "homepage").ToPF(false))
		assert.Equal(t, testURLOld, pfmodel.LinkURL(heldPF, projectfile.LinkHomepage))
		assert.NotEmpty(t, mapperByExtKey(t, supportMappers(held, heldPF), "homepage").ToPF(true))
		assert.Equal(t, testURLNew, pfmodel.LinkURL(heldPF, projectfile.LinkHomepage))
	})

	t.Run("from_pf", func(t *testing.T) {
		pushed, pushPF := &Document{Homepage: testURLOld}, &projectfile.Document{Links: []projectfile.Link{{Type: projectfile.LinkHomepage, URL: testURLNew}}}
		assert.NotEmpty(t, mapperByExtKey(t, supportMappers(pushed, pushPF), "homepage").FromPF(false))
		assert.Equal(t, testURLNew, pushed.Homepage)

		blank, blankPF := &Document{Homepage: testURLOld}, &projectfile.Document{}
		assert.Equal(t, core.Removed, mapperByExtKey(t, supportMappers(blank, blankPF), "homepage").FromPF(true))
		assert.Empty(t, blank.Homepage)
	})
}

func TestMapSupportLinksFromAFileValue(t *testing.T) {
	doc := &Document{Support: &Support{Issues: testIssuesURL, Chat: testChatURL, Forum: testForumURL}}
	pf := &projectfile.Document{}
	mappers := buildMappers(doc, pf)

	for _, tc := range []struct{ key, url, linkType string }{
		{key: testKeySupportIssues, url: testIssuesURL, linkType: projectfile.LinkBugs},
		{key: testKeySupportChat, url: testChatURL, linkType: projectfile.LinkChat},
		{key: testKeySupportForum, url: testForumURL, linkType: projectfile.LinkForum},
	} {
		m := mapperByExtKey(t, mappers, tc.key)
		assert.NotEmpty(t, m.ToPF(false), tc.key)
		assert.Equal(t, tc.url, pfmodel.LinkURL(pf, tc.linkType), tc.key)
		assert.Empty(t, m.FromPF(false), tc.key+" round-trips to a no-op")
	}
	assert.Empty(t, pfmodel.LinkURL(pf, projectfile.LinkHomepage))
	assert.Empty(t, doc.Support.Source, "a mapper this file never declared stays empty")
}

func TestMapSupportDocsNeedsTheMainTag(t *testing.T) {
	untagged := &projectfile.Document{Links: []projectfile.Link{
		{Type: projectfile.LinkDocumentation, URL: "https://spec.example.test"},
	}}
	doc := &Document{Support: &Support{Docs: testDocsURL}}
	m := mapperByExtKey(t, buildMappers(doc, untagged), "support.docs")
	assert.NotEmpty(t, m.ToPF(false), "the mapper writes through the main-documentation selector")
	assert.Equal(t, testDocsURL, pfmodel.MainDocumentationURL(untagged))
	assert.Len(t, untagged.Links, 2, "the inherited spec link stays untouched")

	tagged := &projectfile.Document{}
	require.True(t, pfmodel.SetMainDocumentationURL(tagged, testDocsURL, true))
	agreed := &Document{Support: &Support{Docs: testDocsURL}}
	assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, tagged), "support.docs").FromPF(false))
}

func TestMapSupportSource(t *testing.T) {
	t.Run("to_pf_writes_the_origin_slot_and_defaults_the_vcs", func(t *testing.T) {
		doc, pf := &Document{Support: &Support{Source: testSourceURL}}, &projectfile.Document{}
		m := mapperByExtKey(t, buildMappers(doc, pf), "support.source")
		assert.NotEmpty(t, m.ToPF(false))
		repo := pfmodel.PackagesRepository(pf)
		require.NotNil(t, repo)
		assert.Equal(t, testSourceURL, repo.URL)
		assert.Equal(t, "git", repo.Type)
		assert.Empty(t, m.ToPF(false), "an agreed value is a no-op")
	})

	t.Run("to_pf_holds_a_declared_repository_without_force", func(t *testing.T) {
		doc, pf := &Document{Support: &Support{Source: testSourceURL}}, &projectfile.Document{
			Repositories: []projectfile.Repository{{URL: testURLOld, Role: pfmodel.RepositoryRoleOrigin}},
		}
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "support.source").ToPF(false))
		assert.Equal(t, testURLOld, pfmodel.PackagesRepository(pf).URL)

		agreed, agreedPF := &Document{Support: &Support{Source: testSourceURL}}, &projectfile.Document{
			Repositories: []projectfile.Repository{{URL: testSourceURL, Type: "svn", Role: pfmodel.RepositoryRoleOrigin}},
		}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, agreedPF), "support.source").ToPF(false), "an agreed URL is a no-op")
		assert.Equal(t, "svn", agreedPF.Repositories[0].Type, "an existing VCS type is never replaced")
	})

	t.Run("from_pf_fills_and_clears", func(t *testing.T) {
		sourceless, sourcelessPF := &Document{}, &projectfile.Document{}
		assert.Empty(t, mapperByExtKey(t, buildMappers(sourceless, sourcelessPF), "support.source").FromPF(true), "no pf repository and no support object is a no-op")

		blank, blankPF := &Document{Support: &Support{Source: testSourceURL}}, &projectfile.Document{}
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(blank, blankPF), "support.source").FromPF(true))
		assert.Empty(t, blank.Support.Source)

		filled, filledPF := &Document{}, &projectfile.Document{
			Repositories: []projectfile.Repository{{URL: testSourceURL, Role: pfmodel.RepositoryRoleOrigin}},
		}
		m := mapperByExtKey(t, buildMappers(filled, filledPF), "support.source")
		assert.NotEmpty(t, m.FromPF(false))
		require.NotNil(t, filled.Support)
		assert.Equal(t, testSourceURL, filled.Support.Source)
		assert.Empty(t, m.FromPF(false), "an agreed value is a no-op")
	})

	t.Run("from_pf_skips_a_contact_with_no_composer_shape", func(t *testing.T) {
		// An ORCID-only person converts to an empty composer entry, so no author
		// is written and the file keeps whatever it already carried.
		doc := &Document{Authors: []Person{{Name: testAda}}}
		pf := &projectfile.Document{People: []projectfile.Person{{Orcid: "0000-0002-1825-0097", Roles: []string{testRoleAuthor}}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "authors").FromPF(false), "nothing survives the conversion")
		assert.Equal(t, []Person{{Name: testAda}}, doc.Authors, "a default sync never destroys")
	})
}

func TestMapKeywords(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "keywords").ToPF(true))

		agreed, pf := &Document{Keywords: []string{"a"}}, &projectfile.Document{Keywords: []string{"a"}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, pf), "keywords").ToPF(true))

		held, heldPF := &Document{Keywords: []string{"a"}}, &projectfile.Document{Keywords: []string{"b"}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "keywords").ToPF(false))
		assert.Equal(t, "1 keyword(s)", mapperByExtKey(t, buildMappers(held, heldPF), "keywords").ToPF(true))
		assert.Equal(t, []string{"a"}, heldPF.Keywords)
	})

	t.Run("from_pf", func(t *testing.T) {
		blank, blankPF := &Document{Keywords: []string{"a"}}, &projectfile.Document{}
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(blank, blankPF), "keywords").FromPF(true))
		assert.Nil(t, blank.Keywords)

		agreed, pf := &Document{Keywords: []string{"a"}}, &projectfile.Document{Keywords: []string{"a"}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, pf), "keywords").FromPF(true))

		pushed, pushPF := &Document{Keywords: []string{"a"}}, &projectfile.Document{Keywords: []string{"a", "b"}}
		assert.Equal(t, "2 keyword(s)", mapperByExtKey(t, buildMappers(pushed, pushPF), "keywords").FromPF(false))
		assert.Equal(t, []string{"a", "b"}, pushed.Keywords)
	})
}

func TestMapPeople(t *testing.T) {
	t.Run("to_pf_merges_authors", func(t *testing.T) {
		doc := &Document{Authors: []Person{{Name: testAdaLovelace, Email: testAdaEmail}, {Name: testGraceHopper}}}
		pf := &projectfile.Document{}
		assert.Equal(t, "2 author(s) merged", mapperByExtKey(t, buildMappers(doc, pf), "authors").ToPF(false))
		assert.Equal(t, []string{testRoleAuthor}, pf.People[0].Roles)
		assert.Equal(t, testAda, pf.People[0].GivenNames)
		assert.Equal(t, testLovelace, pf.People[0].FamilyNames)
		assert.Equal(t, testGrace, pf.People[1].GivenNames, "a two-word name splits into given and family")
		assert.Equal(t, "Hopper", pf.People[1].FamilyNames)
	})

	t.Run("to_pf_skips_entryless_authors", func(t *testing.T) {
		doc := &Document{Authors: []Person{{}, {Name: testGraceHopper}}}
		pf := &projectfile.Document{}
		assert.Equal(t, "1 author(s) merged", mapperByExtKey(t, buildMappers(doc, pf), "authors").ToPF(false))
		assert.Len(t, pf.People, 1)

		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{Authors: []Person{{}}}, &projectfile.Document{}), "authors").ToPF(false))
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "authors").ToPF(false))
	})

	t.Run("to_pf_is_a_no_op_when_nothing_new_arrives", func(t *testing.T) {
		doc := &Document{Authors: []Person{{Name: testAdaLovelace, Email: testAdaEmail}}}
		pf := &projectfile.Document{People: []projectfile.Person{{
			GivenNames: testAda, FamilyNames: testLovelace, Email: testAdaEmail, URL: testAdaHomepage, Roles: []string{testRoleAuthor},
		}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "authors").ToPF(false), "an author the document already holds changes nothing")
	})

	t.Run("from_pf_drops_an_undisplayable_author", func(t *testing.T) {
		doc := &Document{Authors: []Person{{Name: "Kept"}}}
		pf := &projectfile.Document{People: []projectfile.Person{
			{Email: "kept@example.test", Roles: []string{testRoleAuthor}},
			{Roles: []string{testRoleAuthor}}, // no displayable name and no contact
		}}
		assert.Equal(t, "1 author(s)", mapperByExtKey(t, buildMappers(doc, pf), "authors").FromPF(false))
		assert.Equal(t, []Person{{Email: "kept@example.test"}}, doc.Authors)
	})

	t.Run("to_pf_records_a_conflict", func(t *testing.T) {
		doc := &Document{Authors: []Person{{Name: testAdaLovelace, Email: testAdaEmail, Homepage: testURLNew}}}
		pf := &projectfile.Document{People: []projectfile.Person{{
			Email: testAdaEmail, URL: testURLOld, Roles: []string{testRoleAuthor},
		}}}
		assert.NotEmpty(t, mapperByExtKey(t, buildMappers(doc, pf), "authors").ToPF(false))
		require.NotNil(t, pf.Rest, "the conflict is stashed for RunSync to surface")
		assert.Equal(t, testURLOld, pf.People[0].URL, "an existing value wins a conflict")
	})

	t.Run("from_pf_writes_declared_authors", func(t *testing.T) {
		cleared := &Document{Authors: []Person{{Name: testAda}}}
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(cleared, &projectfile.Document{}), "authors").FromPF(true))
		assert.Nil(t, cleared.Authors)

		kept := &Document{Authors: []Person{{Name: testAda}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(kept, &projectfile.Document{}), "authors").FromPF(false))
		assert.Equal(t, []Person{{Name: testAda}}, kept.Authors, "a default sync never destroys")

		pf := &projectfile.Document{People: []projectfile.Person{
			{GivenNames: testAda, FamilyNames: testLovelace, Email: testAdaEmail, Roles: []string{testRoleAuthor}},
			{DisplayName: testGraceHopper, Roles: []string{testRoleMaintainer}},
			{GivenNames: "Karl", NameParticle: "van der", NameSuffix: "Jr", FamilyNames: "Moore", Roles: []string{testRoleAuthor}},
			{}, // a person with no displayable name is dropped
		}}
		doc := &Document{}
		assert.Equal(t, "2 author(s)", mapperByExtKey(t, buildMappers(doc, pf), "authors").FromPF(false))
		assert.Equal(t, []Person{
			{Name: testAdaLovelace, Email: testAdaEmail},
			{Name: "Karl van der Moore Jr"},
		}, doc.Authors)

		agreed := &Document{Authors: []Person{
			{Name: testAdaLovelace, Email: testAdaEmail},
			{Name: "Karl van der Moore Jr"},
		}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, pf), "authors").FromPF(false), "an agreed list is a no-op")
	})
}

func TestMapFunding(t *testing.T) {
	t.Run("to_pf", func(t *testing.T) {
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{Funding: []FundingEntry{{}}}, &projectfile.Document{}), "funding").ToPF(false))

		doc, pf := &Document{Funding: []FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}}}, &projectfile.Document{}
		assert.Equal(t, "1 entr(y/ies)", mapperByExtKey(t, buildMappers(doc, pf), "funding").ToPF(false))
		ext, err := pfmodel.GetFundingExtension(pf)
		require.NoError(t, err)
		require.NotNil(t, ext)
		assert.Equal(t, []string{testFundFixtureURL}, ext.Custom)

		held := &Document{Funding: []FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}}}
		heldPF := &projectfile.Document{}
		projectfile.SetExtension(heldPF, pfmodel.FundingExtensionNS, map[string]any{"github": []any{testVendorAcme}})
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "funding").ToPF(false), "a declared funding trove is never overwritten")
		assert.Equal(t, "1 entr(y/ies)", mapperByExtKey(t, buildMappers(held, heldPF), "funding").ToPF(true))

		urlless := &Document{Funding: []FundingEntry{{Type: testFundingType}, {}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(urlless, &projectfile.Document{}), "funding").ToPF(false), "an entry with no URL is nothing to stash")
	})

	t.Run("from_pf", func(t *testing.T) {
		blank, blankPF := &Document{Funding: []FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}}}, &projectfile.Document{}
		assert.Equal(t, core.Removed, mapperByExtKey(t, buildMappers(blank, blankPF), "funding").FromPF(true))
		assert.Nil(t, blank.Funding)

		pf := &projectfile.Document{}
		projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, map[string]any{testFundingType: []any{testFundFixtureURL, "https://other.test/fund"}})
		doc := &Document{}
		assert.Equal(t, "2 entr(y/ies)", mapperByExtKey(t, buildMappers(doc, pf), "funding").FromPF(false))
		assert.Equal(t, []FundingEntry{
			{Type: testFundingType, URL: testFundFixtureURL}, {Type: testFundingType, URL: "https://other.test/fund"},
		}, doc.Funding)
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), "funding").FromPF(false), "an agreed list is a no-op")
	})
}

func TestMapRequirementsRuntime(t *testing.T) {
	t.Run("to_pf_fills_only_platform_keys", func(t *testing.T) {
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{Require: map[string]string{testAcmePackage: testAcmeConstraint}}, &projectfile.Document{}), "require (platform)").ToPF(false))
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "require (platform)").ToPF(false))

		doc, pf := &Document{Require: map[string]string{testPHPKey: testPHPFloor, testExtJSON: "*", testAcmePackage: testAcmeConstraint}}, &projectfile.Document{}
		assert.Equal(t, "2 requirement(s)", mapperByExtKey(t, buildMappers(doc, pf), "require (platform)").ToPF(false))
		assert.Equal(t, map[string]string{testPHPKey: testPHPFloor, testExtJSON: "*"}, pf.Requirements.Runtime)

		agreed, agreedPF := &Document{Require: map[string]string{testPHPKey: testPHPFloor}}, &projectfile.Document{Requirements: &projectfile.Requirements{Runtime: map[string]string{testPHPKey: testPHPFloor}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, agreedPF), "require (platform)").ToPF(false))

		partial, partialPF := &Document{Require: map[string]string{testPHPKey: testPHPFloor, testExtGD: "*"}}, &projectfile.Document{Requirements: &projectfile.Requirements{Runtime: map[string]string{testPHPKey: testPHPFloor, testNodeKey: "20"}}}
		assert.Equal(t, "2 requirement(s)", mapperByExtKey(t, buildMappers(partial, partialPF), "require (platform)").ToPF(false))
		assert.Equal(t, testPHPFloor, partialPF.Requirements.Runtime[testPHPKey], "an agreeing key is skipped")
		assert.Equal(t, "*", partialPF.Requirements.Runtime[testExtGD], "a free key still fills")
		assert.Equal(t, "20", partialPF.Requirements.Runtime[testNodeKey], "a non-platform key is left alone")

		held, heldPF := &Document{Require: map[string]string{testPHPKey: testPHPFloorNew}}, &projectfile.Document{Requirements: &projectfile.Requirements{Runtime: map[string]string{testPHPKey: testPHPFloor}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), "require (platform)").ToPF(false), "a declared floor is held without force")
		assert.Equal(t, "1 requirement(s)", mapperByExtKey(t, buildMappers(held, heldPF), "require (platform)").ToPF(true))
		assert.Equal(t, testPHPFloorNew, heldPF.Requirements.Runtime[testPHPKey])

		mixed, mixedPF := &Document{Require: map[string]string{testPHPKey: testPHPFloor, testExtGD: "*"}}, &projectfile.Document{Requirements: &projectfile.Requirements{Runtime: map[string]string{testPHPKey: ">=7.0", testNodeKey: "20"}}}
		assert.Equal(t, "2 requirement(s)", mapperByExtKey(t, buildMappers(mixed, mixedPF), "require (platform)").ToPF(false), "the count names every platform key, changed or held")
		assert.Equal(t, ">=7.0", mixedPF.Requirements.Runtime[testPHPKey], "a declared floor only moves under force")
		assert.Equal(t, "*", mixedPF.Requirements.Runtime[testExtGD], "the free slot still fills")
		assert.Equal(t, "20", mixedPF.Requirements.Runtime[testNodeKey], "a non-platform runtime key is left alone")
	})

	t.Run("from_pf_writes_platform_keys_only", func(t *testing.T) {
		assert.Empty(t, mapperByExtKey(t, buildMappers(&Document{}, &projectfile.Document{}), "require (platform)").FromPF(false))

		blank := &Document{Require: map[string]string{testPHPKey: testPHPFloor}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(blank, &projectfile.Document{Requirements: &projectfile.Requirements{Runtime: map[string]string{testNodeKey: "20"}}}), "require (platform)").FromPF(false), "no platform key says nothing")
		assert.Equal(t, testPHPFloor, blank.Require[testPHPKey])

		doc, pf := &Document{}, &projectfile.Document{Requirements: &projectfile.Requirements{Runtime: map[string]string{testPHPKey: testPHPFloor, testNodeKey: "20"}}}
		assert.Equal(t, "1 requirement(s)", mapperByExtKey(t, buildMappers(doc, pf), "require (platform)").FromPF(false))
		assert.Equal(t, map[string]string{testPHPKey: testPHPFloor}, doc.Require)

		agreed, agreedPF := &Document{Require: map[string]string{testPHPKey: testPHPFloor}}, &projectfile.Document{Requirements: &projectfile.Requirements{Runtime: map[string]string{testPHPKey: testPHPFloor}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, agreedPF), "require (platform)").FromPF(false))

		untouched := &Document{Require: map[string]string{testPHPKey: testPHPFloor}}
		untouchedPF := &projectfile.Document{Requirements: &projectfile.Requirements{Runtime: map[string]string{testPHPKey: testPHPFloor}}}
		assert.Empty(t, mapperByExtKey(t, buildMappers(untouched, untouchedPF), "require (platform)").FromPF(false), "an agreeing value is skipped")
		assert.Equal(t, map[string]string{testPHPKey: testPHPFloor}, untouched.Require)
	})
}

func TestMapExtFieldPassthrough(t *testing.T) {
	for _, key := range []string{testKeyType, testKeyMinStability, testKeyPreferStable, testKeyAbandoned} {
		t.Run(key, func(t *testing.T) {
			empty := &Document{}
			assert.Empty(t, mapperByExtKey(t, buildMappers(empty, &projectfile.Document{}), key).ToPF(false))

			agreed := &Document{Type: testTypeLibrary, MinimumStability: testMinStability, PreferStable: true, Abandoned: false}
			agreedPF := &projectfile.Document{}
			projectfile.SetExtension(agreedPF, extNamespace, map[string]any{testKeyType: testTypeLibrary, testKeyMinStability: testMinStability, testKeyPreferStable: true, testKeyAbandoned: false})
			assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, agreedPF), key).ToPF(false), "a stashed value that agrees is a no-op")
			assert.Empty(t, mapperByExtKey(t, buildMappers(agreed, agreedPF), key).FromPF(false), "and the reverse direction too")

			held, heldPF := &Document{Type: testTypeLibrary, MinimumStability: testMinStability, PreferStable: true, Abandoned: false}, &projectfile.Document{}
			projectfile.SetExtension(heldPF, extNamespace, map[string]any{testKeyType: "project", testKeyMinStability: "dev", testKeyPreferStable: false, testKeyAbandoned: true})
			assert.Empty(t, mapperByExtKey(t, buildMappers(held, heldPF), key).ToPF(false), "a stashed value is held without force")
			assert.NotEmpty(t, mapperByExtKey(t, buildMappers(held, heldPF), key).ToPF(true))

			pulled, pulledPF := &Document{Type: testTypeLibrary, MinimumStability: testMinStability, PreferStable: true, Abandoned: false}, &projectfile.Document{}
			projectfile.SetExtension(pulledPF, extNamespace, map[string]any{testKeyType: "project", testKeyMinStability: "dev", testKeyPreferStable: false, testKeyAbandoned: true})
			assert.NotEmpty(t, mapperByExtKey(t, buildMappers(pulled, pulledPF), key).FromPF(false))
		})
	}

	t.Run("empty_and_absent_values_stay_put", func(t *testing.T) {
		doc := &Document{Type: ""}
		pf := &projectfile.Document{}
		projectfile.SetExtension(pf, extNamespace, map[string]any{testKeyType: ""})
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), testKeyType).ToPF(true), "an empty file value is nothing to stash")
		assert.Empty(t, mapperByExtKey(t, buildMappers(doc, pf), testKeyType).FromPF(false), "an empty stash is nothing to apply")

		typed := &Document{}
		typedPF := &projectfile.Document{}
		projectfile.SetExtension(typedPF, extNamespace, map[string]any{testKeyType: testTypeLibrary})
		assert.Equal(t, "s:library", mapperByExtKey(t, buildMappers(typed, typedPF), testKeyType).FromPF(false), "the display value is the formatted stash")
		assert.Equal(t, testTypeLibrary, typed.Type)
	})
}

// ── pure helpers ─────────────────────────────────────────────────────────────

func TestIsComposerPlatform(t *testing.T) {
	for _, key := range []string{testPHPKey, "hhvm", "php-64bit", "hhvm-3", testExtJSON, "lib-curl", "composer-plugin-api"} {
		assert.True(t, isComposerPlatform(key), key)
	}
	for _, key := range []string{testAcmePackage, "phpunit/phpunit", "json", "composer", "extlib"} {
		assert.False(t, isComposerPlatform(key), key)
	}
}

func TestPlatformRequireFilters(t *testing.T) {
	assert.Nil(t, platformRequires(nil))
	assert.Nil(t, platformRequires(map[string]string{testAcmePackage: testAcmeConstraint}))
	assert.Equal(t, map[string]string{testPHPKey: testPHPFloor}, platformRequires(map[string]string{testPHPKey: testPHPFloor, testAcmePackage: testAcmeConstraint}))

	assert.Nil(t, filterPlatformKeys(nil))
	assert.Nil(t, filterPlatformKeys(map[string]string{testNodeKey: "20"}))
	assert.Equal(t, map[string]string{testPHPKey: testPHPFloor}, filterPlatformKeys(map[string]string{testPHPKey: testPHPFloor, testNodeKey: "20"}))
}

func TestExtensionNamespaceHelpers(t *testing.T) {
	pf := &projectfile.Document{}
	assert.Nil(t, extMap(pf))
	_, ok := extGet(pf, testKeyType)
	assert.False(t, ok)

	setExtAny(pf, testKeyType, testTypeLibrary)
	stored, ok := extGet(pf, testKeyType)
	require.True(t, ok)
	assert.Equal(t, testTypeLibrary, stored)
	assert.Equal(t, map[string]any{testKeyType: testTypeLibrary}, extMap(pf))
}

func TestGenericAnyHelpers(t *testing.T) {
	empties := []any{nil, "", []any{}, []string{}, map[string]any{}, map[string]string{}}
	for _, v := range empties {
		assert.True(t, isEmptyAny(v), v)
	}
	for _, v := range []any{false, 0, testTypeLibrary, []any{"a"}, map[string]any{"a": 1}} {
		assert.False(t, isEmptyAny(v), v)
	}

	assert.Equal(t, "s:library", formatAny(testTypeLibrary))
	assert.Equal(t, "b:true", formatAny(true))
	assert.Equal(t, "b:false", formatAny(false))
	assert.Equal(t, "<nil>", formatAny(nil))
	assert.Equal(t, "l[s:one,s:two]", formatAny([]any{testKeywordOne, testKeywordTwo}))
	assert.Equal(t, "m{a=s:one,b=s:two}", formatAny(map[string]any{"b": testKeywordTwo, "a": testKeywordOne}))
	assert.Equal(t, "l[s:one]", formatAny(normaliseAny([]string{testKeywordOne})), "a []string normalises to a list")
	assert.Equal(t, "m{a=s:one}", formatAny(normaliseAny(map[string]string{"a": testKeywordOne})), "a map[string]string normalises to a mapping")
	assert.Equal(t, "s:plain", formatAny(normaliseAny("plain")))
	assert.Equal(t, "3", formatAny(3), "an untyped scalar falls back to %v")

	assert.True(t, deepEqualAny([]string{"a"}, []any{"a"}))
	assert.True(t, deepEqualAny(map[string]string{"a": "1"}, map[string]any{"a": "1"}))
	assert.False(t, deepEqualAny("a", "b"))
}

func TestEqualityHelpers(t *testing.T) {
	assert.True(t, equalStringSlice(nil, nil))
	assert.True(t, equalStringSlice([]string{"a"}, []string{"a"}))
	assert.False(t, equalStringSlice([]string{"a"}, []string{"b"}))
	assert.False(t, equalStringSlice([]string{"a"}, []string{"a", "b"}))

	assert.True(t, equalStringMap(nil, nil))
	assert.True(t, equalStringMap(map[string]string{"a": "1"}, map[string]string{"a": "1"}))
	assert.False(t, equalStringMap(map[string]string{"a": "1"}, map[string]string{"a": "2"}))
	assert.False(t, equalStringMap(map[string]string{"a": "1"}, map[string]string{"b": "1"}))

	assert.True(t, equalAuthorList(nil, nil))
	assert.True(t, equalAuthorList([]Person{{Name: "A", Role: testRoleDeveloper}}, []Person{{Name: "A", Role: "Author"}}), "role is not synchronised")
	assert.False(t, equalAuthorList([]Person{{Name: "A"}}, []Person{{Name: "B"}}))
	assert.False(t, equalAuthorList([]Person{{Name: "A"}}, nil))

	assert.True(t, equalFundingComposer(nil, nil))
	assert.True(t, equalFundingComposer([]FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}}, []FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}}))
	assert.False(t, equalFundingComposer([]FundingEntry{{URL: testFundFixtureURL}}, []FundingEntry{{URL: "https://other.test"}}))
	assert.False(t, equalFundingComposer([]FundingEntry{}, []FundingEntry{{}}))
}

func TestFundingExtensionHelpers(t *testing.T) {
	assert.False(t, hasComposerExtFunding(nil))
	assert.Nil(t, composerExtFundingURLs(nil))

	bare := &pfmodel.FundingExtension{}
	assert.False(t, hasComposerExtFunding(bare))
	assert.Nil(t, composerExtFundingURLs(bare))

	ext := &pfmodel.FundingExtension{
		GitHub: []string{testVendorAcme}, Patreon: testVendorAcme, KoFi: testVendorAcme, Liberapay: testVendorAcme,
		Tidelift: testVendorAcme, CommunityBridge: testVendorAcme, IssueHunt: testVendorAcme, OpenCollective: testVendorAcme,
		LFXCrowdfunding: testVendorAcme, Polar: testVendorAcme, BuyMeACoffee: testVendorAcme, ThanksDev: testVendorAcme,
		Custom: []string{testFundFixtureURL}, Path: "acme.yml", EntityType: "person", EntityRole: testRoleMaintainer,
	}
	assert.True(t, hasComposerExtFunding(ext))
	assert.Equal(t, []string{testFundFixtureURL}, composerExtFundingURLs(ext))

	m := composerFundingExtToMap(ext)
	assert.Equal(t, []any{testVendorAcme}, m["github"])
	assert.Equal(t, []any{testFundFixtureURL}, m[testFundingType])
	for _, key := range []string{"patreon", "ko-fi", "liberapay", "tidelift", "community-bridge", "issuehunt", "open-collective", "lfx-crowdfunding", "polar", "buy-me-a-coffee", "thanks-dev"} {
		assert.Equal(t, testVendorAcme, m[key], key)
	}
	assert.Equal(t, "acme.yml", m["path"])
	assert.Equal(t, "person", m["entity-type"])
	assert.Equal(t, testRoleMaintainer, m["entity-role"])
	assert.NotContains(t, m, "unknown-key")
	assert.NotContains(t, m, "channels")
}

func TestEnsureHelpersAndPersonConversion(t *testing.T) {
	pf := &projectfile.Document{}
	ensureLicense(pf)
	require.NotNil(t, pf.License)
	ensureRequirements(pf)
	require.NotNil(t, pf.Requirements)

	seeded := &projectfile.Document{License: &projectfile.License{Spdx: testSPDXMIT}, Requirements: &projectfile.Requirements{}}
	ensureLicense(seeded)
	assert.Equal(t, testSPDXMIT, seeded.License.Spdx, "an existing licence slot is never replaced")
	ensureRequirements(seeded)
	require.NotNil(t, seeded.Requirements)

	doc := &Document{}
	ensureSupport(doc)
	require.NotNil(t, doc.Support)
	ensureSupport(doc)
	require.NotNil(t, doc.Support, "a populated support object is left alone")

	assert.Equal(t, projectfile.Person{GivenNames: testAda, FamilyNames: testLovelace, Email: testAdaEmail, URL: testAdaHomepage, Roles: []string{testRoleAuthor}},
		composerPersonToPF(Person{Name: testAdaLovelace, Email: testAdaEmail, Homepage: testAdaHomepage}))
	assert.Equal(t, projectfile.Person{DisplayName: testGrace, Roles: []string{testRoleAuthor}},
		composerPersonToPF(Person{Name: testGrace}))
	assert.Equal(t, projectfile.Person{Roles: []string{testRoleAuthor}},
		composerPersonToPF(Person{Name: "  "}))

	assert.Equal(t, Person{Name: testAdaLovelace, Email: testAdaEmail, Homepage: testAdaHomepage},
		pfPersonToComposer(projectfile.Person{GivenNames: testAda, FamilyNames: testLovelace, Email: testAdaEmail, URL: testAdaHomepage}))

	assert.Equal(t, testGraceHopper, personDisplayName(projectfile.Person{DisplayName: testGraceHopper, GivenNames: testGrace, FamilyNames: "Hopper"}))
	assert.Equal(t, "Karl van der Moore Jr", personDisplayName(projectfile.Person{GivenNames: "Karl", NameParticle: "van der", NameSuffix: "Jr", FamilyNames: "Moore"}))
	assert.Empty(t, personDisplayName(projectfile.Person{GivenNames: testGrace}))

	people := []projectfile.Person{
		{Roles: []string{testRoleAuthor, testRoleMaintainer}, DisplayName: "A"},
		{Roles: []string{testRoleMaintainer}, DisplayName: "B"},
		{Roles: nil, DisplayName: "C"},
	}
	assert.Len(t, filterByRole(people, testRoleAuthor), 1)
	assert.Len(t, filterByRole(people, testRoleMaintainer), 2)
	assert.Empty(t, filterByRole(nil, testRoleAuthor))
}

// ── RunSync integration ──────────────────────────────────────────────────────

func writePFFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestRunSyncCreateWritesTheFile(t *testing.T) {
	dir := t.TempDir()
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: widget\n")
	pf := &projectfile.Document{Identity: projectfile.Identity{Namespace: testComposerNS, Name: testVendorWidget, Version: testVersionOne}}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath})
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.True(t, res.ExtChanged)
	assert.NotEmpty(t, res.ExtFields)

	got, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, testComposerName, got.Name)
	assert.Equal(t, testVersionOne, got.Version)
}

func TestRunSyncNoCreateFailsOnAMissingFile(t *testing.T) {
	dir := t.TempDir()
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: widget\n")

	_, err := core.RunSync(Bridge{}, &projectfile.Document{}, core.Options{Dir: dir, PFPath: pfPath, NoCreate: true})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "composer.json")
}

func TestRunSyncConvergesWithoutDestroying(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: widget\n")
	pf := &projectfile.Document{Identity: projectfile.Identity{Namespace: testComposerNS, Name: testVendorWidget, Version: "9.9.9"}}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath})
	require.NoError(t, err)
	assert.Equal(t, "version", res.ExtFields[0].Key)
	assert.Equal(t, "9.9.9", res.ExtFields[0].Value)

	got, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "9.9.9", got.Version, "the projectfile wins a modelled field")
	assert.Equal(t, testTypeLibrary, got.Type, "a modelled-but-absent pf field preserves the file value")
	assert.Equal(t, []string{testPHPKey, testVendorWidget}, got.Keywords)
	assert.True(t, got.Rest.Has("autoload"), "unknown keys survive the sync")
}

func TestRunSyncDryRunWritesNothing(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: widget\n")
	before := readFile(t, dir)
	pf := &projectfile.Document{Identity: projectfile.Identity{Namespace: testComposerNS, Name: testVendorWidget, Version: "9.9.9"}}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, DryRun: true})
	require.NoError(t, err)
	assert.True(t, res.DryRun)
	assert.Equal(t, before, readFile(t, dir), "a dry run leaves the file alone")
	assert.Equal(t, "9.9.9", pf.Identity.Version, "the caller's document is never mutated")
}

func TestRunSyncWriteIsAuthoritative(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: widget\n")
	pf := &projectfile.Document{Identity: projectfile.Identity{Namespace: testComposerNS, Name: testVendorWidget}}

	_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeWrite})
	require.NoError(t, err)

	got, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, testComposerName, got.Name)
	assert.Equal(t, "1.2.3", got.Version, "a package version is the file's own identity")
	assert.Empty(t, got.Description)
	assert.Empty(t, got.Keywords)
	assert.Empty(t, got.Time)
	assert.Nil(t, got.License)
	assert.Nil(t, got.Authors)
	assert.Nil(t, got.Funding)
	require.NotNil(t, got.Support, "the unmapped support keys stay on the canvas")
	assert.Empty(t, got.Support.Issues)
	assert.Equal(t, "help@example.test", got.Support.Email)
	assert.Equal(t, testTypeLibrary, got.Type, "an ext passthrough only ever gap-fills")
	assert.Equal(t, testPHPFloor, got.Require[testPHPKey], "a platform require only ever gap-fills")
	assert.Equal(t, testAcmeConstraint, got.Require[testAcmePackage], "a dependency set belongs to the file")
}

func TestRunSyncReadPullsTheFileIntoTheProjectfile(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: placeholder\n")
	pf := &projectfile.Document{}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead})
	require.NoError(t, err)
	assert.True(t, res.PFChanged)

	assert.Equal(t, testComposerNS, pf.Identity.Namespace)
	assert.Equal(t, testVendorWidget, pf.Identity.Name)
	assert.Equal(t, "1.2.3", pf.Identity.Version)
	assert.Equal(t, testDescription, projectfile.ExtractLocalizedString(pf.Identity.Summary))
	assert.Equal(t, "2026-01-02T03:04:05+00:00", pf.Identity.Released)
	assert.Equal(t, "MIT OR Apache-2.0", pf.License.Spdx)
	assert.Equal(t, testIssuesURL, pfmodel.LinkURL(pf, projectfile.LinkBugs))
	assert.Equal(t, testSourceURL, pfmodel.PackagesRepository(pf).URL)
	assert.Equal(t, map[string]string{testPHPKey: testPHPFloor, testExtJSON: "*"}, pf.Requirements.Runtime)
	assert.Equal(t, []string{testPHPKey, testVendorWidget}, pf.Keywords)
	require.Len(t, pf.People, 1)
	assert.Equal(t, testAda, pf.People[0].GivenNames)
	ext, err := pfmodel.GetFundingExtension(pf)
	require.NoError(t, err)
	assert.Equal(t, []string{testFundFixtureURL}, ext.Custom)

	extComposer, ok := extGet(pf, testKeyType)
	require.True(t, ok)
	assert.Equal(t, testTypeLibrary, extComposer)

	onDisk, err := projectfile.Read(pfPath)
	require.NoError(t, err)
	assert.Equal(t, testVendorWidget, onDisk.Identity.Name, "only the fields the sync touched land on disk")
}

func TestRunSyncSurfacesAPersonConflict(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)
	pfPath := writePFFile(t, dir, "---\nidentity:\n  name: widget\n")
	pf := &projectfile.Document{People: []projectfile.Person{{
		Email: testAdaEmail, URL: testURLOld, Roles: []string{testRoleAuthor},
	}}}

	var stderr bytes.Buffer
	_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead, Stderr: &stderr})
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "conflicting", "a person conflict reaches the caller")
	assert.Equal(t, testURLOld, pf.People[0].URL, "an existing value wins the conflict")
	assert.Nil(t, pf.Rest, "the ledger is consumed, not left behind")
}

// localizedEN builds the English-only LocalizedString the mappers read.
func localizedEN(v string) *projectfile.LocalizedString {
	ls := &projectfile.LocalizedString{}
	projectfile.SetLocalizedEN(&ls, v)
	return ls
}

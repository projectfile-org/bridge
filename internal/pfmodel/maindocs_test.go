// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// The shared-spec include every project carries: a documentation link that is
// reading material, not the project's own docs. The selection rule under test
// is that it never leaks into a single documentation slot.
const specSiteURL = "https://projectfile.org"

// docsExampleURL is the project's own documentation site in every case below.
const docsExampleURL = "https://docs.example.test"

func docWithLinks(links ...projectfile.Link) *projectfile.Document {
	return &projectfile.Document{Links: links}
}

func TestMainDocumentationURL(t *testing.T) {
	t.Run("picks_the_main_tagged_entry", func(t *testing.T) {
		doc := docWithLinks(
			projectfile.Link{Type: projectfile.LinkDocumentation, URL: specSiteURL},
			projectfile.Link{
				Type:  projectfile.LinkDocumentation,
				URL:   docsExampleURL,
				Extra: map[string]any{keyTags: []any{TagMainDocumentation}},
			},
		)
		assert.Equal(t, docsExampleURL, MainDocumentationURL(doc))
	})

	t.Run("untagged_documentation_links_are_invisible", func(t *testing.T) {
		doc := docWithLinks(
			projectfile.Link{Type: projectfile.LinkDocumentation, URL: specSiteURL},
		)
		assert.Empty(t, MainDocumentationURL(doc),
			"an inherited spec site must not become the project's Documentation URL")
	})

	t.Run("tag_on_a_foreign_type_is_ignored", func(t *testing.T) {
		doc := docWithLinks(projectfile.Link{
			Type:  projectfile.LinkHomepage,
			URL:   "https://example.test",
			Extra: map[string]any{keyTags: []any{TagMainDocumentation}},
		})
		assert.Empty(t, MainDocumentationURL(doc))
	})

	t.Run("nil_and_empty_documents", func(t *testing.T) {
		assert.Empty(t, MainDocumentationURL(nil))
		assert.Empty(t, MainDocumentationURL(&projectfile.Document{}))
	})
}

func TestSetMainDocumentationURL(t *testing.T) {
	t.Run("appends_a_tagged_entry_when_none_exists", func(t *testing.T) {
		doc := docWithLinks(projectfile.Link{Type: projectfile.LinkDocumentation, URL: specSiteURL})
		require.True(t, SetMainDocumentationURL(doc, docsExampleURL, true))
		require.Len(t, doc.Links, 2)
		assert.Equal(t, docsExampleURL, doc.Links[1].URL)
		assert.Equal(t, []string{TagMainDocumentation}, LinkTags(doc.Links[1]),
			"the entry must carry the tag or the reverse direction cannot find it")
		assert.Equal(t, docsExampleURL, MainDocumentationURL(doc))
	})

	t.Run("tags_a_same_url_entry_instead_of_duplicating", func(t *testing.T) {
		doc := docWithLinks(projectfile.Link{
			Type:  projectfile.LinkDocumentation,
			URL:   docsExampleURL,
			Extra: map[string]any{keyTags: []any{"readme"}},
		})
		require.True(t, SetMainDocumentationURL(doc, docsExampleURL, true))
		require.Len(t, doc.Links, 1)
		assert.Equal(t, []string{"readme", TagMainDocumentation}, LinkTags(doc.Links[0]),
			"a curated tag list is merged, never replaced")
	})

	t.Run("updates_the_tagged_entry_with_gap_fill_semantics", func(t *testing.T) {
		tagged := projectfile.Link{
			Type:  projectfile.LinkDocumentation,
			URL:   "https://old.example.test",
			Extra: map[string]any{keyTags: []any{TagMainDocumentation}},
		}
		t.Run("without_force_an_existing_url_wins", func(t *testing.T) {
			doc := docWithLinks(tagged)
			assert.False(t, SetMainDocumentationURL(doc, "https://new.example.test", false))
			assert.Equal(t, "https://old.example.test", MainDocumentationURL(doc))
		})
		t.Run("force_overwrites", func(t *testing.T) {
			doc := docWithLinks(tagged)
			assert.True(t, SetMainDocumentationURL(doc, "https://new.example.test", true))
			assert.Equal(t, "https://new.example.test", MainDocumentationURL(doc))
		})
		t.Run("gap_fills_an_empty_url", func(t *testing.T) {
			empty := tagged
			empty.URL = ""
			doc := docWithLinks(empty)
			assert.True(t, SetMainDocumentationURL(doc, "https://new.example.test", false))
			assert.Equal(t, "https://new.example.test", MainDocumentationURL(doc))
		})
	})

	t.Run("idempotent_when_already_recorded", func(t *testing.T) {
		doc := docWithLinks(projectfile.Link{
			Type:  projectfile.LinkDocumentation,
			URL:   docsExampleURL,
			Extra: map[string]any{keyTags: []any{TagMainDocumentation}},
		})
		assert.False(t, SetMainDocumentationURL(doc, docsExampleURL, true))
	})

	t.Run("nil_document_and_empty_url_are_no_ops", func(t *testing.T) {
		assert.False(t, SetMainDocumentationURL(nil, specSiteURL, true))
		assert.False(t, SetMainDocumentationURL(&projectfile.Document{}, "", true))
	})
}

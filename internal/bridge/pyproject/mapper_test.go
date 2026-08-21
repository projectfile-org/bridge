// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pyproject

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// mapperByExtKey picks one FieldMapper out of a built list by its ExtKey.
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

// docsExampleURL is the project's own documentation site; the spec site in the
// cases below plays the inherited "piece of documentation".
const docsExampleURL = "https://docs.example.test"

// TestURLDocumentationNeedsMainTag pins the fleet regression: the shared
// conventions include unions an untagged documentation link (the spec site)
// onto every project, and it must never leak into [project.urls].
func TestURLDocumentationNeedsMainTag(t *testing.T) {
	newPF := func() *projectfile.Document {
		return &projectfile.Document{Links: []projectfile.Link{{
			Type:  projectfile.LinkDocumentation,
			URL:   "https://projectfile.org",
			Extra: map[string]any{"tags": []any{"readme"}},
		}}}
	}

	t.Run("untagged_link_is_not_the_documentation", func(t *testing.T) {
		pf := newPF()
		py := &Document{}
		m := mapperByExtKey(t, buildMappers(py, pf), "urls.Documentation")
		assert.Empty(t, m.FromPF(true), "force must not promote reading material")
		assert.Nil(t, py.Project.URLs)
	})

	t.Run("main_tagged_link_flows_to_urls", func(t *testing.T) {
		pf := newPF()
		require.True(t, pfmodel.SetMainDocumentationURL(pf, docsExampleURL, true))
		py := &Document{}
		m := mapperByExtKey(t, buildMappers(py, pf), "urls.Documentation")
		assert.Equal(t, docsExampleURL, m.FromPF(false))
		assert.Equal(t, map[string]string{"Documentation": docsExampleURL}, py.Project.URLs)
	})

	t.Run("urls_documentation_lands_as_a_tagged_link", func(t *testing.T) {
		pf := newPF()
		py := &Document{Project: Project{URLs: map[string]string{
			"Documentation": docsExampleURL,
		}}}
		m := mapperByExtKey(t, buildMappers(py, pf), "urls.Documentation")
		assert.NotEmpty(t, m.ToPF(true))
		assert.Equal(t, docsExampleURL, pfmodel.MainDocumentationURL(pf),
			"the reverse direction must find what ToPF wrote")
		assert.Len(t, pf.Links, 2, "the inherited spec link stays untouched")
	})
}

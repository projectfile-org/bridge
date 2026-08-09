// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// YAML keys reused across the shield fixtures below. Declared as constants to
// satisfy goconst.
const (
	keyShieldName = "name"
	keyShieldImg  = "img"
	keyShieldHref = "href"
	keyShields    = "shields"
)

// GetReadmeExtension must surface shields with all four fields preserved.
func TestGetReadmeExtensionShields(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			pfmodel.ReadmeExtensionNS: map[string]any{
				"blocks": []any{"basics", "badges", "license"},
				keyShields: []any{
					map[string]any{
						keyShieldName: "dockerhub pulls",
						keyShieldImg:  "https://img.shields.io/docker/pulls/foo",
						keyShieldHref: "https://hub.docker.com/r/foo",
						"alt":         "Docker Hub pulls",
					},
					map[string]any{
						// alt omitted — render layer must fall back to name.
						keyShieldName: "go report",
						keyShieldImg:  "https://goreportcard.com/badge/foo",
						keyShieldHref: "https://goreportcard.com/report/foo",
					},
				},
			},
		},
	}

	ext, err := pfmodel.GetReadmeExtension(doc)
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, []string{"basics", "badges", "license"}, ext.Blocks)
	require.Len(t, ext.Shields, 2)

	assert.Equal(t, "dockerhub pulls", ext.Shields[0].Name)
	assert.Equal(t, "https://img.shields.io/docker/pulls/foo", ext.Shields[0].Img)
	assert.Equal(t, "https://hub.docker.com/r/foo", ext.Shields[0].Href)
	assert.Equal(t, "Docker Hub pulls", ext.Shields[0].Alt)

	assert.Equal(t, "go report", ext.Shields[1].Name)
	assert.Empty(t, ext.Shields[1].Alt, "alt must stay empty so the render layer can fall back")
}

// Non-map shield entries are skipped silently, matching the extras behavior.
func TestGetReadmeExtensionShieldsSkipsMalformed(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			pfmodel.ReadmeExtensionNS: map[string]any{
				keyShields: []any{
					"not-a-map",
					map[string]any{keyShieldName: "ok", keyShieldImg: "i", keyShieldHref: "h"},
				},
			},
		},
	}

	ext, err := pfmodel.GetReadmeExtension(doc)
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Shields, 1)
	assert.Equal(t, "ok", ext.Shields[0].Name)
}

// GetReadmeExtension must parse a shield's advisory `priority` so the render
// layer can order badges within a row. An unset priority stays the zero value;
// the render layer owns the unset→PriorityDefault promotion, keeping the model
// an honest mirror of the source.
func TestGetReadmeExtensionShieldsParsesPriority(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			pfmodel.ReadmeExtensionNS: map[string]any{
				keyShields: []any{
					map[string]any{
						keyShieldName: "support-ukraine",
						keyShieldImg:  "https://example.test/ukr.svg",
						keyPriority:   300,
					},
					map[string]any{
						keyShieldName: "changelog",
						keyShieldImg:  "https://example.test/log.svg",
						// priority omitted — stays zero.
					},
				},
			},
		},
	}

	ext, err := pfmodel.GetReadmeExtension(doc)
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Shields, 2)
	assert.Equal(t, 300, ext.Shields[0].Priority, "explicit priority parsed")
	assert.Zero(t, ext.Shields[1].Priority, "absent priority stays zero; render layer promotes it")
}

// GetReadmeExtension must preserve the lang→text map of an extras content
// entry so language-aware renderers can resolve per-lang, while Content still
// carries the default resolution for existing consumers.
func TestGetReadmeExtensionExtrasPreservesLangMap(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			pfmodel.ReadmeExtensionNS: map[string]any{
				"extras": []any{
					map[string]any{
						"name": "notice",
						"content": map[string]any{
							"en": "Hello",
							"es": "Hola",
							"uk": "Привіт",
						},
					},
				},
			},
		},
	}

	ext, err := pfmodel.GetReadmeExtension(doc)
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Extras, 1)

	assert.Equal(t, "notice", ext.Extras[0].Name)
	assert.Equal(t, "Hello", ext.Extras[0].Content,
		"Content carries the default (en) resolution for back-compat")
	require.NotNil(t, ext.Extras[0].ContentByLang,
		"ContentByLang preserves the raw map")
	assert.Equal(t, "Hola", ext.Extras[0].ContentByLang["es"])
	assert.Equal(t, "Привіт", ext.Extras[0].ContentByLang["uk"])
}

// A bare-string extras content entry must NOT populate ContentByLang — the
// bridge treats ContentByLang == nil as "single-language, no per-lang render".
func TestGetReadmeExtensionExtrasBareStringHasNilContentByLang(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			pfmodel.ReadmeExtensionNS: map[string]any{
				"extras": []any{
					map[string]any{
						"name":    "notice",
						"content": "Just a string",
					},
				},
			},
		},
	}

	ext, err := pfmodel.GetReadmeExtension(doc)
	require.NoError(t, err)
	require.Len(t, ext.Extras, 1)
	assert.Equal(t, "Just a string", ext.Extras[0].Content)
	assert.Nil(t, ext.Extras[0].ContentByLang,
		"bare string has no lang map")
}

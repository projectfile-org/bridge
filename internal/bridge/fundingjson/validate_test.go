// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fundingjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// Validation runs over the generic JSON shape, not the Go structs.
func TestNormalizeForValidationRoundTrip(t *testing.T) {
	got, err := normalizeForValidation(fundingSection{Channels: []fundingChannel{{GUID: valStripe, Type: valPaymentProvider}}})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		keyChannels: []any{map[string]any{keyGUID: valStripe, keyType: valPaymentProvider}},
		keyPlans:    nil,
	}, got)
}

// A value the encoder refuses is a stated error, never a dropped manifest.
func TestNormalizeForValidationRejectsUnencodable(t *testing.T) {
	_, err := normalizeForValidation(struct{ Ch chan int }{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "normalize for validation")
}

// The rendered manifest satisfies the embedded schema it ships with.
func TestValidateOutputAcceptsRenderedManifest(t *testing.T) {
	out, err := Bridge{}.Render(fullDoc(), core.Options{Offline: true})
	require.NoError(t, err)
	var manifest map[string]any
	require.NoError(t, json.Unmarshal(out.Files["funding.json"], &manifest))
	assert.NoError(t, validateOutput(manifest))
}

// An entity type outside the enum fails with the pointer path.
func TestValidateOutputRejectsInvalidManifest(t *testing.T) {
	invalid := map[string]any{"entity": map[string]any{keyType: "collective"}, "funding": map[string]any{}}
	err := validateOutput(invalid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/entity/type")
}

// The schema compiles once per process and every render reuses it.
func TestLoadSchemaIsCached(t *testing.T) {
	first, err := loadSchema()
	require.NoError(t, err)
	require.NotNil(t, first)
	again, err := loadSchema()
	require.NoError(t, err)
	assert.Same(t, first, again, "the compiled schema must be reused")
}

// An invalid ECMAScript pattern is reported, never silently accepted.
func TestEcma262RegexpRejectsInvalidPattern(t *testing.T) {
	re, err := ecma262Regexp("^[a-z0-9-]+$")
	require.NoError(t, err)
	assert.True(t, re.MatchString("widget"))
	assert.False(t, re.MatchString("Widget"))
	assert.Equal(t, "^[a-z0-9-]+$", re.String())

	_, err = ecma262Regexp("[unclosed")
	require.Error(t, err)
}

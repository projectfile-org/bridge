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

// The namespace every map-parsing helper case below reads, kept as a constant so goconst sees one spelling.
const testNS = "org.projectfile.ai"

// TestStrValReadsScalarStrings pins the three outcomes a map read can have: the value, the absent key, and a value typed as something else.
func TestStrValReadsScalarStrings(t *testing.T) {
	assert.Equal(t, valueYes, strVal(map[string]any{"k": valueYes}, "k"))
	assert.Empty(t, strVal(map[string]any{"k": valueYes}, "missing"))
	assert.Empty(t, strVal(map[string]any{"k": 7}, "k"), "a non-string scalar is not a value")
}

// TestStrListValAcceptsBothEncoderShapes pins the coercion the three decoders need: YAML hands back []any, a typed path hands back []string.
func TestStrListValAcceptsBothEncoderShapes(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, strListVal(map[string]any{"k": []any{"a", "b"}}, "k"))
	assert.Equal(t, []string{"a", "b"}, strListVal(map[string]any{"k": []string{"a", "b"}}, "k"))
	assert.Nil(t, strListVal(map[string]any{}, "k"), "an absent key is no list")
}

// TestToStrSliceDropsNonStrings: one malformed item costs that item, not the whole list — a credit nobody can render is noise, not a fact worth failing over.
func TestToStrSliceDropsNonStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "c"}, toStrSlice([]any{"a", 3, "c"}))
	assert.Equal(t, []string{"a"}, toStrSlice([]string{"a"}))
	assert.Nil(t, toStrSlice("a"), "a scalar is not a list")
	assert.Empty(t, toStrSlice([]any{}))
}

// TestStrMapValSkipsNonStringEntries pins the open-vocabulary rule: one mistyped entry must not cost the reader the whole policy.
func TestStrMapValSkipsNonStringEntries(t *testing.T) {
	m := map[string]any{"activities": map[string]any{
		activityPRReview: valueAssisted, useDocs: 42,
	}}
	out := strMapVal(m, "activities")
	assert.Equal(t, map[string]string{activityPRReview: valueAssisted}, out)
	assert.Nil(t, strMapVal(m, "missing"), "an absent key is no map")
	assert.Nil(t, strMapVal(map[string]any{"x": notAMapScalar}, "x"), "a scalar is no map")
}

// TestIntValCoercesTheThreeParserShapes pins the numeric coercion, and the refusal of a value that is not a number.
func TestIntValCoercesTheThreeParserShapes(t *testing.T) {
	assert.Equal(t, 7, intVal(map[string]any{"k": 7}, "k"))
	assert.Equal(t, 7, intVal(map[string]any{"k": int64(7)}, "k"))
	assert.Equal(t, 7, intVal(map[string]any{"k": float64(7)}, "k"))
	assert.Zero(t, intVal(map[string]any{}, "k"))
	assert.Zero(t, intVal(map[string]any{"k": "7"}, "k"), "a numeric string is refused, not parsed")
}

// TestFloatValCoercesEveryNumericShape covers the compiler's own type switch.
func TestFloatValCoercesEveryNumericShape(t *testing.T) {
	assert.InDelta(t, 2.5, floatVal(map[string]any{"k": float64(2.5)}, "k"), 0.001)
	assert.InDelta(t, 2.5, floatVal(map[string]any{"k": float32(2.5)}, "k"), 0.001)
	assert.InDelta(t, 3, floatVal(map[string]any{"k": 3}, "k"), 0.001)
	assert.InDelta(t, 3, floatVal(map[string]any{"k": int64(3)}, "k"), 0.001)
	assert.Zero(t, floatVal(map[string]any{}, "k"))
	assert.Zero(t, floatVal(map[string]any{"k": "2.5"}, "k"), "a numeric string is refused")
}

// TestBoolValOnlyReadsRealBools: a truthy string is not a toggle, so a typo reads as off rather than silently arming a feature.
func TestBoolValOnlyReadsRealBools(t *testing.T) {
	assert.True(t, boolVal(map[string]any{"k": true}, "k"))
	assert.False(t, boolVal(map[string]any{"k": false}, "k"))
	assert.False(t, boolVal(map[string]any{}, "k"))
	assert.False(t, boolVal(map[string]any{"k": valueTrue}, "k"))
}

// TestBoolValDefaultTrueReadsANestedToggle pins the forge-fields default: a missing or mistyped leaf stays ON.
func TestBoolValDefaultTrueReadsANestedToggle(t *testing.T) {
	on := map[string]any{"f": map[string]any{keyTopics: true}}
	off := map[string]any{"f": map[string]any{keyTopics: false}}
	assert.True(t, boolValDefaultTrue(on, "f", keyTopics))
	assert.False(t, boolValDefaultTrue(off, "f", keyTopics))
	assert.True(t, boolValDefaultTrue(map[string]any{}, "f", keyTopics), "an absent table stays on")
	assert.True(t, boolValDefaultTrue(map[string]any{"f": notAMapping}, "f", keyTopics), "a non-map table stays on")
	assert.True(t, boolValDefaultTrue(map[string]any{"f": map[string]any{}}, "f", keyTopics), "an absent key stays on")
	assert.True(t, boolValDefaultTrue(map[string]any{"f": map[string]any{keyTopics: valueYes}}, "f", keyTopics), "a mistyped leaf stays on")
}

// TestLocalizedStringValAcceptsBothEncodings pins the two forms spec §7 allows for a reserved localized-string field, and the nil that means "no statement".
func TestLocalizedStringValAcceptsBothEncodings(t *testing.T) {
	bare := localizedStringVal(map[string]any{"s": keyText}, "s")
	require.NotNil(t, bare)
	assert.Equal(t, keyText, bare.Bare)
	assert.Empty(t, bare.Langs)

	langs := localizedStringVal(map[string]any{"s": map[string]any{"en": "Hi", "es": msgHola, "uk": ""}}, "s")
	require.NotNil(t, langs)
	assert.Equal(t, map[string]string{"en": "Hi", "es": msgHola}, langs.Langs, "an empty translation is not a translation")
	assert.Empty(t, langs.Bare)

	assert.Nil(t, localizedStringVal(map[string]any{"s": ""}, "s"), "an empty scalar is no statement")
	assert.Nil(t, localizedStringVal(map[string]any{"s": map[string]any{}}, "s"), "an all-empty map is no statement")
	assert.Nil(t, localizedStringVal(map[string]any{"s": 7}, "s"), "a scalar number is no statement")
	assert.Nil(t, localizedStringVal(map[string]any{}, "s"), "an absent key is no statement")
}

// TestLookupNSReportsAMalformedNamespace pins the one error every accessor forwards: the namespace exists but is not a map.
func TestLookupNSReportsAMalformedNamespace(t *testing.T) {
	_, present, err := lookupNS(&projectfile.Document{}, testNS)
	require.NoError(t, err)
	assert.False(t, present)

	m, present, err := lookupNS(&projectfile.Document{Extensions: map[string]any{testNS: testNS}}, testNS)
	require.Error(t, err, "a namespace that is not a map is a parse error, not an absence")
	assert.True(t, present, "the namespace is present even though its shape is wrong")
	assert.Nil(t, m)

	m, present, err = lookupNS(&projectfile.Document{Extensions: map[string]any{testNS: map[string]any{}}}, testNS)
	require.NoError(t, err)
	assert.True(t, present)
	assert.NotNil(t, m)
}

// TestLookupNSResolvesDottedKeys pins the core façade the helpers sit on: a flat map key and a fully dotted header are the same namespace.
func TestLookupNSResolvesDottedKeys(t *testing.T) {
	doc := &projectfile.Document{Extensions: map[string]any{
		"org": map[string]any{authorityPF: map[string]any{"ai": map[string]any{keyAttitude: stanceCautious}}},
	}}
	m, present, err := lookupNS(doc, testNS)
	require.NoError(t, err)
	require.True(t, present)
	assert.Equal(t, stanceCautious, strVal(m, keyAttitude))
}

// TestHasExtensionReportsPresenceNotShape.
func TestHasExtensionReportsPresenceNotShape(t *testing.T) {
	doc := &projectfile.Document{Extensions: map[string]any{testNS: testNS}}
	assert.False(t, HasExtension(doc, IgnoresExtensionNS))
	assert.True(t, HasExtension(doc, testNS), "a malformed namespace is still present")
	assert.False(t, HasExtension(nil, testNS))
}

// TestRankOfMapsUnsetToTheDefault pins the reading shared by every priority-aware sort: zero is the author's "sort me last", not a missing key.
func TestRankOfMapsUnsetToTheDefault(t *testing.T) {
	assert.Equal(t, PriorityDefault, RankOf(0))
	assert.Equal(t, 1, RankOf(1), "a low number is a deliberate rank, never the default")
	assert.Equal(t, PriorityDefault, RankOf(PriorityDefault))
}

// TestByPriorityDescHigherFirst pins the one place the direction lives.
func TestByPriorityDescHigherFirst(t *testing.T) {
	assert.Negative(t, ByPriorityDesc(90, 10), "the higher priority sorts first")
	assert.Positive(t, ByPriorityDesc(10, 90))
	assert.Zero(t, ByPriorityDesc(PriorityDefault, PriorityDefault), "equal ranks tie")
}

// TestIgnoresGenerates pins the authority rule: an explicit list is complete, an absent one generates every target.
func TestIgnoresGenerates(t *testing.T) {
	assert.True(t, (*IgnoresExtension)(nil).Generates(targetGit), "no extension generates everything")
	assert.True(t, (&IgnoresExtension{}).Generates(targetGit), "no list generates everything")
	selected := &IgnoresExtension{Generate: []string{targetDocker, targetContainer}}
	assert.True(t, selected.Generates(targetDocker))
	assert.False(t, selected.Generates(targetGit), "a target the list omits is not generated")
}

// TestPatternsAndFilter pin the only way an inherited pattern is lost: a target excludes it verbatim. `!pattern` cannot express that.
func TestPatternsAndFilter(t *testing.T) {
	assert.Nil(t, (*IgnoreTargetOverride)(nil).Patterns(), "no override inherits the shared list verbatim")
	assert.Equal(t, []string{"a", "b"}, (&IgnoreTargetOverride{Include: []string{"a", "b"}}).Patterns())

	filtered := &IgnoreTargetOverride{Include: []string{"a", "b", "c"}, Exclude: []string{"b"}}
	assert.Equal(t, []string{"a", "c"}, filtered.Patterns(), "an excluded pattern never reaches the file")
	assert.Equal(t, []string{"a", "c"}, (&IgnoreTargetOverride{Exclude: []string{"b"}}).Filter([]string{"a", "b", "c"}))
	assert.Equal(t, []string{"a"}, (&IgnoreTargetOverride{Exclude: []string{" b "}}).Filter([]string{"a", "b"}), "both sides are trimmed")
	assert.Equal(t, []string{"a"}, (&IgnoreTargetOverride{}).Filter([]string{"a"}), "no exclude list keeps everything")
}

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

// extDoc builds a document carrying one extension namespace.
func extDoc(ns string, v any) *projectfile.Document {
	return &projectfile.Document{Extensions: map[string]any{ns: v}}
}

// wrap1 normalizes the two-value accessors onto one shape so the table below can name them all.
func wrap1[T any](f func(*projectfile.Document) (*T, error)) func(*projectfile.Document) (any, error) {
	return func(doc *projectfile.Document) (any, error) {
		v, err := f(doc)
		return v, err
	}
}

// wrapList is the slice-returning analogue for the accessor that hands back a list of artifacts.
func wrapList(f func(*projectfile.Document) ([]Artifact, error)) func(*projectfile.Document) (any, error) {
	return func(doc *projectfile.Document) (any, error) {
		v, err := f(doc)
		return v, err
	}
}

// Every accessor answers an absent namespace with (nil, nil) and a non-map one with an error; one table pins the pair for all of them.
func TestEveryAccessorAnswersAbsenceAndMalformedShape(t *testing.T) {
	accessors := map[string]func(*projectfile.Document) (any, error){
		AcknowledgementsExtensionNS: wrap1(GetAcknowledgementsExtension),
		CodeOfConductExtensionNS:    wrap1(GetCodeOfConductExtension),
		DEIExtensionNS:              wrap1(GetDEIExtension),
		AIExtensionNS:               wrap1(GetAIExtension),
		ContributingExtensionNS:     wrap1(GetContributingExtension),
		SupportExtensionNS:          wrap1(GetSupportExtension),
		SecurityExtensionNS:         wrap1(GetSecurityExtension),
		ReleaseExtensionNS:          wrap1(GetReleaseExtension),
		ConventionsExtensionNS:      wrap1(GetConventionsExtension),
		IgnoresExtensionNS:          wrap1(GetIgnoresExtension),
		EditorsExtensionNS:          wrap1(GetEditorsExtension),
		VulnerabilitiesExtensionNS:  wrap1(GetVulnerabilitiesExtension),
		FundingExtensionNS:          wrap1(GetFundingExtension),
		ForgeExtensionNS:            wrap1(GetForgeExtension),
		CodeOwnersExtensionNS:       wrap1(GetCodeOwnersExtension),
		FragmentsExtensionNS:        wrap1(GetFragmentsExtension),
		ReadmeExtensionNS:           wrap1(GetReadmeExtension),
		CitationExtensionNS:         wrap1(GetCitationExtension),
		I18NExtensionNS:             wrap1(GetI18NExtension),
		ArtifactsExtensionNS:        wrapList(GetArtifacts),
		AttributesExtensionNS:       wrap1(GetAttributesExtension),
	}
	for ns, get := range accessors {
		t.Run(ns+"_absent", func(t *testing.T) {
			out, err := get(&projectfile.Document{})
			require.NoError(t, err, "an absent namespace is not an error")
			assert.Nil(t, out)
		})
		t.Run(ns+"_malformed", func(t *testing.T) {
			out, err := get(extDoc(ns, []any{junkWord, "a", notAList}))
			require.Error(t, err, "a namespace that is not a map must not parse as empty")
			assert.Nil(t, out)
		})
	}
}

// TestAcknowledgementsParsesStringsAndMaps pins the two entry shapes and the drop of anything else: a credit nobody can render is noise.
func TestAcknowledgementsParsesStringsAndMaps(t *testing.T) {
	ext, err := GetAcknowledgementsExtension(extDoc(AcknowledgementsExtensionNS, map[string]any{
		"contributors": []any{nameAda, map[string]any{keyName: givenGrace, fxURL: urlXTest, "detail": "reviewer"}, 7, map[string]any{fxURL: "https://y.test"}, ""},
		"thanks":       []any{},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Contributors, 2, "a non-map, a nameless map and an empty string are all dropped")
	assert.Equal(t, Acknowledgement{Name: nameAda}, ext.Contributors[0])
	assert.Equal(t, Acknowledgement{Name: givenGrace, URL: urlXTest, Detail: "reviewer"}, ext.Contributors[1])
	assert.Empty(t, ext.Thanks)
	assert.Empty(t, ext.Sponsors)
	assert.Empty(t, ext.Credits)
}

// TestCodeOfConductReadsItsThreeFields.
func TestCodeOfConductReadsItsThreeFields(t *testing.T) {
	ext, err := GetCodeOfConductExtension(extDoc(CodeOfConductExtensionNS, map[string]any{
		"covenant": "2.1", "scope": "worldwide", keyHowToLink: true,
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, "2.1", ext.Covenant)
	assert.Equal(t, "worldwide", ext.Scope)
	assert.True(t, ext.HowToLink)
}

// TestDEIAbsentMeansNoStatementAndDisabledMeansGated pins the two states the bridge must keep apart: no namespace at all, versus one present but off.
func TestDEIAbsentMeansNoStatementAndDisabledMeansGated(t *testing.T) {
	absent, err := GetDEIExtension(&projectfile.Document{})
	require.NoError(t, err)
	assert.Nil(t, absent)

	off, err := GetDEIExtension(extDoc(DEIExtensionNS, map[string]any{"enabled": false, "scope": "internal"}))
	require.NoError(t, err)
	require.NotNil(t, off, "present-but-disabled is a statement, not an absence")
	assert.False(t, off.Enabled)
	assert.Equal(t, "internal", off.Scope)

	on, err := GetDEIExtension(extDoc(DEIExtensionNS, map[string]any{
		"enabled": true, "last-reviewed": dateReviewed, keyHowToLink: true,
		keyMetrics: map[string]any{"newcomer-experiences": []any{"mentoring"}, "weird": 3},
	}))
	require.NoError(t, err)
	require.NotNil(t, on)
	assert.True(t, on.Enabled)
	assert.Equal(t, dateReviewed, on.LastReviewed)
	assert.True(t, on.HowToLink)
	require.Len(t, on.Metrics, 2, "an unknown key is accepted — peaceful cohabitation")
	assert.Equal(t, []string{"mentoring"}, on.Metrics["newcomer-experiences"])
	assert.Nil(t, on.Metrics["weird"], "a metric whose value is not a list carries no bullets")
}

// TestAIDefaultsResolveWhenUnset pins the spec defaults for the two fields a consumer reads as policy: autonomy and who it binds.
func TestAIDefaultsResolveWhenUnset(t *testing.T) {
	ext, err := GetAIExtension(extDoc(AIExtensionNS, map[string]any{keyAttitude: stanceCautious}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, aiDefaultAutonomy, ext.Autonomy)
	assert.Equal(t, aiDefaultAppliesTo, ext.AppliesTo)
	assert.Equal(t, stanceCautious, ext.Attitude)

	stated, err := GetAIExtension(extDoc(AIExtensionNS, map[string]any{
		"autonomy": "supervised", "applies-to": "maintainers", keyFilename: "AI.md",
		"statement":         map[string]any{"en": "We disclose."},
		"disclose-required": true, "disclose-trailer": "Assisted-By: AI",
		"disclose-details": []any{"tool"}, "obligations": []any{"review"},
		"issue-required": true, "excluded-labels": []any{"deps"},
		"enforcement": []any{"label"}, "content-signals": []any{"no-ai"},
		"activities": map[string]any{activityPRReview: valueAssisted}, "project-use": map[string]any{useDocs: "full"},
		keyHowToLink: true,
	}))
	require.NoError(t, err)
	require.NotNil(t, stated)
	assert.Equal(t, "supervised", stated.Autonomy)
	assert.Equal(t, "maintainers", stated.AppliesTo)
	assert.Equal(t, "AI.md", stated.Filename)
	require.NotNil(t, stated.Statement)
	assert.Equal(t, "We disclose.", stated.Statement.Langs["en"], "a per-language statement resolves per language")
	assert.True(t, stated.DiscloseRequired)
	assert.Equal(t, "Assisted-By: AI", stated.DiscloseTrailer)
	assert.Equal(t, []string{"tool"}, stated.DiscloseDetails)
	assert.Equal(t, []string{"review"}, stated.Obligations)
	assert.True(t, stated.IssueRequired)
	assert.Equal(t, []string{"deps"}, stated.ExcludedLabels)
	assert.Equal(t, []string{"label"}, stated.Enforcement)
	assert.Equal(t, []string{"no-ai"}, stated.ContentSignals)
	assert.Equal(t, map[string]string{activityPRReview: valueAssisted}, stated.Activities)
	assert.Equal(t, map[string]string{useDocs: "full"}, stated.ProjectUse)
	assert.True(t, stated.HowToLink)
}

// TestAIFilenameRejectsAPath pins the write-outside-the-project guard: a separator or a `..` segment is refused, never sanitized into something writable.
func TestAIFilenameRejectsAPath(t *testing.T) {
	for _, bad := range []string{"docs/AI.md", "/etc/AI.md", `C:\AI.md`, ".", ".."} {
		_, err := GetAIExtension(extDoc(AIExtensionNS, map[string]any{keyFilename: bad}))
		require.Error(t, err, "filename %q must be refused", bad)
	}
	ext, err := GetAIExtension(extDoc(AIExtensionNS, map[string]any{keyFilename: "LLM.md"}))
	require.NoError(t, err)
	assert.Equal(t, "LLM.md", ext.Filename)
}

// TestAIPolicyFilenameResolvesTheDeclaredAndDefaultNames pins the declared name, the default name, and the empty that means no policy.
func TestAIPolicyFilenameResolvesTheDeclaredAndDefaultNames(t *testing.T) {
	assert.Equal(t, DefaultAIPolicyFile, AIPolicyFilename(extDoc(AIExtensionNS, map[string]any{keyAttitude: stanceCautious})))
	assert.Equal(t, "LLM.md", AIPolicyFilename(extDoc(AIExtensionNS, map[string]any{keyFilename: "LLM.md"})))
	assert.Empty(t, AIPolicyFilename(&projectfile.Document{}), "no declared policy is no file to link to")
	assert.Empty(t, AIPolicyFilename(extDoc(AIExtensionNS, map[string]any{keyFilename: "../escape"})),
		"a malformed filename is not restated as a cross-link")
}

// TestContributingParsesToggles pins the toggle reads beside the scalar fields.
func TestContributingParsesToggles(t *testing.T) {
	ext, err := GetContributingExtension(extDoc(ContributingExtensionNS, map[string]any{
		"sections": []any{"dev-setup"}, "cla-url": "https://cla.test", "chat-url": "https://chat.test",
		"coc-url": "https://coc.test", "recommend-to-follow": true, keyHowToLink: true,
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, []string{"dev-setup"}, ext.Sections)
	assert.Equal(t, "https://cla.test", ext.CLAURL)
	assert.Equal(t, "https://chat.test", ext.ChatURL)
	assert.Equal(t, "https://coc.test", ext.CoCURL)
	assert.Equal(t, projectfile.ParseToggle(true), ext.RecommendToFollow)
	assert.Equal(t, projectfile.Toggle{}, ext.RecommendToStar, "an unset toggle is off, not on")
	assert.True(t, ext.HowToLink)
}

// TestSupportParsesEOLRowsAndSkipsMalformed pins the EOL table: a non-map row costs that row only.
func TestSupportParsesEOLRowsAndSkipsMalformed(t *testing.T) {
	ext, err := GetSupportExtension(extDoc(SupportExtensionNS, map[string]any{
		"response-time": "48h", keyHowToLink: true,
		"eol": []any{map[string]any{"version": series1x, "date": dateReviewed}, junkRow},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, "48h", ext.ResponseTime)
	assert.True(t, ext.HowToLink)
	require.Len(t, ext.EOL, 1)
	assert.Equal(t, EOLEntry{Version: series1x, Date: dateReviewed}, ext.EOL[0])
}

// TestSecurityReadsEveryField pins the whole SECURITY surface at once.
func TestSecurityReadsEveryField(t *testing.T) {
	ext, err := GetSecurityExtension(extDoc(SecurityExtensionNS, map[string]any{
		"contact": "sec@test", "report-url": "https://sec.test/report", "supported-versions": []any{series1x},
		"disclosure-window": "90d", "gpg-key": "KEY", "gpg-fingerprint": "ABCD", "bug-bounty-url": "https://bounty.test",
		keyHowToLink: true,
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, "sec@test", ext.Contact)
	assert.Equal(t, "https://sec.test/report", ext.ReportURL)
	assert.Equal(t, []string{series1x}, ext.SupportedVersions)
	assert.Equal(t, "90d", ext.DisclosureWindow)
	assert.Equal(t, "KEY", ext.GPGKey)
	assert.Equal(t, "ABCD", ext.GPGFingerprint)
	assert.Equal(t, "https://bounty.test", ext.BugBountyURL)
	assert.True(t, ext.HowToLink)
}

// TestReleaseParsesBranchesAndSkipsMalformed pins the branch-to-channel map and the drop of a row that is not a map.
func TestReleaseParsesBranchesAndSkipsMalformed(t *testing.T) {
	ext, err := GetReleaseExtension(extDoc(ReleaseExtensionNS, map[string]any{
		"tag-format": "v${version}", "changelog": commitStyleConventional, workflowTrunk: "dev", "stable-branch": branchMain,
		"branches": []any{
			map[string]any{keyPattern: branchMain, "channel": "stable", keyPrerelease: false},
			map[string]any{keyPattern: "beta", "channel": keyPrerelease, keyPrerelease: true},
			junkRow,
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, "v${version}", ext.TagFormat)
	assert.Equal(t, commitStyleConventional, ext.Changelog)
	assert.Equal(t, "dev", ext.Trunk)
	assert.Equal(t, branchMain, ext.StableBranch)
	require.Len(t, ext.Branches, 2)
	assert.Equal(t, ReleaseBranch{Pattern: branchMain, Channel: "stable"}, ext.Branches[0])
	assert.Equal(t, ReleaseBranch{Pattern: "beta", Channel: keyPrerelease, Prerelease: true}, ext.Branches[1])

	ext, err = GetReleaseExtension(extDoc(ReleaseExtensionNS, map[string]any{"branches": branchMain}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Nil(t, ext.Branches, "a branch list that is not a list is no branch list")
}

// TestConventionsParsesScalarsAndPerLanguageTables pins the split: a scalar key reads directly, a map key is a per-stack-tag override.
func TestConventionsParsesScalarsAndPerLanguageTables(t *testing.T) {
	ext, err := GetConventionsExtension(extDoc(ConventionsExtensionNS, map[string]any{
		keyCommitStyle: commitStyleConventional, "workflow": workflowTrunk, "versioning": "semver",
		"style-guide-url": "https://style.test",
		keyCodeStyle:      map[string]any{"line-length": 100},
		"go":              map[string]any{"style-guide-url": urlGoGuide, keyExtra: "ignored"},
		"not-a-map":       "ignored too",
	}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, commitStyleConventional, ext.CommitStyle)
	assert.Equal(t, workflowTrunk, ext.Workflow)
	assert.Equal(t, "semver", ext.Versioning)
	assert.Equal(t, "https://style.test", ext.StyleGuideURL)
	assert.Equal(t, CodeStyle{LineLength: 100}, ext.CodeStyle)
	assert.Equal(t, map[string]LangConventions{"go": {StyleGuideURL: urlGoGuide}}, ext.Languages)
}

// TestConventionsCodeStyleAbsentLeavesLineLengthZero pins the reading every consumer uses for "not declared".
func TestConventionsCodeStyleAbsentLeavesLineLengthZero(t *testing.T) {
	ext, err := GetConventionsExtension(extDoc(ConventionsExtensionNS, map[string]any{"workflow": workflowTrunk}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Zero(t, ext.CodeStyle.LineLength)

	ext, err = GetConventionsExtension(extDoc(ConventionsExtensionNS, map[string]any{keyCodeStyle: notAMapping}))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Zero(t, ext.CodeStyle.LineLength, "a code-style that is not a map declares nothing")
}

// TestConventionsStyleGuideURLPrefersTopLevelThenStackTag pins the resolution order, including the nil document.
func TestConventionsStyleGuideURLPrefersTopLevelThenStackTag(t *testing.T) {
	conv := &ConventionsExtension{
		StyleGuideURL: "https://top.test",
		Languages:     map[string]LangConventions{"go": {StyleGuideURL: urlGoGuide}},
	}
	assert.Equal(t, "https://top.test", ConventionsStyleGuideURL(conv, []string{"go"}))
	assert.Equal(t, urlGoGuide, ConventionsStyleGuideURL(&ConventionsExtension{
		Languages: conv.Languages,
	}, []string{"python", "go"}))
	assert.Empty(t, ConventionsStyleGuideURL(&ConventionsExtension{}, []string{"go"}))
	assert.Empty(t, ConventionsStyleGuideURL(nil, []string{"go"}))
}

// TestPushGoalsListsSortedPushableGoals pins the filter: a goal gated to schedule or dispatch only never runs on a push, and every other goal does.
func TestPushGoalsListsSortedPushableGoals(t *testing.T) {
	doc := extDoc("org.projectfile.ci", map[string]any{
		keyNodes: map[string]any{
			"published":   map[string]any{keyGoal: true, keyWhen: map[string]any{keyEvents: []any{keyPush}}},
			"audited":     map[string]any{keyGoal: true, keyWhen: map[string]any{keyEvents: []any{keyPush, "schedule"}}},
			"dated":       map[string]any{keyGoal: true, keyWhen: map[string]any{keyEvents: []any{"schedule", "dispatch"}}},
			"on-dispatch": map[string]any{keyGoal: true, keyWhen: map[string]any{keyEvents: []any{"dispatch"}}},
			"no-when":     map[string]any{keyGoal: true},
			"not-a-goal":  map[string]any{keyGoal: false},
		},
	})
	assert.Equal(t, []string{"audited", "no-when", "published"}, PushGoals(doc))
}

// TestPushGoalsOnEmptyDocument is the degenerate case: no CI namespace at all.
func TestPushGoalsOnEmptyDocument(t *testing.T) {
	assert.Empty(t, PushGoals(&projectfile.Document{}))
	assert.Empty(t, PushGoals(extDoc("org.projectfile.ci", notAMapping)))
}

// TestGetConventionsFragmentsForwardsTheParseError keeps the conventions reader on the same contract as every other accessor.
func TestGetConventionsFragmentsForwardsTheParseError(t *testing.T) {
	_, err := GetConventionsFragments(extDoc(ConventionsExtensionNS, []any{junkWord, "a", notAList}))
	require.Error(t, err)
}

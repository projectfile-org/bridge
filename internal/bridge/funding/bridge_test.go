// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package funding

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// Fixture values the funding suite shares, each named once so goconst sees one literal.
const (
	fxName      = "widget"
	fxTitle     = "Widget"
	fxSpdx      = "MIT"
	fxGiven     = "Ada"
	fxFamily    = "Lovelace"
	fxEmail     = "ada@example.org"
	fxYear      = 2026
	fxGitHub    = "octocat"
	fxPatreon   = "octocat-patreon"
	fxKoFi      = "octocat-kofi"
	fxLiberapay = "octocat-liberapay"
	fxTidelift  = "octocat-tidelift"
	fxCrowd     = "octocat-lfx"
	fxCommunity = "octocat-bridge"
	fxIssueHunt = "octocat-issuehunt"
	fxPolar     = "octocat-polar"
	fxCoffee    = "octocat-coffee"
	fxThanks    = "octocat-thanks"
	fxOpenColl  = "octocat-collective"
	fxCustomA   = "https://buymeacoffee.com/octocat"
	fxCustomB   = "https://ko-fi.com/octocat"
)

// Extension and user-config keys repeated across fixtures and assertions.
const (
	keyGitHub   = "github"
	keyCustom   = "custom"
	keyPath     = "path"
	keyOpenColl = "open-collective"
	keyFunding  = "funding"
	cfgRel      = "assets/FUNDING.yml"
	assetsDir   = "assets"
)

// File names and paths the bridge resolves.
const (
	fileName     = "FUNDING.yml"
	tplName      = "FUNDING.yml.tmpl"
	defaultRel   = ".github/FUNDING.yml"
	overridePath = "docs/FUNDING.yml"
)

// User-config tree the fixture helper writes.
const (
	xdgApp  = "projectfile"
	cliFile = "cli.json"
	xdgEnv  = "XDG_CONFIG_HOME"
)

// pointUserConfigAt aims $XDG_CONFIG_HOME at a temp tree holding ext as cli.json and drops the load cache.
func pointUserConfigAt(t *testing.T, ext map[string]any) {
	t.Helper()
	dir := t.TempDir()
	if ext != nil {
		body, err := json.Marshal(ext)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, xdgApp), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, xdgApp, cliFile), body, 0o644))
	}
	t.Setenv(xdgEnv, dir)
	userconfig.SetIgnored(false)
	t.Cleanup(func() { userconfig.SetIgnored(true) })
}

// fundDoc is a document carrying every FUNDING.yml provider and a deterministic copyright line.
func fundDoc(t *testing.T) *projectfile.Document {
	t.Helper()
	pointUserConfigAt(t, nil)
	pf := &projectfile.Document{
		Identity:  projectfile.Identity{Name: fxName, Title: &projectfile.LocalizedString{Bare: fxTitle}},
		License:   &projectfile.License{Spdx: fxSpdx},
		People:    []projectfile.Person{{GivenNames: fxGiven, FamilyNames: fxFamily, Email: fxEmail, Roles: []string{projectfile.RoleCopyright}}},
		Copyright: &projectfile.Copyright{Year: fxYear},
	}
	return withFunding(pf, allProviders())
}

// allProviders is the full [org.projectfile.funding] namespace, keyed by the package's own slug constants.
func allProviders() map[string]any {
	return map[string]any{
		keyGitHub:              []any{fxGitHub},
		fundingPatreon:         fxPatreon,
		fundingKofi:            fxKoFi,
		fundingLiberapay:       fxLiberapay,
		fundingTidelift:        fxTidelift,
		fundingCommunityBridge: fxCommunity,
		fundingIssueHunt:       fxIssueHunt,
		keyOpenColl:            fxOpenColl,
		fundingLFXCrowd:        fxCrowd,
		fundingPolar:           fxPolar,
		fundingBuyMeACoffee:    fxCoffee,
		fundingThanksDev:       fxThanks,
		keyCustom:              []any{fxCustomA, fxCustomB},
	}
}

// allExt is the same namespace as a typed extension, for the pure-function cases.
func allExt() *pfmodel.FundingExtension {
	return &pfmodel.FundingExtension{
		GitHub:          []string{fxGitHub},
		Patreon:         fxPatreon,
		KoFi:            fxKoFi,
		Liberapay:       fxLiberapay,
		Tidelift:        fxTidelift,
		CommunityBridge: fxCommunity,
		IssueHunt:       fxIssueHunt,
		OpenCollective:  fxOpenColl,
		LFXCrowdfunding: fxCrowd,
		Polar:           fxPolar,
		BuyMeACoffee:    fxCoffee,
		ThanksDev:       fxThanks,
		Custom:          []string{fxCustomA, fxCustomB},
	}
}

// withFunding attaches [org.projectfile.funding] to pf.
func withFunding(pf *projectfile.Document, ext any) *projectfile.Document {
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, ext)
	return pf
}

func TestBridgeIdentity(t *testing.T) {
	b := Bridge{}
	assert.Equal(t, "funding", b.Name())
	assert.Equal(t, fileName, b.Filename())
	assert.Equal(t, []string{"FUNDING.yaml"}, b.Aliases())
	label, provider := b.Labels()
	assert.Equal(t, fileName, label)
	assert.Equal(t, "projectfile", provider)
	assert.True(t, b.Policy().Marker, "YAML carries a comment, so the managed marker can sit in it")
}

func TestBridgeExists(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, nil)
	assert.False(t, Bridge{}.Exists(dir), "an empty directory holds no FUNDING.yml")
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, defaultRel)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, defaultRel), []byte("github: []"), 0o644))
	assert.True(t, Bridge{}.Exists(dir), "the default path is the probed one")
	require.NoError(t, os.Remove(filepath.Join(dir, defaultRel)))
	assert.False(t, Bridge{}.Exists(dir), "a removed file reads absent again")
}

func TestBridgeExistsFollowsTheUserConfigPath(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, map[string]any{"generate": map[string]any{"defaults": map[string]any{fileName: map[string]any{keyPath: cfgRel}}}})
	assert.False(t, Bridge{}.Exists(dir), "the override names a file that is not written yet")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, assetsDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, cfgRel), []byte("github: []"), 0o644))
	assert.True(t, Bridge{}.Exists(dir), "the override path is the probed one")
}

func TestBridgeFullPathResolvesTierPrecedence(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, nil)
	assert.Equal(t, filepath.Join(dir, defaultRel), Bridge{}.FullPath(dir, nil), "no override keeps the built-in path")

	overridden := withFunding(&projectfile.Document{}, map[string]any{keyPath: overridePath})
	assert.Equal(t, filepath.Join(dir, overridePath), Bridge{}.FullPath(dir, overridden), "an in-pf path beats the built-in default")

	pointUserConfigAt(t, map[string]any{"generate": map[string]any{"defaults": map[string]any{fileName: map[string]any{keyPath: cfgRel}}}})
	assert.Equal(t, filepath.Join(dir, cfgRel), Bridge{}.FullPath(dir, nil), "the user config beats the built-in default")
	assert.Equal(t, filepath.Join(dir, cfgRel), Bridge{}.FullPath(dir, overridden), "the user config outranks an in-pf path")
}

// The rendered envelope is byte-pinned: YAML start, the hash-style REUSE header, the marker and both list blocks.
func TestRenderEnvelopeIsByteStable(t *testing.T) {
	pf := fundDoc(t)
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, map[string]any{
		keyGitHub: []any{fxGitHub},
		keyCustom: []any{fxCustomA, fxCustomB},
	})
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	require.Len(t, out.Files, 1)

	want := core.YAMLDocStart + core.REUSEHeader(pf, core.StyleHash) +
		core.Marker + "\n" +
		"# Edit projectfile.toml [org.projectfile.funding] to customise this file —\n" +
		"# direct edits will be overwritten on the next `pf-cli generate FUNDING.yml`.\n" +
		"github:\n  - " + fxGitHub + "\n" +
		"custom:\n  - " + fxCustomA + "\n  - " + fxCustomB + "\n"
	assert.Equal(t, want, string(out.Files[defaultRel]))
}

func TestRenderWritesEveryProviderBlock(t *testing.T) {
	out, err := Bridge{}.Render(fundDoc(t), core.Options{})
	require.NoError(t, err)
	body := string(out.Files[defaultRel])

	assert.True(t, strings.HasPrefix(body, core.YAMLDocStart), "the artefact opens with the YAML document start")
	assert.Contains(t, body, fmt.Sprintf("# SPDX-FileCopyrightText: %d %s %s <%s>", fxYear, fxGiven, fxFamily, fxEmail))
	assert.Contains(t, body, "# SPDX-License-Identifier: "+fxSpdx)
	assert.Contains(t, body, core.Marker, "every generated file carries the sentinel")
	assert.Contains(t, body, "github:\n  - "+fxGitHub)
	for _, block := range [][2]string{
		{"patreon: ", fxPatreon},
		{"ko_fi: ", fxKoFi},
		{"liberapay: ", fxLiberapay},
		{"tidelift: ", fxTidelift},
		{"community_bridge: ", fxCommunity},
		{"issuehunt: ", fxIssueHunt},
		{"open_collective: ", fxOpenColl},
		{"lfx_crowdfunding: ", fxCrowd},
		{"polar: ", fxPolar},
		{"buy_me_a_coffee: ", fxCoffee},
		{"thanks_dev: ", fxThanks},
	} {
		assert.Contains(t, body, block[0]+block[1], block[0])
	}
	assert.Contains(t, body, "custom:\n  - "+fxCustomA+"\n  - "+fxCustomB+"\n")
	assert.True(t, strings.HasSuffix(body, fxCustomB+"\n"), "the last provider block ends the file")
}

// A document with no namespace reads as an empty extension, so the user config is the only source that can fill it.
func TestRenderFallsBackToTheUserConfig(t *testing.T) {
	pointUserConfigAt(t, map[string]any{keyFunding: map[string]any{keyGitHub: []any{fxGitHub}}})
	out, err := Bridge{}.Render(&projectfile.Document{Identity: projectfile.Identity{Name: fxName}}, core.Options{})
	require.NoError(t, err, "the user config supplies what the document lacks")
	assert.Contains(t, string(out.Files[defaultRel]), "github:\n  - "+fxGitHub)
}

func TestRenderFailsWithoutAnyProvider(t *testing.T) {
	pointUserConfigAt(t, nil)
	_, err := Bridge{}.Render(&projectfile.Document{Identity: projectfile.Identity{Name: fxName}}, core.Options{})
	require.Error(t, err, "an empty FUNDING.yml would advertise that no funding is accepted")
	assert.Contains(t, err.Error(), "no funding providers configured")
	assert.Contains(t, err.Error(), "projectfile.org/spec/v1#funding")
}

func TestRenderFailsOnAMalformedNamespace(t *testing.T) {
	pointUserConfigAt(t, nil)
	_, err := Bridge{}.Render(withFunding(&projectfile.Document{}, "not-a-map"), core.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), pfmodel.FundingExtensionNS+" is not a map")
}

func TestRenderFailsOnABrokenProjectLocalTemplate(t *testing.T) {
	dir := t.TempDir()
	tplDir := filepath.Join(dir, core.LocalTemplatesDir)
	require.NoError(t, os.MkdirAll(tplDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tplDir, tplName), []byte("{{.NoSuchField"), 0o644))
	_, err := Bridge{}.Render(fundDoc(t), core.Options{Dir: dir})
	require.Error(t, err, "a project-local template that does not parse must fail the render")
}

func TestRenderIsByteStable(t *testing.T) {
	first, err := Bridge{}.Render(fundDoc(t), core.Options{})
	require.NoError(t, err)
	for range 4 {
		again, err := Bridge{}.Render(fundDoc(t), core.Options{})
		require.NoError(t, err)
		assert.Equal(t, first.Files[defaultRel], again.Files[defaultRel])
	}
}

func TestRenderKeysTheOutputByTheInPfPath(t *testing.T) {
	providers := allProviders()
	providers[keyPath] = overridePath
	pf := withFunding(fundDoc(t), providers)
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	require.Len(t, out.Files, 1)
	assert.Contains(t, out.Files, overridePath, "the in-pf path is the output key, not the default")
}

func TestRequiredFieldsPromptsForThreeProviders(t *testing.T) {
	pointUserConfigAt(t, nil)
	missing := Bridge{}.RequiredFields(&projectfile.Document{})
	require.Len(t, missing, 3)
	assert.Equal(t, "[org.projectfile.funding]."+keyGitHub+" (handle)", missing[0].Field)
	assert.Equal(t, "[org.projectfile.funding].open-collective", missing[1].Field)
	assert.Equal(t, "[org.projectfile.funding].other (slug=value; slug=value)", missing[2].Field)
	for _, m := range missing {
		assert.NotEmpty(t, m.Hint, m.Field)
		require.NotNil(t, m.Setter, m.Field)
	}
}

func TestRequiredFieldsIsEmptyWhenAProviderExists(t *testing.T) {
	pointUserConfigAt(t, nil)
	assert.Nil(t, Bridge{}.RequiredFields(withFunding(&projectfile.Document{}, map[string]any{fundingPatreon: fxPatreon})))
	assert.Nil(t, Bridge{}.RequiredFields(withFunding(&projectfile.Document{}, map[string]any{keyCustom: []any{fxCustomA}})))
}

func TestRequiredFieldsPromptsOnAMalformedNamespace(t *testing.T) {
	pointUserConfigAt(t, nil)
	require.Len(t, Bridge{}.RequiredFields(withFunding(&projectfile.Document{}, "not-a-map")), 3, "a broken namespace must not panic the prompt list")
}

func TestRequiredFieldsSettersWriteTheExtension(t *testing.T) {
	pointUserConfigAt(t, nil)
	pf := &projectfile.Document{}
	missing := Bridge{}.RequiredFields(pf)

	require.NoError(t, missing[0].Setter(fxGitHub))
	require.NoError(t, missing[1].Setter(fxOpenColl))
	require.NoError(t, missing[2].Setter(fundingPatreon+"="+fxPatreon))

	ext, err := pfmodel.GetFundingExtension(pf)
	require.NoError(t, err)
	assert.Equal(t, []string{fxGitHub}, ext.GitHub)
	assert.Equal(t, fxOpenColl, ext.OpenCollective)
	assert.Equal(t, fxPatreon, ext.Patreon)
	assert.Nil(t, Bridge{}.RequiredFields(pf), "the filled providers must close every prompt")
}

func TestRequiredFieldsSettersAcceptBlankValues(t *testing.T) {
	pointUserConfigAt(t, nil)
	pf := &projectfile.Document{}
	missing := Bridge{}.RequiredFields(pf)
	require.NoError(t, missing[0].Setter(""), "a blank handle means no GitHub sponsor")
	require.NoError(t, missing[1].Setter(""))
	assert.Empty(t, mustFundingExtension(t, pf).GitHub)
	assert.Len(t, Bridge{}.RequiredFields(pf), 3, "no provider filled leaves the prompts open")
}

func TestRequiredFieldsSettersRejectBadInput(t *testing.T) {
	pointUserConfigAt(t, nil)
	other := Bridge{}.RequiredFields(&projectfile.Document{})[2].Setter
	cases := []struct{ name, input, want string }{
		{"bare slug", fundingPatreon, "expected slug=value"},
		{"unknown slug", "nosuch=" + fxPatreon, "unknown funding slug"},
		{"empty value", fundingPatreon + "=", "empty value for slug"},
		{"empty field", "  ", "no funding provider provided"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := other(tc.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestParseOthers(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  *pfmodel.FundingExtension
		bad   string
	}{
		{name: "blank input changes nothing", input: "   ", want: &pfmodel.FundingExtension{}},
		{name: "empty entries are skipped", input: ";;" + fundingPatreon + "=" + fxPatreon + ";;", want: &pfmodel.FundingExtension{Patreon: fxPatreon}},
		{
			name: "every secondary slug assigns its field",
			input: strings.Join([]string{
				fundingPatreon + "=" + fxPatreon,
				fundingKofi + "=" + fxKoFi,
				fundingLiberapay + "=" + fxLiberapay,
				fundingTidelift + "=" + fxTidelift,
				fundingCommunityBridge + "=" + fxCommunity,
				fundingIssueHunt + "=" + fxIssueHunt,
				fundingLFXCrowd + "=" + fxCrowd,
				fundingPolar + "=" + fxPolar,
				fundingBuyMeACoffee + "=" + fxCoffee,
				fundingThanksDev + "=" + fxThanks,
				keyCustom + "=" + fxCustomA,
			}, ";"),
			want: &pfmodel.FundingExtension{
				Patreon: fxPatreon, KoFi: fxKoFi, Liberapay: fxLiberapay, Tidelift: fxTidelift,
				CommunityBridge: fxCommunity, IssueHunt: fxIssueHunt,
				LFXCrowdfunding: fxCrowd, Polar: fxPolar, BuyMeACoffee: fxCoffee, ThanksDev: fxThanks,
				Custom: []string{fxCustomA},
			},
		},
		{
			name:  "custom splits on commas and trims",
			input: keyCustom + "=" + fxCustomA + ", " + fxCustomB + " ,",
			want:  &pfmodel.FundingExtension{Custom: []string{fxCustomA, fxCustomB}},
		},
		{
			name:  "the last value of a repeated slug wins",
			input: fundingPatreon + "=one;" + fundingPatreon + "=" + fxPatreon,
			want:  &pfmodel.FundingExtension{Patreon: fxPatreon},
		},
		{name: "bare slug", input: fundingPatreon, bad: "expected slug=value"},
		{name: "unknown slug", input: "nosuch=" + fxPatreon, bad: "unknown funding slug"},
		{name: "empty value", input: fundingPatreon + "=", bad: "empty value for slug"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext := &pfmodel.FundingExtension{}
			err := parseOthers(tc.input, ext)
			if tc.bad != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.bad)
				assert.Equal(t, &pfmodel.FundingExtension{}, ext, "a refused entry must not half-apply")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, ext)
		})
	}
}

func TestResolvePathHonoursTheInPfOverride(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, nil)
	assert.Equal(t, filepath.Join(dir, defaultRel), resolvePath(dir, nil))
	assert.Equal(t, filepath.Join(dir, defaultRel), resolvePath(dir, &projectfile.Document{}))
	assert.Equal(t, filepath.Join(dir, overridePath), resolvePath(dir, withFunding(&projectfile.Document{}, map[string]any{keyPath: overridePath})))
	assert.Equal(t, defaultRel, relPath(nil))
	assert.Equal(t, defaultRel, relPath(&projectfile.Document{}))
	assert.Equal(t, overridePath, relPath(withFunding(&projectfile.Document{}, map[string]any{keyPath: overridePath})))
}

func TestSaveFundingExtensionRoundTripsEveryField(t *testing.T) {
	ext := allExt()
	ext.Path = overridePath
	pf := &projectfile.Document{}
	require.NoError(t, saveFundingExtension(pf, ext))
	assert.Equal(t, ext, mustFundingExtension(t, pf))

	bare := &projectfile.Document{}
	require.NoError(t, saveFundingExtension(bare, &pfmodel.FundingExtension{}))
	assert.Equal(t, &pfmodel.FundingExtension{}, mustFundingExtension(t, bare), "an empty extension writes no keys")
}

func TestHasProviders(t *testing.T) {
	assert.False(t, hasProviders(nil), "a nil extension carries no provider")
	assert.False(t, hasProviders(&pfmodel.FundingExtension{}))
	assert.False(t, hasProviders(&pfmodel.FundingExtension{Patreon: "", KoFi: ""}), "empty strings are not providers")
	assert.True(t, hasProviders(&pfmodel.FundingExtension{GitHub: []string{fxGitHub}}))
	assert.True(t, hasProviders(&pfmodel.FundingExtension{Custom: []string{fxCustomA}}))

	scalars := []struct {
		name string
		ext  *pfmodel.FundingExtension
	}{
		{fundingPatreon, &pfmodel.FundingExtension{Patreon: fxPatreon}},
		{fundingKofi, &pfmodel.FundingExtension{KoFi: fxKoFi}},
		{fundingLiberapay, &pfmodel.FundingExtension{Liberapay: fxLiberapay}},
		{fundingTidelift, &pfmodel.FundingExtension{Tidelift: fxTidelift}},
		{fundingCommunityBridge, &pfmodel.FundingExtension{CommunityBridge: fxCommunity}},
		{fundingIssueHunt, &pfmodel.FundingExtension{IssueHunt: fxIssueHunt}},
		{keyOpenColl, &pfmodel.FundingExtension{OpenCollective: fxOpenColl}},
		{fundingLFXCrowd, &pfmodel.FundingExtension{LFXCrowdfunding: fxCrowd}},
		{fundingPolar, &pfmodel.FundingExtension{Polar: fxPolar}},
		{fundingBuyMeACoffee, &pfmodel.FundingExtension{BuyMeACoffee: fxCoffee}},
		{fundingThanksDev, &pfmodel.FundingExtension{ThanksDev: fxThanks}},
	}
	for _, tc := range scalars {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, hasProviders(tc.ext), tc.name)
		})
	}
}

func TestSummaryNamesEveryConfiguredProvider(t *testing.T) {
	assert.Empty(t, summary(&pfmodel.FundingExtension{}))
	assert.Equal(t, "github(1)", summary(&pfmodel.FundingExtension{GitHub: []string{fxGitHub}}))
	assert.Equal(t, "github(2), custom(1)", summary(&pfmodel.FundingExtension{GitHub: []string{fxGitHub, fxCustomA}, Custom: []string{fxCustomB}}))

	// The scalar half walks a map, so only the presence of each part is pinned, not their order.
	parts := summary(allExt())
	for _, label := range []string{
		"github(1)", fundingPatreon, fundingKofi, fundingLiberapay, fundingTidelift,
		fundingCommunityBridge, fundingIssueHunt, keyOpenColl, fundingLFXCrowd,
		fundingPolar, fundingBuyMeACoffee, fundingThanksDev, "custom(2)",
	} {
		assert.Contains(t, parts, label)
	}
}

func TestStringsToAnys(t *testing.T) {
	assert.Empty(t, stringsToAnys(nil))
	assert.Equal(t, []any{fxCustomA, fxCustomB}, stringsToAnys([]string{fxCustomA, fxCustomB}))
}

func mustFundingExtension(t *testing.T, pf *projectfile.Document) *pfmodel.FundingExtension {
	t.Helper()
	ext, err := pfmodel.GetFundingExtension(pf)
	require.NoError(t, err)
	require.NotNil(t, ext)
	return ext
}

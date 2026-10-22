// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/v4/internal/source"
)

// fixture identity values, each named once so goconst sees a single literal.
const (
	fxNamespace = "org.example"
	fxName      = "my-tool"
	fxVersion   = "1.2.3"
	fxLicense   = "MIT"
	fxSummary   = "A tool that does X for Y"
	fxTitle     = "My Tool"
	fxTitleES   = "Mi Herramienta"
	fxOrg       = "Acme"
	fxKeyword   = "cli"
	fxStack     = "node"
	fxCreated   = "2026-01-02"
	fxModified  = "2026-02-03"
)

// packageJSON is the npm source fixture: its scoped name yields namespace npm.<scope> and every other field feeds the merge.
const packageJSON = `{
  "name": "@org.example/my-tool",
  "version": "1.2.3",
  "description": "A tool that does X for Y",
  "license": "MIT",
  "homepage": "https://example.test/home",
  "bugs": {"url": "https://example.test/issues"},
  "repository": {"type": "git", "url": "git+https://example.test/repo.git"},
  "keywords": ["cli", "tooling"],
  "author": {"name": "Damian Buho", "email": "damian.buho@example.test"}
}`

// pointUserConfigAt aims $XDG_CONFIG_HOME at a temp tree holding ext as cli.json and drops the load cache.
func pointUserConfigAt(t *testing.T, ext map[string]any) {
	t.Helper()
	dir := t.TempDir()
	if ext != nil {
		body, err := json.Marshal(ext)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "projectfile"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile", "cli.json"), body, 0o644))
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	userconfig.SetIgnored(false)
	t.Cleanup(func() { userconfig.SetIgnored(true) })
}

// seedUserConfig writes one init-defaults block, so each JSON key is a single literal in this package.
func seedUserConfig(t *testing.T, noScan bool, namespace, license, format string) {
	t.Helper()
	init := map[string]any{}
	if noScan {
		init["no-scan"] = true
	}
	if namespace != "" {
		init["namespace-default"] = namespace
	}
	if license != "" {
		init["license-default"] = license
	}
	if format != "" {
		init["format"] = format
	}
	pointUserConfigAt(t, map[string]any{"init": init})
}

// scratchDir makes an empty directory with the given basename inside the test's temp tree.
func scratchDir(t *testing.T, base string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), base)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	return dir
}

// writeNamed drops body as name inside dir.
func writeNamed(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

// readWritten loads the projectfile Run just produced at path.
func readWritten(t *testing.T, path string) *projectfile.Document {
	t.Helper()
	doc, err := projectfile.Read(path)
	require.NoError(t, err)
	return doc
}

// A full non-interactive run writes every field the npm source and the stack scanner supply.
func TestRunWritesDocumentFromSourcesAndFlags(t *testing.T) {
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)
	writeNamed(t, dir, "package.json", packageJSON)

	require.NoError(t, Run(Options{Dir: dir, NonInteractive: true, License: fxLicense}))

	doc := readWritten(t, filepath.Join(dir, "projectfile."+formatYAML))
	assert.Equal(t, "npm."+fxNamespace, doc.Identity.Namespace)
	assert.Equal(t, fxName, doc.Identity.Name)
	assert.Equal(t, fxVersion, doc.Identity.Version)
	assert.Equal(t, fxLicense, doc.License.Spdx)
	assert.Equal(t, fxSummary, doc.Identity.Summary.Bare)
	assert.Contains(t, doc.Keywords, fxKeyword)
	assert.Equal(t, projectfile.RepositoryRoleOrigin, doc.Repositories[0].Role)
	assert.Contains(t, doc.Stack, fxStack, "the stack scanner runs unless NoScan")
}

// A second run in the same directory refuses to clobber the document it finds.
func TestRunRefusesExistingProjectfile(t *testing.T) {
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)
	writeNamed(t, dir, "projectfile.yaml", "identity: {name: kept}\n")

	err := Run(Options{Dir: dir, NonInteractive: true, Namespace: fxNamespace, Name: fxName})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "projectfile already exists")
	body, readErr := os.ReadFile(filepath.Join(dir, "projectfile.yaml"))
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "kept", "the refused run wrote nothing")
}

// A non-interactive run with no identity to work from fails loudly instead of writing a hollow document.
func TestRunNonInteractiveReportsMissingRequiredFields(t *testing.T) {
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)

	err := Run(Options{Dir: dir, NonInteractive: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required fields")
	assert.Contains(t, err.Error(), "identity.namespace")
	_, statErr := os.Stat(filepath.Join(dir, "projectfile."+formatYAML))
	assert.True(t, os.IsNotExist(statErr), "a failed run left no document behind")
}

// A malformed source file aborts the run: the extractor error names the file at fault.
func TestRunReportsSourceExtractionFailure(t *testing.T) {
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)
	writeNamed(t, dir, "package.json", "{ not json")

	err := Run(Options{Dir: dir, NonInteractive: true, Namespace: fxNamespace, Name: fxName})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "discover sources")
	assert.Contains(t, err.Error(), "package.json")
}

// A scanner failure is advisory: the run warns and still writes the document.
func TestRunSurvivesScannerFailure(t *testing.T) {
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "locked"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "locked"), 0o755) })

	require.NoError(t, Run(Options{Dir: dir, NonInteractive: true, Namespace: fxNamespace, Name: fxName}))

	doc := readWritten(t, filepath.Join(dir, "projectfile."+formatYAML))
	assert.Equal(t, fxNamespace, doc.Identity.Namespace)
}

// A write the filesystem refuses surfaces as an error instead of a silent success.
func TestRunReportsWriteFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions, so no write can be refused here")
	}
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)
	writeNamed(t, dir, "projectfile.yaml.lock", "")
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := Run(Options{Dir: dir, NonInteractive: true, Namespace: fxNamespace, Name: fxName})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "write projectfile")
}

// An unsupported --format is refused before any prompt or write happens.
func TestRunRejectsUnsupportedFormat(t *testing.T) {
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)

	err := Run(Options{Dir: dir, NonInteractive: true, Namespace: fxNamespace, Name: fxName, Format: "xml"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

// User-config init defaults fill the gaps an interactive run would otherwise prompt for.
func TestRunAppliesUserConfigInitDefaults(t *testing.T) {
	seedUserConfig(t, false, fxNamespace, "Apache-2.0", formatTOML)
	dir := scratchDir(t, fxName)

	require.NoError(t, Run(Options{Dir: dir, NonInteractive: true, Name: fxName}))

	doc := readWritten(t, filepath.Join(dir, "projectfile."+formatTOML))
	assert.Equal(t, fxNamespace, doc.Identity.Namespace, "the namespace default applies non-interactively")
	assert.Equal(t, "Apache-2.0", doc.License.Spdx, "the license default applies on every run")
	assert.Equal(t, fxName, doc.Identity.Name)
}

// init.no_scan from the user config suppresses the scanner pass exactly like the flag.
func TestRunHonoursUserConfigNoScan(t *testing.T) {
	seedUserConfig(t, true, "", "", "")
	dir := scratchDir(t, fxName)
	writeNamed(t, dir, "package.json", packageJSON)

	require.NoError(t, Run(Options{Dir: dir, NonInteractive: true, License: fxLicense}))

	doc := readWritten(t, filepath.Join(dir, "projectfile."+formatYAML))
	assert.Empty(t, doc.Stack, "no_scan skipped the stack scanner")
	assert.Equal(t, fxName, doc.Identity.Name)
}

// NoScan taken from the flag leaves the stack empty while the source fields still land.
func TestRunNoScanFlagSkipsScanners(t *testing.T) {
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)
	writeNamed(t, dir, "package.json", packageJSON)

	require.NoError(t, Run(Options{Dir: dir, NonInteractive: true, NoScan: true}))

	doc := readWritten(t, filepath.Join(dir, "projectfile."+formatYAML))
	assert.Empty(t, doc.Stack)
	assert.NotEmpty(t, doc.Keywords, "declarative sources still run under NoScan")
}

// The directory basename is the identity.name suggestion a scripted run adopts silently.
func TestRunAdoptsDirectoryBasenameAsName(t *testing.T) {
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)

	require.NoError(t, Run(Options{Dir: dir, NonInteractive: true, Namespace: fxNamespace}))

	doc := readWritten(t, filepath.Join(dir, "projectfile."+formatYAML))
	assert.Equal(t, fxName, doc.Identity.Name)
	assert.Equal(t, fxNamespace, doc.Identity.Namespace)
}

// An interactive run fails at the format picker when no terminal is attached.
func TestRunInteractiveFailsWithoutTerminal(t *testing.T) {
	skipIfRealTerminal(t)
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)

	err := Run(Options{Dir: dir, Namespace: fxNamespace, Name: fxName})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "format selection")
}

// An interactive run with a known format reaches the required-fields prompt and fails there.
func TestRunInteractiveReachesMissingFieldsPrompt(t *testing.T) {
	skipIfRealTerminal(t)
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)

	err := Run(Options{Dir: dir, Format: formatYAML})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "collect fields")
}

// An interactive run with every required field known reaches the optional-fields prompt.
func TestRunInteractiveReachesOptionalFieldsPrompt(t *testing.T) {
	skipIfRealTerminal(t)
	pointUserConfigAt(t, nil)
	dir := scratchDir(t, fxName)

	err := Run(Options{Dir: dir, Format: formatYAML, Namespace: fxNamespace, Name: fxName})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "optional fields")
}

// findMissing queues a prompt per still-empty required field, carrying the ucfg defaults as prefill.
func TestFindMissingQueuesRequiredFields(t *testing.T) {
	p := &source.Partial{}

	missing := findMissing(p, missingDefaults{namespace: fxNamespace, name: fxName})

	require.Len(t, missing, 2)
	assert.Equal(t, "identity.namespace", missing[0].Label)
	assert.Equal(t, fxNamespace, missing[0].Default)
	assert.Equal(t, fxName, missing[1].Default)
	require.NotNil(t, missing[0].Setter)
	missing[0].Setter(fxNamespace)
	assert.Equal(t, fxNamespace, *p.Namespace)
	missing[1].Setter(fxName)
	assert.Equal(t, fxName, *p.Name)
}

// findMissing stays empty once both required fields carry a value.
func TestFindMissingEmptyWhenSatisfied(t *testing.T) {
	p := &source.Partial{Namespace: strPtr(fxNamespace), Name: strPtr(fxName)}

	assert.Empty(t, findMissing(p, missingDefaults{}))
}

// An empty-string value is as missing as a nil one, so the prompt still fires.
func TestFindMissingTreatsEmptyStringAsMissing(t *testing.T) {
	p := &source.Partial{Namespace: strPtr("")}

	missing := findMissing(p, missingDefaults{})

	require.Len(t, missing, 2)
	assert.Equal(t, "identity.namespace", missing[0].Label)
}

// buildDocument maps every Partial field and synthesises the schema key and a default kind.
func TestBuildDocumentMapsEveryField(t *testing.T) {
	p := &source.Partial{
		Namespace: strPtr("npm." + fxNamespace),
		Name:      strPtr(fxName),
		Title:     &source.LocalizedString{Bare: fxTitle, Langs: map[string]string{"es": fxTitleES}},
		Summary:   &source.LocalizedString{Bare: fxSummary},
		Version:   strPtr(fxVersion),
		License:   strPtr(fxLicense),
		Kind:      strPtr("SoftwareSourceCode"),
		People: []source.PersonEntry{
			{Name: fxOrg, Email: "hi@example.test", Roles: []string{"maintainer"}},
			{FamilyNames: "Buho", GivenNames: "Damian", Email: "damian.buho@example.test", Orcid: "0000-0002-1825-0097", Affiliation: fxOrg},
		},
		Keywords:     []string{fxKeyword, "tooling"},
		Repositories: []projectfile.Repository{{URL: "https://example.test/repo.git", Type: "git"}},
		Links:        []projectfile.Link{{Type: projectfile.LinkBugs, URL: "https://example.test/issues"}},
		Stack:        []string{fxStack},
		Created:      strPtr(fxCreated),
		Modified:     strPtr(fxModified),
	}

	doc := buildDocument(p)

	assert.Equal(t, "https://projectfile.org/schema/v1.json", doc.Schema)
	assert.Equal(t, "SoftwareSourceCode", doc.Kind)
	assert.Equal(t, "npm."+fxNamespace, doc.Identity.Namespace)
	assert.Equal(t, fxName, doc.Identity.Name)
	assert.Equal(t, fxVersion, doc.Identity.Version)
	assert.Equal(t, fxTitle, doc.Identity.Title.Bare)
	assert.Equal(t, fxTitleES, doc.Identity.Title.Langs["es"])
	assert.Equal(t, fxSummary, doc.Identity.Summary.Bare)
	assert.Nil(t, doc.Identity.Summary.Langs, "a Bare-only summary carries no Langs map")
	assert.Equal(t, fxLicense, doc.License.Spdx)
	assert.Equal(t, projectfile.RepositoryRoleOrigin, doc.Repositories[0].Role, "ensurePrimary promoted the only remote")
	require.Len(t, doc.Links, 1)
	require.NotNil(t, doc.Links[0].Label, "backfillLinkLabels labelled the bugs link")
	require.Len(t, doc.Organizations, 1, "a person with no family name is an organization")
	assert.Equal(t, fxOrg, doc.Organizations[0].Name)
	require.Len(t, doc.People, 1)
	assert.Equal(t, "Buho", doc.People[0].FamilyNames)
	assert.Equal(t, "0000-0002-1825-0097", doc.People[0].Orcid)
	assert.Equal(t, fxOrg, doc.People[0].Affiliation)
	assert.Equal(t, []string{fxKeyword, "tooling"}, doc.Keywords)
	assert.Equal(t, []string{fxStack}, doc.Stack)
	assert.Equal(t, fxCreated, doc.Identity.Created)
	assert.Equal(t, fxModified, doc.Identity.Modified)
}

// A bare Partial still yields a valid skeleton: schema, default kind, and nothing else.
func TestBuildDocumentOnEmptyPartial(t *testing.T) {
	doc := buildDocument(&source.Partial{})

	assert.Equal(t, "https://projectfile.org/schema/v1.json", doc.Schema)
	assert.Equal(t, "SoftwareSourceCode", doc.Kind, "an unset kind falls back to the default")
	assert.Empty(t, doc.Repositories)
	assert.Nil(t, doc.License)
	assert.Equal(t, "", doc.Identity.Created, "an unset created date is not emitted")
}

// A localized summary carries its Langs map through; a Bare-only one does not gain an empty map.
func TestBuildDocumentSummaryLocalization(t *testing.T) {
	localized := buildDocument(&source.Partial{Summary: &source.LocalizedString{Bare: fxSummary, Langs: map[string]string{"es": "Una herramienta"}}})
	assert.Equal(t, fxSummary, localized.Identity.Summary.Bare)
	assert.Equal(t, "Una herramienta", localized.Identity.Summary.Langs["es"])

	bare := buildDocument(&source.Partial{Summary: &source.LocalizedString{Bare: fxSummary}})
	assert.Equal(t, fxSummary, bare.Identity.Summary.Bare)
	assert.Nil(t, bare.Identity.Summary.Langs, "a Bare-only summary keeps no Langs map")
}

// Empty-string dates and licenses are dropped rather than written as blank values.
func TestBuildDocumentDropsEmptyStrings(t *testing.T) {
	p := &source.Partial{Created: strPtr(""), Modified: strPtr(""), License: strPtr("")}

	doc := buildDocument(p)

	assert.Equal(t, "", doc.Identity.Created)
	assert.Equal(t, "", doc.Identity.Modified)
	assert.Nil(t, doc.License)
}

// ensurePrimary leaves a declared origin alone and otherwise promotes the first entry.
func TestEnsurePrimary(t *testing.T) {
	declared := []projectfile.Repository{
		{URL: "https://example.test/fork.git", Role: "fork"},
		{URL: "https://example.test/origin.git", Role: projectfile.RepositoryRoleOrigin},
	}
	ensurePrimary(declared)
	assert.Equal(t, "fork", declared[0].Role, "an existing origin wins")

	promoted := []projectfile.Repository{{URL: "https://example.test/a.git"}, {URL: "https://example.test/b.git"}}
	ensurePrimary(promoted)
	assert.Equal(t, projectfile.RepositoryRoleOrigin, promoted[0].Role)
	assert.Equal(t, "", promoted[1].Role, "only one repository is promoted")
}

// backfillLinkLabels labels the plain-noun link types and leaves the rest untouched.
func TestBackfillLinkLabels(t *testing.T) {
	doc := &projectfile.Document{Identity: projectfile.Identity{Title: &projectfile.LocalizedString{Bare: fxTitle}}}
	doc.Links = []projectfile.Link{
		{Type: projectfile.LinkBugs, URL: "https://example.test/issues"},
		{Type: projectfile.LinkHomepage, URL: "https://example.test/home"},
		{Type: projectfile.LinkDocumentation, URL: "https://example.test/docs"},
		{Type: projectfile.LinkChat, URL: "https://example.test/chat"},
		{Type: projectfile.LinkBugs, URL: "https://example.test/other", Label: &projectfile.LocalizedString{Bare: "kept"}},
	}

	backfillLinkLabels(doc)

	for i, want := range []string{projectfile.LinkBugs, projectfile.LinkHomepage, projectfile.LinkDocumentation} {
		require.NotNil(t, doc.Links[i].Label, "link type %s gained a label", want)
	}
	assert.Nil(t, doc.Links[3].Label, "an unlisted link type keeps no label")
	require.NotNil(t, doc.Links[4].Label)
	assert.Equal(t, "kept", doc.Links[4].Label.Bare, "an existing label is never overwritten")
}

// validateFormat accepts the four supported spellings and nothing else.
func TestValidateFormat(t *testing.T) {
	for _, ok := range []string{formatYAML, "yml", formatTOML, "json"} {
		assert.NoError(t, validateFormat(ok), "%s is a supported encoding", ok)
	}
	err := validateFormat("xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xml")
	assert.Contains(t, err.Error(), "yaml, toml, or json")
}

// countFields reports how many document fields the Partial populated, counting populated values only.
func TestCountFields(t *testing.T) {
	full := &source.Partial{
		Namespace: strPtr(fxNamespace), Name: strPtr(fxName),
		Title: &source.LocalizedString{Bare: fxTitle}, Summary: &source.LocalizedString{Bare: fxSummary},
		Version: strPtr(fxVersion), License: strPtr(fxLicense), Kind: strPtr("SoftwareSourceCode"),
		People: []source.PersonEntry{{Name: fxOrg}}, Keywords: []string{fxKeyword},
		Repositories: []projectfile.Repository{{URL: "https://example.test/repo.git"}},
		Links:        []projectfile.Link{{Type: projectfile.LinkHomepage, URL: "https://example.test"}},
		Stack:        []string{fxStack},
	}
	assert.Equal(t, 12, countFields(full))

	assert.Equal(t, 0, countFields(&source.Partial{}))

	blank := &source.Partial{Version: strPtr(""), License: strPtr(""), Title: &source.LocalizedString{}}
	assert.Equal(t, 1, countFields(blank), "an empty-string version and license do not count, a present title does")
}

// strPtr is the tiny pointer helper the fixtures need; source.StringPtr would drop the empty string.
func strPtr(s string) *string { return &s }

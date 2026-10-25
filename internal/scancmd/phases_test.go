// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package scancmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/describe"
	"projectfile.org/projectfile/bridge/v4/internal/rootflags"
)

const (
	pfFileName = "projectfile.yaml"
	scanBin    = "pf-bridge-scan"
	badScanner = "not-a-scanner"
)

// The forge fixture carries one literal template, so the scanner materializes it against no placeholder.
const forgeTemplateFixture = `---
identity:
  name: scancase
org:
  projectfile:
    forge:
      links:
        - type: source-code
          url: https://github.com/acme/proj
`

const minimalFixture = `---
identity:
  name: scancase
`

// TestMain binds the command once here: a second Main call re-registers every flag and re-adds the root.
func TestMain(m *testing.M) {
	// Bind latches every PF_BRIDGE_* default at register time, so a developer's exported OFFLINE or TIMEOUT would reach the suite.
	for _, env := range []string{
		"PF_BRIDGE_QUIET", "PF_BRIDGE_VERBOSE", "PF_CLI_VERBOSE", "PF_BRIDGE_OFFLINE",
		"PF_BRIDGE_NO_COLOR", "PF_BRIDGE_COLORS", "PF_BRIDGE_SORTED", "PF_BRIDGE_IGNORE_USER_CONFIG",
		"PF_BRIDGE_FAIL_ON", "PF_BRIDGE_TIMEOUT",
	} {
		_ = os.Unsetenv(env)
	}
	argv := os.Args
	os.Args = []string{scanBin, describe.Flag} // the probe returns before cobra, which would exit on the suite's own flags
	Main(scanBin)
	os.Args = argv
	os.Exit(m.Run())
}

func fixtureDir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, pfFileName), []byte(body), 0o644))
	return dir
}

func readDoc(t *testing.T, dir string) *projectfile.Document {
	t.Helper()
	doc, _, err := projectfile.ReadWithOptions(dir, rootflags.ReadOpts())
	require.NoError(t, err)
	return doc
}

func TestRunScanNamedRefusesStrictWithPrune(t *testing.T) {
	resetScanVars(t)
	scanStrict, scanPrune = true, true
	err := runScanNamed(t.TempDir(), filepath.Join(t.TempDir(), pfFileName), []string{scannerStacks})
	assert.EqualError(t, err, "--strict and --prune are mutually exclusive")
}

func TestRunScanNamedRunsTheRequestedPhases(t *testing.T) {
	t.Run("no names run no phase", func(t *testing.T) {
		resetScanVars(t)
		dir := fixtureDir(t, forgeTemplateFixture)
		require.NoError(t, runScanNamed(dir, filepath.Join(dir, pfFileName), nil))
		assert.Empty(t, readDoc(t, dir).Links, "nothing ran, so no link was materialized")
	})

	t.Run("non-stack names run the generic phase", func(t *testing.T) {
		resetScanVars(t)
		dir := fixtureDir(t, forgeTemplateFixture)
		require.NoError(t, runScanNamed(dir, filepath.Join(dir, pfFileName), []string{scannerForge}))
		doc := readDoc(t, dir)
		require.Len(t, doc.Links, 1)
		assert.Equal(t, projectfile.LinkSourceCode, doc.Links[0].Type)
		assert.Equal(t, "https://github.com/acme/proj", doc.Links[0].URL)
	})
}

func TestRunGenericPhaseReportsAReadFailure(t *testing.T) {
	err := runGenericPhase(t.TempDir(), filepath.Join(t.TempDir(), pfFileName), []string{scannerForge})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read projectfile")
}

func TestRunGenericPhaseWritesTheMergedDocument(t *testing.T) {
	resetScanVars(t)
	dir := fixtureDir(t, forgeTemplateFixture)
	pfPath := filepath.Join(dir, pfFileName)
	require.NoError(t, runGenericPhase(dir, pfPath, []string{scannerForge}))
	assert.Len(t, readDoc(t, dir).Links, 1, "the materialized link reaches the file")
}

func TestRunGenericPhaseWarnsAndStillWritesOnAnUnknownScanner(t *testing.T) {
	resetScanVars(t)
	dir := fixtureDir(t, minimalFixture)
	require.NoError(t, runGenericPhase(dir, filepath.Join(dir, pfFileName), []string{badScanner}))
	assert.Empty(t, readDoc(t, dir).Links)
	assert.FileExists(t, filepath.Join(dir, pfFileName), "an unknown scanner never blocks the write")
}

func TestRunGenericPhaseDryRunLeavesTheFileUntouched(t *testing.T) {
	resetScanVars(t)
	scanDryRun = true
	dir := fixtureDir(t, forgeTemplateFixture)
	require.NoError(t, runGenericPhase(dir, filepath.Join(dir, pfFileName), []string{scannerForge}))
	body, err := os.ReadFile(filepath.Join(dir, pfFileName))
	require.NoError(t, err)
	assert.Equal(t, forgeTemplateFixture, string(body), "the dry run prints and writes nothing")
}

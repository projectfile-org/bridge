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

// fixtureDir writes body as this directory's projectfile and returns the directory.
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

func TestRunScanNamedSurfacesAPhaseFailure(t *testing.T) {
	t.Run("the stacks phase read failure", func(t *testing.T) {
		resetScanVars(t)
		dir := t.TempDir()
		err := runScanNamed(dir, filepath.Join(dir, pfFileName), []string{scannerStacks})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read projectfile")
	})

	t.Run("the generic phase read failure", func(t *testing.T) {
		resetScanVars(t)
		dir := t.TempDir()
		err := runScanNamed(dir, filepath.Join(dir, pfFileName), []string{scannerForge})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read projectfile")
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

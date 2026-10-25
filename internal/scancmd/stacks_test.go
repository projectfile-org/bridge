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
)

const (
	goModFile = "go.mod"
	stackGo   = "go"
)

const noStackFixture = `---
identity:
  name: scancase
`

const staleStackFixture = `---
identity:
  name: scancase
technologies:
  - go
`

// markerDir builds a fixture dir with one stack marker file beside the projectfile.
func markerDir(t *testing.T, body, marker string) string {
	t.Helper()
	dir := fixtureDir(t, body)
	require.NoError(t, os.WriteFile(filepath.Join(dir, marker), []byte("module scancase\n"), 0o644))
	return dir
}

func TestRunStacksPhaseReportsAReadFailure(t *testing.T) {
	resetScanVars(t)
	err := runStacksPhase(t.TempDir(), filepath.Join(t.TempDir(), pfFileName))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read projectfile")
}

func TestRunStacksPhaseAddsDetectedTags(t *testing.T) {
	resetScanVars(t)
	dir := markerDir(t, noStackFixture, goModFile)
	require.NoError(t, runStacksPhase(dir, filepath.Join(dir, pfFileName)))
	assert.Equal(t, []string{stackGo}, readDoc(t, dir).Stack, "the detected marker reaches the file")
}

func TestRunStacksPhaseAddNothingWhenAlreadyDeclared(t *testing.T) {
	resetScanVars(t)
	dir := markerDir(t, staleStackFixture, goModFile)
	require.NoError(t, runStacksPhase(dir, filepath.Join(dir, pfFileName)))
	assert.Equal(t, []string{stackGo}, readDoc(t, dir).Stack)
}

func TestRunStacksPhaseCheckWarnsOnAStaleTag(t *testing.T) {
	resetScanVars(t)
	scanCheck = true
	dir := fixtureDir(t, staleStackFixture)
	require.NoError(t, runStacksPhase(dir, filepath.Join(dir, pfFileName)), "--check is read-only")
	assert.Equal(t, []string{stackGo}, readDoc(t, dir).Stack)
}

func TestRunStacksPhaseStrictFailsOnAStaleTag(t *testing.T) {
	resetScanVars(t)
	scanStrict = true
	dir := fixtureDir(t, staleStackFixture)
	err := runStacksPhase(dir, filepath.Join(dir, pfFileName))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 stale tag(s) in [technologies]")
}

func TestRunStacksPhaseStrictPassesOnEvidence(t *testing.T) {
	resetScanVars(t)
	scanStrict = true
	dir := markerDir(t, staleStackFixture, goModFile)
	require.NoError(t, runStacksPhase(dir, filepath.Join(dir, pfFileName)))
}

func TestRunStacksPhasePrunesStaleTags(t *testing.T) {
	resetScanVars(t)
	scanPrune = true
	dir := fixtureDir(t, staleStackFixture)
	require.NoError(t, runStacksPhase(dir, filepath.Join(dir, pfFileName)))
	assert.Empty(t, readDoc(t, dir).Stack, "the tag with no filesystem evidence is gone")
}

func TestRunStacksPhasePruneDryRunWritesNothing(t *testing.T) {
	resetScanVars(t)
	scanPrune, scanDryRun = true, true
	dir := fixtureDir(t, staleStackFixture)
	require.NoError(t, runStacksPhase(dir, filepath.Join(dir, pfFileName)))
	body, err := os.ReadFile(filepath.Join(dir, pfFileName))
	require.NoError(t, err)
	assert.Equal(t, staleStackFixture, string(body))
}

func TestRunStacksPhaseDryRunLeavesAnAdditionUnwritten(t *testing.T) {
	resetScanVars(t)
	scanDryRun = true
	dir := markerDir(t, noStackFixture, goModFile)
	require.NoError(t, runStacksPhase(dir, filepath.Join(dir, pfFileName)))
	body, err := os.ReadFile(filepath.Join(dir, pfFileName))
	require.NoError(t, err)
	assert.Equal(t, noStackFixture, string(body))
}

func TestRunScanNamedRunsTheStacksPhase(t *testing.T) {
	resetScanVars(t)
	dir := markerDir(t, noStackFixture, goModFile)
	require.NoError(t, runScanNamed(dir, filepath.Join(dir, pfFileName), []string{scannerStacks}))
	assert.Equal(t, []string{stackGo}, readDoc(t, dir).Stack)
}

func TestRunScanNamedRunsBothPhases(t *testing.T) {
	resetScanVars(t)
	dir := fixtureDir(t, noStackFixture)
	require.NoError(t, os.WriteFile(filepath.Join(dir, goModFile), []byte("module scancase\n"), 0o644))
	require.NoError(t, runScanNamed(dir, filepath.Join(dir, pfFileName), []string{scannerStacks, scannerForge}))
	assert.Equal(t, []string{stackGo}, readDoc(t, dir).Stack, "stacks ran")
	assert.Empty(t, readDoc(t, dir).Links, "the fixture declares no forge templates")
}

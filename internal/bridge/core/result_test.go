// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// every field/count the Format tests share, named once for goconst.
const (
	testExtName = "package.json"
	testPFName  = "projectfile"
)

func change(key, value string) core.FieldChange {
	return core.FieldChange{Key: key, Value: value, From: testPFName, To: testExtName}
}

// A converged sync says so in plain words, whatever the mode.
func TestResultFormatReportsNoChanges(t *testing.T) {
	assert.Equal(t, "Already in sync — no changes needed", (&core.Result{
		ExtName: testExtName, PFName: testPFName,
	}).Format())
}

// The create-with-both-sides summary is the one string that names both files.
func TestResultFormatSummarisesCreateWithBothSides(t *testing.T) {
	got := (&core.Result{
		Created: true, ExtChanged: true, PFChanged: true,
		ExtName: testExtName, PFName: testPFName,
		ExtFields: []core.FieldChange{change("name", "x")},
		PFFields:  []core.FieldChange{change("version", "1")},
	}).Format()

	assert.True(t, strings.HasPrefix(got,
		"Created package.json (1 fields) and Updated projectfile (1 fields)"),
		"got: %s", got)
	assert.Contains(t, got, "\n  name (x) copied from projectfile to package.json")
	assert.Contains(t, got, "\n  version (1) copied from projectfile to package.json")
}

// Each remaining switch arm renders its own headline, ext fields before pf fields.
func TestResultFormatCoversEverySummaryArm(t *testing.T) {
	created := (&core.Result{
		Created: true, ExtChanged: true,
		ExtName: testExtName, PFName: testPFName,
		ExtFields: []core.FieldChange{change("name", "x")},
	}).Format()
	assert.True(t, strings.HasPrefix(created, "Created package.json (1 fields)"), "got: %s", created)

	both := (&core.Result{
		ExtChanged: true, PFChanged: true,
		ExtName: testExtName, PFName: testPFName,
		ExtFields: []core.FieldChange{change("name", "x")},
		PFFields:  []core.FieldChange{change("version", "1")},
	}).Format()
	assert.True(t, strings.HasPrefix(both,
		"Bridged: updated package.json (1 fields) and projectfile (1 fields)"), "got: %s", both)

	extOnly := (&core.Result{
		ExtChanged: true,
		ExtName:    testExtName, PFName: testPFName,
		ExtFields: []core.FieldChange{change("name", "x")},
	}).Format()
	assert.True(t, strings.HasPrefix(extOnly, "Bridged: updated package.json (1 fields)"), "got: %s", extOnly)

	pfOnly := (&core.Result{
		PFChanged: true,
		ExtName:   testExtName, PFName: testPFName,
		PFFields: []core.FieldChange{change("version", "1")},
	}).Format()
	assert.True(t, strings.HasPrefix(pfOnly, "Bridged: updated projectfile (1 fields)"), "got: %s", pfOnly)
}

// Dry-run swaps the verbs so a simulation is never misread as a write.
func TestResultFormatIsDryRunAware(t *testing.T) {
	got := (&core.Result{
		DryRun: true, Created: true, ExtChanged: true, PFChanged: true,
		ExtName: testExtName, PFName: testPFName,
		ExtFields: []core.FieldChange{change("name", "x")},
		PFFields:  []core.FieldChange{change("version", "1")},
	}).Format()

	assert.True(t, strings.HasPrefix(got,
		"Would create package.json (1 fields) and Would update projectfile (1 fields)"),
		"got: %s", got)
}

// A cleared field gets its own line shape: "removed", not a copy.
func TestResultFormatMarksRemovedFields(t *testing.T) {
	got := (&core.Result{
		ExtChanged: true,
		ExtName:    testExtName, PFName: testPFName,
		ExtFields: []core.FieldChange{{Key: "keywords", Value: core.Removed, From: testExtName, To: testExtName}},
	}).Format()

	assert.Contains(t, got, "\n  keywords removed from package.json (absent from package.json)")
}

// A summary with no field lines is the whole output — a --force header rewrite.
func TestResultFormatDropsFieldSectionWhenEmpty(t *testing.T) {
	assert.Equal(t, "Bridged: updated package.json (0 fields)",
		(&core.Result{ExtChanged: true, ExtName: testExtName}).Format())
}

// Trunc caps the display value at 60 bytes with an ellipsis; the boundary values pin both sides of the cut.
func TestTruncCapsAtSixtyBytes(t *testing.T) {
	assert.Equal(t, "", core.Trunc(""))
	assert.Equal(t, strings.Repeat("a", 60), core.Trunc(strings.Repeat("a", 60)),
		"a value at the cap is untouched")
	assert.Equal(t, strings.Repeat("a", 57)+"...", core.Trunc(strings.Repeat("a", 61)),
		"a value past the cap loses its tail")
	assert.Len(t, core.Trunc(strings.Repeat("a", 200)), 60)
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/v4/internal/source"
)

const (
	fxScanA     = "scan-a"
	fxScanB     = "scan-b"
	fxScanC     = "scan-c"
	fxUnknown   = "scan-unknown"
	fxNameA     = "alpha"
	fxNameB     = "beta"
	fxStackGo   = "go"
	fxStackNode = "node"
	fxHitField  = "stack:go"
	fxErrBoom   = "boom"
	fxPFFailMsg = "unknown scanner: "
	fxJoinMsg   = "scanners failed:"
)

// fakeScanner is a Scanner whose every answer, and its Scan call count, is chosen by the test.
type fakeScanner struct {
	name    string
	detect  bool
	partial *source.Partial
	hits    []Hit
	err     error
	scanned int
}

func (f *fakeScanner) Name() string       { return f.name }
func (f *fakeScanner) Detect(string) bool { return f.detect }

func (f *fakeScanner) Scan(string) (*source.Partial, []Hit, error) {
	f.scanned++
	return f.partial, f.hits, f.err
}

// namesOf collects the registry's keys in the order List returned them.
func namesOf(scanners []Scanner) []string {
	out := make([]string, len(scanners))
	for i, s := range scanners {
		out[i] = s.Name()
	}
	return out
}

// snapshotRegistry empties the process-global registry for one case and restores it after: Register has no unregister, so a test that declares its own set asserts an exact one.
func snapshotRegistry(t *testing.T) {
	t.Helper()
	mu.Lock()
	savedReg, savedByName := registered, byName
	registered, byName = nil, map[string]Scanner{}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		registered, byName = savedReg, savedByName
	})
}

// TestRegisterLookupAndList pins the registry contract: Lookup answers by name, and List copies and sorts for a stable --list the caller cannot corrupt.
func TestRegisterLookupAndList(t *testing.T) {
	snapshotRegistry(t)
	Register(&fakeScanner{name: fxScanB})
	Register(&fakeScanner{name: fxScanA})

	got, ok := Lookup(fxScanA)
	require.True(t, ok)
	assert.Equal(t, fxScanA, got.Name())
	_, ok = Lookup(fxUnknown)
	assert.False(t, ok, "an unregistered name is absent, not zero")

	assert.Equal(t, []string{fxScanA, fxScanB}, namesOf(List()))
	list := List()
	list[0] = nil
	assert.Equal(t, []string{fxScanA, fxScanB}, namesOf(List()), "the caller's slice is a copy")
}

// TestRegisterRefusesEmptyAndDuplicateNames pins both init-time programmer errors panic with their message, leaving the earlier registration intact.
func TestRegisterRefusesEmptyAndDuplicateNames(t *testing.T) {
	snapshotRegistry(t)
	assert.PanicsWithValue(t, "scanners.Register: empty Name", func() { Register(&fakeScanner{}) })
	Register(&fakeScanner{name: fxScanA})
	assert.PanicsWithValue(t, "scanners.Register: duplicate Name "+fxScanA, func() { Register(&fakeScanner{name: fxScanA}) })

	got, ok := Lookup(fxScanA)
	require.True(t, ok, "a refused duplicate leaves the first registration in place")
	assert.Equal(t, fxScanA, got.Name())
}

// TestRunAllWithNoScanners pins the empty-registry answer: an empty partial, no findings, no error, for both runners.
func TestRunAllWithNoScanners(t *testing.T) {
	snapshotRegistry(t)
	root := t.TempDir()

	merged, hits, err := RunAll(root)
	require.NoError(t, err)
	assert.Equal(t, &source.Partial{}, merged)
	assert.Empty(t, hits)

	merged, hits, err = RunNamed(root, nil)
	require.NoError(t, err)
	assert.Equal(t, &source.Partial{}, merged)
	assert.Empty(t, hits)
}

// TestRunAllSkipsInapplicableScanners pins the cheap probe: a Detect miss never reaches Scan, so an inapplicable scanner costs nothing.
func TestRunAllSkipsInapplicableScanners(t *testing.T) {
	snapshotRegistry(t)
	inactive := &fakeScanner{name: fxScanA}
	active := &fakeScanner{name: fxScanB, detect: true, partial: &source.Partial{Stack: []string{fxStackGo}}}
	Register(inactive)
	Register(active)

	merged, hits, err := RunAll(t.TempDir())
	require.NoError(t, err)
	assert.Zero(t, inactive.scanned, "Detect false skips Scan entirely")
	assert.Equal(t, 1, active.scanned)
	assert.Equal(t, []string{fxStackGo}, merged.Stack)
	assert.Empty(t, hits)
}

// TestRunAllMergesApplicableScanners pins the merge: fields gap-fill across scanners and every finding is reported in registration order.
func TestRunAllMergesApplicableScanners(t *testing.T) {
	snapshotRegistry(t)
	Register(&fakeScanner{
		name: fxScanA, detect: true,
		partial: &source.Partial{Name: source.StringPtr(fxNameA)},
		hits:    []Hit{{Source: fxScanA, Field: fxHitField}},
	})
	Register(&fakeScanner{
		name: fxScanB, detect: true,
		partial: &source.Partial{Stack: []string{fxStackGo}},
		hits:    []Hit{{Source: fxScanB, Field: fxHitField}},
	})

	merged, hits, err := RunAll(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, source.StringPtr(fxNameA), merged.Name)
	assert.Equal(t, []string{fxStackGo}, merged.Stack)
	assert.Equal(t, []Hit{{Source: fxScanA, Field: fxHitField}, {Source: fxScanB, Field: fxHitField}}, hits)
}

// TestRunAllFirstRegisteredWins pins registration order as the merge precedence: the first scanner to declare a field keeps it under MergePartials' existing-wins rule.
func TestRunAllFirstRegisteredWins(t *testing.T) {
	snapshotRegistry(t)
	Register(&fakeScanner{name: fxScanA, detect: true, partial: &source.Partial{Name: source.StringPtr(fxNameA)}})
	Register(&fakeScanner{name: fxScanB, detect: true, partial: &source.Partial{Name: source.StringPtr(fxNameB)}})

	merged, _, err := RunAll(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, source.StringPtr(fxNameA), merged.Name)
}

// TestRunAllIgnoresANilPartial pins the nil guard: a scanner reporting nothing neither merges nor fails the run, and the rest still fill.
func TestRunAllIgnoresANilPartial(t *testing.T) {
	snapshotRegistry(t)
	empty := &fakeScanner{name: fxScanA, detect: true}
	Register(empty)
	Register(&fakeScanner{name: fxScanB, detect: true, partial: &source.Partial{Stack: []string{fxStackNode}}})

	merged, _, err := RunAll(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, 1, empty.scanned, "a nil partial still ran and reported")
	assert.Equal(t, []string{fxStackNode}, merged.Stack)
}

// TestRunAllKeepsRunningAfterAFailure pins one scanner's failure is named and wrapped, yet neither aborts the rest nor discards the partial output.
func TestRunAllKeepsRunningAfterAFailure(t *testing.T) {
	snapshotRegistry(t)
	boom := errors.New(fxErrBoom)
	broken := &fakeScanner{name: fxScanA, detect: true, err: boom}
	good := &fakeScanner{name: fxScanB, detect: true, partial: &source.Partial{Name: source.StringPtr(fxNameB)}}
	Register(broken)
	Register(good)

	merged, _, err := RunAll(t.TempDir())
	require.Error(t, err)
	assert.ErrorIs(t, err, boom, "the cause rides the joined error")
	assert.Contains(t, err.Error(), fxScanA+": ", "the failing scanner is named")
	assert.Equal(t, source.StringPtr(fxNameB), merged.Name, "partial output survives one failure")
	assert.Equal(t, 1, good.scanned, "the rest still run")
}

// TestRunAllJoinsEveryFailure pins the two-failure shape: one message names every scanner, and a healthy one still merges.
func TestRunAllJoinsEveryFailure(t *testing.T) {
	snapshotRegistry(t)
	Register(&fakeScanner{name: fxScanA, detect: true, err: errors.New("first down")})
	Register(&fakeScanner{name: fxScanB, detect: true, err: errors.New("second down")})
	Register(&fakeScanner{name: fxScanC, detect: true, partial: &source.Partial{Stack: []string{fxStackGo}}})

	merged, _, err := RunAll(t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), fxJoinMsg)
	assert.Contains(t, err.Error(), fxScanA+": first down")
	assert.Contains(t, err.Error(), fxScanB+": second down")
	assert.Equal(t, []string{fxStackGo}, merged.Stack)
}

// TestRunNamedRunsOnlyNamedScanners pins the named set and its order: an unnamed scanner never runs and the first NAMED one wins, whatever the registration order.
func TestRunNamedRunsOnlyNamedScanners(t *testing.T) {
	snapshotRegistry(t)
	Register(&fakeScanner{
		name: fxScanA, detect: true,
		partial: &source.Partial{Name: source.StringPtr(fxNameA), Stack: []string{fxStackGo}},
		hits:    []Hit{{Source: fxScanA, Field: fxHitField}},
	})
	Register(&fakeScanner{
		name: fxScanB, detect: true,
		partial: &source.Partial{Name: source.StringPtr(fxNameB)},
		hits:    []Hit{{Source: fxScanB, Field: fxHitField}},
	})
	unnamed := &fakeScanner{name: fxScanC, detect: true}
	Register(unnamed)

	merged, hits, err := RunNamed(t.TempDir(), []string{fxScanB, fxScanA})
	require.NoError(t, err)
	assert.Zero(t, unnamed.scanned, "an unnamed scanner never runs")
	assert.Equal(t, source.StringPtr(fxNameB), merged.Name, "the first NAMED scanner wins")
	assert.Equal(t, []Hit{{Source: fxScanB, Field: fxHitField}, {Source: fxScanA, Field: fxHitField}}, hits)
}

// TestRunNamedReportsUnknownNamesAndSkipsInactive pins an unknown name errors without aborting the rest, and a Detect miss skips under RunNamed too.
func TestRunNamedReportsUnknownNamesAndSkipsInactive(t *testing.T) {
	snapshotRegistry(t)
	known := &fakeScanner{name: fxScanA, detect: true, partial: &source.Partial{Stack: []string{fxStackGo}}}
	inactive := &fakeScanner{name: fxScanC}
	broken := &fakeScanner{name: fxScanB, detect: true, err: errors.New(fxErrBoom)}
	Register(known)
	Register(inactive)
	Register(broken)

	merged, _, err := RunNamed(t.TempDir(), []string{fxUnknown, fxScanA, fxScanC, fxScanB})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fxPFFailMsg+fxUnknown)
	assert.Contains(t, err.Error(), fxScanB+": "+fxErrBoom, "a named scanner's failure is joined too")
	assert.Equal(t, []string{fxStackGo}, merged.Stack, "a bad name skips, the resolvable ones still run")
	assert.Zero(t, inactive.scanned, "a Detect miss is skipped even when named")
	assert.Equal(t, 1, broken.scanned, "a failing named scanner still ran")

	_, _, err = RunNamed(t.TempDir(), []string{fxUnknown})
	require.EqualError(t, err, fxPFFailMsg+fxUnknown, "one unknown name passes through joined as itself")
}

// TestHasProjectfile pins the Detect probe: exactly one projectfile sibling answers true, none or the two-sibling ambiguity answer false.
func TestHasProjectfile(t *testing.T) {
	assert.False(t, HasProjectfile(t.TempDir()))

	yaml := t.TempDir()
	writeProjectfile(t, yaml, "projectfile.yaml")
	assert.True(t, HasProjectfile(yaml))

	toml := t.TempDir()
	writeProjectfile(t, toml, "projectfile.toml")
	assert.True(t, HasProjectfile(toml), "every encoding the spec allows is a projectfile")

	both := t.TempDir()
	writeProjectfile(t, both, "projectfile.yaml")
	writeProjectfile(t, both, "projectfile.toml")
	assert.False(t, HasProjectfile(both), "two siblings is the spec §3.5 ambiguity, which reads as no projectfile")
}

// writeProjectfile creates one projectfile sibling; DetectPath stats the name alone, so the body only has to exist.
func writeProjectfile(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("identity:\n"), 0o644))
}

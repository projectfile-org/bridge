// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package declared

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// readmeCheck is the run line the cases below declare most often.
const readmeCheck = "pf-bridge readme --check"

// docWithTools builds a document carrying the given org.projectfile.ci.tools
// rows, keyed row name → run line.
func docWithTools(rows map[string]string) *projectfile.Document {
	tools := map[string]any{}
	for name, run := range rows {
		tools[name] = map[string]any{runKey: run}
	}
	return &projectfile.Document{
		Extensions: map[string]any{
			ciNS: map[string]any{toolsKey: tools},
		},
	}
}

func TestChecksOfKeepsOnlyBridgeCheckRows(t *testing.T) {
	pf := docWithTools(map[string]string{
		"pf-bridge-gitignore-check":    "pf-bridge ignore .gitignore --check",
		"pf-bridge-gitignore-generate": "pf-bridge ignore .gitignore --force",
		"pf-bridge-readme-check":       readmeCheck,
		"pf-validate":                  "pf-cli validate",
		"go-vet":                       "go vet ./...",
		"pf-bridge-check":              "pf-bridge check",
	})

	runs := ChecksOf(pf)
	if len(runs) != 2 {
		t.Fatalf("expected 2 declared checks, got %d: %+v", len(runs), runs)
	}
	// Sorted by row name: gitignore before readme.
	if runs[0].Name != "ignore" || strings.Join(runs[0].Args, " ") != ".gitignore --check" {
		t.Errorf("first run = %+v", runs[0])
	}
	if runs[1].Name != "readme" || strings.Join(runs[1].Args, " ") != "--check" {
		t.Errorf("second run = %+v", runs[1])
	}
	if runs[0].Tool != "pf-bridge-gitignore-check" {
		t.Errorf("tool name lost: %q", runs[0].Tool)
	}
}

// Two rows spelling the same invocation must exec one child, not two: an
// include chain can land the same check under two names.
func TestChecksOfDedupes(t *testing.T) {
	pf := docWithTools(map[string]string{
		"pf-bridge-readme-check": readmeCheck,
		"readme-drift":           readmeCheck,
	})
	if runs := ChecksOf(pf); len(runs) != 1 {
		t.Fatalf("expected 1 deduped check, got %d: %+v", len(runs), runs)
	}
}

func TestChecksOfWithoutManifest(t *testing.T) {
	if runs := ChecksOf(&projectfile.Document{}); runs != nil {
		t.Fatalf("expected no checks without a ci manifest, got %+v", runs)
	}
	if runs := ChecksOf(docWithTools(nil)); runs != nil {
		t.Fatalf("expected no checks from an empty manifest, got %+v", runs)
	}
}

// A row whose `run` is missing, non-string, or not a pf-bridge line must be
// skipped rather than crash the sweep — the manifest is open to any tool.
func TestChecksOfIgnoresMalformedRows(t *testing.T) {
	pf := &projectfile.Document{
		Extensions: map[string]any{
			ciNS: map[string]any{toolsKey: map[string]any{
				"no-run":     map[string]any{"image": "PF_BRIDGE_IMAGE"},
				"not-a-map":  readmeCheck,
				"not-string": map[string]any{runKey: 42},
				"bare":       map[string]any{runKey: "pf-bridge"},
			}},
		},
	}
	if runs := ChecksOf(pf); runs != nil {
		t.Fatalf("expected malformed rows to be skipped, got %+v", runs)
	}
}

// writeManifest lays a projectfile.yaml down in dir whose ci.tools rows map row
// name to run line — the on-disk shape docWithTools holds in memory.
func writeManifest(t *testing.T, dir string, rows map[string]string) {
	t.Helper()
	var doc strings.Builder
	doc.WriteString("org.projectfile.ci:\n  tools:\n")
	for name, run := range rows {
		fmt.Fprintf(&doc, "    %s:\n      run: %s\n", name, run)
	}
	if err := os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte(doc.String()), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// Checks resolves the manifest off disk exactly as the dispatcher calls it,
// with the zero options a default CLI run carries.
func TestChecksReadsManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, map[string]string{
		"pf-bridge-ignore-check":  "pf-bridge ignore .gitignore --check",
		"pf-bridge-license-check": "pf-bridge license --check",
	})

	runs, err := Checks(dir, projectfile.ReadOptions{})
	if err != nil {
		t.Fatalf("Checks: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 declared checks, got %d: %+v", len(runs), runs)
	}
	// Sorted by row name: ignore before license, as ChecksOf guarantees.
	if runs[0].Name != "ignore" || strings.Join(runs[0].Args, " ") != ".gitignore --check" {
		t.Errorf("first run = %+v", runs[0])
	}
	if runs[0].Tool != "pf-bridge-ignore-check" {
		t.Errorf("tool name lost: %q", runs[0].Tool)
	}
	if runs[1].Name != "license" || strings.Join(runs[1].Args, " ") != "--check" {
		t.Errorf("second run = %+v", runs[1])
	}
}

// The reader walks no parent directory, so a manifestless temp dir is the
// deterministic read failure Checks wraps for the dispatcher's warning.
func TestChecksWrapsReadError(t *testing.T) {
	runs, err := Checks(t.TempDir(), projectfile.ReadOptions{})
	if err == nil {
		t.Fatal("expected a read error with no projectfile present")
	}
	if runs != nil {
		t.Errorf("runs alongside the error: %+v", runs)
	}
	if msg := err.Error(); !strings.Contains(msg, "read projectfile:") || !strings.Contains(msg, "no projectfile found") {
		t.Errorf("error = %q", msg)
	}
}

// An org.projectfile.ci that is not a table holds no rows to read, and the
// sweep skips the project rather than crashing on it.
func TestChecksOfWithScalarCI(t *testing.T) {
	pf := &projectfile.Document{Extensions: map[string]any{ciNS: "not-a-table"}}
	if runs := ChecksOf(pf); runs != nil {
		t.Fatalf("expected no checks from a scalar ci namespace, got %+v", runs)
	}
}

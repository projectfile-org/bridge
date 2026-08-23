// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package declared

import (
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

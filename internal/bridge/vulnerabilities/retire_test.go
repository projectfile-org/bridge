// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package vulnerabilities

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

const retireStaleID = "CVE-2026-1472"

// decodeJSONC drops the full-line // comments and parses the rest as JSON.
func decodeJSONC(t *testing.T, body string) map[string]any {
	t.Helper()
	var lines []string
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "//") {
			continue
		}
		lines = append(lines, l)
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.Join(lines, "\n")), &out), "the body must parse as JSONC")
	return out
}

// scannerFiles is one row per scanner file: the managed form carries the dialect's marker, the detached form is the same state with it stripped.
var scannerFiles = []struct {
	scanner, filename, managed, detached string
}{
	{
		scannerTrivy, ".trivyignore",
		core.Marker + "\n" + retireStaleID + "\n",
		retireStaleID + "\n",
	},
	{
		scannerGrype, testGrypeYAML,
		core.YAMLDocStart + core.Marker + "\nignore:\n  - vulnerability: " + retireStaleID + "\n",
		core.YAMLDocStart + "ignore:\n  - vulnerability: " + retireStaleID + "\n",
	},
	{
		scannerOSV, "osv-scanner.toml",
		core.Marker + "\n[[IgnoredVulns]]\nid = \"" + retireStaleID + "\"\n",
		"[[IgnoredVulns]]\nid = \"" + retireStaleID + "\"\n",
	},
	{
		scannerAuditCI, "audit-ci.jsonc",
		core.MarkerSlash + "\n{\n  \"allowlist\": [\"" + retireStaleID + "\"]\n}\n",
		"{\n  \"allowlist\": [\"" + retireStaleID + "\"]\n}\n",
	},
}

// The empty source (absent namespace or zero entries) retires the file it once generated — deleted for the auto-discovered ignores, an empty-allowlist rewrite for audit-ci whose wrapper fails on a missing --config file.
func TestRenderEmptySourceRetires(t *testing.T) {
	for _, row := range scannerFiles {
		t.Run(row.scanner, func(t *testing.T) {
			for _, doc := range []*projectfile.Document{
				{Identity: projectfile.Identity{Name: "p"}}, // namespace absent
				sampleDoc(nil), // suppress emptied
			} {
				out, err := Bridge{scanner: row.scanner, filename: row.filename}.Render(doc, core.Options{})
				require.NoError(t, err)
				assert.Empty(t, out.Files)
				body, ok := out.Remove[row.filename]
				require.True(t, ok, row.filename+" must ask for retirement")
				if row.scanner != scannerAuditCI {
					assert.Nil(t, body, row.filename+" retires by deletion")
					continue
				}
				require.NotNil(t, body)
				assert.Contains(t, string(body), core.MarkerSlash, "the empty state keeps its generated header")
				assert.Contains(t, string(body), `"low": true`)
				assert.Contains(t, string(body), "$schema")
				assert.NotContains(t, string(body), retireStaleID)
				config := decodeJSONC(t, string(body))
				allowlist, exists := config["allowlist"]
				require.True(t, exists, "the empty state keeps the config shape")
				assert.Empty(t, allowlist)
			}
		})
	}
}

// `generate` narrows which scanners receive fresh files; it never shields a stale managed copy — an empty source leaves no suppression standing anywhere the bridge owns.
func TestGenerateOptOutDoesNotShieldRetirement(t *testing.T) {
	doc := sampleDoc(nil, scannerTrivy)
	for _, row := range scannerFiles {
		out, err := Bridge{scanner: row.scanner, filename: row.filename}.Render(doc, core.Options{})
		require.NoError(t, err)
		_, ok := out.Remove[row.filename]
		assert.True(t, ok, row.filename+" must retire on an empty source")
	}
}

// The issue's acceptance, end to end: emptying the list and regenerating leaves no stale suppression in any managed scan file, while a detached copy stays its owner's file.
func TestEmptyListRegenerationLeavesNoStaleSuppression(t *testing.T) {
	t.Run("managed", func(t *testing.T) {
		dir := t.TempDir()
		for _, row := range scannerFiles {
			require.NoError(t, os.WriteFile(filepath.Join(dir, row.filename), []byte(row.managed), 0o644))
		}
		for _, row := range scannerFiles {
			b := Bridge{scanner: row.scanner, filename: row.filename}
			require.NoError(t, core.RunRender(b, sampleDoc(nil), core.Options{Dir: dir}), row.filename)
		}
		assert.NoFileExists(t, filepath.Join(dir, ".trivyignore"))
		assert.NoFileExists(t, filepath.Join(dir, testGrypeYAML))
		assert.NoFileExists(t, filepath.Join(dir, "osv-scanner.toml"))
		data, err := os.ReadFile(filepath.Join(dir, "audit-ci.jsonc"))
		require.NoError(t, err)
		config := decodeJSONC(t, string(data))
		assert.Empty(t, config["allowlist"])
		assert.NotContains(t, string(data), retireStaleID)
	})

	t.Run("detached", func(t *testing.T) {
		dir := t.TempDir()
		for _, row := range scannerFiles {
			require.NoError(t, os.WriteFile(filepath.Join(dir, row.filename), []byte(row.detached), 0o644))
		}
		for _, row := range scannerFiles {
			b := Bridge{scanner: row.scanner, filename: row.filename}
			require.NoError(t, core.RunRender(b, sampleDoc(nil), core.Options{Dir: dir}), row.filename)
			data, err := os.ReadFile(filepath.Join(dir, row.filename))
			require.NoError(t, err)
			assert.Contains(t, string(data), retireStaleID, "a detached copy keeps every byte — it belongs to its owner")
		}
	})
}

// The gate names every managed stale file — delete- and rewrite-shaped alike — and still changes nothing on disk.
func TestCheckNamesEveryManagedStaleFile(t *testing.T) {
	for _, row := range scannerFiles {
		t.Run(row.filename, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, row.filename), []byte(row.managed), 0o644))
			b := Bridge{scanner: row.scanner, filename: row.filename}
			err := core.RunRender(b, sampleDoc(nil), core.Options{Dir: dir, Check: true, DryRun: true})
			require.Error(t, err)
			assert.Contains(t, err.Error(), row.filename)
			data, readErr := os.ReadFile(filepath.Join(dir, row.filename))
			require.NoError(t, readErr)
			assert.Equal(t, row.managed, string(data), "the check must never write")
		})
	}
}

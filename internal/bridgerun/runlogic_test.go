// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package bridgerun

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/v4/internal/bridge"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/warn"
)

// Shared literals live here because goconst counts occurrences across every _test.go file of the package, not within one file.
const (
	pfFileName  = "projectfile.yaml"
	stackNode   = "node"
	stackPython = "python"
	spdxMIT     = "MIT"
	envTrue     = "1"
	envFalse    = "0"
	envGarbage  = "garbage"
	argFile     = "x.json"
	argDir      = "mydir"
	dirTo       = "to"
	dirFrom     = "from"
)

// writeFixtureProjectfile writes the minimal document every run test reads; technologies carries the Stack declaration.
func writeFixtureProjectfile(t *testing.T, dir, technologies string) {
	t.Helper()
	body := "---\n" +
		"# SPDX-FileCopyrightText: 2026 Tester\n" +
		"#\n" +
		// REUSE-IgnoreStart
		"# SPDX-License-Identifier: " + spdxMIT + "\n" +
		// REUSE-IgnoreEnd
		"$schema: https://projectfile.org/schema/v1.json\n" +
		"identity:\n  name: " + filepath.Base(dir) + "\n"
	if technologies != "" {
		body += "technologies: [" + technologies + "]\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, pfFileName), []byte(body), 0o644))
}

// isolateUserConfig points the user-config cache at dir and drops it, so no personal default reaches a run; the cleanup re-ignores it.
func isolateUserConfig(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", dir)
	userconfig.SetIgnored(false)
	t.Cleanup(func() { userconfig.SetIgnored(true) })
}

// resetGlobals clears the flag vars, the warning ledger and the log style so one run's state never reaches the next test.
func resetGlobals(t *testing.T) {
	t.Helper()
	savedInput := promptInput
	warn.Reset()
	require.NoError(t, genlog.SetColor(genlog.ColorNever))
	genlog.SetQuiet(true)
	t.Cleanup(func() {
		bridgeForce, bridgeDryRun, bridgePreview, bridgeCreateAll = false, false, false, false
		bridgeCheck, bridgeNoDiff, bridgeFailOnDrift, bridgeList, bridgeReuseCanonical = false, false, false, false, false
		promptInput = savedInput
		bridge.Scope("")
		warn.Reset()
		genlog.SetQuiet(false)
		require.NoError(t, genlog.SetColor(genlog.ColorAuto))
	})
}

// noTTY points the prompt stream at a pipe end — never a terminal — so the non-interactive branches read as they do in CI.
func noTTY(t *testing.T) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	promptInput = r
	t.Cleanup(func() { _ = r.Close(); _ = w.Close() })
}

// newCmd is the cobra command a run receives, with its output captured.
func newCmd() (*cobra.Command, *bytes.Buffer) {
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "pf-bridge-test"}
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	return cmd, &out
}

func TestEnvFlag(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{envTrue, true}, {"true", true}, {envFalse, false}, {envGarbage, false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("PF_BRIDGE_TEST_ENV", tc.value)
			assert.Equal(t, tc.want, envFlag("PF_BRIDGE_TEST_ENV"))
			assert.False(t, envFlag("PF_BRIDGE_TEST_UNSET"))
		})
	}
}

func TestFailOnDriftResolution(t *testing.T) {
	t.Run("flag alone", func(t *testing.T) {
		resetGlobals(t)
		bridgeFailOnDrift = true
		assert.True(t, failOnDrift())
	})
	t.Run("flag overrides a false env", func(t *testing.T) {
		resetGlobals(t)
		bridgeFailOnDrift = true
		t.Setenv(EnvFailOnDrift, envFalse)
		assert.True(t, failOnDrift())
	})
	t.Run("env raises the gate", func(t *testing.T) {
		resetGlobals(t)
		t.Setenv(EnvFailOnDrift, envTrue)
		assert.True(t, failOnDrift())
	})
	t.Run("unset and unparseable stay lenient", func(t *testing.T) {
		resetGlobals(t)
		t.Setenv(EnvFailOnDrift, envGarbage)
		assert.False(t, failOnDrift())
	})
}

func TestParseBridgeArgsTable(t *testing.T) {
	resetGlobals(t)
	for _, tc := range []struct {
		name    string
		args    []string
		mode    core.Mode
		file    string
		dir     string
		wantErr bool
	}{
		{"no args", nil, core.ModeSync, "", ".", false},
		{"filename alone", []string{argFile}, core.ModeSync, argFile, ".", false},
		{"to forces write", []string{dirTo, argFile}, core.ModeWrite, argFile, ".", false},
		{"from with dir forces read", []string{dirFrom, argFile, argDir}, core.ModeRead, argFile, argDir, false},
		{"to needs a filename", []string{dirTo}, "", "", "", true},
		{"from needs a filename", []string{dirFrom}, "", "", "", true},
		{"too many positionals", []string{argFile, argDir, "extra"}, "", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, file, dir, err := parseBridgeArgs(tc.args)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.mode, mode)
			assert.Equal(t, tc.file, file)
			assert.Equal(t, tc.dir, dir)
		})
	}
}

func TestStackSummaryNamesTheDeclaredSet(t *testing.T) {
	assert.Equal(t, "(none declared — running all)", stackSummary(nil))
	assert.Equal(t, "node, python", stackSummary([]string{stackNode, stackPython}))
}

func TestPolicySummaryNamesTheOverwriteRule(t *testing.T) {
	assert.Contains(t, policySummary(core.Policy{Marker: true}), "managed")
	assert.Equal(t, "renderer — overwrite-always", policySummary(core.Policy{}))
}

func TestReadMerged(t *testing.T) {
	t.Run("minimal document", func(t *testing.T) {
		dir := t.TempDir()
		resetGlobals(t)
		isolateUserConfig(t, dir)
		writeFixtureProjectfile(t, dir, "")
		pf, err := readMerged(dir)
		require.NoError(t, err)
		require.NotNil(t, pf)
	})
	t.Run("broken yaml", func(t *testing.T) {
		dir := t.TempDir()
		resetGlobals(t)
		isolateUserConfig(t, dir)
		require.NoError(t, os.WriteFile(filepath.Join(dir, pfFileName), []byte("identity: ["), 0o644))
		_, err := readMerged(dir)
		require.Error(t, err)
	})
	t.Run("no document at all", func(t *testing.T) {
		dir := t.TempDir()
		resetGlobals(t)
		isolateUserConfig(t, dir)
		_, err := readMerged(dir)
		require.Error(t, err)
	})
}

func TestBridgeFilenamesListsEveryRegistration(t *testing.T) {
	resetGlobals(t)
	bridge.Register(fakeRenderer{filenamesFile})
	assert.Contains(t, bridgeFilenames(), filenamesFile)
}

func TestPrintListWritesOneNamePerLine(t *testing.T) {
	resetGlobals(t)
	cmd, out := newCmd()
	require.NoError(t, printList(cmd, []string{"one.md", "two.md"}))
	assert.Equal(t, "one.md\ntwo.md\n", out.String())
}

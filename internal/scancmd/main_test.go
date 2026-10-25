// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package scancmd

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/v4/internal/describe"
)

// The re-exec gate hands the child the argv the parent wants Main to answer with.
const (
	helperEnv  = "PF_SCAN_TEST_MAIN"
	helperArgv = "PF_SCAN_TEST_ARGV"
	envTrue    = "1"
	argvSep    = "\x1f"
	mainReturn = "main-returned"
)

// TestMain binds the command once here: a second Main call re-registers every flag and re-adds the root.
func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == envTrue {
		_ = flag.Set("test.run", "^TestScanMainHelper$") // a child with no -test.run would run every test and spawn again
		os.Exit(m.Run())
	}
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

// TestScanMainHelper is the child half: the parent re-execs this binary, so the child's own Main reads an os.Exit.
func TestScanMainHelper(t *testing.T) {
	if os.Getenv(helperEnv) != envTrue {
		t.Skip("the parent drives this test in a re-exec'd child")
	}
	os.Args = []string{scanBin}
	if argv := os.Getenv(helperArgv); argv != "" {
		os.Args = append(os.Args, strings.Split(argv, argvSep)...)
	}
	Main(scanBin)
	fmt.Println(mainReturn)
}

// runAsChild re-execs this test binary so Main runs in the child while the parent reads its exit code and streams.
func runAsChild(t *testing.T, argv ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestScanMainHelper$")
	cmd.Env = append(os.Environ(), helperEnv+"="+envTrue, helperArgv+"="+strings.Join(argv, argvSep))
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "a gated child must exit, not fail to start")
		code = exitErr.ExitCode()
	}
	return out.String(), errOut.String(), code
}

// runCmd executes the bound root with output captured, after the option vars return to their defaults.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	resetScanVars(t)
	var out bytes.Buffer
	scanCmd.SetOut(&out)
	scanCmd.SetArgs(args)
	err := scanCmd.Execute()
	return out.String(), err
}

func TestMainFlagSurface(t *testing.T) {
	assert.Equal(t, scanBin+" [scanner...] [directory]", scanCmd.Use, "Main names the root after the binary it builds")
	for _, name := range []string{"dry-run", "check", "strict", "prune", "force"} {
		assert.NotNil(t, scanCmd.Flags().Lookup(name), name)
	}
	assert.Equal(t, "false", scanCmd.Flags().Lookup("dry-run").DefValue)
	assert.NotNil(t, scanCmd.Flags().ShorthandLookup("n"))
	assert.NotNil(t, scanCmd.Flags().ShorthandLookup("f"))
}

func TestScanRunERunsTheNamedScanners(t *testing.T) {
	dir := fixtureDir(t, forgeTemplateFixture)
	_, err := runCmd(t, scannerForge, dir)
	require.NoError(t, err)
	assert.Len(t, readDoc(t, dir).Links, 1, "the dispatcher reaches the generic phase through RunE")
}

func TestScanRunEOpensThePickerWithoutNames(t *testing.T) {
	dir := fixtureDir(t, minimalFixture)
	_, err := runCmd(t, dir) // a bare directory is no scanner name, so the picker is the only path
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TTY", "no terminal means the picker fails rather than blocking")
}

func TestScanRunEReportsADetectionFailure(t *testing.T) {
	_, err := runCmd(t, scannerForge, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "detect projectfile")
}

func TestMainAnswersTheDescribeProbe(t *testing.T) {
	out, _, code := runAsChild(t, describe.Flag)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, scanCmd.Short)
}

func TestMainExitsZeroOnAScanRun(t *testing.T) {
	dir := fixtureDir(t, forgeTemplateFixture)
	out, _, code := runAsChild(t, scannerForge, dir)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, mainReturn)
	assert.Len(t, readDoc(t, dir).Links, 1)
}

func TestMainExitsOneOnAFailingRun(t *testing.T) {
	_, errOut, code := runAsChild(t, scannerForge, t.TempDir())
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "Error: detect projectfile")
}

// The picker's own error path: a run whose stdin is no terminal answers with the transport error.
func TestRunScanPickerReportsTheTerminalError(t *testing.T) {
	if f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		_ = f.Close()
		t.Skip("a controlling terminal would block on the interactive picker")
	}
	err := runScanPicker(t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TTY")
}

func TestRunStacksPhaseReportsAScanFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root lists a directory with no read permission")
	}
	resetScanVars(t)
	dir := fixtureDir(t, noStackFixture)
	require.NoError(t, os.WriteFile(filepath.Join(dir, goModFile), []byte("module scancase\n"), 0o644))
	require.NoError(t, os.Chmod(dir, 0o111)) // the file opens by name, but the walk cannot list
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	err := runStacksPhase(dir, filepath.Join(dir, pfFileName))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scan:")
}

func TestRunStacksPhaseReportsAWriteFailure(t *testing.T) {
	resetScanVars(t)
	scanPrune = true
	dir := fixtureDir(t, staleStackFixture)
	err := runStacksPhase(dir, dir) // the write target is a directory, so Write refuses it
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported projectfile extension")
}

func TestRunStacksPhaseReportsAnAdditionWriteFailure(t *testing.T) {
	resetScanVars(t)
	dir := markerDir(t, noStackFixture, goModFile)
	err := runStacksPhase(dir, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported projectfile extension")
}

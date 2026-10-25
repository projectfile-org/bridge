// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package initcmd

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
	binName    = "pf-bridge-init"
	helperEnv  = "PF_INIT_TEST_MAIN"
	helperArgv = "PF_INIT_TEST_ARGV"
	envTrue    = "1"
	argvSep    = "\x1f"
	mainReturn = "main-returned"
	pfFile     = "projectfile.yaml"
	ignoreUser = "--ignore-user-config"
	gateTest   = "^TestInitGateHelper$"
	gateRun    = "TestInitGateHelper"

	flagFormat    = "--format"
	flagNonInt    = "--non-interactive"
	flagNamespace = "--namespace"
	flagName      = "--name"
	flagNoScan    = "--no-scan"
)

// Expected values and message fragments, each named once so no literal repeats.
const (
	fxNamespace   = "org.example"
	fxName        = "my-tool"
	formatYAML    = "yaml"
	formatXML     = "xml"
	errMissing    = "missing required fields"
	errFormatBad  = "unsupported format"
	fieldName     = "identity.namespace"
	createdMsg    = "created projectfile.yaml"
	errPrefix     = "Error:"
	aliasScaffold = "scaffold"
)

// TestMain binds the command once here: a second Main call re-registers every flag and re-adds every command.
func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == envTrue {
		_ = flag.Set("test.run", gateTest) // a child with no -test.run would run every test and spawn again
		os.Exit(m.Run())
	}
	// Bind latches every PF_BRIDGE_* default at register time, so a developer's exported OFFLINE or QUIET would otherwise reach the suite.
	for _, env := range []string{
		"PF_BRIDGE_QUIET", "PF_BRIDGE_VERBOSE", "PF_CLI_VERBOSE", "PF_BRIDGE_OFFLINE",
		"PF_BRIDGE_NO_COLOR", "PF_BRIDGE_COLORS", "PF_BRIDGE_SORTED", "PF_BRIDGE_IGNORE_USER_CONFIG",
		"PF_BRIDGE_FAIL_ON", "PF_BRIDGE_TIMEOUT",
	} {
		_ = os.Unsetenv(env)
	}
	argv := os.Args
	os.Args = []string{binName, describe.Flag} // the probe returns before cobra, which would exit on the suite's own flags
	Main(binName)
	os.Args = argv
	os.Exit(m.Run())
}

// TestInitGateHelper is the child half: the parent re-execs this binary, so the child's own Main reads an os.Exit.
func TestInitGateHelper(t *testing.T) {
	if os.Getenv(helperEnv) != envTrue {
		t.Skip("the parent drives this test in a re-exec'd child")
	}
	os.Args = []string{binName}
	if argv := os.Getenv(helperArgv); argv != "" {
		os.Args = append(os.Args, strings.Split(argv, argvSep)...)
	}
	Main(binName)
	fmt.Println(mainReturn)
}

// runAsChild re-execs this test binary so Main runs in the child while the parent reads its exit code and streams.
func runAsChild(t *testing.T, argv ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+gateRun+"$")
	cmd.Env = append(os.Environ(), helperEnv+"="+envTrue, helperArgv+"="+strings.Join(argv, argvSep))
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "a gated child must exit, not fail to start")
		code = exitErr.ExitCode()
	}
	return out.String(), errOut.String(), code
}

// runInit executes the bound root with output captured, after every option var returns to its flag default.
func runInit(t *testing.T, args ...string) (string, error) {
	t.Helper()
	resetInitVars(t)
	var out bytes.Buffer
	initCmd.SetOut(&out)
	initCmd.SetErr(&out)
	initCmd.SetArgs(append([]string{ignoreUser}, args...))
	err := initCmd.Execute()
	return out.String(), err
}

// resetInitVars restores each option var: pflag only rewrites a global when the flag is registered, never when it is parsed.
func resetInitVars(t *testing.T) {
	t.Helper()
	initFormat, initNonInteractive, initNoScan = "", false, false
	initNamespace, initName, initLicense = "", "", ""
	t.Cleanup(func() {
		initFormat, initNonInteractive, initNoScan = "", false, false
		initNamespace, initName, initLicense = "", "", ""
	})
}

// scratchDir makes an empty directory with the given basename inside the test's temp tree.
func scratchDir(t *testing.T, base string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), base)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	return dir
}

func TestMainFlagSurface(t *testing.T) {
	assert.Equal(t, binName+" [directory]", initCmd.Use, "Main names the root after the binary it builds")
	assert.Contains(t, initCmd.Aliases, aliasScaffold, aliasScaffold)
	assert.True(t, initCmd.SilenceUsage, "a failing Execute must not print usage into the gate child's stream")
	for _, name := range []string{"format", "non-interactive", "namespace", "name", "license", "no-scan"} {
		assert.NotNil(t, initCmd.Flags().Lookup(name), name)
	}
	assert.NotNil(t, initCmd.Flags().ShorthandLookup("f"), "the format flag carries its shorthand")
	assert.Equal(t, "", initCmd.Flags().Lookup("format").DefValue, "format")
	assert.Equal(t, "false", initCmd.Flags().Lookup("non-interactive").DefValue, "non-interactive")
	assert.NotNil(t, initCmd.PersistentFlags().Lookup("offline"), "Bind arms the shared rootflags family")
}

func TestInitWritesDocumentFromPositionalDir(t *testing.T) {
	dir := scratchDir(t, fxName)

	out, err := runInit(t, flagNonInt, flagNamespace, fxNamespace, flagName, fxName, flagFormat, formatYAML, flagNoScan, dir)

	require.NoError(t, err, out)
	assert.Contains(t, out, createdMsg, out)
	written := filepath.Join(dir, pfFile)
	_, statErr := os.Stat(written)
	assert.NoError(t, statErr, written)
}

// A run with no positional directory scaffolds the current one, so the test must chdir out of the repository.
func TestInitDefaultsToCurrentDirectory(t *testing.T) {
	dir := scratchDir(t, fxName)
	t.Chdir(dir)

	out, err := runInit(t, flagNonInt, flagNamespace, fxNamespace, flagName, fxName, flagNoScan)

	require.NoError(t, err, out)
	assert.Contains(t, out, createdMsg, out)
	_, statErr := os.Stat(filepath.Join(dir, pfFile))
	assert.NoError(t, statErr, pfFile)
}

func TestInitNonInteractiveReportsMissingFields(t *testing.T) {
	dir := scratchDir(t, fxName)

	_, err := runInit(t, flagNonInt, flagNoScan, dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), errMissing)
	assert.Contains(t, err.Error(), fieldName)
	_, statErr := os.Stat(filepath.Join(dir, pfFile))
	assert.True(t, os.IsNotExist(statErr), "a failed run left no document behind")
}

func TestInitRejectsUnsupportedFormat(t *testing.T) {
	dir := scratchDir(t, fxName)

	_, err := runInit(t, flagNonInt, flagNamespace, fxNamespace, flagName, fxName, flagFormat, formatXML, dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), errFormatBad)
}

// A successful Main returns to its caller, so the gated child reaches its witness line with exit code 0.
func TestMainGateSucceeds(t *testing.T) {
	dir := scratchDir(t, fxName)

	out, _, code := runAsChild(t, ignoreUser, flagNonInt, flagNamespace, fxNamespace, flagName, fxName, flagFormat, formatYAML, flagNoScan, dir)

	assert.Equal(t, 0, code, out)
	assert.Contains(t, out, mainReturn, out)
	_, statErr := os.Stat(filepath.Join(dir, pfFile))
	assert.NoError(t, statErr, pfFile)
}

// A failing Execute exits 1 from Main itself, the only place the exit code is decided.
func TestMainGateExitsOnFailure(t *testing.T) {
	dir := scratchDir(t, fxName)

	out, errOut, code := runAsChild(t, ignoreUser, flagNonInt, flagNoScan, dir)

	assert.Equal(t, 1, code, errOut)
	assert.Contains(t, errOut, errPrefix, errOut)
	assert.Contains(t, errOut, errMissing, errOut)
	assert.NotContains(t, out, mainReturn, "a failed Main must exit, not return")
}

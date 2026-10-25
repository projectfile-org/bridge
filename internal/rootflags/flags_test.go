// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package rootflags

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/netfetch"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
)

const (
	testUse     = "pf-bridge-test"
	testVer     = "1.2.3"
	colourBad   = "bogus"
	levelError  = "error"
	levelWarn   = "warning"
	envTrue     = "1"
	flagQuiet   = "--quiet"
	flagVerbose = "--verbose"
	flagOffline = "--offline"
	flagTimeout = "--timeout"
	flagColours = "--colors"
	flagHelp    = "--help"
	failOnEnv   = "PF_BRIDGE_FAIL_ON"
	coloursEnv  = "PF_BRIDGE_COLORS"
	noColourEnv = "PF_BRIDGE_NO_COLOR"
	timeoutEnv  = "PF_BRIDGE_TIMEOUT"
	doneMessage = "bridge-runner-done"
	helpTail    = "Every --flag above defaults"
)

// TestMain drops every variable Bind latches, so a developer shell cannot reach the suite through its own export.
func TestMain(m *testing.M) {
	for _, env := range []string{
		"PF_BRIDGE_QUIET", "PF_BRIDGE_VERBOSE", "PF_CLI_VERBOSE", "PF_BRIDGE_OFFLINE",
		"PF_BRIDGE_NO_COLOR", coloursEnv, "PF_BRIDGE_SORTED", "PF_BRIDGE_IGNORE_USER_CONFIG",
		failOnEnv, timeoutEnv,
	} {
		_ = os.Unsetenv(env)
	}
	os.Exit(m.Run())
}

// resetFlags returns every option var to its flag default, since pflag rewrites a global when the flag is registered, never when it is parsed.
func resetFlags(t *testing.T) {
	t.Helper()
	restoreDefaults()
	t.Cleanup(restoreDefaults)
}

func restoreDefaults() {
	quietFlag, verboseFlag = false, false
	ignoreUserConfigFlag, offlineFlag, sortedFlag = false, false, false
	failOnFlag = failOnFlagValue{level: projectfile.FailOnError}
	colorsFlag, timeoutFlag = genlog.ColorAuto, netfetch.DefaultTimeout
	_ = genlog.SetColor(genlog.ColorAuto)
	genlog.SetOutput(os.Stderr)
	genlog.SetResultOutput(os.Stdout)
	genlog.SetQuiet(false)
	genlog.SetVerbose(false)
	userconfig.SetIgnored(false)
	projectfile.SetYAMLOutputSorted(false)
	netfetch.SetTimeout(netfetch.DefaultTimeout)
}

// newRoot binds a fresh command with both output streams captured, so a case names only its argv.
func newRoot(t *testing.T, args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	resetFlags(t)
	root := &cobra.Command{Use: testUse, Version: testVer, Run: func(*cobra.Command, []string) {}}
	Bind(root)
	out, trace := &bytes.Buffer{}, &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(out)
	genlog.SetOutput(trace)
	root.SetArgs(args)
	return root, out, trace
}

func TestVersionShortAndLongFlags(t *testing.T) {
	for _, arg := range []string{"-V", "--version"} {
		root := &cobra.Command{Use: testUse, Version: testVer, Run: func(*cobra.Command, []string) {}}
		resetFlags(t)
		Bind(root)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{arg})
		assert.NoError(t, root.Execute(), arg)
		assert.Equal(t, testUse+" version "+testVer+"\n", out.String(), arg)
	}
}

func TestVerboseKeepsShortV(t *testing.T) {
	root := &cobra.Command{Use: testUse, Version: testVer, Run: func(*cobra.Command, []string) {}}
	resetFlags(t)
	Bind(root)
	assert.Equal(t, "verbose", root.PersistentFlags().ShorthandLookup("v").Name)
	assert.Equal(t, "version", root.Flags().ShorthandLookup("V").Name)
}

// TestReadOptsCarriesOfflineAndFailOn pins that the reader options carry the two values a projectfile read honours.
func TestReadOptsCarriesOfflineAndFailOn(t *testing.T) {
	resetFlags(t)
	offlineFlag = true
	failOnFlag.level = projectfile.FailOnWarning

	assert.True(t, Offline(), "an explicit --offline reaches the reader")
	opts := ReadOpts()
	assert.True(t, opts.Offline, "the reader sees offline")
	assert.Equal(t, projectfile.FailOnWarning, opts.FailOn, "the reader sees the severity")
}

// TestEnvDefaultsReachTheFlags pins that Bind latches each variable, and that NO_COLOR outranks COLORS.
func TestEnvDefaultsReachTheFlags(t *testing.T) {
	resetFlags(t)
	t.Setenv(failOnEnv, levelWarn)
	t.Setenv(coloursEnv, genlog.ColorAlways)
	t.Setenv(noColourEnv, envTrue)
	t.Setenv(timeoutEnv, "7s")

	root, _, _ := newRoot(t)

	assert.Equal(t, projectfile.FailOnWarning, failOnFlag.level, failOnEnv+" sets the include-failure level")
	assert.Equal(t, genlog.ColorNever, colorsFlag, noColourEnv+" outranks "+coloursEnv)
	assert.Equal(t, genlog.ColorNever, root.PersistentFlags().Lookup("colors").DefValue,
		coloursEnv+" survives registration as the flag default")
	assert.Equal(t, 7*time.Second, timeoutFlag, timeoutEnv+" bounds one network attempt")
}

// TestPreRunPushesEveryGlobal pins that one run latches the parsed flags into the core packages it drives.
func TestPreRunPushesEveryGlobal(t *testing.T) {
	root, out, trace := newRoot(t, flagVerbose, flagOffline, flagTimeout, "3s")
	require.NoError(t, root.Execute())

	assert.Equal(t, 3*time.Second, netfetch.Timeout(), flagTimeout+" bounds every attempt")
	assert.Contains(t, trace.String(), "offline mode", flagOffline+" logs the mode it entered")
	assert.Contains(t, trace.String(), "timeout", "the debug line reports the latency budget it set")
	genlog.Success(doneMessage)
	assert.Contains(t, out.String(), doneMessage, "results reach the writer the command named")
}

// TestPreRunQuietSilencesTheTrace pins that --quiet suppresses the operational log lines even under --verbose.
func TestPreRunQuietSilencesTheTrace(t *testing.T) {
	root, _, trace := newRoot(t, flagVerbose, flagQuiet, flagOffline)
	require.NoError(t, root.Execute())

	assert.Empty(t, trace.String(), flagQuiet+" suppresses the operational log lines")
}

// TestPreRunRefusesUnknownColours pins that a bad colour stops the run before any global is latched.
func TestPreRunRefusesUnknownColours(t *testing.T) {
	root, _, _ := newRoot(t, flagColours+"="+colourBad, flagTimeout+"=9s")
	before := netfetch.Timeout()

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown colour mode", colourBad+" is not a colour mode")
	assert.Equal(t, before, netfetch.Timeout(), "the timeout latch never runs once colours are refused")
}

// TestHelpFuncWarnsOnUnknownColours pins that help still prints on a bad colour and warns instead of failing.
func TestHelpFuncWarnsOnUnknownColours(t *testing.T) {
	root, out, trace := newRoot(t, flagHelp, flagColours+"="+colourBad)
	require.NoError(t, root.Execute())

	assert.Contains(t, trace.String(), "unknown colour mode", "help refuses the colour and warns")
	assert.Contains(t, out.String(), helpTail, "the help template carries the env-var note")
}

// TestFailOnFlagValue pins the accepted spellings and that a refused value leaves the level it found.
func TestFailOnFlagValue(t *testing.T) {
	defaultLevel := failOnFlagValue{}
	assert.Equal(t, levelError, defaultLevel.String(), "the default severity spells error")
	warnLevel := failOnFlagValue{level: projectfile.FailOnWarning}
	assert.Equal(t, levelWarn, warnLevel.String(), "warning names the strict level")
	assert.Equal(t, "string", defaultLevel.Type(), "the flag reports as a string so a value parses")

	for _, tc := range []struct{ in, want string }{
		{in: "", want: levelError},
		{in: levelError, want: levelError},
		{in: levelWarn, want: levelWarn},
	} {
		var value failOnFlagValue
		require.NoError(t, value.Set(tc.in), tc.in)
		assert.Equal(t, tc.want, value.String(), tc.in)
	}

	refused := failOnFlagValue{level: projectfile.FailOnWarning}
	err := refused.Set(colourBad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value")
	assert.Equal(t, levelWarn, refused.String(), "a refused value leaves the level it found")
}

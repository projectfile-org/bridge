// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/commands"
	"projectfile.org/projectfile/bridge/v4/internal/describe"
)

// The re-exec gate: the parent hands the child the dispatcher argv to answer with.
const (
	helperEnv  = "PF_BRIDGE_TEST_MAIN"
	helperArgv = "PF_BRIDGE_TEST_ARGV"
	envTrue    = "1"
	argvSep    = "\x1f"
	stubName   = "zz-stub"
	missName   = "nosuchfile"
	stubPrint  = "stub-linked"
	mainReturn = "main-returned"
	gateSleep  = "sleep"
	probeWait  = 3 * time.Second

	cmdCompletion = "completion"
	npmDesc       = "package.json — two-way sync"
)

// TestMain pins a gated child to the helper, so a re-exec'd test binary never re-runs the suite.
func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == envTrue {
		_ = flag.Set("test.run", "^TestMainGateHelper$") // a child with no -test.run would run every test and spawn again
	}
	os.Exit(m.Run())
}

// TestMainGateHelper is the child half of every main() case: the parent re-execs
// this binary with the gate set and the child answers as pf-bridge would.
func TestMainGateHelper(t *testing.T) {
	if os.Getenv(helperEnv) != envTrue {
		t.Skip("the parent drives this test in a re-exec'd child")
	}
	if describe.Requested(os.Args[1:]) {
		switch os.Getenv(helperArgv) {
		case gateSleep:
			time.Sleep(probeWait) // outlives the describe deadline on purpose
			return
		case missName:
			os.Exit(1) // a child that fails the probe
		}
		fmt.Println(describe.Flag) // the probe a dispatcher sends every child
		return
	}
	// A gated main() links one inert stub and no real bridge, so the dispatch
	// branch never touches the repository the suite itself runs in.
	commands.Mains = map[string]func(string){stubName: func(string) { fmt.Println(stubPrint) }}
	os.Args = []string{prefix + stubName}
	if argv := os.Getenv(helperArgv); argv != "" {
		os.Args = append(os.Args, strings.Split(argv, argvSep)...)
	}
	t.Setenv(helperArgv, stubName) // grandchildren answer as the inert stub, or the fan-out respawns itself forever
	main()
	fmt.Println(mainReturn)
}

// runAsChild re-execs this test binary so main() runs in the child while the
// parent reads its exit code and streams: the only way to observe an os.Exit.
func runAsChild(t *testing.T, argv ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainGateHelper$")
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

// childEnv sets the persona an in-process spawn hands its children: a stub child
// that exits 0, or one that fails.
func childEnv(t *testing.T, code int) {
	t.Helper()
	t.Setenv(helperEnv, envTrue)
	t.Setenv(helperArgv, stubName)
	if code != 0 {
		t.Setenv(helperArgv, missName)
	}
}

// withLinkedSubcommands replaces the linked subcommands for one test, so a
// fan-out spawns a known, inert child instead of every real bridge.
func withLinkedSubcommands(t *testing.T, names ...string) {
	t.Helper()
	saved := commands.Mains
	t.Cleanup(func() { commands.Mains = saved })
	commands.Mains = map[string]func(string){}
	for _, n := range names {
		commands.Mains[n] = func(string) { fmt.Println(stubPrint) }
	}
}

// declaredDir writes a projectfile declaring the given manifest rows.
func declaredDir(t *testing.T, rows map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	body := &strings.Builder{}
	body.WriteString("identity:\n  name: stub\norg:\n  projectfile:\n    ci:\n      tools:\n")
	for _, name := range slices.Sorted(maps.Keys(rows)) {
		fmt.Fprintf(body, "        %s:\n          run: %s\n", name, rows[name])
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte(body.String()), 0o600))
	return dir
}

func TestMainWithNoArgsPrintsUsage(t *testing.T) {
	out, _, code := runAsChild(t)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "keep every project file in sync")
	assert.Contains(t, out, "Shared flags")
	assert.Contains(t, out, "pf-bridge <bridge> [args]")
	assert.Contains(t, out, "Help per bridge")
}

func TestMainHelpAndVersionFlags(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		out, _, code := runAsChild(t, arg)
		assert.Equal(t, 0, code, arg)
		assert.Contains(t, out, "Usage:", arg)
	}
	for _, arg := range []string{"-V", "--version", "version"} {
		out, _, code := runAsChild(t, arg)
		assert.Equal(t, 0, code, arg)
		assert.Contains(t, out, "pf-bridge version", arg)
	}
}

func TestMainListFlagGroupsByCategory(t *testing.T) {
	out, _, code := runAsChild(t, "--list")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "Other bridges:")
	assert.Contains(t, out, describe.Flag)
}

func TestMainCompletionShells(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{cmdCompletion}, "_pf_bridge() { local cmds="},
		{[]string{cmdCompletion, "bash"}, "complete -F _pf_bridge"},
		{[]string{cmdCompletion, "zsh"}, "#compdef pf-bridge"},
		{[]string{cmdCompletion, "fish"}, "complete -c pf-bridge"},
	}
	for _, tc := range cases {
		out, _, code := runAsChild(t, tc.args...)
		assert.Equal(t, 0, code, tc.args)
		assert.Contains(t, out, tc.want, tc.args)
	}
}

func TestMainRejectsUnknownCompletion(t *testing.T) {
	_, errOut, code := runAsChild(t, cmdCompletion, "nosuchshell")
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "unsupported shell")
}

func TestMainRejectsLeadingFlag(t *testing.T) {
	_, errOut, code := runAsChild(t, noDiffFlag)
	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "must follow the command")
}

func TestMainExitsOneOnUnknownCommand(t *testing.T) {
	_, errOut, code := runAsChild(t, missName)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "unknown command")
}

func TestMainExitsOneOnBadFanOutGrammar(t *testing.T) {
	_, errOut, code := runAsChild(t, dirTo)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, `expected "to" all`)
}

func TestMainFanOutAndCheckVerbsSucceed(t *testing.T) {
	for _, arg := range []string{cmdAll, cmdCheck} {
		out, _, code := runAsChild(t, arg)
		assert.Equal(t, 0, code, arg)
		assert.Contains(t, out, mainReturn, arg)
	}
}

func TestMainDispatchesLinkedCommand(t *testing.T) {
	out, _, code := runAsChild(t, stubName)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, stubPrint)
	assert.Contains(t, out, mainReturn)
}

func TestRunAll(t *testing.T) {
	err := runAll([]string{dirTo})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `expected "to" all`)

	withLinkedSubcommands(t, stubName)
	childEnv(t, 0)
	assert.NoError(t, runAll([]string{dirTo, cmdAll}))
	assert.NoError(t, runAll([]string{cmdAll, noDiffFlag}))
}

func TestFanOut(t *testing.T) {
	withLinkedSubcommands(t, stubName)
	childEnv(t, 0)

	assert.NoError(t, fanout("", nil, []string{flagCheck}), "the empty set sweeps what is discovered")
	assert.NoError(t, fanout(dirTo, []string{stubName}, nil))

	childEnv(t, 0)
	err := fanout("", []string{forgeName}, nil)
	require.Error(t, err, "a tool binary is not a file bridge")
	assert.Contains(t, err.Error(), "no pf-bridge-*")
}

func TestRunChildren(t *testing.T) {
	require.ErrorContains(t, runChildren(nil), "no pf-bridge-*")

	withLinkedSubcommands(t, stubName)
	childEnv(t, 0)
	assert.NoError(t, runChildren([]child{{name: stubName, args: []string{cmdAll}}}))

	childEnv(t, 1)
	assert.ErrorContains(t, runChildren([]child{{name: stubName, args: []string{cmdAll}}}),
		"1/1 bridge binaries failed")
}

func TestWithDescriptions(t *testing.T) {
	childEnv(t, 0)
	entries := withDescriptions([]string{stubName})
	require.Len(t, entries, 1)
	assert.Equal(t, stubName, entries[0].name)
	assert.Equal(t, describe.Flag, entries[0].desc)

	t.Setenv(helperArgv, gateSleep)
	entries = withDescriptions([]string{stubName, npmName})
	require.Len(t, entries, 2)
	assert.Empty(t, entries[0].desc, "a child past the deadline keeps an empty description")
	assert.Equal(t, npmName, entries[1].name, "a spent deadline stops the walk")
}

func TestProbeDescribe(t *testing.T) {
	childEnv(t, 0)
	assert.Equal(t, describe.Flag, probeDescribe(context.Background(), stubName))

	childEnv(t, 1)
	assert.Empty(t, probeDescribe(context.Background(), stubName))
}

func TestRunCheckAll(t *testing.T) {
	dir := declaredDir(t, map[string]string{
		"pf-bridge-scan-check":   "pf-bridge scan all --check",
		"pf-bridge-absent-check": "pf-bridge " + missName + " --check",
		"pf-bridge-stub-check":   "pf-bridge " + stubName + " --check",
	})
	withLinkedSubcommands(t, stubName)
	childEnv(t, 0)

	t.Run("the declared set", func(t *testing.T) {
		t.Chdir(dir)
		assert.NoError(t, runCheckAll(nil))
	})
	t.Run("no manifest falls back to the path sweep", func(t *testing.T) {
		t.Chdir(t.TempDir())
		assert.NoError(t, runCheckAll([]string{flagCheck}))
	})
	t.Run("names and --all narrow it", func(t *testing.T) {
		for _, args := range [][]string{{stubName}, {flagAll}, {npmName, flagCheck}} {
			assert.NoError(t, runCheckAll(args), args)
		}
	})
}

func TestDeclaredChildren(t *testing.T) {
	dir := declaredDir(t, map[string]string{
		"pf-bridge-scan-check":   "pf-bridge scan all --check",
		"pf-bridge-absent-check": "pf-bridge " + missName + " --check",
		"pf-bridge-stub-check":   "pf-bridge " + stubName + " --check",
	})
	withLinkedSubcommands(t, stubName)
	t.Chdir(dir)

	children := declaredChildren([]string{flagCheck})
	require.Len(t, children, 1, "a tool binary and a missing bridge are warned and skipped")
	assert.Equal(t, stubName, children[0].name)
	assert.Equal(t, []string{flagCheck}, children[0].args)
}

func TestReadOpts(t *testing.T) {
	assert.Equal(t, projectfile.ReadOptions{FailOn: projectfile.FailOnError},
		readOpts([]string{flagCheck}))
	assert.Equal(t, projectfile.ReadOptions{FailOn: projectfile.FailOnError, Offline: true},
		readOpts([]string{flagOffline, flagCheck}))
}

func TestModeTag(t *testing.T) {
	assert.Equal(t, "[rw]", modeTag(npmDesc))
	assert.Equal(t, "[ro]", modeTag("README.md — one-way render"))
	assert.Equal(t, "[ro]", modeTag("x — sync and render"))
	assert.Equal(t, "    ", modeTag("no capability stated"))
}

func TestRenderGrouped(t *testing.T) {
	entries := []bridgeEntry{
		{name: forgeName, desc: "push identity"},
		{name: npmName, desc: npmDesc},
		{name: "readme", desc: "README.md — one-way render"},
		{name: "zz-unknown", desc: "no category"},
	}

	var out strings.Builder
	renderGrouped(&out, entries, false)
	assert.Contains(t, out.String(), "Tools:")
	assert.Contains(t, out.String(), catManifests+":")
	assert.Contains(t, out.String(), "Other bridges:")
	assert.NotContains(t, out.String(), catCite+":", "an empty category is skipped")

	renderGrouped(&out, entries[:1], false)
	assert.Contains(t, out.String(), "Tools:")

	renderGrouped(&out, []bridgeEntry{{name: npmName}}, true)
	assert.Contains(t, out.String(), "\x1b[1m"+npmName+"\x1b[22m")
}

func TestRenderGroupedWithoutTools(t *testing.T) {
	var out strings.Builder
	renderGrouped(&out, []bridgeEntry{{name: npmName, desc: npmDesc}}, false)
	assert.NotContains(t, out.String(), "Tools:")
	assert.Contains(t, out.String(), catManifests+":")
}

func TestUseColor(t *testing.T) {
	plain := colorFile(t)
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("NO_COLOR", "")
	assert.False(t, useColor(plain), "a plain file carries no colour profile")

	forced := colorFile(t)
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	assert.True(t, useColor(forced), "a forced profile answers at least ANSI")
}

func TestUsageAndListWithoutBridges(t *testing.T) {
	withLinkedSubcommands(t)

	usageF := colorFile(t)
	usage(usageF)
	assert.Contains(t, readBody(t, usageF), "Installed: (none found on PATH)")

	listF := colorFile(t)
	listBridges(listF)
	assert.Contains(t, readBody(t, listF), "(no pf-bridge-* binaries found on PATH)")
}

// colorFile returns a fresh temp file, since genlog profiles per *os.File.
func colorFile(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "color")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// readBody drains f from the start, since usage and listBridges write into it.
func readBody(t *testing.T, f *os.File) string {
	t.Helper()
	_, err := f.Seek(0, 0)
	require.NoError(t, err)
	body, err := io.ReadAll(f)
	require.NoError(t, err)
	return string(body)
}

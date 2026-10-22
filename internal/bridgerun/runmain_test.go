// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package bridgerun

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/describe"
	"projectfile.org/projectfile/bridge/v4/internal/warn"
)

// Each fake claims a unique filename: the registry is process-global and Register panics on a duplicate.
const (
	syncFile      = "zz-sync.json"
	outFile       = "zz-out.md"
	bareFile      = "zz-bare.md"
	reqFile       = "zz-required.json"
	noNameFile    = "zz-noname.json"
	batchFile     = "zz-batch.json"
	driftFile     = "zz-drift.json"
	strictFile    = "zz-strict.json"
	previewFile   = "zz-preview.json"
	allFile       = "zz-all.json"
	stackFileA    = "zz-stack-a.json"
	stackFileB    = "zz-stack-b.json"
	failFile      = "zz-fail.json"
	describeFile  = "zz-describe.md"
	listFile      = "zz-list.md"
	filenamesFile = "zz-filenames.md"
	renderBody    = "rendered body\n"
	testBinary    = "pf-bridge-bridgerun"
	helperEnv     = "PF_BRIDGE_TEST_MAIN"
	callRead      = "read:"
	callWrite     = "write:"
	missingHint   = "the project name"
	brokenYAML    = "identity: ["
)

// runLog records the driver calls a batch makes, in order.
type runLog struct{ calls []string }

func (l *runLog) add(call string) { l.calls = append(l.calls, call) }

func (l *runLog) has(call string) bool { return slices.Contains(l.calls, call) }

// bareBridge implements only the identity surface, so the dispatcher has no capability to branch on.
type bareBridge struct{ filename string }

func (b bareBridge) Name() string      { return b.filename }
func (b bareBridge) Filename() string  { return b.filename }
func (b bareBridge) Aliases() []string { return nil }
func (b bareBridge) Labels() (string, string) {
	return b.filename, "projectfile"
}

func (b bareBridge) FullPath(dir string, _ *projectfile.Document) string {
	return dir + "/" + b.filename
}
func (b bareBridge) Exists(string) bool  { return false }
func (b bareBridge) Policy() core.Policy { return core.Policy{} }

// outRenderer renders one fixed file so the write path can assert on disk.
type outRenderer struct{ fakeRenderer }

func (r outRenderer) Render(*projectfile.Document, core.Options) (core.Output, error) {
	return core.Output{Files: map[string][]byte{r.filename: []byte(renderBody)}}, nil
}

// recordSyncer is a two-way bridge that records every driver call; drift makes BuildMappers report one moved field.
type recordSyncer struct {
	fakeRenderer
	log     *runLog
	readErr error
	drift   bool
}

func (s recordSyncer) Exists(string) bool { return true }

func (s recordSyncer) NewEmpty() any { return map[string]any{} }

func (s recordSyncer) Clone(doc any) any { return doc }

func (s recordSyncer) Read(string) (any, error) {
	s.log.add(callRead + s.filename)
	if s.readErr != nil {
		return nil, s.readErr
	}
	return map[string]any{}, nil
}

func (s recordSyncer) Write(string, any) error {
	s.log.add(callWrite + s.filename)
	return nil
}

func (s recordSyncer) BuildMappers(any, *projectfile.Document) core.MapperList {
	if !s.drift {
		return nil
	}
	return core.MapperList{{
		ExtKey: "name", PFKey: "identity.name",
		FromPF: func(bool) string { return renderBody },
	}}
}

// stackSyncer runs only when the project declares node.
type stackSyncer struct{ recordSyncer }

func (s stackSyncer) Stacks() []string { return []string{stackNode} }

// requiredSyncer reports one missing field, which a non-interactive run skips.
type requiredSyncer struct{ recordSyncer }

func (s requiredSyncer) RequiredFields(*projectfile.Document) []core.Missing {
	return []core.Missing{{Field: "identity.name", Hint: missingHint}}
}

// withArgs swaps os.Args for the run and restores it.
func withArgs(t *testing.T, argv ...string) {
	t.Helper()
	saved := os.Args
	os.Args = argv
	t.Cleanup(func() { os.Args = saved })
}

// captureStdout swaps os.Stdout for a pipe, which the returned closure drains synchronously once the run has finished.
func captureStdout(t *testing.T) func() []byte {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	saved := os.Stdout
	os.Stdout = w
	return func() []byte {
		os.Stdout = saved
		require.NoError(t, w.Close())
		body, _ := io.ReadAll(r)
		_ = r.Close()
		return body
	}
}

// projectDir writes the fixture project into a fresh temp dir and returns it.
func projectDir(t *testing.T, technologies string) string {
	t.Helper()
	dir := t.TempDir()
	resetGlobals(t)
	isolateUserConfig(t, dir)
	writeFixtureProjectfile(t, dir, technologies)
	return dir
}

func TestRunBridgeSyncerPath(t *testing.T) {
	dir := projectDir(t, "")
	log := &runLog{}
	bridge.Register(recordSyncer{fakeRenderer{syncFile}, log, nil, false})
	bridgeDryRun = true
	cmd, out := newCmd()
	require.NoError(t, runBridge(cmd, []string{syncFile, dir}))
	assert.True(t, log.has(callRead+syncFile))
	assert.False(t, log.has(callWrite+syncFile))
	assert.Empty(t, out.String())
}

func TestRunBridgeRendererWritesFile(t *testing.T) {
	dir := projectDir(t, "")
	bridge.Register(outRenderer{fakeRenderer{outFile}})
	bridgeCreateAll = true
	cmd, _ := newCmd()
	require.NoError(t, runBridge(cmd, []string{dirTo, outFile, dir}))
	got, err := os.ReadFile(filepath.Join(dir, outFile))
	require.NoError(t, err)
	assert.Equal(t, renderBody, string(got))
}

func TestRunBridgeRendererRejectsRead(t *testing.T) {
	dir := projectDir(t, "")
	cmd, _ := newCmd()
	err := runBridge(cmd, []string{dirFrom, outFile, dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write-only")
}

func TestRunBridgeDetectPathFails(t *testing.T) {
	dir := t.TempDir()
	resetGlobals(t)
	isolateUserConfig(t, dir)
	cmd, _ := newCmd()
	err := runBridge(cmd, []string{syncFile, dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "detect projectfile")
}

func TestRunBridgeRequiredFieldsSkippedWithoutTTY(t *testing.T) {
	dir := projectDir(t, "")
	log := &runLog{}
	bridge.Register(requiredSyncer{recordSyncer{fakeRenderer{reqFile}, log, nil, false}})
	noTTY(t)
	bridgeDryRun = true
	cmd, _ := newCmd()
	require.NoError(t, runBridge(cmd, []string{reqFile, dir}))
	assert.True(t, log.has(callRead+reqFile))
}

func TestRunBridgeNoNameNonInteractive(t *testing.T) {
	bridge.Register(recordSyncer{fakeRenderer{noNameFile}, &runLog{}, nil, false})
	noTTY(t)
	cmd, out := newCmd()
	require.NoError(t, runBridge(cmd, nil))
	assert.Contains(t, out.String(), noNameFile)

	resetGlobals(t)
	noTTY(t)
	bridgeForce = true
	cmd, _ = newCmd()
	err := runBridge(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none named")
}

func TestRunBridgeNoNameBatchesUnderCheck(t *testing.T) {
	dir := projectDir(t, "")
	// The batch inherits the positional dir, which is "." when none is named, so the test runs from the fixture directory.
	t.Chdir(dir)
	log := &runLog{}
	bridge.Register(recordSyncer{fakeRenderer{batchFile}, log, nil, false})
	noTTY(t)
	bridgeCheck = true
	bridgeDryRun = true
	bridgeCreateAll = true
	cmd, _ := newCmd()
	require.NoError(t, runBridge(cmd, nil))
	assert.True(t, log.has(callRead+batchFile))
}

func TestRunBridgeCheckReportsDrift(t *testing.T) {
	dir := projectDir(t, "")
	bridge.Register(recordSyncer{fakeRenderer{driftFile}, &runLog{}, nil, true})
	cmd, _ := newCmd()
	bridgeCheck = true
	require.NoError(t, runBridge(cmd, []string{driftFile, dir}))
	assert.Positive(t, warn.Count())
}

func TestRunBridgeCheckDriftFailsUnderFailOnDrift(t *testing.T) {
	dir := projectDir(t, "")
	bridge.Register(recordSyncer{fakeRenderer{strictFile}, &runLog{}, nil, true})
	t.Setenv(EnvFailOnDrift, envTrue)
	cmd, _ := newCmd()
	bridgeCheck = true
	err := runBridge(cmd, []string{strictFile, dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drifted from the projectfile")
}

func TestRunBridgePreviewWarnsOnSyncer(t *testing.T) {
	dir := projectDir(t, "")
	log := &runLog{}
	bridge.Register(recordSyncer{fakeRenderer{previewFile}, log, nil, false})
	cmd, _ := newCmd()
	bridgePreview = true
	// A syncer has no rendered bytes, so --preview degrades to the dry-run summary and warns through genlog, not the ledger.
	require.NoError(t, runBridge(cmd, []string{previewFile, dir}))
	assert.True(t, log.has(callRead+previewFile))
	assert.False(t, log.has(callWrite+previewFile))
}

func TestRunAllBridges(t *testing.T) {
	// The batch runs every registered bridge, so a deliberately broken one registers after these subtests.
	t.Run("runs every registered bridge", func(t *testing.T) {
		dir := projectDir(t, "")
		log := &runLog{}
		bridge.Register(recordSyncer{fakeRenderer{allFile}, log, nil, false})
		bridgeDryRun = true
		bridgeCreateAll = true
		cmd, _ := newCmd()
		require.NoError(t, runAllBridges(core.ModeSync, dir, cmd))
		assert.True(t, log.has(callRead+allFile))
	})
	t.Run("stack filter drops a non-matching bridge", func(t *testing.T) {
		cfg := t.TempDir()
		resetGlobals(t)
		isolateUserConfig(t, cfg)
		missDir := t.TempDir()
		writeFixtureProjectfile(t, missDir, stackPython)
		matchDir := t.TempDir()
		writeFixtureProjectfile(t, matchDir, stackNode)
		log := &runLog{}
		bridge.Register(stackSyncer{recordSyncer{fakeRenderer{stackFileA}, log, nil, false}})
		bridge.Register(stackSyncer{recordSyncer{fakeRenderer{stackFileB}, log, nil, false}})
		bridgeDryRun = true
		bridgeCreateAll = true
		cmd, _ := newCmd()
		require.NoError(t, runAllBridges(core.ModeSync, missDir, cmd))
		assert.False(t, log.has(callRead+stackFileA))
		assert.False(t, log.has(callRead+stackFileB))
		require.NoError(t, runAllBridges(core.ModeSync, matchDir, cmd))
		assert.True(t, log.has(callRead+stackFileA))
		assert.True(t, log.has(callRead+stackFileB))
	})
	t.Run("one failure is collected and summarised", func(t *testing.T) {
		dir := projectDir(t, "")
		bridge.Register(recordSyncer{fakeRenderer{failFile}, &runLog{}, errors.New("boom"), false})
		bridgeDryRun = true
		cmd, _ := newCmd()
		err := runAllBridges(core.ModeSync, dir, cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), failFile)
		assert.Positive(t, warn.Count())
	})
	t.Run("detect failure names the missing document", func(t *testing.T) {
		resetGlobals(t)
		cmd, _ := newCmd()
		err := runAllBridges(core.ModeSync, t.TempDir(), cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "detect projectfile")
	})
	t.Run("read failure propagates", func(t *testing.T) {
		dir := t.TempDir()
		resetGlobals(t)
		isolateUserConfig(t, dir)
		require.NoError(t, os.WriteFile(filepath.Join(dir, pfFileName), []byte(brokenYAML), 0o644))
		cmd, _ := newCmd()
		err := runAllBridges(core.ModeSync, dir, cmd)
		require.Error(t, err)
	})
}

func TestRunAllBridgeOne(t *testing.T) {
	t.Run("renderer under read direction", func(t *testing.T) {
		dir := projectDir(t, "")
		pfPath, err := projectfile.DetectPath(dir)
		require.NoError(t, err)
		cmd, _ := newCmd()
		err = runAllBridgeOne(fakeRenderer{outFile}, core.ModeRead, dir, pfPath, cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "write-only")
	})
	t.Run("read failure propagates", func(t *testing.T) {
		dir := t.TempDir()
		resetGlobals(t)
		isolateUserConfig(t, dir)
		cmd, _ := newCmd()
		err := runAllBridgeOne(recordSyncer{fakeRenderer{syncFile}, &runLog{}, nil, false},
			core.ModeSync, dir, filepath.Join(dir, pfFileName), cmd)
		require.Error(t, err)
	})
	t.Run("unimplemented bridge fails its row", func(t *testing.T) {
		dir := projectDir(t, "")
		pfPath, err := projectfile.DetectPath(dir)
		require.NoError(t, err)
		cmd, _ := newCmd()
		err = runAllBridgeOne(bareBridge{bareFile}, core.ModeSync, dir, pfPath, cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "implements neither")
	})
}

// TestRunBridgeRefusesUnknownAndUncapable registers a deliberately broken bridge, so it runs after every batch test expecting a clean sweep.
func TestRunBridgeRefusesUnknownAndUncapable(t *testing.T) {
	dir := projectDir(t, "")
	cmd, _ := newCmd()
	err := runBridge(cmd, []string{"nope.json", dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown bridge filename")

	err = runBridge(cmd, []string{syncFile, argDir, "extra"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many positional args")

	bridge.Register(bareBridge{bareFile})
	err = runBridge(cmd, []string{bareFile, dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "implements neither")
}

func TestRunBridgeLockedPropagatesReadFailure(t *testing.T) {
	dir := t.TempDir()
	resetGlobals(t)
	isolateUserConfig(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, pfFileName), []byte(brokenYAML), 0o644))
	pfPath, err := projectfile.DetectPath(dir)
	require.NoError(t, err)
	cmd, _ := newCmd()
	err = runBridgeLocked(bareBridge{bareFile}, core.ModeSync, dir, pfPath, cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read projectfile")
}

func TestMainAnswersDescribeProbe(t *testing.T) {
	bridge.Register(fakeRenderer{describeFile})
	withArgs(t, testBinary, describe.Flag)
	done := captureStdout(t)
	Main(testBinary)
	got := string(done())
	assert.Contains(t, got, describeFile)
}

func TestMainListsRegisteredBridges(t *testing.T) {
	bridge.Register(fakeRenderer{listFile})
	withArgs(t, testBinary, "--list")
	done := captureStdout(t)
	Main(testBinary)
	got := string(done())
	assert.Contains(t, got, listFile)
}

// TestMainEnvFlagsPrimeTheGlobals pins the flag wiring: every PF_BRIDGE_* env var is read before registration, but BoolVarP then resets each global to its default, so only an explicit CLI flag reaches a run.
func TestMainEnvFlagsPrimeTheGlobals(t *testing.T) {
	for _, env := range []string{
		"PF_BRIDGE_FORCE", "PF_BRIDGE_DRY_RUN", "PF_BRIDGE_PREVIEW",
		"PF_BRIDGE_CREATE_ALL", "PF_BRIDGE_CHECK", "PF_BRIDGE_NO_DIFF",
		"PF_BRIDGE_REUSE_CANONICAL",
	} {
		t.Setenv(env, envTrue)
	}
	t.Setenv(EnvFailOnDrift, envTrue)
	withArgs(t, testBinary, "--list", "--force", "--check")
	done := captureStdout(t)
	Main(testBinary)
	got := string(done())
	assert.True(t, bridgeForce)
	assert.True(t, bridgeCheck)
	assert.Contains(t, got, listFile)
}

// TestMainExitsNonZeroOnFailure re-execs the test binary: the child runs a failing command in-process, the only way to read an os.Exit code.
func TestMainExitsNonZeroOnFailure(t *testing.T) {
	if os.Getenv(helperEnv) == envTrue {
		os.Args = []string{testBinary, "no-such-bridge-file"}
		Main(testBinary)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainExitsNonZeroOnFailure$")
	cmd.Env = append(os.Environ(), helperEnv+"="+envTrue)
	err := cmd.Run()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
}

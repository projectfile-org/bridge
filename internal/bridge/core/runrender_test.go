// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

const (
	testStubTXT = "STUB.txt"
	testFileA   = "A.txt"
	testFileB   = "B.txt"
)

// ── Stub Renderer ────────────────────────────────────────────────────────────

type stubRenderer struct {
	filename string
	policy   core.Policy
	// Single-file output when files is nil.
	content []byte
	// Multi-file output; takes priority over content when set.
	files map[string][]byte
}

func (r *stubRenderer) Name() string             { return "stub-renderer" }
func (r *stubRenderer) Filename() string         { return r.filename }
func (r *stubRenderer) Aliases() []string        { return nil }
func (r *stubRenderer) Labels() (string, string) { return r.filename, pfLabel }
func (r *stubRenderer) Policy() core.Policy      { return r.policy }

func (r *stubRenderer) Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, r.filename))
	return err == nil
}

func (r *stubRenderer) FullPath(dir string, _ *projectfile.Document) string {
	return filepath.Join(dir, r.filename)
}

func (r *stubRenderer) Render(_ *projectfile.Document, _ core.Options) (core.Output, error) {
	if r.files != nil {
		return core.Output{Files: r.files}, nil
	}
	return core.Output{Files: map[string][]byte{r.filename: r.content}}, nil
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestRunRenderCreatesFile(t *testing.T) {
	dir := t.TempDir()
	r := &stubRenderer{filename: testStubTXT, content: []byte("hello")}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir}))
	data, err := os.ReadFile(filepath.Join(dir, testStubTXT))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestRunRenderDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	r := &stubRenderer{filename: testStubTXT, content: []byte("hello")}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, DryRun: true}))
	_, err := os.Stat(filepath.Join(dir, testStubTXT))
	assert.True(t, os.IsNotExist(err), "dry-run must not create the file")
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestRunRenderPreviewDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	r := &stubRenderer{filename: testStubTXT, content: []byte("hello")}
	out := captureStdout(t, func() {
		require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Preview: true}))
	})
	assert.Equal(t, "hello", out)
	_, err := os.Stat(filepath.Join(dir, testStubTXT))
	assert.True(t, os.IsNotExist(err), "preview must not create the file")
}

// TestRunRenderPreviewBypassesPolicyGate pins the point of --preview: it shows
// what a render WOULD produce regardless of whether the write gate would
// refuse it, so trying out a template never requires clearing --force first.
func TestRunRenderPreviewBypassesPolicyGate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, testStubTXT)
	require.NoError(t, os.WriteFile(target, []byte("hand-edited, no marker"), 0o644))

	r := &stubRenderer{
		filename: testStubTXT,
		policy:   core.Policy{Marker: true},
		content:  []byte("would-be generated content"),
	}
	out := captureStdout(t, func() {
		require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Preview: true}))
	})
	assert.Equal(t, "would-be generated content", out)
	data, _ := os.ReadFile(target)
	assert.Equal(t, "hand-edited, no marker", string(data), "preview must not touch the existing file")
}

func TestRunRenderPreviewMultiFileHeaders(t *testing.T) {
	dir := t.TempDir()
	r := &stubRenderer{
		filename: testFileA,
		files: map[string][]byte{
			testFileA: []byte("content-a"),
			testFileB: []byte("content-b"),
		},
	}
	out := captureStdout(t, func() {
		require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Preview: true}))
	})
	assert.Equal(t, "--- A.txt ---\ncontent-a--- B.txt ---\ncontent-b", out)
}

func TestRunRenderMarkerRefusesHandEdited(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, testStubTXT)
	require.NoError(t, os.WriteFile(target, []byte("hand-edited, no marker"), 0o644))

	r := &stubRenderer{
		filename: testStubTXT,
		policy:   core.Policy{Marker: true},
		content:  []byte("generated"),
	}
	err := core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir})
	assert.Error(t, err, "marker policy must refuse a file that lacks the sentinel")
}

func TestRunRenderMarkerAllowsManagedFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, testStubTXT)
	require.NoError(t, os.WriteFile(target, []byte("# pf-cli-managed: yes\nold"), 0o644))

	r := &stubRenderer{
		filename: testStubTXT,
		policy:   core.Policy{Marker: true},
		content:  []byte("# pf-cli-managed: yes\nnew"),
	}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir}))
	data, _ := os.ReadFile(target)
	assert.Equal(t, "# pf-cli-managed: yes\nnew", string(data))
}

func TestRunRenderMarkerForceOverridesCheck(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, testStubTXT)
	require.NoError(t, os.WriteFile(target, []byte("no marker here"), 0o644))

	r := &stubRenderer{
		filename: testStubTXT,
		policy:   core.Policy{Marker: true},
		content:  []byte("forced content"),
	}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir, Force: true}))
	data, _ := os.ReadFile(target)
	assert.Equal(t, "forced content", string(data))
}

func TestRunRenderMultiFile(t *testing.T) {
	dir := t.TempDir()
	r := &stubRenderer{
		filename: testFileA,
		files: map[string][]byte{
			testFileA: []byte("content-a"),
			testFileB: []byte("content-b"),
		},
	}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir}))

	a, err := os.ReadFile(filepath.Join(dir, testFileA))
	require.NoError(t, err)
	assert.Equal(t, "content-a", string(a))

	b, err := os.ReadFile(filepath.Join(dir, testFileB))
	require.NoError(t, err)
	assert.Equal(t, "content-b", string(b))
}

func TestRunRenderCreatesSubdirectory(t *testing.T) {
	dir := t.TempDir()
	r := &stubRenderer{
		filename: "sub/dir/FILE.txt",
		files: map[string][]byte{
			"sub/dir/FILE.txt": []byte("nested"),
		},
	}
	require.NoError(t, core.RunRender(r, &projectfile.Document{}, core.Options{Dir: dir}))
	data, err := os.ReadFile(filepath.Join(dir, "sub", "dir", "FILE.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

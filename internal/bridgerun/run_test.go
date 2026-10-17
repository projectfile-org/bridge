// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package bridgerun

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/rootflags"
)

// Fakes for the registry-driven describeLine: Bridge identity plus the one
// capability that decides the direction suffix. Names sort a/b/c so the
// expected line reads in registration-independent order.
type fakeRenderer struct{ filename string }

func (f fakeRenderer) Name() string      { return f.filename }
func (f fakeRenderer) Filename() string  { return f.filename }
func (f fakeRenderer) Aliases() []string { return nil }
func (f fakeRenderer) Labels() (string, string) {
	return f.filename, "projectfile"
}

func (f fakeRenderer) FullPath(dir string, _ *projectfile.Document) string {
	return dir + "/" + f.filename
}
func (f fakeRenderer) Exists(string) bool  { return false }
func (f fakeRenderer) Policy() core.Policy { return core.Policy{} }
func (f fakeRenderer) Render(*projectfile.Document, core.Options) (core.Output, error) {
	return core.Output{}, nil
}

type fakeSyncer struct{ fakeRenderer }

func (f fakeSyncer) NewEmpty() any            { return nil }
func (f fakeSyncer) Clone(doc any) any        { return doc }
func (f fakeSyncer) Read(string) (any, error) { return nil, nil }
func (f fakeSyncer) Write(string, any) error  { return nil }
func (f fakeSyncer) BuildMappers(any, *projectfile.Document) core.MapperList {
	return nil
}

type fakeDescriber struct{ fakeRenderer }

func (f fakeDescriber) Describe() string { return "custom line (one-way render)" }

func TestDescribeLine(t *testing.T) {
	bridge.Register(fakeDescriber{fakeRenderer{"a-fragments"}})
	bridge.Register(fakeRenderer{"b-readme.md"})
	bridge.Register(fakeRenderer{"b2-dockerignore"})

	// Files of one binary share one direction, so the suffix is stated once.
	assert.Equal(t,
		"custom line (one-way render), "+
			"b-readme.md, b2-dockerignore — one-way render",
		describeLine())

	// A syncer joining the set switches the whole binary to mixed direction.
	bridge.Register(fakeSyncer{fakeRenderer{"c-package.json"}})
	assert.Equal(t,
		"custom line (one-way render), "+
			"b-readme.md, b2-dockerignore, c-package.json — sync and render",
		describeLine())
}

func TestNoTerminalStdinListsInsteadOfPrompting(t *testing.T) {
	bridge.Register(fakeRenderer{"tty-one"})
	bridge.Register(fakeRenderer{"tty-two"})
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	defer func() { _ = r.Close(); _ = w.Close() }()
	saved := promptInput
	promptInput = r
	defer func() { promptInput = saved }()

	assert.False(t, canPrompt())
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	assert.NoError(t, runBridge(cmd, nil))
	assert.Contains(t, out.String(), "tty-one\ntty-two\n")
}

// TestHelpTemplateStylesOnlyForColour pins that the help title is plain in a pipe and styled under --colors=always.
func TestHelpTemplateStylesOnlyForColour(t *testing.T) {
	tmpl := helpTemplate("pf-bridge-readme", "BASE\n")
	for _, tc := range []struct {
		mode string
		ansi bool
	}{{genlog.ColorAuto, false}, {genlog.ColorAlways, true}} {
		assert.NoError(t, genlog.SetColor(tc.mode))
		var buf bytes.Buffer
		_, _ = fmt.Fprint(genlog.Styled(&buf), tmpl)
		got := buf.String()
		assert.Contains(t, got, "pf-bridge-readme")
		assert.Contains(t, got, "BASE")
		assert.Equal(t, tc.ansi, strings.Contains(got, "\x1b["), "colors=%s: %q", tc.mode, got)
	}
	assert.NoError(t, genlog.SetColor(genlog.ColorAuto))
}

// TestRootHelpCarriesTitleAndNoteOnce pins Main’s help wiring: the title leads and the env-var note prints once.
func TestRootHelpCarriesTitleAndNoteOnce(t *testing.T) {
	root := &cobra.Command{Use: "pf-bridge-readme", Run: func(*cobra.Command, []string) {}}
	rootflags.Bind(root)
	root.SetHelpTemplate(helpTemplate("pf-bridge-readme", root.HelpTemplate()))
	var buf bytes.Buffer
	root.SetOut(genlog.Styled(&buf))
	root.SetArgs([]string{"--help"})
	assert.NoError(t, root.Execute())
	got := buf.String()
	assert.True(t, strings.HasPrefix(got, "pf-bridge-readme\n"), "help starts %q", got)
	assert.Equal(t, 1, strings.Count(got, "defaults from its PF_BRIDGE_* env var"))
}

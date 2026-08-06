// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releaserc

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const filenameReleaseRC = ".releaserc.yaml"

// Bridge renders `.releaserc.yaml` from [org.projectfile.release]. It is a
// derive-only Renderer: a semantic-release config never round-trips back into
// the abstract release intent (the plugin array is a lowering artefact, not
// reversible to the model).
type Bridge struct{}

func (Bridge) Name() string             { return "releaserc" }
func (Bridge) Filename() string         { return filenameReleaseRC }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return filenameReleaseRC, "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{Marker: true} }

func (Bridge) Exists(dir string) bool {
	_, err := os.Stat(core.PathOrDefault(dir, filenameReleaseRC, filenameReleaseRC))
	return err == nil
}

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return core.PathOrDefault(dir, filenameReleaseRC, filenameReleaseRC)
}

func (Bridge) Render(pf *projectfile.Document, _ core.Options) (core.Output, error) {
	ext, err := pfmodel.GetReleaseExtension(pf)
	if err != nil {
		return core.Output{}, err
	}
	// A release config with no triggering branch is meaningless; refuse rather
	// than emit a file that looks configured but releases nothing.
	if ext == nil || len(ext.Branches) == 0 {
		return core.Output{}, fmt.Errorf(`%s: no release configuration.
  Add [org.projectfile.release] with at least one release branch, e.g.
    ["org.projectfile.release"]
    tag-format = "v${version}"
    [[org.projectfile.release.branches]]
    pattern = "main"
    channel = "latest"
  Spec reference: https://projectfile.org/spec/v1#release`, filenameReleaseRC)
	}

	// The forge KIND (not a hardcoded host) selects the release plugin, so the
	// same release intent lowers correctly on kiota.ch, codeberg.org, or github.
	forgePlugin, forgeLabel, err := resolveForgePlugin(pf)
	if err != nil {
		return core.Output{}, err
	}

	cfg := releaseConfig{
		TagFormat: orDefault(ext.TagFormat, "v${version}"),
		Branches:  branchEntries(ext.Branches),
		Plugins:   plugins(ext.Changelog, forgePlugin),
	}

	genlog.Decision("forge_plugin", forgeLabel, "org.projectfile.forge / primary repository host", "")
	genlog.Decision("tag_format", cfg.TagFormat, "[org.projectfile.release].tag-format (default v${version})", "")
	genlog.Decision("branches", fmt.Sprintf("%d", len(cfg.Branches)), "[org.projectfile.release].branches", "")
	genlog.Decision("changelog", orDefault(ext.Changelog, "(none)"), "[org.projectfile.release].changelog", "")

	// yaml.v3's default Marshal uses 4-space indent; the project convention
	// (rawdoc.YAMLNode.Marshal, projectfile.writeYAMLClean) is 2 spaces, so
	// the encoder is constructed explicitly. yaml.v3 never emits the YAML
	// document-start marker — YAMLDocStart is prepended at the assembly step
	// so the file passes yamllint (rule document-start: present).
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		_ = enc.Close()
		return core.Output{}, err
	}
	if err := enc.Close(); err != nil {
		return core.Output{}, err
	}
	out := []byte(core.YAMLDocStart + core.REUSEHeader(pf, core.StyleHash) + core.Marker + "\n" + buf.String())
	return core.Output{Files: map[string][]byte{filenameReleaseRC: out}}, nil
}

// resolveForgePlugin selects the ONE semantic-release forge plugin from the
// primary repository's forge kind (the-last-stand "option A"). semantic-release
// runs every plugin's verifyConditions at start, so emitting a second forge's
// plugin would hard-fail without that forge's token — we never emit both.
func resolveForgePlugin(pf *projectfile.Document) (any, string, error) {
	repo := pfmodel.PrimaryRepository(pf)
	if repo == nil || repo.URL == "" {
		return nil, "", fmt.Errorf("%s: no primary repository URL — cannot pick a release forge plugin (set repositories[].url)", filenameReleaseRC)
	}
	var kinds map[string]string
	if fe, _ := pfmodel.GetForgeExtension(pf); fe != nil {
		kinds = fe.Kinds
	}
	// kinds (from org.projectfile.forge) wins over the host-pattern guess, so a
	// bare self-hosted Forgejo host like kiota.ch can declare itself.
	kind := hostmatch.ResolveKindWithKinds(repo.URL, kinds)
	_, host := hostmatch.Resolve(repo.URL)
	switch kind {
	case hostmatch.KindGitHub:
		return dq("@semantic-release/github"), "github", nil
	case hostmatch.KindForgejo:
		// Forgejo and Codeberg both use @markwylde/semantic-release-gitea (the
		// semantic-release org ships no gitea/forgejo plugin; this maintained
		// fork is the one that exists on npm and honours giteaUrl — Forgejo keeps
		// Gitea API compatibility); the instance URL distinguishes kiota.ch from
		// codeberg.org.
		return []any{dq("@markwylde/semantic-release-gitea"), map[string]string{"giteaUrl": "https://" + host}},
			"gitea (" + host + ")", nil
	default:
		return nil, "", fmt.Errorf("%s: unsupported forge kind %q for host %q — only github and forgejo/gitea release plugins are wired", filenameReleaseRC, kind, host)
	}
}

// branchEntries maps the abstract branch-to-channel list onto semantic-release
// branch objects, dropping empty channel / false prerelease for a clean file.
func branchEntries(in []pfmodel.ReleaseBranch) []branchEntry {
	out := make([]branchEntry, 0, len(in))
	for _, b := range in {
		out = append(out, branchEntry{
			Name:       b.Pattern,
			Channel:    b.Channel,
			Prerelease: b.Prerelease,
		})
	}
	return out
}

// plugins assembles the MINIMAL semantic-release plugin chain: commit-analyzer +
// release-notes-generator + (optional) changelog + the forge release plugin. No
// git write-back plugin — that opinion is deliberately left out of v1.
func plugins(changelog string, forgePlugin any) []any {
	out := []any{
		dq("@semantic-release/commit-analyzer"),
		dq("@semantic-release/release-notes-generator"),
	}
	if changelog != "" && changelog != "none" {
		out = append(out, dq("@semantic-release/changelog"))
	}
	return append(out, forgePlugin)
}

// dq wraps s as a double-quoted YAML scalar node. yaml.v3 picks single quotes
// by default when a string needs quoting (e.g. "@semantic-release/..." — the
// leading @ is a YAML reserved indicator), but the project's quote-style
// convention is one kind everywhere: double quotes.
func dq(s string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: s,
		Style: yaml.DoubleQuotedStyle,
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

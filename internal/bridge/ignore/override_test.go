// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package ignore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	testTgz               = "*.tgz"
	testSecrets           = ".secrets/"
	testGit               = ".git"
	testExtClaude         = "claude"
	testExtContainer      = extKeyContainer
	testFilenameDocker    = ".dockerignore"
	testFilenameGitignore = ".gitignore"
	testReports           = "reports/"
	testEnv               = ".env"
	testInclude           = "include"
	testMd                = "*.md"
)

// npm and claude now have typed override slots; overrideFor must resolve them.
// (trivy is gone from this family — vulnerability IDs live in
// org.projectfile.vulnerabilities and fan out via bridge/vulnerabilities.)
func TestOverrideForNpmClaude(t *testing.T) {
	ext := &pfmodel.IgnoresExtension{
		Npm:       &pfmodel.IgnoreTargetOverride{Include: []string{testTgz}, Exclude: []string{stackNode}},
		Claude:    &pfmodel.IgnoreTargetOverride{Include: []string{testSecrets}},
		Container: &pfmodel.IgnoreTargetOverride{Include: []string{testGit}, Exclude: []string{stackNode}},
	}
	assert.Equal(t, []string{testTgz}, includesFor(extKeyNPM, ext))
	assert.Equal(t, []string{testSecrets}, includesFor(testExtClaude, ext))
	assert.Equal(t, []string{testGit}, includesFor(testExtContainer, ext))
	// Unset slots still resolve to nil.
	assert.Nil(t, overrideFor(extKeyGit, ext))
}

// Include overrides land in the assembled .npmignore body under user-include.
func TestAssembleNpmIncludeApplied(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	ext := &pfmodel.IgnoresExtension{
		Npm: &pfmodel.IgnoreTargetOverride{Include: []string{testTgz, "coverage/"}},
	}
	body := string(assemble(pf, ".npmignore", "npm", ext))
	assert.Contains(t, body, "user-include")
	assert.Contains(t, body, testTgz)
	assert.Contains(t, body, "coverage/")
}

func TestAssembleClaudeIncludeApplied(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	ext := &pfmodel.IgnoresExtension{
		Claude: &pfmodel.IgnoreTargetOverride{Include: []string{testSecrets, testReports}},
	}
	body := string(assemble(pf, ".claudeignore", testExtClaude, ext))
	assert.Contains(t, body, "user-include")
	assert.Contains(t, body, testSecrets)
	assert.Contains(t, body, testReports)
}

func TestAssembleContainerIncludeApplied(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	ext := &pfmodel.IgnoresExtension{
		Container: &pfmodel.IgnoreTargetOverride{Include: []string{testGit, testMd}},
	}
	body := string(assemble(pf, ".containerignore", testExtContainer, ext))
	assert.Contains(t, body, testGit)
	assert.Contains(t, body, testMd)
}

// Undeclared docker: mirrors container: verbatim. buildah build reads
// .containerignore INSTEAD of .dockerignore when both exist (never merges
// them), so the deferred file must not fall back to nothing.
func TestAssembleDockerFallsBackToContainer(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	ext := &pfmodel.IgnoresExtension{
		Container: &pfmodel.IgnoreTargetOverride{Include: []string{testGit, testMd}, Exclude: []string{testMd}},
	}
	assert.Equal(t, ext.Container, overrideFor(extKeyDocker, ext))
	body := string(assemble(pf, testFilenameDocker, extKeyDocker, ext))
	assert.Contains(t, body, testGit)
	assert.NotContains(t, body, testMd)
}

// A declared docker: (even include-only) takes over completely — no
// merging with container:, matching how every other target already behaves.
func TestAssembleDockerDeclaredOverridesContainer(t *testing.T) {
	ext := &pfmodel.IgnoresExtension{
		Container: &pfmodel.IgnoreTargetOverride{Include: []string{"container-only/"}},
		Docker:    &pfmodel.IgnoreTargetOverride{Include: []string{testGit}},
	}
	assert.Equal(t, []string{testGit}, includesFor(extKeyDocker, ext))
}

// GetIgnoresExtension must parse the npm/claude/container sub-tables off the extension.
func TestGetIgnoresExtensionNpmClaude(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{
				"npm":            map[string]any{testInclude: []any{testTgz}, "exclude": []any{stackNode}},
				testExtClaude:    map[string]any{"include": []any{testSecrets}},
				testExtContainer: map[string]any{"include": []any{testGit}, "exclude": []any{stackNode}},
			},
		},
	}
	ext, err := pfmodel.GetIgnoresExtension(doc)
	assert.NoError(t, err)
	assert.Equal(t, []string{testTgz}, ext.Npm.Include)
	assert.Equal(t, []string{stackNode}, ext.Npm.Exclude)
	assert.Equal(t, []string{testSecrets}, ext.Claude.Include)
	assert.Equal(t, []string{testGit}, ext.Container.Include)
	assert.Equal(t, []string{stackNode}, ext.Container.Exclude)
}

// Bare list shorthand: git: [".secrets/", "reports/"] means include-only.
func TestGetIgnoresExtensionBareList(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{
				extKeyGit: []any{testSecrets, testReports},
			},
		},
	}
	ext, err := pfmodel.GetIgnoresExtension(doc)
	assert.NoError(t, err)
	assert.Equal(t, []string{testSecrets, testReports}, ext.Git.Include)
	assert.Nil(t, ext.Git.Exclude)
	// Bare list entries land in assembled body.
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	body := string(assemble(pf, testFilenameGitignore, extKeyGit, ext))
	assert.Contains(t, body, testReports)
}

// When no includes and no extras exist, assemble returns nil so the caller
// skips writing an empty file.
func TestAssembleEmptyBodyNoFile(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	body := assemble(pf, testFilenameGitignore, extKeyGit, nil)
	assert.Nil(t, body)
}

// Render returns an empty Output when assemble produces no content.
func TestRenderEmptyOutput(t *testing.T) {
	b := Bridge{filename: testFilenameGitignore, extKey: extKeyGit}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	out, err := b.Render(pf, core.Options{})
	assert.NoError(t, err)
	assert.Empty(t, out.Files)
}

// exclude SUBTRACTS: the only way to drop a pattern an m6e language fragment
// injected, since `includes:` deep-merge unions lists and never replaces them.
func TestAssembleExcludeDropsInheritedPattern(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	ext := &pfmodel.IgnoresExtension{
		Git: &pfmodel.IgnoreTargetOverride{
			Include: []string{"/dist/", testReports},
			Exclude: []string{"/dist/"},
		},
	}
	body := string(assemble(pf, testFilenameGitignore, extKeyGit, ext))
	assert.NotContains(t, body, "/dist/")
	assert.Contains(t, body, testReports)
}

// `extra` fans out to every target, so a target's exclude must reach it too —
// otherwise one file can never opt out of a pattern that belongs everywhere else.
func TestAssembleExcludeReachesExtra(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	ext := &pfmodel.IgnoresExtension{
		Extra:  []string{testEnv, testSecrets},
		Claude: &pfmodel.IgnoreTargetOverride{Include: []string{testGit}, Exclude: []string{testEnv}},
	}
	body := string(assemble(pf, ".claudeignore", testExtClaude, ext))
	assert.NotContains(t, body, testEnv)
	assert.Contains(t, body, testSecrets)
}

// An exclude naming nothing present is inert, not an error.
func TestAssembleExcludeUnmatchedIsInert(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	ext := &pfmodel.IgnoresExtension{
		Git: &pfmodel.IgnoreTargetOverride{Include: []string{testReports}, Exclude: []string{"nothing/"}},
	}
	body := string(assemble(pf, testFilenameGitignore, extKeyGit, ext))
	assert.Contains(t, body, testReports)
}

// A re-include must follow the pattern it answers: these files resolve
// last-match-wins, and "!" sorts below every letter.
func TestAssembleNegationFollowsItsPattern(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	ext := &pfmodel.IgnoresExtension{
		Git: &pfmodel.IgnoreTargetOverride{
			Include: []string{"dist/", "!dist/keep.txt", testReports},
		},
	}
	body := string(assemble(pf, testFilenameGitignore, extKeyGit, ext))
	assert.Greater(t, strings.Index(body, "!dist/keep.txt"), strings.Index(body, "dist/\n"),
		"a re-include emitted above its pattern is inert")
	block := body[strings.Index(body, "# >>> user-include"):strings.Index(body, "# <<< user-include")]
	assert.NotContains(t, block, "\n\n", "no blank line may split the block")
}

// An explicit generate list is authoritative: an unlisted target emits nothing.
func TestRenderGenerateListIsAuthoritative(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{
				"generate":   []any{extKeyGit},
				extKeyGit:    []any{testReports},
				extKeyDocker: []any{testReports},
			},
		},
	}
	git, err := Bridge{filename: testFilenameGitignore, extKey: extKeyGit}.Render(doc, core.Options{})
	assert.NoError(t, err)
	assert.Len(t, git.Files, 1)

	docker, err := Bridge{filename: testFilenameDocker, extKey: extKeyDocker}.Render(doc, core.Options{})
	assert.NoError(t, err)
	assert.Empty(t, docker.Files)
}

// An absent generate list means every target generates — no project has to
// enumerate the files it already gets.
func TestRenderGenerateAbsentMeansAll(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{extKeyDocker: []any{testReports}},
		},
	}
	out, err := Bridge{filename: testFilenameDocker, extKey: extKeyDocker}.Render(doc, core.Options{})
	assert.NoError(t, err)
	assert.Len(t, out.Files, 1)
}

// An opt-in target emits nothing until its sub-namespace exists — otherwise a
// new row here would create the file in every project in the fleet.
func TestRenderOptInSilentWithoutNamespace(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{"extra": []any{testEnv}},
		},
	}
	out, err := Bridge{filename: ".textlintignore", extKey: extKeyTextlint, optIn: true}.Render(doc, core.Options{})
	assert.NoError(t, err)
	assert.Empty(t, out.Files, "extra alone must not conjure an opt-in file")
}

// Declaring the sub-namespace turns the target on, and extra rides along.
func TestRenderOptInEmitsOnceDeclared(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			pfmodel.IgnoresExtensionNS: map[string]any{
				"extra":        []any{testEnv},
				extKeyTextlint: []any{"LICENSES/"},
				extKeyFd:       []any{"LICENSES/"},
			},
		},
	}
	out, err := Bridge{filename: ".textlintignore", extKey: extKeyTextlint, optIn: true}.Render(doc, core.Options{})
	assert.NoError(t, err)
	body := string(out.Files[".textlintignore"])
	assert.Contains(t, body, "LICENSES/")
	assert.Contains(t, body, testEnv)

	fd, err := Bridge{filename: ".fdignore", extKey: extKeyFd, optIn: true}.Render(doc, core.Options{})
	assert.NoError(t, err)
	assert.Contains(t, string(fd.Files[".fdignore"]), "LICENSES/")
}

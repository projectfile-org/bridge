// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forgecmd

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

	"projectfile.org/projectfile/bridge/v4/internal/buildinfo"
	"projectfile.org/projectfile/bridge/v4/internal/describe"
	"projectfile.org/projectfile/bridge/v4/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// The re-exec gate hands the child the argv the parent wants Main to answer with.
const (
	binName    = "pf-bridge-forge"
	helperEnv  = "PF_FORGE_TEST_MAIN"
	helperArgv = "PF_FORGE_TEST_ARGV"
	envTrue    = "1"
	argvSep    = "\x1f"
	mainReturn = "main-returned"
	pfFile     = "projectfile.yaml"
	ignoreUser = "--ignore-user-config"

	cmdList    = "list"
	cmdPush    = "push"
	flagOffln  = "--offline"
	flagDryRun = "--dry-run"
	flagVer    = "-V"

	repoGitHub    = "https://github.com/acme/alpha"
	repoCodeberg  = "https://codeberg.org/acme/beta"
	repoSelfHost  = "https://code.example.com/acme/gamma"
	repoArchive   = "https://github.com/acme/delta"
	repoKindOnExt = "https://code.example.com/acme/epsilon"
	hostSelfHost  = "code.example.com"
	hostOther     = "other.example.com"

	kindGitHub  = "github"
	kindForgejo = "forgejo"
	kindUnknown = "unknown"
	roleArchive = "archive"

	tokenPresent  = "present"
	tokenMissing  = "missing"
	visPublic     = "public"
	visUndeclared = "undeclared"
	statusReady   = "ready"
)

// The cell values a list row shows once every flag and env default applied.
const (
	noEntries       = "(no entries in top-level repositories[])"
	unsupportedHost = "unsupported host"
	skipArchive     = "skipped (archive role)"
	noTokenCodeberg = "no token (set PF_FORGE_TOKEN_CODEBERG_ORG or FORGEJO_TOKEN)"
	noTokenSelfHost = "no token (set PF_FORGE_TOKEN_CODE_EXAMPLE_COM or FORGEJO_TOKEN)"
)

// Fixture bodies: one document per shape the two commands read.
const (
	emptyFixture = `---
# SPDX-FileCopyrightText: 2026 Tester
#
# SPDX-License-Identifier: MIT
$schema: https://projectfile.org/schema/v1.json
identity:
  name: emptycase
`

	pushFixture = `---
# SPDX-FileCopyrightText: 2026 Tester
#
# SPDX-License-Identifier: MIT
$schema: https://projectfile.org/schema/v1.json
identity:
  name: pushcase
repositories:
  - url: https://github.com/acme/alpha
    role: origin
`

	listFixture = `---
# SPDX-FileCopyrightText: 2026 Tester
#
# SPDX-License-Identifier: MIT
$schema: https://projectfile.org/schema/v1.json
identity:
  name: listcase
  visibility: public
repositories:
  - url: https://github.com/acme/alpha
  - url: https://codeberg.org/acme/beta
  - url: https://code.example.com/acme/gamma
  - url: https://github.com/acme/delta
    role: archive
`

	kindFixture = `---
# SPDX-FileCopyrightText: 2026 Tester
#
# SPDX-License-Identifier: MIT
$schema: https://projectfile.org/schema/v1.json
identity:
  name: kindcase
org:
  projectfile:
    forge:
      kinds:
        code.example.com: forgejo
repositories:
  - url: https://code.example.com/acme/epsilon
`

	malformedRelease = `---
# SPDX-FileCopyrightText: 2026 Tester
#
# SPDX-License-Identifier: MIT
$schema: https://projectfile.org/schema/v1.json
identity:
  name: releasecase
org:
  projectfile:
    release: 5
`
)

// TestMain binds the command once here: a second Main call re-registers every flag and re-adds every command.
func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == envTrue {
		_ = flag.Set("test.run", "^TestMainGateHelper$") // a child with no -test.run would run every test and spawn again
		os.Exit(m.Run())
	}
	// Bind latches every PF_BRIDGE_* default at register time, so a developer's exported OFFLINE or QUIET would otherwise reach the suite.
	for _, env := range []string{
		"PF_BRIDGE_QUIET", "PF_BRIDGE_VERBOSE", "PF_CLI_VERBOSE", "PF_BRIDGE_OFFLINE",
		"PF_BRIDGE_NO_COLOR", "PF_BRIDGE_COLORS", "PF_BRIDGE_SORTED", "PF_BRIDGE_IGNORE_USER_CONFIG",
	} {
		_ = os.Unsetenv(env)
	}
	argv := os.Args
	os.Args = []string{binName, describe.Flag} // the probe returns before cobra, which would exit on the suite's own flags
	Main(binName)
	os.Args = argv
	os.Exit(m.Run())
}

// TestMainGateHelper is the child half: the parent re-execs this binary, so the child's own Main reads an os.Exit.
func TestMainGateHelper(t *testing.T) {
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

// runCmd executes the bound root with output captured, after the option vars return to their defaults.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	resetPushVars(t)
	var out bytes.Buffer
	forgeCmd.SetOut(&out)
	forgeCmd.SetArgs(append([]string{ignoreUser}, args...))
	err := forgeCmd.Execute()
	return out.String(), err
}

// resetPushVars restores each option var, since pflag only resets them when the flag is registered.
func resetPushVars(t *testing.T) {
	t.Helper()
	forgePushDryRun, forgePushRepos, forgePushFields, forgePushInsecureTLS = false, nil, nil, false
	t.Cleanup(func() {
		forgePushDryRun, forgePushRepos, forgePushFields, forgePushInsecureTLS = false, nil, nil, false
	})
}

// clearTokens empties every env var ResolveToken reads, so no run dials a forge over the network.
func clearTokens(t *testing.T) {
	t.Helper()
	for _, env := range []string{
		"GITHUB_TOKEN", "GITLAB_TOKEN", "FORGEJO_TOKEN", "GITEA_TOKEN",
		"PF_FORGE_TOKEN_GITHUB_COM", "PF_FORGE_TOKEN_KIOTA_CH", "PF_FORGE_TOKEN_CODEBERG_ORG",
		"PF_FORGE_TOKEN_CODE_EXAMPLE_COM",
	} {
		t.Setenv(env, "")
	}
}

// writeProjectfile writes body into a fresh temp project and returns its directory.
func writeProjectfile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, pfFile), []byte(body), 0o644))
	return dir
}

// assertRow checks one table line: URL, kind, token, visibility and the status, the only cell that may carry spaces.
func assertRow(t *testing.T, out, url, kind, token, visibility, status string) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, url) {
			continue
		}
		fields := strings.Fields(line)
		require.Len(t, fields, 4+len(strings.Fields(status)), url)
		assert.Equal(t, []string{url, kind, token, visibility}, fields[:4], url)
		assert.Equal(t, status, strings.Join(fields[4:], " "), url)
		return
	}
	t.Fatalf("no row for %s in:\n%s", url, out)
}

func TestMainFlagSurface(t *testing.T) {
	assert.Equal(t, binName, forgeCmd.Use, "Main names the root after the binary it builds")
	for _, name := range []string{"dry-run", "repo", "field", "insecure-skip-tls"} {
		assert.NotNil(t, forgePushCmd.Flags().Lookup(name), name)
	}
	assert.Equal(t, "false", forgePushCmd.Flags().Lookup("dry-run").DefValue)
	assert.NotNil(t, forgeCmd.Flags().ShorthandLookup("V"), "Bind arms the version flag on a root that declares Version")
}

func TestParsePushArgs(t *testing.T) {
	dir := writeProjectfile(t, emptyFixture)
	cases := []struct {
		name      string
		args      []string
		wantForge string
		wantDir   string
	}{
		{"no positional runs every forge from the current directory", nil, "", "."},
		{"a directory holding a projectfile is the directory", []string{dir}, "", dir},
		{"a bare name is the forge filter", []string{kindGitHub}, kindGitHub, "."},
		{"the second positional is the directory", []string{kindGitHub, dir}, kindGitHub, dir},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			forge, gotDir := parsePushArgs(tc.args)
			assert.Equal(t, tc.wantForge, forge)
			assert.Equal(t, tc.wantDir, gotDir)
		})
	}
}

func TestIsProjectfileDir(t *testing.T) {
	assert.True(t, isProjectfileDir(writeProjectfile(t, emptyFixture)))
	assert.False(t, isProjectfileDir(t.TempDir()), "a directory with no projectfile reads as a forge name")
	assert.False(t, isProjectfileDir(filepath.Join(t.TempDir(), "absent")))
}

func TestResolveListKind(t *testing.T) {
	rule := &hostmatch.Rule{Kind: hostmatch.KindGitHub}
	cases := []struct {
		name string
		ext  *pfmodel.ForgeExtension
		rule *hostmatch.Rule
		want string
	}{
		{"neither source names a kind", nil, nil, ""},
		{"the host rule names the kind", nil, rule, kindGitHub},
		{"the extension overrides the host", &pfmodel.ForgeExtension{Kinds: map[string]string{hostSelfHost: kindForgejo}}, nil, kindForgejo},
		{"an extension miss falls through to the host rule", &pfmodel.ForgeExtension{Kinds: map[string]string{hostOther: kindForgejo}}, rule, kindGitHub},
		{"an empty extension kind falls through to the host rule", &pfmodel.ForgeExtension{Kinds: map[string]string{hostSelfHost: ""}}, rule, kindGitHub},
		{"an extension without kinds keeps the host rule", &pfmodel.ForgeExtension{}, rule, kindGitHub},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resolveListKind(tc.ext, tc.rule, hostSelfHost))
		})
	}
}

func TestForgeListWithoutRepositories(t *testing.T) {
	dir := writeProjectfile(t, emptyFixture)
	out, err := runCmd(t, cmdList, dir)
	require.NoError(t, err)
	assert.Equal(t, noEntries+"\n", out)
}

func TestForgeListReadFailure(t *testing.T) {
	t.Chdir(t.TempDir()) // a batch inherits dir "." when no positional is named, so the empty cwd fails the read
	_, err := runCmd(t, cmdList)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read projectfile")
}

func TestForgeListRows(t *testing.T) {
	clearTokens(t)
	t.Setenv("GITHUB_TOKEN", "list-token")
	dir := writeProjectfile(t, listFixture)
	out, err := runCmd(t, cmdList, dir)
	require.NoError(t, err)

	assertRow(t, out, repoGitHub, kindGitHub, tokenPresent, visPublic, statusReady)
	assertRow(t, out, repoCodeberg, kindForgejo, tokenMissing, visPublic, noTokenCodeberg)
	assertRow(t, out, repoSelfHost, kindUnknown, tokenMissing, visPublic, unsupportedHost)
	assertRow(t, out, repoArchive, kindGitHub, tokenPresent, visPublic, skipArchive)
}

func TestForgeListExtensionKindsAndUndeclaredVisibility(t *testing.T) {
	clearTokens(t)
	dir := writeProjectfile(t, kindFixture)
	out, err := runCmd(t, cmdList, dir)
	require.NoError(t, err)
	assertRow(t, out, repoKindOnExt, kindForgejo, tokenMissing, visUndeclared, noTokenSelfHost)
}

func TestForgePushReadFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := runCmd(t, cmdPush)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read projectfile")
}

func TestForgePushWithNoRepositories(t *testing.T) {
	dir := writeProjectfile(t, emptyFixture)
	out, err := runCmd(t, cmdPush, dir)
	require.NoError(t, err)
	assert.Contains(t, out, "pushed changes to 0/0 repo(s)")
}

func TestForgePushDryRunPreviewsTheSummary(t *testing.T) {
	dir := writeProjectfile(t, emptyFixture)
	out, err := runCmd(t, cmdPush, flagDryRun, dir)
	require.NoError(t, err)
	assert.Contains(t, out, "DRY RUN")
	assert.Contains(t, out, "would push changes to 0/0 repo(s)")
}

func TestForgePushSurfacesPushFailure(t *testing.T) {
	dir := writeProjectfile(t, malformedRelease)
	_, err := runCmd(t, cmdPush, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "org.projectfile.release is not a map")
}

func TestForgePushSkipsEveryRepoWithoutAToken(t *testing.T) {
	clearTokens(t)
	dir := writeProjectfile(t, pushFixture)
	out, err := runCmd(t, cmdPush, dir)
	require.NoError(t, err)
	assert.Contains(t, out, "pushed changes to 0/1 repo(s)")
	assert.Contains(t, out, repoGitHub+": skipped — no token (set PF_FORGE_TOKEN_GITHUB_COM or GITHUB_TOKEN)")
}

func TestMainAnswersTheDescribeProbe(t *testing.T) {
	out, _, code := runAsChild(t, describe.Flag)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, forgeCmd.Short)
}

func TestMainExitsZeroOnAListRun(t *testing.T) {
	dir := writeProjectfile(t, emptyFixture)
	out, _, code := runAsChild(t, ignoreUser, cmdList, dir)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, mainReturn)
	assert.Contains(t, out, noEntries)
}

func TestMainExitsOneOnOfflinePush(t *testing.T) {
	dir := writeProjectfile(t, emptyFixture) // the offline refusal fires before the document is read
	_, errOut, code := runAsChild(t, ignoreUser, flagOffln, cmdPush, dir)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "Error: forge push requires network access")
}

func TestMainExitsOneOnAFailingCommand(t *testing.T) {
	t.Chdir(t.TempDir())
	_, errOut, code := runAsChild(t, cmdPush)
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "Error: read projectfile")
}

func TestMainPrintsTheVersion(t *testing.T) {
	out, _, code := runAsChild(t, flagVer)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, buildinfo.Version)
}

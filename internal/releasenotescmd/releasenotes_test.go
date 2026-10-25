// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotescmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/v4/internal/buildinfo"
	"projectfile.org/projectfile/bridge/v4/internal/describe"
	"projectfile.org/projectfile/bridge/v4/internal/releasenotes"
)

// defaultTimeout is the flag default pflag gives each Duration option at registration.
const defaultTimeout = 30 * time.Second

// The re-exec gate hands the child the argv the parent wants Main to answer with.
const (
	binName    = "pf-bridge-release-notes"
	helperEnv  = "PF_RELEASE_NOTES_TEST_MAIN"
	helperArgv = "PF_RELEASE_NOTES_TEST_ARGV"
	envTrue    = "1"
	argvSep    = "\x1f"
	mainReturn = "main-returned"
	pfFile     = "projectfile.yaml"
	ignoreUser = "--ignore-user-config"

	cmdRender  = "render"
	cmdPublish = "publish"

	flagTag    = "--tag"
	flagServer = "--server"
	flagRepo   = "--repository"
	flagNote   = "--note-file"
	flagDryRun = "--dry-run"
	flagVer    = "-V"
)

// Fixture bodies: one document per case; the org.projectfile key keeps Extensions non-nil for RenderForge's setPath.
const (
	notesFixture = `---
# SPDX-FileCopyrightText: 2026 Tester
#
# SPDX-License-Identifier: MIT
$schema: https://projectfile.org/schema/v1.json
identity:
  name: notescase
org:
  projectfile:
    security:
      policy: default
`
	authorAnn      = "Ann"
	featSubject    = "feat: first light"
	hostKiota      = "kiota"
	releaseTag     = "v1.2.0"
	priorTag       = "1.1.0"
	repoSlug       = "o/x"
	tokenValue     = "secret"
	magnetAsset    = "o-x.magnet"
	magnetBody     = "magnet:?xt=urn:btih:abc"
	noteBody       = "Smaller and faster."
	priorBody      = "Human half.\n"
	apiRoot        = "/api/v1/repos/"
	oneCommit      = "1 commit, 1 contributor"
	notGitRepo     = "not a git repository"
	tagRequired    = "--tag is required"
	serverRequired = "--server and --repository are required"
	gateTest       = "^TestMainGateHelper$"
)

// Token env vars firstEnv reads, named once so no literal repeats across files.
const (
	envForgejoToken = "FORGEJO_TOKEN"
	envGHToken      = "GH_TOKEN"
	envGitHubToken  = "GITHUB_TOKEN"
)

// TestMain binds the command once here: a second Main call re-registers every flag and re-adds every command.
func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == envTrue {
		_ = flag.Set("test.run", gateTest) // a child with no -test.run would run every test and spawn again
		os.Exit(m.Run())
	}
	// git localizes its own diagnostics, so the suite pins the locale every error assertion reads.
	_ = os.Setenv("LC_ALL", "C")
	_ = os.Setenv("LANG", "C")
	// Bind latches every PF_BRIDGE_* default and the publish flags latch GITHUB_* at register time.
	for _, env := range []string{
		"PF_BRIDGE_QUIET", "PF_BRIDGE_VERBOSE", "PF_CLI_VERBOSE", "PF_BRIDGE_OFFLINE",
		"PF_BRIDGE_NO_COLOR", "PF_BRIDGE_COLORS", "PF_BRIDGE_SORTED", "PF_BRIDGE_IGNORE_USER_CONFIG",
		"PF_BRIDGE_FAIL_ON", "PF_BRIDGE_TIMEOUT",
		"GITHUB_REF_NAME", "GITHUB_SERVER_URL", "GITHUB_API_URL", "GITHUB_REPOSITORY",
		envForgejoToken, envGHToken, envGitHubToken,
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
	cmd := exec.Command(os.Args[0], "-test.run="+gateTest)
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
	resetVars(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs(append([]string{ignoreUser}, args...))
	err := rootCmd.Execute()
	return out.String(), err
}

// resetVars restores each option var to its flag default, since pflag only resets them when the flag is registered.
func resetVars(t *testing.T) {
	t.Helper()
	defaultVars()
	t.Cleanup(defaultVars)
}

// defaultVars hands every option var the value its flag default carries, so a zeroed Timeout never reaches a run.
func defaultVars() {
	opts = releasenotes.Options{To: "HEAD", Timeout: defaultTimeout}
	noteFile = ""
	pub = releasenotes.PublishOptions{Timeout: defaultTimeout}
}

// clearTokens empties every env var firstEnv reads, so no run dials a forge over the network.
func clearTokens(t *testing.T) {
	t.Helper()
	for _, env := range []string{envForgejoToken, envGHToken, envGitHubToken} {
		t.Setenv(env, "")
	}
}

// isolate drops every inherited GIT_* variable so a hook's GIT_DIR never reaches the fixture.
func isolate(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "GIT_") {
			t.Setenv(k, "")
			require.NoError(t, os.Unsetenv(k))
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

type repo struct {
	t   *testing.T
	dir string
	day int
}

func newRepo(t *testing.T) *repo {
	isolate(t)
	r := &repo{t: t, dir: t.TempDir()}
	r.git(authorAnn, "init", "--quiet", "--initial-branch=main")
	return r
}

// git runs one command as author, one fixture day after the previous commit.
func (r *repo) git(author string, args ...string) {
	r.t.Helper()
	date := "2026-01-01T00:00:00Z"
	if args[0] == "commit" {
		r.day++
		date = fmt.Sprintf("2026-01-%02dT00:00:00Z", r.day)
	}
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
	cmd.Dir = r.dir
	email := strings.ToLower(author) + "@example.org"
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+author, "GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME="+author, "GIT_COMMITTER_EMAIL="+email,
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	out, err := cmd.CombinedOutput()
	require.NoError(r.t, err, "git %v: %s", args, out)
}

// commit writes files (an empty value deletes one) and commits them as author.
func (r *repo) commit(author, message string, files map[string]string) {
	r.t.Helper()
	for path, body := range files {
		full := filepath.Join(r.dir, path)
		if body == "" {
			require.NoError(r.t, os.Remove(full))
			continue
		}
		require.NoError(r.t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(r.t, os.WriteFile(full, []byte(body), 0o644))
	}
	r.git(author, "add", "--all")
	r.git(author, "commit", "--quiet", "--allow-empty", "--message", message)
}

// tag names the current head with one release tag.
func (r *repo) tag(name string) {
	r.t.Helper()
	r.git(authorAnn, "tag", name)
}

// releaseServer answers the one release, the tag list and the magnet asset a dry run reads.
func releaseServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "token "+tokenValue, req.Header.Get("Authorization"))
		switch {
		case req.Method == http.MethodGet && req.URL.Path == apiRoot+repoSlug+"/releases/tags/"+releaseTag:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 7, "body": priorBody, "assets": []map[string]string{
				{"name": magnetAsset, "browser_download_url": srv.URL + "/dl/" + magnetAsset},
			}})
		case req.Method == http.MethodGet && req.URL.Path == apiRoot+repoSlug+"/releases":
			_ = json.NewEncoder(w).Encode([]map[string]string{{"tag_name": releaseTag}, {"tag_name": priorTag}})
		case req.URL.Path == "/dl/"+magnetAsset:
			_, _ = w.Write([]byte(magnetBody + "\n"))
		default:
			t.Errorf("unexpected %s %s", req.Method, req.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// unpublishableServer refuses every release read, so publish surfaces the forge's answer.
func unpublishableServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestMainFlagSurface(t *testing.T) {
	assert.Equal(t, binName, rootCmd.Use, "Main names the root after the binary it builds")
	assert.Equal(t, buildinfo.Version, rootCmd.Version)
	assert.True(t, rootCmd.SilenceUsage, "Main silences cobra so the one Error line is the whole story")
	assert.True(t, rootCmd.SilenceErrors)
	var names []string
	for _, c := range rootCmd.Commands() {
		names = append(names, c.Name())
	}
	assert.ElementsMatch(t, []string{cmdRender, cmdPublish}, names, "Main adds both subcommands")
	for name, def := range map[string]string{
		"tag": "", "from": "", "to": "HEAD", "tag-prefix": "", "note-file": "", "timeout": "30s",
	} {
		got := renderCmd.Flags().Lookup(name)
		require.NotNil(t, got, name)
		assert.Equal(t, def, got.DefValue, name)
	}
	for _, name := range []string{"tag", "tag-prefix", "server", "api", "repository", "forge", "dry-run", "timeout"} {
		assert.NotNil(t, publishCmd.Flags().Lookup(name), name)
	}
	assert.NotNil(t, rootCmd.Flags().ShorthandLookup("V"), "Bind arms the version flag on a root that declares Version")
}

func TestForgeSlug(t *testing.T) {
	cases := []struct{ server, want string }{
		{"https://kiota.ch", hostKiota},
		{"https://codeberg.org/acme/x", "codeberg"},
		{"kiota.ch", hostKiota},
		{"https://kiota.ch/", hostKiota},
		{"https://localhost:8443", "localhost:8443"},
	}
	for _, tc := range cases {
		t.Run(tc.server, func(t *testing.T) {
			assert.Equal(t, tc.want, forgeSlug(tc.server))
		})
	}
}

func TestFirstEnvPicksTheFirstSetVariable(t *testing.T) {
	clearTokens(t)
	assert.Empty(t, firstEnv("ABSENT", "ALSO_ABSENT"), "an unset name is skipped, not returned empty")
	t.Setenv("LATER", "second")
	assert.Equal(t, "second", firstEnv("ABSENT", "LATER"))
	t.Setenv("EARLIER", "first")
	assert.Equal(t, "first", firstEnv("EARLIER", "LATER"))
}

func TestRenderPrintsTheBodyOfTheRange(t *testing.T) {
	r := newRepo(t)
	r.commit(authorAnn, featSubject, nil)
	out, err := runCmd(t, cmdRender, r.dir)
	require.NoError(t, err)
	assert.Contains(t, out, oneCommit)
}

func TestRenderPlacesTheNoteUnderTheSummaryLine(t *testing.T) {
	r := newRepo(t)
	r.commit(authorAnn, featSubject, nil)
	note := filepath.Join(r.dir, "NOTES.md")
	require.NoError(t, os.WriteFile(note, []byte(noteBody), 0o644))
	out, err := runCmd(t, cmdRender, flagNote, note, r.dir)
	require.NoError(t, err)
	assert.Contains(t, out, noteBody)
	assert.Less(t, strings.Index(out, oneCommit), strings.Index(out, noteBody), "the summary line comes first")
}

func TestRenderFailsOnAnUnreadableNoteFile(t *testing.T) {
	r := newRepo(t)
	r.commit(authorAnn, featSubject, nil)
	_, err := runCmd(t, cmdRender, flagNote, filepath.Join(r.dir, "absent.md"), r.dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read note file")
}

func TestRenderFailsOutsideAGitRepository(t *testing.T) {
	_, err := runCmd(t, cmdRender, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), notGitRepo)
}

func TestRenderDefaultsToTheCurrentDirectory(t *testing.T) {
	r := newRepo(t)
	r.commit(authorAnn, featSubject, nil)
	t.Chdir(r.dir)
	out, err := runCmd(t, cmdRender)
	require.NoError(t, err)
	assert.Contains(t, out, oneCommit)
}

func TestPublishRequiresTheTag(t *testing.T) {
	_, err := runCmd(t, cmdPublish)
	require.Error(t, err)
	assert.Contains(t, err.Error(), tagRequired)
}

func TestPublishRequiresServerAndRepository(t *testing.T) {
	r := newRepo(t)
	r.commit(authorAnn, featSubject, nil)
	_, err := runCmd(t, cmdPublish, flagTag, releaseTag, r.dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), serverRequired)
}

func TestPublishRefusesWithoutAToken(t *testing.T) {
	clearTokens(t)
	r := newRepo(t)
	r.commit(authorAnn, featSubject, nil)
	_, err := runCmd(t, cmdPublish, flagTag, releaseTag, flagServer, "https://kiota.example", flagRepo, repoSlug, r.dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no token")
}

func TestPublishDryRunDerivesTheAPIAndForge(t *testing.T) {
	clearTokens(t)
	t.Setenv(envForgejoToken, tokenValue) // firstEnv reads the forge token before the agent's own GitHub one
	r := newRepo(t)
	r.commit(authorAnn, featSubject, map[string]string{pfFile: notesFixture})
	r.tag(releaseTag)
	srv := releaseServer(t)
	out, err := runCmd(t, cmdPublish, flagDryRun, flagTag, releaseTag, flagServer, srv.URL, flagRepo, repoSlug, r.dir)
	require.NoError(t, err)
	assert.Contains(t, out, releasenotes.ForgeMarker)
	assert.Contains(t, out, priorBody, "the body the forge already holds is carried over")
	assert.Contains(t, out, "/compare/"+priorTag+"..."+releaseTag)
	assert.Contains(t, out, magnetBody)
	assert.Contains(t, out, "]("+magnetBody+")", "the magnet asset renders as one link, not as its filename")
}

func TestPublishSurfacesAnUnreadableRelease(t *testing.T) {
	clearTokens(t)
	t.Setenv(envForgejoToken, tokenValue)
	r := newRepo(t)
	r.commit(authorAnn, featSubject, map[string]string{pfFile: notesFixture})
	r.tag(releaseTag)
	srv := unpublishableServer(t)
	_, err := runCmd(t, cmdPublish, flagDryRun, flagTag, releaseTag, flagServer, srv.URL, flagRepo, repoSlug, r.dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestMainAnswersTheDescribeProbe(t *testing.T) {
	out, _, code := runAsChild(t, describe.Flag)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, rootCmd.Short)
}

func TestMainExitsZeroOnARenderRun(t *testing.T) {
	r := newRepo(t)
	r.commit(authorAnn, featSubject, nil)
	out, _, code := runAsChild(t, cmdRender, r.dir)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, mainReturn)
	assert.Contains(t, out, oneCommit)
}

func TestMainExitsOneOnAFailingRender(t *testing.T) {
	_, errOut, code := runAsChild(t, cmdRender, t.TempDir())
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "Error: git tag", "Main writes the one Error line cobra silenced")
	assert.Contains(t, errOut, notGitRepo)
}

func TestMainPrintsTheVersion(t *testing.T) {
	out, _, code := runAsChild(t, flagVer)
	assert.Equal(t, 0, code)
	assert.Contains(t, out, buildinfo.Version)
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotes

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const attachDoc = forgeDoc + `    release:
      attach: ["reports/*.log", "../outside.log", "/etc/passwd", "reports/link.log"]
`

// attachFixture lays out a project with one report, a symlink leaving the project and a file beside it.
func attachFixture(t *testing.T) *repo {
	t.Helper()
	r := newRepo(t)
	r.commit("Ann", "feat: first", map[string]string{fixtureDoc: attachDoc})
	r.git("Ann", "tag", fixtureTag)
	require.NoError(t, os.MkdirAll(filepath.Join(r.dir, "reports"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, "reports", "a.log"), []byte("clean"), 0o600))
	outside := filepath.Join(filepath.Dir(r.dir), "outside.log")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(r.dir, "reports", "link.log")))
	return r
}

func TestAttachFilesStaysInsideProject(t *testing.T) {
	r := attachFixture(t)
	files, err := attachFiles(r.dir, Options{}.Read)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "a.log", filepath.Base(files[0]))
}

func TestPublishReplacesAndAttachesAsset(t *testing.T) {
	r := attachFixture(t)
	var deleted, uploaded []string
	var sent []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/repos/o/x/releases/tags/"+fixtureTag:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 7, "body": "Human.\n", "assets": []map[string]any{{"id": 5, "name": "a.log"}}})
		case req.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]string{})
		case req.Method == http.MethodDelete:
			deleted = append(deleted, req.URL.Path)
		case req.Method == http.MethodPost && req.URL.Path == "/repos/o/x/releases/7/assets":
			require.NoError(t, req.ParseMultipartForm(1<<20))
			file, header, err := req.FormFile("attachment")
			require.NoError(t, err)
			sent, _ = io.ReadAll(file)
			uploaded = append(uploaded, header.Filename+"|"+req.URL.Query().Get("name"))
		case req.Method == http.MethodPatch:
		default:
			t.Errorf("unexpected %s %s", req.Method, req.URL.Path)
		}
	}))
	defer srv.Close()
	po := PublishOptions{
		ForgeOptions: ForgeOptions{Options: Options{Dir: r.dir, Tag: fixtureTag, Prefix: "v"}, Forge: fixtureForge, Server: fixtureHost, Repo: fixtureRepo},
		API:          srv.URL, Token: "secret",
	}
	_, err := Publish(t.Context(), po)
	require.NoError(t, err)
	assert.Equal(t, []string{"/repos/o/x/releases/7/assets/5"}, deleted)
	assert.Equal(t, []string{"a.log|a.log"}, uploaded)
	assert.Equal(t, "clean", string(sent))
}

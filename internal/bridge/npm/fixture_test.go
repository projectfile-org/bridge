// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// Shared literals for the internal test package. The sibling npm_test.go keeps
// its own set for the external one, so the two packages name theirs apart.
const (
	fxNpmName        = "widget"
	fxEngineName     = "node"
	fxNpmScope       = "acme"
	fxNpmNamespaceNS = "npm.acme"
	fxNpmScopedName  = "@acme/widget"
	fxLinux          = "linux"
	fxDarwin         = "darwin"
	fxWindows        = "windows"
	fxWin32          = "win32"
	fxAMD64          = "amd64"
	fxArm64          = "arm64"
	fxRiscv          = "riscv64"
	fxX64            = "x64"
	fxIa32           = "ia32"
	fx386            = "386"
	fxEngineNode     = ">=20"
	fxEngineBump     = ">=22"
	fxLicenseMIT     = "MIT"
	fxRoleAuthor     = "author"
	fxRoleContrib    = "contributor"
	fxRoleMaintain   = "maintainer"
	fxGivenJean      = "Jean"
	fxGivenAda       = "Ada Lovelace"
	fxEmailA         = "a@x.test"
	fxEmailReal      = "real@example.test"
	fxEmailAda       = "ada@x.test"
	fxKeyURL         = "url"
	fxURLAda         = "https://ada.test"
	fxKeyType        = "type"
	fxKeyEmail       = "email"
	fxKeyCustom      = "custom"
	fxValueCustom    = "custom"
	fxValueMine      = "mine"
	fxMutated        = "mutated"
	fxPkgDir         = "pkg"
	fxVersionOne     = "1.0.0"
	fxVersionTwo     = "2.0.0"
	fxRepoURL        = "https://github.com/acme/widget.git"
	fxIssuesURL      = "https://example.test/issues"
	fxFundURL        = "https://example.test/fund"
	fxFundingURL     = "https://example.com/fund"
	fxPkgURL         = "https://example.test/pkg.git"
	fxURLExampleTest = "https://example.test"
	fxURLA           = "https://a.test"
	fxURLB           = "https://b.test"
	fxURLX           = "https://x.test"
	fxURLOnly        = "https://only.test"
	fxKeywordTest    = "test"
)

// writePkgJSON is this package's package.json writer; the npm_test.go helper of
// the same name lives in the external test package and is not reachable here.
func writePkgJSON(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, Filename), []byte(content), 0o644))
}

// pfFixture writes a projectfile beside package.json and returns its path, so a
// sync that mutates the document has a real file to reconcile back into.
func pfFixture(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

// mapperRow returns the entry for one package.json key, so a test names the
// field it exercises instead of indexing the list positionally.
func mapperRow(t *testing.T, doc *Document, pf *projectfile.Document, extKey string) core.FieldMapper {
	t.Helper()
	for _, m := range (Bridge{}).BuildMappers(doc, pf) {
		if m.ExtKey == extKey {
			return m
		}
	}
	t.Fatalf("no mapper for %s", extKey)
	return core.FieldMapper{}
}

// firstKey returns the single key of a one-entry map, so an assertion reads the
// engine name without depending on map iteration order.
func firstKey(m map[string]string) string {
	for k := range m {
		return k
	}
	return ""
}

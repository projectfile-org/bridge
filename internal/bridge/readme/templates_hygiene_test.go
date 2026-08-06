// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package readme

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplatesCarryNoLicenseHeader guards the fix for the header leak: block
// templates are content fragments concatenated under the document's own REUSE
// header, so a per-fragment header would render into the README. Compliance
// lives in a `.tmpl.license` sidecar instead — this asserts no embedded `.tmpl`
// body carries an SPDX line, catching a header pasted back in by habit.
func TestTemplatesCarryNoLicenseHeader(t *testing.T) {
	err := fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() || !strings.HasSuffix(path, tmplExt) {
			return nil
		}
		body, err := templatesFS.ReadFile(path)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "SPDX-",
			"%s ships a license header; move it to %s.license", path, path)
		return nil
	})
	require.NoError(t, err)
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package all_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	_ "projectfile.org/projectfile/bridge/internal/scanners/all"
	"projectfile.org/projectfile/bridge/internal/scanners/core"
)

func TestRunAllOnEmptyDirReportsNoFailure(t *testing.T) {
	_, _, err := core.RunAll(t.TempDir())
	assert.NoError(t, err)
}

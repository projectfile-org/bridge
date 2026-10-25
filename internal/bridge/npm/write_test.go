// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteRefusesAnUnmarshallableField pins that a typed field no encoder can render fails the write before any byte lands.
func TestWriteRefusesAnUnmarshallableField(t *testing.T) {
	dir := t.TempDir()
	doc := &Document{Name: fxNpmName, Version: fxVersionOne, Author: func() {}}

	err := Write(dir, doc)
	require.Error(t, err, "a func held in an any field cannot be marshalled")
	assert.ErrorContains(t, err, "marshal package.json document")
	assert.False(t, Exists(dir), "nothing is written when the typed view cannot be marshalled")
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// TestWrapLocalizedTextlintNonEnglish wraps any non-English language variant
// in the disable/enable pair, so the English-only dictionary rules do not
// flag legitimate translated prose. The pair must enclose the whole body and
// the enable directive must close the file.
func TestWrapLocalizedTextlintNonEnglish(t *testing.T) {
	out := string(core.WrapLocalizedTextlint([]byte("# Código de Conducta\n"), "es"))
	assert.Equal(t,
		"<!-- textlint-disable terminology,common-misspellings -->\n# Código de Conducta\n<!-- textlint-enable -->\n",
		out)
}

// TestWrapLocalizedTextlintEnglishPassthrough pins the English path as a no-op:
// the disabled rules are built for English copy, so the canonical file must
// come out byte-identical to its input.
func TestWrapLocalizedTextlintEnglishPassthrough(t *testing.T) {
	body := []byte("# Code of Conduct\n")
	assert.Equal(t, body, core.WrapLocalizedTextlint(body, "en"))
}

// TestWrapLocalizedTextlintPadsMissingTrailingNewline ensures a body without a
// final newline still lands the enable directive on its own line.
func TestWrapLocalizedTextlintPadsMissingTrailingNewline(t *testing.T) {
	out := string(core.WrapLocalizedTextlint([]byte("Політика"), "uk"))
	assert.Equal(t,
		"<!-- textlint-disable terminology,common-misspellings -->\nПолітика\n<!-- textlint-enable -->\n",
		out)
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "bytes"

// textlintDisable and textlintEnable bracket a body so the English-only
// terminology rule does not flag legitimate translated prose. The rule's
// dictionary is built for English copy, so a Spanish or Ukrainian sentence
// trips false positives on words that are simply correct in that language.
const (
	textlintDisable = "<!-- textlint-disable terminology -->"
	textlintEnable  = "<!-- textlint-enable -->"
)

// defaultLanguageEN is the BCP 47 tag whose generated copy the terminology
// rule is built for. Only variants whose content is NOT in this language get
// the disable/enable bracket. Compared by content language, not by
// default-language status: a Spanish-default project's Spanish root file is
// wrapped (its content is Spanish), while its docs/en/ variant is not.
const defaultLanguageEN = "en"

// WrapLocalizedTextlint brackets body with the textlint disable/enable pair
// when lang is a non-English language, and returns body unchanged for English.
// What we are trying to do: keep the workspace lint gate green for translated
// docs without disabling the rule globally, by scoping the disable to the one
// thing it must not touch — non-English prose.
func WrapLocalizedTextlint(body []byte, lang string) []byte {
	if lang == defaultLanguageEN {
		return body
	}
	out := make([]byte, 0, len(body)+len(textlintDisable)+len(textlintEnable)+2)
	out = append(out, textlintDisable...)
	out = append(out, '\n')
	out = append(out, body...)
	if !bytes.HasSuffix(body, []byte("\n")) {
		out = append(out, '\n')
	}
	out = append(out, textlintEnable...)
	out = append(out, '\n')
	return out
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package scaffold

import (
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/v4/internal/source"
)

// labels of the two required fields, reused across the prompt-model tests.
const (
	labelNamespace = "identity.namespace"
	labelName      = "identity.name"
)

// descriptions double as the prompt placeholders in these tests.
const (
	descNamespace = "Reverse-DNS owner handle"
	descName      = "URL-safe project slug"
)

// skipIfRealTerminal skips a test that would drive a real bubbletea program, which blocks on an attached TTY.
func skipIfRealTerminal(t *testing.T) {
	t.Helper()
	if f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		_ = f.Close()
		t.Skip("a real controlling terminal would block the interactive program")
	}
}

// keyPress builds the KeyPressMsg a terminal would send for one keystroke.
func keyPress(code rune, text string, mod tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Text: text, Mod: mod})
}

// printable sends one printable keystroke through the model.
func printable(m tea.Model, text string) tea.Model {
	next, _ := m.Update(keyPress('a', text, 0))
	return next
}

// press sends a special keystroke through the model and returns the resulting command.
func press(m tea.Model, code rune, mod tea.KeyMod) (tea.Model, tea.Cmd) {
	next, cmd := m.Update(keyPress(code, "", mod))
	return next, cmd
}

// requiredFields are the two MissingField entries every field prompt is built from.
func requiredFields(captured *[]string) []MissingField {
	return []MissingField{
		{Label: labelNamespace, Description: descNamespace, Default: fxNamespace, Setter: func(v string) { *captured = append(*captured, labelNamespace+"="+v) }},
		{Label: labelName, Description: descName, Setter: func(v string) { *captured = append(*captured, labelName+"="+v) }},
	}
}

// The format picker reports the terminal error it cannot avoid without a TTY.
func TestPromptFormatFailsWithoutTerminal(t *testing.T) {
	skipIfRealTerminal(t)

	_, err := promptFormat()

	require.Error(t, err)
}

// newFieldPromptModel wires one textinput per field, prefilling the default and styling the placeholder.
func TestNewFieldPromptModel(t *testing.T) {
	var captured []string
	m := newFieldPromptModel(requiredFields(&captured))

	require.Len(t, m.multi.Inputs(), 2)
	assert.Equal(t, []string{labelNamespace, labelName}, m.labels)
	assert.Equal(t, []string{descNamespace, descName}, m.descs)
	require.Len(t, m.setters, 2)
	assert.Equal(t, fxNamespace, m.multi.Input(0).Value(), "the default is prefilled as editable text")
	assert.Equal(t, descNamespace, m.multi.Input(0).Placeholder)
	assert.Equal(t, "", m.multi.Input(1).Value(), "a field with no default starts blank")
	assert.Equal(t, 0, m.multi.Focused(), "the first field takes focus")
}

// fieldPromptModel.Init arms the cursor blink.
func TestFieldPromptModelInitBlinks(t *testing.T) {
	var captured []string

	assert.NotNil(t, newFieldPromptModel(requiredFields(&captured)).Init())
}

// enter refuses to submit while any required input is blank and names the offender.
func TestFieldPromptModelRejectsBlankRequiredField(t *testing.T) {
	var captured []string
	m := newFieldPromptModel(requiredFields(&captured))

	next, cmd := press(m, tea.KeyEnter, 0)
	m = next.(fieldPromptModel)

	assert.Nil(t, cmd, "a rejected submit does not quit")
	assert.False(t, m.submitted)
	assert.Contains(t, m.err, labelName)
	assert.Equal(t, 1, m.multi.Focused(), "focus jumps to the blank field")
	assert.Empty(t, captured, "no setter ran")
}

// Typing into the focused input, then enter, submits every value the model holds.
func TestFieldPromptModelSubmitsFilledFields(t *testing.T) {
	var captured []string
	m := newFieldPromptModel(requiredFields(&captured))

	next, _ := press(m, tea.KeyEnter, 0)
	m = next.(fieldPromptModel)
	require.False(t, m.submitted, "the blank second field blocks the first submit")
	require.Equal(t, 1, m.multi.Focused(), "focus jumped to the blank field")

	m = printable(m, "my-tool").(fieldPromptModel)
	next, cmd := press(m, tea.KeyEnter, 0)
	m = next.(fieldPromptModel)

	assert.NotNil(t, cmd, "a complete submit quits the program")
	assert.True(t, m.submitted)
	assert.Equal(t, fxNamespace, m.multi.Input(0).Value(), "the prefilled default survived")
	assert.Equal(t, "my-tool", m.multi.Input(1).Value(), "the typed value landed in the focused field")
}

// Navigating away from a rejected submit clears the error it left behind.
func TestFieldPromptModelClearsErrorOnNavigation(t *testing.T) {
	var captured []string
	m := newFieldPromptModel(requiredFields(&captured))

	next, _ := press(m, tea.KeyEnter, 0)
	require.NotEmpty(t, next.(fieldPromptModel).err, "the rejected submit set an error")

	next, _ = press(next, tea.KeyTab, 0)
	assert.Empty(t, next.(fieldPromptModel).err, "navigation clears the stale error")
}

// ctrl+c and esc both abandon the prompt without submitting.
func TestFieldPromptModelQuitsOnCancel(t *testing.T) {
	for _, cancel := range []struct {
		name string
		code rune
		mod  tea.KeyMod
	}{
		{"ctrl+c", 'c', tea.ModCtrl},
		{"esc", tea.KeyEsc, 0},
	} {
		t.Run(cancel.name, func(t *testing.T) {
			var captured []string
			m := newFieldPromptModel(requiredFields(&captured))

			next, cmd := press(m, cancel.code, cancel.mod)
			got := next.(fieldPromptModel)

			assert.NotNil(t, cmd, "%s quits the program", cancel.name)
			assert.False(t, got.submitted)
		})
	}
}

// tab/down and shift+tab/up walk the focus through every field.
func TestFieldPromptModelNavigation(t *testing.T) {
	for _, nav := range []struct {
		name string
		code rune
		mod  tea.KeyMod
		want int
	}{
		{"tab forward", tea.KeyTab, 0, 1},
		{"down forward", tea.KeyDown, 0, 1},
		{"shift+tab back", tea.KeyTab, tea.ModShift, 1},
		{"up back", tea.KeyUp, 0, 1},
	} {
		t.Run(nav.name, func(t *testing.T) {
			var captured []string
			m := newFieldPromptModel(requiredFields(&captured))

			next, cmd := press(m, nav.code, nav.mod)
			got := next.(fieldPromptModel)

			assert.Nil(t, cmd, "navigation does not quit")
			assert.Equal(t, nav.want, got.multi.Focused(), "focus wraps from the last field back to the first")
		})
	}
}

// The view lists every label and description and surfaces the error line.
func TestFieldPromptModelView(t *testing.T) {
	var captured []string
	m := newFieldPromptModel(requiredFields(&captured))
	m.err = labelNamespace + " is required"

	view := m.View().Content

	assert.Contains(t, view, "Provide required fields:")
	assert.Contains(t, view, labelNamespace+":")
	assert.Contains(t, view, labelName+":")
	assert.Contains(t, view, descNamespace)
	assert.Contains(t, view, m.err)
	assert.Contains(t, view, "tab/↑↓ next field, enter confirm, ctrl+c cancel")
}

// newOptionalFieldsModel focuses exactly the fields the Partial still lacks.
func TestNewOptionalFieldsModel(t *testing.T) {
	blank := newOptionalFieldsModel(&source.Partial{})
	assert.Equal(t, 0, blank.multi.Focused(), "license is the first focusable field when nothing is set")

	licensed := newOptionalFieldsModel(&source.Partial{License: strPtr(fxLicense)})
	assert.Equal(t, 1, licensed.multi.Focused(), "a set license is skipped in the focus order")

	complete := newOptionalFieldsModel(&source.Partial{License: strPtr(fxLicense), Title: &source.LocalizedString{Bare: fxTitle}, Summary: &source.LocalizedString{Bare: fxSummary}})
	assert.Len(t, complete.multi.Inputs(), 3, "the three inputs are always built")
}

// optionalFieldsModel.Init arms the cursor blink.
func TestOptionalFieldsModelInitBlinks(t *testing.T) {
	assert.NotNil(t, newOptionalFieldsModel(&source.Partial{}).Init())
}

// enter accepts everything typed, ctrl+c/esc abandon, and tab walks the focus.
func TestOptionalFieldsModelUpdate(t *testing.T) {
	m := newOptionalFieldsModel(&source.Partial{License: strPtr(fxLicense)})
	m = printable(m, fxTitle).(optionalFieldsModel)

	next, cmd := press(m, tea.KeyEnter, 0)
	m = next.(optionalFieldsModel)
	assert.True(t, m.submitted, "enter accepts every field at once")
	assert.NotNil(t, cmd, "enter quits the program")
	assert.Equal(t, fxTitle, m.multi.Input(optTitleIdx).Value(), "the typed title landed in its input")

	fresh := newOptionalFieldsModel(&source.Partial{})
	next, cmd = press(fresh, 'c', tea.ModCtrl)
	assert.NotNil(t, cmd, "ctrl+c quits the program")
	assert.False(t, next.(optionalFieldsModel).submitted)

	next, cmd = press(fresh, tea.KeyEsc, 0)
	assert.NotNil(t, cmd, "esc quits the program")
	assert.False(t, next.(optionalFieldsModel).submitted)

	next, cmd = press(fresh, tea.KeyTab, 0)
	assert.Nil(t, cmd, "tab only moves focus")
	assert.Equal(t, 1, next.(optionalFieldsModel).multi.Focused())

	back, cmd := press(next, tea.KeyTab, tea.ModShift)
	assert.Nil(t, cmd, "shift+tab only moves focus")
	assert.Equal(t, 0, back.(optionalFieldsModel).multi.Focused(), "shift+tab walks back")
}

// The optional view shows an input for a missing field and the sourced value for a known one.
func TestOptionalFieldsModelView(t *testing.T) {
	p := &source.Partial{License: strPtr(fxLicense), Title: &source.LocalizedString{Bare: fxTitle}}
	m := newOptionalFieldsModel(p)

	view := m.View().Content

	assert.Contains(t, view, "Optional fields")
	assert.Contains(t, view, "("+fxLicense+" from source)")
	assert.Contains(t, view, "(My Tool from source)")
	assert.Contains(t, view, "identity.summary (one-line pitch):", "a missing field keeps its input prompt")
	assert.Contains(t, view, "license.spdx:")
	assert.Contains(t, view, "tab switch, enter accept all, ctrl+c cancel")

	blank := newOptionalFieldsModel(&source.Partial{}).View().Content
	assert.Contains(t, blank, "identity.title (human-readable name):")
	assert.NotContains(t, blank, "from source", "no sourced value is annotated when nothing is set")

	summarised := newOptionalFieldsModel(&source.Partial{Summary: &source.LocalizedString{Bare: fxSummary}}).View().Content
	assert.Contains(t, summarised, "("+fxSummary+" from source)")
}

// Each row takes the focus marker in turn, so the view reflects the navigated focus.
func TestOptionalFieldsModelViewMarksFocusedRow(t *testing.T) {
	m := newOptionalFieldsModel(&source.Partial{})

	for step, want := range []string{"identity.title (human-readable name):", "identity.summary (one-line pitch):"} {
		next, _ := press(m, tea.KeyTab, 0)
		m = next.(optionalFieldsModel)
		view := m.View().Content
		assert.Contains(t, view, ">", "step %d keeps the focus marker", step)
		assert.Contains(t, view, want)
	}
}

// extractLS resolves the display string for every LocalizedString shape.
func TestExtractLocalizedString(t *testing.T) {
	assert.Equal(t, "", extractLS(nil))
	assert.Equal(t, fxTitle, extractLS(&source.LocalizedString{Bare: fxTitle}))
	assert.Equal(t, fxTitle, extractLS(&source.LocalizedString{Langs: map[string]string{"en": fxTitle, "es": fxTitleES}}))
	assert.Equal(t, fxTitleES, extractLS(&source.LocalizedString{Langs: map[string]string{"es": fxTitleES}}))
	assert.Equal(t, "", extractLS(&source.LocalizedString{}))
}

// When every optional field already carries a value the prompt is skipped entirely.
func TestPromptOptionalFieldsSkipsWhenAllSet(t *testing.T) {
	p := &source.Partial{
		License: strPtr(fxLicense),
		Title:   &source.LocalizedString{Bare: fxTitle},
		Summary: &source.LocalizedString{Bare: fxSummary},
	}

	require.NoError(t, promptOptionalFields(p))

	assert.Equal(t, fxLicense, *p.License, "a skipped prompt leaves the partial untouched")
}

// Without a terminal the optional-fields prompt fails instead of hanging.
func TestPromptOptionalFieldsFailsWithoutTerminal(t *testing.T) {
	skipIfRealTerminal(t)

	err := promptOptionalFields(&source.Partial{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt")
}

// Without a terminal the required-fields prompt fails instead of hanging.
func TestPromptMissingFieldsFailsWithoutTerminal(t *testing.T) {
	skipIfRealTerminal(t)
	var captured []string

	err := promptMissingFields(&source.Partial{}, requiredFields(&captured))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt")
	assert.Empty(t, captured, "a failed prompt set no field")
}

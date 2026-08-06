// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package scaffold

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kiota.ch/projectfile/core/v2/pkg/selector"
	"projectfile.org/projectfile/bridge/internal/source"
)

var (
	highlight = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	dimmed    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	label     = lipgloss.NewStyle().Bold(true)
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type formatItem struct {
	name string
	desc string
}

var formatItems = []formatItem{
	{name: formatYAML, desc: "YAML (recommended)"},
	{name: formatTOML, desc: "TOML"},
	{name: "json", desc: "JSON"},
}

// promptFormat delegates to the shared selector. Same UX as the previous
// inline bubbletea model — extracting kept the keybindings (up/down/j/k,
// enter, q/esc) and the colour scheme identical.
func promptFormat() (string, error) {
	chosen, err := selector.Run(selector.Choices[formatItem]{
		Title:  "Select output format:",
		Items:  formatItems,
		Label:  func(f formatItem) string { return f.name },
		Detail: func(f formatItem) string { return f.desc },
	})
	if err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			return "", fmt.Errorf("cancelled")
		}
		return "", err
	}
	return chosen.name, nil
}

type fieldPromptModel struct {
	multi     *selector.MultiInput
	labels    []string
	descs     []string
	submitted bool
	err       string
	setters   []func(string)
}

func newFieldPromptModel(fields []MissingField) fieldPromptModel {
	inputs := make([]textinput.Model, len(fields))
	labels := make([]string, len(fields))
	descs := make([]string, len(fields))
	setters := make([]func(string), len(fields))

	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.Description
		ti.PromptStyle = highlight
		ti.Width = 40
		// Prefill carries forward user-config defaults as editable text —
		// not as a placeholder (which would vanish on first keystroke).
		// CursorEnd so the user lands ready to extend or backspace through
		// the value instead of typing inside it.
		if f.Default != "" {
			ti.SetValue(f.Default)
			ti.CursorEnd()
		}
		inputs[i] = ti
		labels[i] = f.Label
		descs[i] = f.Description
		setters[i] = f.Setter
	}

	return fieldPromptModel{
		multi:   selector.NewMultiInput(inputs, nil),
		labels:  labels,
		descs:   descs,
		setters: setters,
	}
}

func (m fieldPromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m fieldPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			for i, ti := range m.multi.Inputs() {
				if strings.TrimSpace(ti.Value()) == "" {
					m.err = fmt.Sprintf("%s is required", m.labels[i])
					m.multi.JumpTo(i)
					return m, nil
				}
			}
			m.submitted = true
			return m, tea.Quit
		case tea.KeyTab, tea.KeyDown:
			m.err = ""
			m.multi.Next()
			return m, nil
		case tea.KeyShiftTab, tea.KeyUp:
			m.err = ""
			m.multi.Prev()
			return m, nil
		}
	}

	cmd := m.multi.Update(msg)
	return m, cmd
}

func (m fieldPromptModel) View() string {
	var b strings.Builder
	b.WriteString("\n  Provide required fields:\n\n")
	focused := m.multi.Focused()
	for i, ti := range m.multi.Inputs() {
		focus := "  "
		if focused == i {
			focus = highlight.Render("> ")
		}
		fmt.Fprintf(&b, "  %s %s\n", focus, label.Render(m.labels[i]+":"))
		fmt.Fprintf(&b, "    %s\n", ti.View())
		fmt.Fprintf(&b, "    %s\n\n", dimmed.Render(m.descs[i]))
	}
	if m.err != "" {
		fmt.Fprintf(&b, "  %s\n", errStyle.Render(m.err))
	}
	b.WriteString("  tab/↑↓ next field, enter confirm, ctrl+c cancel\n")
	return b.String()
}

func promptMissingFields(_ *source.Partial, fields []MissingField) error {
	model := newFieldPromptModel(fields)
	prog := tea.NewProgram(model)
	m, err := prog.Run()
	if err != nil {
		return fmt.Errorf("prompt: %w", err)
	}
	fm := m.(fieldPromptModel)
	if !fm.submitted {
		return fmt.Errorf("cancelled")
	}

	for i, ti := range fm.multi.Inputs() {
		fm.setters[i](strings.TrimSpace(ti.Value()))
	}

	return nil
}

// Indices into the optional-fields MultiInput. Named so the View can match
// them to label/Partial-field pairs without magic numbers.
const (
	optLicenseIdx = 0
	optTitleIdx   = 1
	optSummaryIdx = 2
)

type optionalFieldsModel struct {
	multi     *selector.MultiInput
	submitted bool
	partial   *source.Partial
}

func newOptionalFieldsModel(p *source.Partial) optionalFieldsModel {
	li := textinput.New()
	li.Placeholder = "e.g. MIT, Apache-2.0"
	li.Width = 40
	ti := textinput.New()
	ti.Placeholder = "e.g. My Cool Project"
	ti.Width = 40
	si := textinput.New()
	si.Placeholder = "e.g. A tool that does X for Y"
	si.Width = 60

	var focusable []int
	if p.License == nil || *p.License == "" {
		focusable = append(focusable, optLicenseIdx)
	}
	if p.Title == nil {
		focusable = append(focusable, optTitleIdx)
	}
	if p.Summary == nil {
		focusable = append(focusable, optSummaryIdx)
	}

	return optionalFieldsModel{
		multi:   selector.NewMultiInput([]textinput.Model{li, ti, si}, focusable),
		partial: p,
	}
}

func (m optionalFieldsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m optionalFieldsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			m.submitted = true
			return m, tea.Quit
		case tea.KeyTab:
			m.multi.Next()
			return m, nil
		case tea.KeyShiftTab:
			m.multi.Prev()
			return m, nil
		}
	}

	cmd := m.multi.Update(msg)
	return m, cmd
}

func (m optionalFieldsModel) View() string {
	var b strings.Builder
	b.WriteString("\n  Optional fields (enter to accept, leave blank to skip):\n\n")
	focused := m.multi.Focused()

	focus0 := "  "
	if focused == optLicenseIdx {
		focus0 = highlight.Render("> ")
	}
	licenseLabel := "license.spdx:"
	if m.partial.License != nil && *m.partial.License != "" {
		licenseLabel = fmt.Sprintf("license.spdx: %s", dimmed.Render("("+*m.partial.License+" from source)"))
		fmt.Fprintf(&b, "  %s %s\n\n", focus0, dimmed.Render(licenseLabel))
	} else {
		fmt.Fprintf(&b, "  %s %s\n", focus0, label.Render(licenseLabel))
		fmt.Fprintf(&b, "    %s\n\n", m.multi.Input(optLicenseIdx).View())
	}

	focus1 := "  "
	if focused == optTitleIdx {
		focus1 = highlight.Render("> ")
	}
	titleLabel := "identity.title:"
	if m.partial.Title != nil {
		titleLabel = fmt.Sprintf("identity.title: %s", dimmed.Render("("+extractLS(m.partial.Title)+" from source)"))
		fmt.Fprintf(&b, "  %s %s\n\n", focus1, dimmed.Render(titleLabel))
	} else {
		fmt.Fprintf(&b, "  %s %s\n", focus1, label.Render(titleLabel))
		fmt.Fprintf(&b, "    %s\n\n", m.multi.Input(optTitleIdx).View())
	}

	focus2 := "  "
	if focused == optSummaryIdx {
		focus2 = highlight.Render("> ")
	}
	summaryLabel := "identity.summary:"
	if m.partial.Summary != nil {
		summaryLabel = fmt.Sprintf("identity.summary: %s", dimmed.Render("("+extractLS(m.partial.Summary)+" from source)"))
		fmt.Fprintf(&b, "  %s %s\n\n", focus2, dimmed.Render(summaryLabel))
	} else {
		fmt.Fprintf(&b, "  %s %s\n", focus2, label.Render(summaryLabel))
		fmt.Fprintf(&b, "    %s\n\n", m.multi.Input(optSummaryIdx).View())
	}

	b.WriteString("  tab switch, enter accept all, ctrl+c cancel\n")
	return b.String()
}

func extractLS(ls *source.LocalizedString) string {
	if ls == nil {
		return ""
	}
	if ls.Bare != "" {
		return ls.Bare
	}
	if v, ok := ls.Langs["en"]; ok {
		return v
	}
	for _, v := range ls.Langs {
		return v
	}
	return ""
}

func promptOptionalFields(p *source.Partial) error {
	licenseAlreadySet := p.License != nil && *p.License != ""
	titleAlreadySet := p.Title != nil
	summaryAlreadySet := p.Summary != nil

	if licenseAlreadySet && titleAlreadySet && summaryAlreadySet {
		return nil
	}

	model := newOptionalFieldsModel(p)
	prog := tea.NewProgram(model)
	m, err := prog.Run()
	if err != nil {
		return fmt.Errorf("prompt: %w", err)
	}
	fm := m.(optionalFieldsModel)
	if !fm.submitted {
		os.Exit(0)
	}

	if v := strings.TrimSpace(fm.multi.Input(optLicenseIdx).Value()); v != "" && !licenseAlreadySet {
		p.License = source.StringPtr(v)
	}
	if v := strings.TrimSpace(fm.multi.Input(optTitleIdx).Value()); v != "" && !titleAlreadySet {
		p.Title = &source.LocalizedString{Bare: v}
	}
	if v := strings.TrimSpace(fm.multi.Input(optSummaryIdx).Value()); v != "" && !summaryAlreadySet {
		p.Summary = &source.LocalizedString{Bare: v}
	}

	return nil
}

package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModalField identifies a focusable field within the unified nucleus modal.
type ModalField int

const (
	ModalFieldProfile ModalField = iota // inline profile picker (skipped when no profiles)
	ModalFieldTask                      // task description text input
)

// NucleusModalContext carries pre-filled data when the modal is opened.
type NucleusModalContext struct {
	JiraKey     string
	JiraSummary string
}

// ModalRequest signals to the parent model what action to take after an Update.
// At most one field is non-zero per call.
type ModalRequest struct {
	Submit *ModalSubmit // non-nil: user confirmed the form
	Cancel bool         // user pressed Esc / Ctrl-C
}

// ModalSubmit carries the confirmed form values.
type ModalSubmit struct {
	Task    string
	Profile string
	JiraKey string
}

// NucleusModal is a self-contained form widget for creating a nucleus.
// It does not call any services; it communicates with the parent model via
// ModalRequest return values from Update.
type NucleusModal struct {
	focused ModalField

	taskInput textinput.Model

	profileNames  []string
	profilePaths  map[string]string
	profileCursor int
	profilesReady bool

	jiraKey string

	err string

	width int
}

// NewNucleusModal creates an initialised modal with default values.
func NewNucleusModal(width int) NucleusModal {
	task := textinput.New()
	task.Placeholder = "task description"
	task.CharLimit = 120

	return NucleusModal{
		focused:   ModalFieldTask,
		taskInput: task,
		width:     width,
	}
}

// Open resets the modal state and pre-fills it from ctx. It returns the updated
// modal and a tea.Cmd to start cursor blinking in the active text input.
func (m NucleusModal) Open(ctx NucleusModalContext) (NucleusModal, tea.Cmd) {
	m.jiraKey = ctx.JiraKey
	m.err = ""
	m.profileCursor = 0

	// Reset inputs.
	m.taskInput.Reset()
	m.taskInput.Blur()

	// Pre-fill from context.
	if ctx.JiraSummary != "" {
		m.taskInput.SetValue(ctx.JiraSummary)
		m.taskInput.CursorEnd()
	}

	// Focus on the task field immediately.
	m.focused = ModalFieldTask
	updated, cmd := m.focusField()
	return updated, cmd
}

// SetProfiles provides the available Claude profiles.
func (m NucleusModal) SetProfiles(names []string, paths map[string]string) NucleusModal {
	m.profileNames = names
	m.profilePaths = paths
	m.profilesReady = true
	if m.profileCursor >= len(names) {
		m.profileCursor = 0
	}
	return m
}

// visibleFields returns the ordered list of fields shown in the modal.
func (m NucleusModal) visibleFields() []ModalField {
	var fields []ModalField
	if len(m.profileNames) > 0 {
		fields = append(fields, ModalFieldProfile)
	}
	fields = append(fields, ModalFieldTask)
	return fields
}

// advanceFocus moves focus to the next visible field (wrapping).
func (m NucleusModal) advanceFocus() (NucleusModal, tea.Cmd) {
	return m.moveFocus(1)
}

// retreatFocus moves focus to the previous visible field (wrapping).
func (m NucleusModal) retreatFocus() (NucleusModal, tea.Cmd) {
	return m.moveFocus(-1)
}

func (m NucleusModal) moveFocus(dir int) (NucleusModal, tea.Cmd) {
	fields := m.visibleFields()
	if len(fields) == 0 {
		return m, nil
	}
	cur := 0
	for i, f := range fields {
		if f == m.focused {
			cur = i
			break
		}
	}
	next := ((cur + dir) % len(fields) + len(fields)) % len(fields)
	m = m.blurField()
	m.focused = fields[next]
	return m.focusField()
}

// blurField blurs the currently focused text input (if any).
func (m NucleusModal) blurField() NucleusModal {
	if m.focused == ModalFieldTask {
		m.taskInput.Blur()
	}
	return m
}

// focusField activates the text input for the currently focused field.
// Returns the updated modal and a blink cmd when a text input is focused.
func (m NucleusModal) focusField() (NucleusModal, tea.Cmd) {
	if m.focused == ModalFieldTask {
		cmd := m.taskInput.Focus()
		return m, cmd
	}
	return m, nil
}

// selectedProfile returns the display name of the currently selected profile,
// or "" when no profiles are configured.
func (m NucleusModal) selectedProfile() string {
	if len(m.profileNames) == 0 || m.profileCursor >= len(m.profileNames) {
		return ""
	}
	return m.profileNames[m.profileCursor]
}

// Update handles a single key event and returns the updated modal, a request for
// the parent, and any cmd produced (e.g. cursor blinking from text inputs).
func (m NucleusModal) Update(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m = m.blurField()
		return m, ModalRequest{Cancel: true}, nil

	case "enter":
		return m.trySubmit()

	case "tab":
		updated, cmd := m.advanceFocus()
		return updated, ModalRequest{}, cmd

	case "shift+tab":
		updated, cmd := m.retreatFocus()
		return updated, ModalRequest{}, cmd
	}

	switch m.focused {
	case ModalFieldProfile:
		return m.updateProfileField(msg)
	case ModalFieldTask:
		return m.updateTaskField(msg)
	}

	return m, ModalRequest{}, nil
}

func (m NucleusModal) updateProfileField(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	switch msg.String() {
	case "k":
		if m.profileCursor > 0 {
			m.profileCursor--
		}
	case "j":
		if m.profileCursor < len(m.profileNames)-1 {
			m.profileCursor++
		}
	}
	switch msg.Type {
	case tea.KeyUp:
		if m.profileCursor > 0 {
			m.profileCursor--
		}
	case tea.KeyDown:
		if m.profileCursor < len(m.profileNames)-1 {
			m.profileCursor++
		}
	}
	return m, ModalRequest{}, nil
}

func (m NucleusModal) updateTaskField(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	var cmd tea.Cmd
	m.taskInput, cmd = m.taskInput.Update(msg)
	return m, ModalRequest{}, cmd
}

// trySubmit validates the form and returns a Submit ModalRequest on success.
func (m NucleusModal) trySubmit() (NucleusModal, ModalRequest, tea.Cmd) {
	task := strings.TrimSpace(m.taskInput.Value())
	if task == "" {
		m.err = "task description is required"
		m = m.blurField()
		m.focused = ModalFieldTask
		updated, cmd := m.focusField()
		return updated, ModalRequest{}, cmd
	}

	return m, ModalRequest{Submit: &ModalSubmit{
		Task:    task,
		Profile: m.selectedProfile(),
		JiraKey: m.jiraKey,
	}}, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

// View renders the modal content. The parent wraps this in renderOverlay.
func (m NucleusModal) View() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("New Nucleus") + "\n\n")

	if len(m.profileNames) > 0 {
		m.renderProfileField(&sb)
		sb.WriteString("\n\n")
	}

	if m.jiraKey != "" {
		sb.WriteString(StyleLabel.Render("Jira") + StyleValue.Render(m.jiraKey) + "\n\n")
	}

	m.renderTaskField(&sb)

	if m.err != "" {
		sb.WriteString("\n\n")
		sb.WriteString(StyleError.Render(m.err))
	}

	sb.WriteString("\n\n")
	sb.WriteString(
		StyleDim.Render("tab") + " next   " +
			StyleDim.Render("shift+tab") + " prev   " +
			StyleDim.Render("enter") + " create   " +
			StyleDim.Render("esc") + " cancel",
	)

	return sb.String()
}

// fieldLabel renders the label for a field, highlighted when focused.
func (m NucleusModal) fieldLabel(field ModalField, text string) string {
	if m.focused == field {
		return lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render(text)
	}
	return StyleLabel.Render(text)
}

func (m NucleusModal) renderProfileField(sb *strings.Builder) {
	sb.WriteString(m.fieldLabel(ModalFieldProfile, "Profile") + "\n")
	for i, name := range m.profileNames {
		path := ""
		if m.profilePaths != nil {
			path = m.profilePaths[name]
		}
		if i == m.profileCursor {
			row := "  > " + truncate(name, 22) + "  " + path
			if m.focused == ModalFieldProfile {
				sb.WriteString(StyleSelected.Render(row) + "\n")
			} else {
				sb.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Render(row) + "\n")
			}
		} else {
			sb.WriteString("    " + truncate(name, 22) + "  " + StyleDim.Render(path) + "\n")
		}
	}
}

func (m NucleusModal) renderTaskField(sb *strings.Builder) {
	sb.WriteString(m.fieldLabel(ModalFieldTask, "Task") + "\n")
	sb.WriteString(m.taskInput.View())
}

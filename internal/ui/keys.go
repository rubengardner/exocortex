package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap holds every key binding used by the TUI.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Goto      key.Binding
	Nvim      key.Binding
	CloseNvim key.Binding
	New       key.Binding
	Delete    key.Binding
	Refresh   key.Binding
	Confirm   key.Binding
	Cancel    key.Binding
	Help      key.Binding
	Quit      key.Binding
	Submit    key.Binding
	NextField key.Binding
	Respawn       key.Binding
	TogglePreview key.Binding
	Board         key.Binding
	GitHub        key.Binding
	AddNeuron     key.Binding
	OpenBrowser   key.Binding
	Filter        key.Binding
}

// DefaultKeys returns the standard key bindings.
func DefaultKeys() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Goto: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "goto ClaudeCode"),
		),
		Nvim: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "open nvim"),
		),
		CloseNvim: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "close nvim"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new agent"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "remove"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "cancel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "submit"),
		),
		NextField: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next field"),
		),
		Respawn: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "respawn / review"),
		),
		TogglePreview: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "toggle preview"),
		),
		Board: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "jira board"),
		),
		GitHub: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "github PRs"),
		),
		AddNeuron: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add neuron"),
		),
		OpenBrowser: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter PRs"),
		),
	}
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Goto, k.Nvim, k.New, k.Delete, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Goto, k.Nvim, k.CloseNvim},
		{k.New, k.Delete, k.Refresh, k.AddNeuron},
		{k.Respawn, k.TogglePreview, k.Board, k.GitHub, k.Help, k.Quit},
	}
}

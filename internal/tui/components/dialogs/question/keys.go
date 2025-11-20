package question

import "charm.land/bubbles/v2/key"

type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Space     key.Binding
	Enter     key.Binding
	Cancel    key.Binding
	Prev      key.Binding
	Next      key.Binding
	Backspace key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Space: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "next/submit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Prev: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "previous question"),
		),
		Next: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next question"),
		),
		Backspace: key.NewBinding(
			key.WithKeys("backspace"),
			key.WithHelp("backspace", "delete character"),
		),
	}
}

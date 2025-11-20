package planmode

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Yes        key.Binding
	No         key.Binding
	Close      key.Binding
	LeftRight  key.Binding
	EnterSpace key.Binding
	Tab        key.Binding
}

func DefaultKeymap() KeyMap {
	return KeyMap{
		Yes: key.NewBinding(
			key.WithKeys("y", "Y"),
		),
		No: key.NewBinding(
			key.WithKeys("n", "N"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
		),
		LeftRight: key.NewBinding(
			key.WithKeys("left", "right", "h", "l"),
		),
		EnterSpace: key.NewBinding(
			key.WithKeys("enter", " "),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
		),
	}
}

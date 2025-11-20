package editor

import (
	"charm.land/bubbles/v2/key"
)

type EditorKeyMap struct {
	AddFile        key.Binding
	SendMessage    key.Binding
	OpenEditor     key.Binding
	Newline        key.Binding
	ToggleThinking key.Binding
}

func DefaultEditorKeyMap() EditorKeyMap {
	return EditorKeyMap{
		AddFile: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "add file"),
		),
		SendMessage: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send"),
		),
		OpenEditor: key.NewBinding(
			key.WithKeys("ctrl+o"),
			key.WithHelp("ctrl+o", "open editor"),
		),
		Newline: key.NewBinding(
			key.WithKeys("shift+enter", "ctrl+j", "alt+enter"),
			// "ctrl+j" is a common keybinding for newline in many editors.
			// "alt+enter" works on macOS Terminal.
			// "shift+enter" works when terminal is configured (via terminal-setup).
			key.WithHelp("shift+enter/ctrl+j", "newline"),
		),
		ToggleThinking: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle thinking"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k EditorKeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.AddFile,
		k.SendMessage,
		k.OpenEditor,
		k.Newline,
		k.ToggleThinking,
		AttachmentsKeyMaps.AttachmentDeleteMode,
		AttachmentsKeyMaps.DeleteAllAttachments,
		AttachmentsKeyMaps.Escape,
	}
}

type DeleteAttachmentKeyMaps struct {
	AttachmentDeleteMode key.Binding
	Escape               key.Binding
	DeleteAllAttachments key.Binding
}

// TODO: update this to use the new keymap concepts
var AttachmentsKeyMaps = DeleteAttachmentKeyMaps{
	AttachmentDeleteMode: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r+{i}", "delete attachment at index i"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc", "alt+esc"),
		key.WithHelp("esc", "cancel delete mode"),
	),
	DeleteAllAttachments: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("ctrl+r+r", "delete all attachments"),
	),
}

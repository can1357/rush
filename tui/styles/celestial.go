package styles

import "charm.land/lipgloss/v2"

func NewCelestialTheme() *Theme {
	t := &Theme{
		Name:   "celestial",
		IsDark: true,

		Primary:   ParseHex("#E95378"), // activeBorder/progressBar.background
		Secondary: ParseHex("#FAB795"), // strings/terminal yellow
		Tertiary:  ParseHex("#25B0BC"), // terminal blue (from ansiBlue)
		Accent:    ParseHex("#B877DB"), // keywords/storage

		// Backgrounds - mapped from VSCode theme
		BgBase:        ParseHex("#0b0c0f"), // editor.background
		BgBaseLighter: ParseHex("#0E0F13"), // activityBar.background
		BgSubtle:      ParseHex("#1b1c25"), // editorWidget.background
		BgOverlay:     ParseHex("#2E303E"), // button.background

		// Foregrounds - from VSCode theme
		FgBase:      ParseHex("#D5D8DA"),   // foreground
		FgMuted:     ParseHex("#6C6F93"),   // comments (from tokenColors)
		FgHalfMuted: ParseHex("#BBBBBB"),   // CSS properties
		FgSubtle:    ParseHex("#D5D8DA80"), // sideBar.foreground
		FgSelected:  ParseHex("#D5D8DA"),   // list.activeSelectionForeground

		// Borders
		Border:      ParseHex("#22252e"), // focusBorder
		BorderFocus: ParseHex("#E95378"), // tab.activeBorder

		// Status colors
		Success: ParseHex("#27D797"), // gitDecoration.untrackedResourceForeground
		Error:   ParseHex("#F43E5C"), // errorForeground
		Warning: ParseHex("#FAB795"), // terminal.ansiYellow
		Info:    ParseHex("#25B0BC"), // from terminal blues

		// Colors
		White: ParseHex("#FFFFFF"),

		// Blues - from terminal colors
		BlueLight: ParseHex("#3FC4DE"), // terminal.ansiBrightBlue
		BlueDark:  ParseHex("#26BBD9"), // terminal.ansiBlue
		Blue:      ParseHex("#25B0BC"), // similar to ansiBlue

		// Yellows - from terminal colors
		Yellow: ParseHex("#FBC3A7"), // terminal.ansiBrightYellow
		Citron: ParseHex("#FAC29A"), // entity.name

		// Greens - from terminal colors
		Green:      ParseHex("#29D398"), // terminal.ansiGreen
		GreenDark:  ParseHex("#27D797"), // list.warningForeground (green tint)
		GreenLight: ParseHex("#3FDAA4"), // terminal.ansiBrightGreen

		// Reds - from terminal colors
		Red:      ParseHex("#E95678"), // terminal.ansiRed
		RedDark:  ParseHex("#F43E5C"), // errorForeground
		RedLight: ParseHex("#EC6A88"), // terminal.ansiBrightRed
		Cherry:   ParseHex("#E95378"), // primary red

		// Editor-specific colors - directly from VSCode theme
		EditorLineNumber:         ParseHex("#D5D8DA1A"), // editorLineNumber.foreground
		EditorLineNumberActive:   ParseHex("#D5D8DA80"), // editorLineNumber.activeForeground
		EditorCursor:             ParseHex("#E95378"),   // editorCursor.foreground
		EditorSelection:          ParseHex("#2E303EB3"), // editor.selectionBackground
		EditorSelectionHighlight: ParseHex("#6C6F934D"), // editor.selectionHighlightBackground
		EditorWordHighlight:      ParseHex("#6C6F9380"), // editor.wordHighlightBackground
		EditorFindMatch:          ParseHex("#6C6F9380"), // editor.findMatchBackground
		EditorFindMatchHighlight: ParseHex("#6C6F934D"), // editor.findMatchHighlightBackground
		EditorLineHighlight:      ParseHex("#2E303E4D"), // editor.lineHighlightBackground

		// Diff colors - from VSCode theme
		DiffInsertedTextBg:  ParseHex("#09F7A01A"), // diffEditor.insertedTextBackground
		DiffRemovedTextBg:   ParseHex("#F43E5C1A"), // diffEditor.removedTextBackground
		DiffInsertLineBg:    ParseHex("#09F7A01A"), // Same as inserted text
		DiffDeleteLineBg:    ParseHex("#F43E5C1A"), // Same as removed text
		DiffInsertLineNumBg: ParseHex("#09F7A0B3"), // editorGutter.addedBackground
		DiffDeleteLineNumBg: ParseHex("#F43E5CB3"), // editorGutter.deletedBackground

		// Terminal ANSI colors - directly from VSCode theme
		AnsiBlue:          ParseHex("#26BBD9"), // terminal.ansiBlue
		AnsiBrightBlue:    ParseHex("#3FC4DE"), // terminal.ansiBrightBlue
		AnsiCyan:          ParseHex("#59E1E3"), // terminal.ansiCyan
		AnsiBrightCyan:    ParseHex("#6BE4E6"), // terminal.ansiBrightCyan
		AnsiGreen:         ParseHex("#29D398"), // terminal.ansiGreen
		AnsiBrightGreen:   ParseHex("#3FDAA4"), // terminal.ansiBrightGreen
		AnsiMagenta:       ParseHex("#EE64AC"), // terminal.ansiMagenta
		AnsiBrightMagenta: ParseHex("#F075B5"), // terminal.ansiBrightMagenta
		AnsiRed:           ParseHex("#E95678"), // terminal.ansiRed
		AnsiBrightRed:     ParseHex("#EC6A88"), // terminal.ansiBrightRed
		AnsiYellow:        ParseHex("#FAB795"), // terminal.ansiYellow
		AnsiBrightYellow:  ParseHex("#FBC3A7"), // terminal.ansiBrightYellow

		// Git decoration colors - from VSCode theme
		GitAdded:     ParseHex("#27D797B3"), // gitDecoration.addedResourceForeground
		GitModified:  ParseHex("#E07267"),   // gitDecoration.modifiedResourceForeground
		GitDeleted:   ParseHex("#F43E5C"),   // gitDecoration.deletedResourceForeground
		GitUntracked: ParseHex("#27D797"),   // gitDecoration.untrackedResourceForeground
		GitIgnored:   ParseHex("#D5D8DA4D"), // gitDecoration.ignoredResourceForeground
	}

	// Text selection - from VSCode theme
	t.TextSelection = lipgloss.NewStyle().
		Foreground(ParseHex("#D5D8DA")). // list.activeSelectionForeground
		Background(ParseHex("#2E303E80")) // list.activeSelectionBackground

	// LSP and MCP status indicators
	t.ItemOfflineIcon = lipgloss.NewStyle().Foreground(ParseHex("#6C6F93")).SetString("●")
	t.ItemBusyIcon = t.ItemOfflineIcon.Foreground(ParseHex("#FAB795")) // warning/yellow
	t.ItemErrorIcon = t.ItemOfflineIcon.Foreground(ParseHex("#F43E5C")) // error
	t.ItemOnlineIcon = t.ItemOfflineIcon.Foreground(ParseHex("#27D797")) // success

	// Yolo mode indicators - using theme accent colors
	t.YoloIconFocused = lipgloss.NewStyle().
		Foreground(ParseHex("#0b0c0f")).  // editor.background
		Background(ParseHex("#FAB795")).  // secondary/warning
		Bold(true).
		SetString(" ! ")
	t.YoloIconBlurred = t.YoloIconFocused.
		Foreground(ParseHex("#6C6F93")).  // muted
		Background(ParseHex("#2E303E"))   // overlay
	t.YoloDotsFocused = lipgloss.NewStyle().
		Foreground(ParseHex("#E95378")).  // primary
		SetString(":::")
	t.YoloDotsBlurred = t.YoloDotsFocused.
		Foreground(ParseHex("#6C6F93"))   // muted

	return t
}
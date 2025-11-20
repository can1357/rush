package styles

import "charm.land/lipgloss/v2"

func NewCelestialTheme() *Theme {
	t := &Theme{
		Name:   "celestial",
		IsDark: true,

		Primary:   ParseHex("#E95378"), // Celestial red
		Secondary: ParseHex("#FAB795"), // Warm orange/peach
		Tertiary:  ParseHex("#25B0BC"), // Cyan
		Accent:    ParseHex("#B877DB"), // Purple

		// Backgrounds
		BgBase:        ParseHex("#0b0c0f"), // Darkest
		BgBaseLighter: ParseHex("#0E0F13"), // Activity bar
		BgSubtle:      ParseHex("#1b1c25"), // Widget/dropdown
		BgOverlay:     ParseHex("#2E303E"), // Overlay/selection

		// Foregrounds
		FgBase:      ParseHex("#D5D8DA"),   // Main text
		FgMuted:     ParseHex("#6C6F93"),   // Muted text
		FgHalfMuted: ParseHex("#BBBBBB"),   // Half muted
		FgSubtle:    ParseHex("#D5D8DA80"), // Subtle (with transparency)
		FgSelected:  ParseHex("#FFFFFF"),   // Selected text

		// Borders
		Border:      ParseHex("#22252e"),
		BorderFocus: ParseHex("#E95378"), // Primary red

		// Status
		Success: ParseHex("#27D797"), // Green
		Error:   ParseHex("#F43E5C"), // Error red
		Warning: ParseHex("#FAB795"), // Orange
		Info:    ParseHex("#25B0BC"), // Cyan

		// Colors
		White: ParseHex("#FFFFFF"),

		BlueLight: ParseHex("#3FC4DE"),
		BlueDark:  ParseHex("#26BBD9"),
		Blue:      ParseHex("#25B0BC"),

		Yellow: ParseHex("#FBC3A7"),
		Citron: ParseHex("#FAC29A"),

		Green:      ParseHex("#29D398"),
		GreenDark:  ParseHex("#27D797"),
		GreenLight: ParseHex("#3FDAA4"),

		Red:      ParseHex("#E95678"),
		RedDark:  ParseHex("#F43E5C"),
		RedLight: ParseHex("#EC6A88"),
		Cherry:   ParseHex("#E95378"),

		// Editor-specific colors
		EditorLineNumber:         ParseHex("#D5D8DA1A"), // Line numbers
		EditorLineNumberActive:   ParseHex("#D5D8DA80"), // Active line number
		EditorCursor:             ParseHex("#E95378"),   // Cursor color
		EditorSelection:          ParseHex("#2E303EB3"), // Selection background
		EditorSelectionHighlight: ParseHex("#6C6F934D"), // Selection highlight
		EditorWordHighlight:      ParseHex("#6C6F9380"), // Word highlight
		EditorFindMatch:          ParseHex("#6C6F9380"), // Find match
		EditorFindMatchHighlight: ParseHex("#6C6F934D"), // Find match highlight
		EditorLineHighlight:      ParseHex("#2E303E4D"), // Current line

		// Diff colors
		DiffInsertedTextBg:  ParseHex("#09F7A01A"), // Inserted text background
		DiffRemovedTextBg:   ParseHex("#F43E5C1A"), // Removed text background
		DiffInsertLineBg:    ParseHex("#09F7A01A"), // Insert line background
		DiffDeleteLineBg:    ParseHex("#F43E5C1A"), // Delete line background
		DiffInsertLineNumBg: ParseHex("#09F7A0B3"), // Insert line number background
		DiffDeleteLineNumBg: ParseHex("#F43E5CB3"), // Delete line number background

		// Terminal ANSI colors
		AnsiBlue:          ParseHex("#26BBD9"),
		AnsiBrightBlue:    ParseHex("#3FC4DE"),
		AnsiCyan:          ParseHex("#59E1E3"),
		AnsiBrightCyan:    ParseHex("#6BE4E6"),
		AnsiGreen:         ParseHex("#29D398"),
		AnsiBrightGreen:   ParseHex("#3FDAA4"),
		AnsiMagenta:       ParseHex("#EE64AC"),
		AnsiBrightMagenta: ParseHex("#F075B5"),
		AnsiRed:           ParseHex("#E95678"),
		AnsiBrightRed:     ParseHex("#EC6A88"),
		AnsiYellow:        ParseHex("#FAB795"),
		AnsiBrightYellow:  ParseHex("#FBC3A7"),

		// Git decoration colors
		GitAdded:     ParseHex("#27D797B3"),
		GitModified:  ParseHex("#E07267"),
		GitDeleted:   ParseHex("#F43E5C"),
		GitUntracked: ParseHex("#27D797"),
		GitIgnored:   ParseHex("#D5D8DA4D"),
	}

	// Text selection.
	t.TextSelection = lipgloss.NewStyle().Foreground(ParseHex("#FFFFFF")).Background(ParseHex("#E95378"))

	// LSP and MCP status.
	t.ItemOfflineIcon = lipgloss.NewStyle().Foreground(ParseHex("#6C6F93")).SetString("●")
	t.ItemBusyIcon = t.ItemOfflineIcon.Foreground(ParseHex("#FAB795"))
	t.ItemErrorIcon = t.ItemOfflineIcon.Foreground(ParseHex("#F43E5C"))
	t.ItemOnlineIcon = t.ItemOfflineIcon.Foreground(ParseHex("#27D797"))

	t.YoloIconFocused = lipgloss.NewStyle().Foreground(ParseHex("#0b0c0f")).Background(ParseHex("#FAB795")).Bold(true).SetString(" ! ")
	t.YoloIconBlurred = t.YoloIconFocused.Foreground(ParseHex("#6C6F93")).Background(ParseHex("#2E303E"))
	t.YoloDotsFocused = lipgloss.NewStyle().Foreground(ParseHex("#E95378")).SetString(":::")
	t.YoloDotsBlurred = t.YoloDotsFocused.Foreground(ParseHex("#6C6F93"))

	return t
}

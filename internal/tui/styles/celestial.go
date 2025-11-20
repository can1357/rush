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
		FgBase:      ParseHex("#D5D8DA"), // Main text
		FgMuted:     ParseHex("#6C6F93"), // Muted text
		FgHalfMuted: ParseHex("#BBBBBB"), // Half muted
		FgSubtle:    ParseHex("#D5D8DA80"), // Subtle (with transparency)
		FgSelected:  ParseHex("#FFFFFF"), // Selected text

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

package diffview

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// LineStyle defines the styles for a given line type in the diff view.
type LineStyle struct {
	LineNumber lipgloss.Style
	Symbol     lipgloss.Style
	Code       lipgloss.Style
}

// Style defines the overall style for the diff view, including styles for
// different line types such as divider, missing, equal, insert, and delete
// lines.
type Style struct {
	DividerLine LineStyle
	MissingLine LineStyle
	EqualLine   LineStyle
	InsertLine  LineStyle
	DeleteLine  LineStyle
}

// DefaultLightStyle provides a default light theme style for the diff view.
func DefaultLightStyle() Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Iron).
				Background(charmtone.Thunder),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Oyster).
				Background(charmtone.Anchovy),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(charmtone.Ash),
			Code: lipgloss.NewStyle().
				Background(charmtone.Ash),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Charcoal).
				Background(charmtone.Ash),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Pepper).
				Background(charmtone.Salt),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Turtle).
				Background(lipgloss.Color("#c8e6c9")),
			Symbol: lipgloss.NewStyle().
				Foreground(charmtone.Turtle).
				Background(lipgloss.Color("#e8f5e9")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Pepper).
				Background(lipgloss.Color("#e8f5e9")),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Cherry).
				Background(lipgloss.Color("#ffcdd2")),
			Symbol: lipgloss.NewStyle().
				Foreground(charmtone.Cherry).
				Background(lipgloss.Color("#ffebee")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Pepper).
				Background(lipgloss.Color("#ffebee")),
		},
	}
}

// DefaultDarkStyle provides a default dark theme style for the diff view.
func DefaultDarkStyle() Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.Sapphire),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Smoke).
				Background(charmtone.Ox),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(charmtone.Charcoal),
			Code: lipgloss.NewStyle().
				Background(charmtone.Charcoal),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(charmtone.Ash).
				Background(charmtone.Charcoal),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(charmtone.Pepper),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#27D797")).
				Background(lipgloss.Color("#1a3d2d")),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#27D797")).
				Background(lipgloss.Color("#1f4a36")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color("#1f4a36")),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F43E5C")).
				Background(lipgloss.Color("#3d1e29")),
			Symbol: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F43E5C")).
				Background(lipgloss.Color("#4a2533")),
			Code: lipgloss.NewStyle().
				Foreground(charmtone.Salt).
				Background(lipgloss.Color("#4a2533")),
		},
	}
}

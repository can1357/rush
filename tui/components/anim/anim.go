// Package anim provides an animated spinner.
package anim

import (
	"image/color"
	"math"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/can1357/rush/csync"
	"github.com/can1357/rush/tui/util"
)

const (
	fps = 30

	// Total frames for one complete wave cycle (enter, cross, exit, pause)
	cycleDurationFrames = 45 // Slightly faster cycle

	// How long (in frames) to pause on empty screen before restarting
	pauseFrames = 8
)

// ============================================================================
// WAVE STYLES - Pick one by uncommenting!
// ============================================================================

var (
	waveShape  = []rune{' ', '·', '·', '∘', '○', '→', '→', '›', '»', '»', '≫', '⟫', '⟫'}
	waveColors = []color.Color{
		color.RGBA{R: 0x10, G: 0x10, B: 0x20, A: 0xff},
		color.RGBA{R: 0x20, G: 0x20, B: 0x40, A: 0xff},
		color.RGBA{R: 0x30, G: 0x30, B: 0x60, A: 0xff},
		color.RGBA{R: 0x40, G: 0x40, B: 0x80, A: 0xff},
		color.RGBA{R: 0x00, G: 0x50, B: 0xa0, A: 0xff},
		color.RGBA{R: 0x00, G: 0x60, B: 0xd0, A: 0xff},
		color.RGBA{R: 0x00, G: 0x70, B: 0xe0, A: 0xff},
		color.RGBA{R: 0x00, G: 0x88, B: 0xff, A: 0xff},
		color.RGBA{R: 0x40, G: 0xa0, B: 0xff, A: 0xff},
		color.RGBA{R: 0x80, G: 0xc0, B: 0xff, A: 0xff},
		color.RGBA{R: 0xb0, G: 0xe0, B: 0xff, A: 0xff},
		color.RGBA{R: 0xe0, G: 0xf0, B: 0xff, A: 0xff},
		color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	}
)

var (
	baseColor         = color.RGBA{R: 0x18, G: 0x18, B: 0x18, A: 0xff} // Dark background
	defaultLabelColor = color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
)

var lastID int64

func nextID() int { return int(atomic.AddInt64(&lastID, 1)) }

type StepMsg struct{ id int }

type Settings struct {
	Size       int
	Label      string
	LabelColor color.Color
}

type Anim struct {
	id               int
	width            int
	cyclingCharWidth int

	label      *csync.Slice[string]
	labelWidth int
	labelColor color.Color

	startTime   time.Time
	initialized atomic.Bool
	step        atomic.Int64
}

func New(opts Settings) *Anim {
	a := &Anim{}
	if opts.Size < 1 {
		opts.Size = 18 // Wider runway for better effect
	}
	if colorIsUnset(opts.LabelColor) {
		opts.LabelColor = defaultLabelColor
	}

	a.id = nextID()
	a.startTime = time.Now()
	a.cyclingCharWidth = opts.Size
	a.labelColor = opts.LabelColor

	a.labelWidth = lipgloss.Width(opts.Label)
	a.width = opts.Size
	if opts.Label != "" {
		a.width += 1 + a.labelWidth
	}

	a.renderLabel(opts.Label)
	return a
}

func (a *Anim) SetLabel(newLabel string) {
	a.labelWidth = lipgloss.Width(newLabel)
	a.width = a.cyclingCharWidth
	if newLabel != "" {
		a.width += 1 + a.labelWidth
	}
	a.renderLabel(newLabel)
}

func (a *Anim) renderLabel(label string) {
	a.label = csync.NewSlice[string]()
	for _, r := range label {
		a.label.Append(lipgloss.NewStyle().Foreground(a.labelColor).Render(string(r)))
	}
}

func (a *Anim) Width() int    { return a.width }
func (a *Anim) Init() tea.Cmd { return a.Step() }

func (a *Anim) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StepMsg:
		if msg.id != a.id {
			return a, nil
		}

		s := a.step.Add(1)
		if s >= int64(cycleDurationFrames+pauseFrames) {
			a.step.Store(0)
		}

		if !a.initialized.Load() && time.Since(a.startTime) >= time.Millisecond*500 {
			a.initialized.Store(true)
		}
		return a, a.Step()
	default:
		return a, nil
	}
}

func (a *Anim) View() string {
	var b strings.Builder

	currentFrame := int(a.step.Load())

	// 1. Calculate Physics
	var headPos float64

	if currentFrame >= cycleDurationFrames {
		// In the pause phase: Wave is gone
		headPos = float64(a.cyclingCharWidth + len(waveShape) + 5)
	} else {
		// Normalized time (0.0 to 1.0)
		t := float64(currentFrame) / float64(cycleDurationFrames)

		// CRITICAL: Aggressive easing for fast acceleration
		// t^4 gives you: very slow start → EXPLOSIVE acceleration → max speed
		// Try t^3.5 for slightly less aggressive, or t^4.5 for even more!
		ease := math.Pow(t, 4.0) // Changed from 2.5 to 4.0!

		totalDistance := float64(a.cyclingCharWidth + len(waveShape))
		startOffset := -2.0 // Start slightly more off-screen
		headPos = startOffset + (ease * totalDistance)
	}

	headIdx := int(math.Round(headPos))

	// 2. Render the Runway
	for x := 0; x < a.cyclingCharWidth; x++ {
		distanceFromHead := headIdx - x
		waveIndex := (len(waveShape) - 1) - distanceFromHead

		var char rune = '·'
		var fg color.Color = baseColor

		if waveIndex >= 0 && waveIndex < len(waveShape) {
			char = waveShape[waveIndex]
			fg = waveColors[waveIndex]
		}

		b.WriteString(lipgloss.NewStyle().Foreground(fg).Render(string(char)))
	}

	// 3. Render Label
	if a.labelWidth > 0 {
		b.WriteString(" ")
		for _, char := range a.label.Seq2() {
			b.WriteString(char)
		}
	}

	return b.String()
}

func (a *Anim) Step() tea.Cmd {
	return tea.Tick(time.Second/time.Duration(fps), func(t time.Time) tea.Msg {
		return StepMsg{id: a.id}
	})
}

func colorIsUnset(c color.Color) bool {
	if c == nil {
		return true
	}
	_, _, _, a := c.RGBA()
	return a == 0
}

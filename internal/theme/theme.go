// internal/theme/theme.go

// Package theme holds the color palette: muted Nord Aurora colors on
// neutral greys. Every bar, chart and highlight uses a single color and
// varies only its tone, from a dark shade to a light tint.
package theme

import (
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

func adaptive(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// The palette
var (
	Rose  = adaptive("#9e3f4a", "#bf616a")
	Clay  = adaptive("#b0603f", "#d08770")
	Sand  = adaptive("#9a7420", "#ebcb8b")
	Sage  = adaptive("#5a7a44", "#a3be8c")
	Mauve = adaptive("#835a80", "#b48ead")

	Text    = adaptive("#27272a", "#e4e4e7")
	Subtext = adaptive("#52525b", "#a1a1aa")
	Overlay = adaptive("#a1a1aa", "#71717a")
	Border  = adaptive("#d4d4d8", "#52525b")
	Surface = adaptive("#e4e4e7", "#3f3f46")
	Base    = adaptive("#fafafa", "#18181b")
)

func hex(c lipgloss.AdaptiveColor) string {
	if lipgloss.HasDarkBackground() {
		return c.Dark
	}
	return c.Light
}

func parse(c lipgloss.AdaptiveColor) colorful.Color {
	parsed, _ := colorful.Hex(hex(c))
	return parsed
}

// Tone ranges over the shades and tints of a single color: from a dark
// shade that fades into the background, through the color itself, to a
// light tint
type Tone struct {
	color lipgloss.AdaptiveColor
	once  sync.Once
	table []colorful.Color
}

// NewTone returns the tonal range of c
func NewTone(c lipgloss.AdaptiveColor) *Tone {
	return &Tone{color: c}
}

const toneSteps = 64

// At returns the tone at position t, from 0 (darkest) to 1 (lightest)
func (r *Tone) At(t float64) colorful.Color {
	r.once.Do(func() {
		base := parse(r.color)
		// Dark enough to read as a shade, light enough to stand out
		// from the empty track
		shade := base.BlendLab(parse(Base), 0.5)
		tint := base.BlendLab(parse(Text), 0.2)
		// The color itself sits two thirds of the way along
		stops := []colorful.Color{shade, shade.BlendLab(base, 0.5), base, tint}
		for i := 0; i < toneSteps; i++ {
			pos := float64(i) / float64(toneSteps-1) * float64(len(stops)-1)
			stop := int(pos)
			if stop >= len(stops)-1 {
				stop = len(stops) - 2
			}
			r.table = append(r.table, stops[stop].BlendLab(stops[stop+1], pos-float64(stop)).Clamped())
		}
	})
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return r.table[int(t*float64(toneSteps-1)+0.5)]
}

// Color returns the tone at position t as a lipgloss color
func (r *Tone) Color(t float64) lipgloss.Color {
	return lipgloss.Color(r.At(t).Hex())
}

var white = colorful.Color{R: 1, G: 1, B: 1}

// Shine lightens c towards white by amount, from 0 to 1, for glare effects
func Shine(c colorful.Color, amount float64) lipgloss.Color {
	return lipgloss.Color(c.BlendLab(white, amount).Clamped().Hex())
}

// Accent pairs a color for titles and borders with its tonal range for
// bars and charts
type Accent struct {
	Color lipgloss.AdaptiveColor
	Tone  *Tone
}

func accent(c lipgloss.AdaptiveColor) Accent {
	return Accent{Color: c, Tone: NewTone(c)}
}

var (
	MauveAccent = accent(Mauve)
	RoseAccent  = accent(Rose)
	SandAccent  = accent(Sand)
	SageAccent  = accent(Sage)
	ClayAccent  = accent(Clay)

	// Brand is the accent of the logo, the active tab and the splash screen
	Brand = MauveAccent

	// Accents is cycled through by Recap slides
	Accents = []Accent{MauveAccent, RoseAccent, SandAccent, SageAccent, ClayAccent}
)

// Common styles
var (
	Bold   = lipgloss.NewStyle().Bold(true).Foreground(Text)
	Normal = lipgloss.NewStyle().Foreground(Text)
	Dim    = lipgloss.NewStyle().Foreground(Subtext)
	Faint  = lipgloss.NewStyle().Foreground(Overlay)
	Track  = lipgloss.NewStyle().Foreground(Surface)
)

// Fg returns a style with the given foreground color
func Fg(c lipgloss.TerminalColor) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

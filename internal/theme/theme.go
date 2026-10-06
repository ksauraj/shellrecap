// internal/theme/theme.go

// Package theme holds the color palette: vivid Tailwind colors on neutral
// zinc greys, with the 400 shades on dark terminals and the 600 shades on
// light ones.
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
	Violet  = adaptive("#7c3aed", "#a78bfa")
	Fuchsia = adaptive("#c026d3", "#e879f9")
	Pink    = adaptive("#db2777", "#f472b6")
	Rose    = adaptive("#e11d48", "#fb7185")
	Orange  = adaptive("#ea580c", "#fb923c")
	Amber   = adaptive("#d97706", "#fbbf24")
	Lime    = adaptive("#65a30d", "#a3e635")
	Green   = adaptive("#16a34a", "#4ade80")

	Text    = adaptive("#27272a", "#e4e4e7")
	Subtext = adaptive("#52525b", "#a1a1aa")
	Overlay = adaptive("#a1a1aa", "#71717a")
	Border  = adaptive("#d4d4d8", "#52525b")
	Surface = adaptive("#e4e4e7", "#3f3f46")
	Base    = adaptive("#fafafa", "#18181b")
)

// Gradient blends smoothly between color stops
type Gradient struct {
	stops []lipgloss.AdaptiveColor
	once  sync.Once
	table []colorful.Color
}

// NewGradient returns a gradient through the given colors
func NewGradient(stops ...lipgloss.AdaptiveColor) *Gradient {
	return &Gradient{stops: stops}
}

const gradientSteps = 64

func hex(c lipgloss.AdaptiveColor) string {
	if lipgloss.HasDarkBackground() {
		return c.Dark
	}
	return c.Light
}

// At returns the color at position t, from 0 to 1
func (g *Gradient) At(t float64) colorful.Color {
	g.once.Do(func() {
		for i := 0; i < gradientSteps; i++ {
			pos := float64(i) / float64(gradientSteps-1) * float64(len(g.stops)-1)
			stop := int(pos)
			if stop >= len(g.stops)-1 {
				stop = len(g.stops) - 2
			}
			from, _ := colorful.Hex(hex(g.stops[stop]))
			to, _ := colorful.Hex(hex(g.stops[stop+1]))
			g.table = append(g.table, from.BlendLab(to, pos-float64(stop)).Clamped())
		}
	})
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return g.table[int(t*float64(gradientSteps-1)+0.5)]
}

// Color returns the color at position t as a lipgloss color
func (g *Gradient) Color(t float64) lipgloss.Color {
	return lipgloss.Color(g.At(t).Hex())
}

var white = colorful.Color{R: 1, G: 1, B: 1}

// Shine brightens c towards white by amount, from 0 to 1, for glare effects
func Shine(c colorful.Color, amount float64) lipgloss.Color {
	return lipgloss.Color(c.BlendLab(white, amount).Clamped().Hex())
}

// Accent pairs a color for titles and borders with a gradient for bars
type Accent struct {
	Color    lipgloss.AdaptiveColor
	Gradient *Gradient
}

var (
	Grape  = Accent{Fuchsia, NewGradient(Violet, Fuchsia)}
	Sunset = Accent{Pink, NewGradient(Pink, Orange)}
	Gold   = Accent{Amber, NewGradient(Orange, Amber)}
	Mint   = Accent{Lime, NewGradient(Lime, Green)}
	Blush  = Accent{Rose, NewGradient(Rose, Fuchsia)}

	// Brand is used for the logo, charts and the active tab
	Brand = NewGradient(Violet, Fuchsia, Pink, Orange, Amber)

	// Accents is cycled through by cards and Recap slides
	Accents = []Accent{Grape, Sunset, Gold, Mint, Blush}
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

// GradientText colors each character of s along the gradient
func GradientText(s string, g *Gradient, bold bool) string {
	runes := []rune(s)
	var out string
	for i, r := range runes {
		t := 0.0
		if len(runes) > 1 {
			t = float64(i) / float64(len(runes)-1)
		}
		out += lipgloss.NewStyle().Bold(bold).Foreground(g.Color(t)).Render(string(r))
	}
	return out
}

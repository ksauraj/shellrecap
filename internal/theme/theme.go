// internal/theme/theme.go

// Package theme holds the color palette. It is based on Catppuccin, using
// Mocha on dark terminals and Latte on light ones, and keeps to its warm
// colors.
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
	Red      = adaptive("#d20f39", "#f38ba8")
	Maroon   = adaptive("#e64553", "#eba0ac")
	Peach    = adaptive("#fe640b", "#fab387")
	Yellow   = adaptive("#df8e1d", "#f9e2af")
	Green    = adaptive("#40a02b", "#a6e3a1")
	Mauve    = adaptive("#8839ef", "#cba6f7")
	Pink     = adaptive("#ea76cb", "#f5c2e7")
	Flamingo = adaptive("#dd7878", "#f2cdcd")

	Text    = adaptive("#4c4f69", "#cdd6f4")
	Subtext = adaptive("#6c6f85", "#a6adc8")
	Overlay = adaptive("#9ca0b0", "#6c7086")
	Surface = adaptive("#bcc0cc", "#45475a")
	Base    = adaptive("#eff1f5", "#1e1e2e")
)

// Accents gives each Recap slide its own color
var Accents = []lipgloss.AdaptiveColor{Red, Peach, Mauve, Yellow, Green, Maroon, Pink, Flamingo}

// gradientStops is the sunset gradient used by bars, charts and the banner
var gradientStops = []lipgloss.AdaptiveColor{Mauve, Pink, Red, Peach, Yellow}

const gradientSteps = 48

var (
	gradientOnce  sync.Once
	gradientTable []lipgloss.Color
)

func hex(c lipgloss.AdaptiveColor) string {
	if lipgloss.HasDarkBackground() {
		return c.Dark
	}
	return c.Light
}

// Gradient returns the color at position t, from 0 to 1, along the sunset
// gradient
func Gradient(t float64) lipgloss.Color {
	gradientOnce.Do(func() {
		for i := 0; i < gradientSteps; i++ {
			pos := float64(i) / float64(gradientSteps-1) * float64(len(gradientStops)-1)
			stop := int(pos)
			if stop >= len(gradientStops)-1 {
				stop = len(gradientStops) - 2
			}
			from, _ := colorful.Hex(hex(gradientStops[stop]))
			to, _ := colorful.Hex(hex(gradientStops[stop+1]))
			gradientTable = append(gradientTable, lipgloss.Color(from.BlendLab(to, pos-float64(stop)).Clamped().Hex()))
		}
	})
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return gradientTable[int(t*float64(gradientSteps-1)+0.5)]
}

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

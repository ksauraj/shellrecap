// internal/theme/theme.go

// Package theme holds the colors. Every bar, chart and highlight uses a
// single color and varies only its tone, from a dark shade to a light
// tint. The palette can be switched while the app runs; the auto palette
// is derived from the terminal's own background color, so it fits any
// terminal theme.
package theme

import (
	"math"
	"os"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/platform"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
)

// The palette in use, set by Use
var (
	Rose, Clay, Sand, Sage, Mauve lipgloss.TerminalColor

	Text, Subtext, Overlay, Border, Surface lipgloss.TerminalColor
	// Base is the background color: the terminal's, or the image's
	Base lipgloss.TerminalColor

	MauveAccent, RoseAccent, SandAccent, SageAccent, ClayAccent Accent

	// Brand is the accent of the logo, the active tab and the splash screen
	Brand Accent

	// Accents is cycled through by Recap slides
	Accents []Accent

	Bold, Normal, Dim, Faint, Track lipgloss.Style

	current Palette
)

// Palette is a complete set of colors. Exact palettes use hex colors;
// the terminal palette uses the terminal's own ANSI colors instead.
type Palette struct {
	Name string
	// Accents, in hex or as ANSI color numbers
	Rose, Clay, Sand, Sage, Mauve string
	// Neutrals, from the background to the main text color
	Base, Surface, Border, Overlay, Subtext, Text string
	// ANSI marks the terminal palette, whose colors are ANSI numbers
	ANSI bool
	// Bright holds each ANSI accent's bright variant, for tints and glare
	Bright map[string]string
}

var (
	// Dark is for dark terminals: muted Nord Aurora accents on neutral greys
	Dark = Palette{
		Name: "dark",
		Rose: "#bf616a", Clay: "#d08770", Sand: "#ebcb8b", Sage: "#a3be8c", Mauve: "#b48ead",
		Base: "#18181b", Surface: "#3f3f46", Border: "#52525b", Overlay: "#71717a", Subtext: "#a1a1aa", Text: "#e4e4e7",
	}

	// Black is for pitch-black terminals and OLED screens, with a little
	// more contrast than Dark
	Black = Palette{
		Name: "black",
		Rose: "#cf717a", Clay: "#e0977f", Sand: "#f0d49b", Sage: "#b3cc9c", Mauve: "#c49fbd",
		Base: "#000000", Surface: "#262626", Border: "#404040", Overlay: "#737373", Subtext: "#a3a3a3", Text: "#f5f5f5",
	}

	// Light is for light terminals: deeper accents on pale greys
	Light = Palette{
		Name: "light",
		Rose: "#9e3f4a", Clay: "#a8562f", Sand: "#8a6512", Sage: "#4f6f3a", Mauve: "#7a4f77",
		Base: "#fafafa", Surface: "#dedee2", Border: "#c4c4ca", Overlay: "#8a8a93", Subtext: "#52525b", Text: "#27272a",
	}

	// Terminal uses the terminal's own 16 colors, so it always matches the
	// terminal's color scheme
	Terminal = Palette{
		Name: "terminal",
		Rose: "1", Clay: "6", Sand: "3", Sage: "2", Mauve: "5",
		Base: "0", Surface: "8", Border: "8", Overlay: "8", Subtext: "8", Text: "",
		ANSI:   true,
		Bright: map[string]string{"1": "9", "2": "10", "3": "11", "5": "13", "6": "14"},
	}
)

// Names lists the themes, in the order the t key cycles through them
var Names = []string{"auto", "dark", "black", "light", "terminal"}

// detected is the palette auto picks, set once by Detect
var detected = Dark

// Named returns the palette for a theme name; "auto" (or an unknown name)
// is the palette picked by Detect
func Named(name string) Palette {
	switch name {
	case "dark":
		return Dark
	case "black":
		return Black
	case "light":
		return Light
	case "terminal":
		return Terminal
	}
	p := detected
	p.Name = "auto"
	return p
}

// Current is the palette in use
func Current() Palette {
	return current
}

// Detect asks the terminal for its background color, or reads it from
// SHELLRECAP_BACKGROUND, and derives the auto palette from it. It must run
// before the TUI starts reading the keyboard, since the answer arrives on
// the same input.
func Detect() {
	// For terminals that don't say what their background is
	if c, err := colorful.Hex(os.Getenv("SHELLRECAP_BACKGROUND")); err == nil {
		detected = Derive(c)
		return
	}
	// The classic Windows console, where Windows PowerShell is blue, says
	// what its background is through the console API instead
	if r, g, b, ok := platform.ConsoleBackground(); ok {
		detected = Derive(colorful.Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255})
		return
	}
	out := termenv.NewOutput(os.Stdout)
	if out.Profile < termenv.ANSI256 {
		// Exact colors would be rounded to 16, so use the terminal's own
		detected = Terminal
		return
	}
	switch bg := out.BackgroundColor().(type) {
	case termenv.RGBColor:
		if c, err := colorful.Hex(string(bg)); err == nil {
			detected = Derive(c)
		}
	case termenv.ANSIColor:
		// From COLORFGBG: white and bright white mean a light terminal
		if bg == 7 || bg == 15 {
			detected = Light
		}
	}
}

// relativeLuminance is the WCAG luminance of c, from 0 to 1
func relativeLuminance(c colorful.Color) float64 {
	r, g, b := c.LinearRgb()
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// contrast is the WCAG contrast ratio between a and b, from 1 to 21
func contrast(a, b colorful.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// legible adjusts c's lightness until it reaches a contrast of at least
// min against bg, keeping its hue
func legible(c, bg colorful.Color, min float64, lighten bool) colorful.Color {
	h, chroma, l := c.Hcl()
	for i := 0; i < 40 && contrast(c, bg) < min; i++ {
		if lighten {
			l += 0.02
		} else {
			l -= 0.02
		}
		c = colorful.Hcl(h, chroma, l).Clamped()
	}
	return c
}

// Derive builds a palette for a terminal with the given background: the
// greys are blended from the background itself, so they share its tint,
// and every accent is adjusted until it reads well against it
func Derive(bg colorful.Color) Palette {
	// Dark backgrounds are the ones white text reads better on
	black := colorful.Color{}
	isDark := contrast(white, bg) >= contrast(black, bg)
	base, toward := Dark, white
	if !isDark {
		base, toward = Light, black
	}
	mix := func(t float64) string { return bg.BlendLab(toward, t).Clamped().Hex() }
	// Text goes as far towards white or black as it takes to read well
	text := 0.9
	for text < 1 && contrast(RGB(mix(text)), bg) < 7 {
		text += 0.02
	}
	accent := func(hex string) string {
		c, _ := colorful.Hex(hex)
		return legible(c, bg, 3.2, isDark).Hex()
	}
	return Palette{
		Name: "auto",
		Rose: accent(base.Rose), Clay: accent(base.Clay), Sand: accent(base.Sand),
		Sage: accent(base.Sage), Mauve: accent(base.Mauve),
		Base: bg.Clamped().Hex(), Surface: mix(0.16), Border: mix(0.3), Overlay: mix(0.45),
		Subtext: mix(0.64), Text: mix(math.Min(text, 1)),
	}
}

// color turns a palette entry into a lipgloss color
func (p Palette) color(v string) lipgloss.TerminalColor {
	if v == "" {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(v)
}

// RGB returns an exact palette entry as a colorful color
func RGB(v string) colorful.Color {
	c, _ := colorful.Hex(v)
	return c
}

// Use switches to a palette
func Use(p Palette) {
	current = p
	Rose, Clay, Sand, Sage, Mauve = p.color(p.Rose), p.color(p.Clay), p.color(p.Sand), p.color(p.Sage), p.color(p.Mauve)
	Text, Subtext, Overlay = p.color(p.Text), p.color(p.Subtext), p.color(p.Overlay)
	Border, Surface, Base = p.color(p.Border), p.color(p.Surface), p.color(p.Base)

	MauveAccent = p.accent(p.Mauve)
	RoseAccent = p.accent(p.Rose)
	SandAccent = p.accent(p.Sand)
	SageAccent = p.accent(p.Sage)
	ClayAccent = p.accent(p.Clay)
	Brand = MauveAccent
	Accents = []Accent{MauveAccent, RoseAccent, SandAccent, SageAccent, ClayAccent}

	Bold = lipgloss.NewStyle().Bold(true).Foreground(Text)
	Normal = lipgloss.NewStyle().Foreground(Text)
	Dim = lipgloss.NewStyle().Foreground(Subtext)
	Faint = lipgloss.NewStyle().Foreground(Overlay)
	Track = lipgloss.NewStyle().Foreground(Surface)
}

func init() {
	Use(Dark)
}

// Accent pairs a color for titles and borders with its tonal range for
// bars and charts
type Accent struct {
	Color lipgloss.TerminalColor
	Tone  *Tone
}

func (p Palette) accent(v string) Accent {
	return Accent{Color: p.color(v), Tone: p.tone(v)}
}

// Tone ranges over the shades and tints of a single color: from a dark
// shade that fades into the background, through the color itself, to a
// light tint. With the terminal palette it switches between an ANSI
// color and its bright variant.
type Tone struct {
	table          []colorful.Color
	normal, bright lipgloss.TerminalColor
}

const toneSteps = 64

func (p Palette) tone(v string) *Tone {
	if p.ANSI {
		bright := p.Bright[v]
		if bright == "" {
			bright = v
		}
		return &Tone{normal: lipgloss.Color(v), bright: lipgloss.Color(bright)}
	}
	base := RGB(v)
	// Dark enough to read as a shade, light enough to stand out from the
	// empty track
	shade := base.BlendLab(RGB(p.Base), 0.5)
	tint := base.BlendLab(RGB(p.Text), 0.2)
	// The color itself sits two thirds of the way along
	stops := []colorful.Color{shade, shade.BlendLab(base, 0.5), base, tint}
	t := &Tone{}
	for i := 0; i < toneSteps; i++ {
		pos := float64(i) / float64(toneSteps-1) * float64(len(stops)-1)
		stop := int(pos)
		if stop >= len(stops)-1 {
			stop = len(stops) - 2
		}
		t.table = append(t.table, stops[stop].BlendLab(stops[stop+1], pos-float64(stop)).Clamped())
	}
	return t
}

func (r *Tone) at(t float64) colorful.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return r.table[int(t*float64(toneSteps-1)+0.5)]
}

// Color returns the tone at position t, from 0 (darkest) to 1 (lightest)
func (r *Tone) Color(t float64) lipgloss.TerminalColor {
	if r.table == nil {
		if t < 0.6 {
			return r.normal
		}
		return r.bright
	}
	return lipgloss.Color(r.at(t).Hex())
}

var white = colorful.Color{R: 1, G: 1, B: 1}

// Shine is the tone at t lightened by a glare of the given strength, from
// 0 to 1
func (r *Tone) Shine(t, glare float64) lipgloss.TerminalColor {
	if r.table == nil {
		if glare > 0.3 {
			return r.bright
		}
		return r.Color(t)
	}
	return lipgloss.Color(r.at(t).BlendLab(white, glare).Clamped().Hex())
}

// Fg returns a style with the given foreground color
func Fg(c lipgloss.TerminalColor) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

// SortedNames is Names sorted, for messages listing the valid themes
func SortedNames() []string {
	names := append([]string(nil), Names...)
	sort.Strings(names)
	return names
}

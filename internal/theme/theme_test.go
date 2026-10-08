package theme

import (
	"math"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// backgrounds are real terminal defaults the auto theme has to fit
var backgrounds = map[string]string{
	"ubuntu aubergine": "#300a24",
	"pitch black":      "#000000",
	"gnome dark":       "#1e1e1e",
	"solarized dark":   "#002b36",
	"dracula":          "#282a36",
	"white":            "#ffffff",
	"solarized light":  "#fdf6e3",
	"mid grey":         "#808080",
}

func TestDeriveIsLegible(t *testing.T) {
	for name, hex := range backgrounds {
		bg := RGB(hex)
		p := Derive(bg)
		for _, accent := range []string{p.Rose, p.Clay, p.Sand, p.Sage, p.Mauve} {
			if c := contrast(RGB(accent), bg); c < 3 {
				t.Errorf("%s: accent %s has a contrast of %.1f, want at least 3", name, accent, c)
			}
		}
		// 7 where it's possible; mid greys can't reach it with any color
		best := math.Max(contrast(white, bg), contrast(colorful.Color{}, bg))
		if c, want := contrast(RGB(p.Text), bg), math.Min(7, 0.95*best); c < want {
			t.Errorf("%s: text %s has a contrast of %.1f, want at least %.1f", name, p.Text, c, want)
		}
		if c := contrast(RGB(p.Subtext), bg); c < 3 {
			t.Errorf("%s: secondary text %s has a contrast of %.1f, want at least 3", name, p.Subtext, c)
		}
		// The empty track must still show against the background
		if c := contrast(RGB(p.Surface), bg); c < 1.15 {
			t.Errorf("%s: the track %s barely shows (contrast %.2f)", name, p.Surface, c)
		}
	}
}

func TestDeriveSharesTheBackgroundsTint(t *testing.T) {
	bg := RGB(backgrounds["ubuntu aubergine"])
	p := Derive(bg)
	bgHue, _, _ := bg.Hcl()
	for _, grey := range []string{p.Surface, p.Border, p.Overlay} {
		h, chroma, _ := RGB(grey).Hcl()
		if chroma < 0.02 {
			t.Errorf("grey %s is neutral, want it tinted like the background", grey)
		}
		if d := math.Abs(h - bgHue); d > 20 && d < 340 {
			t.Errorf("grey %s has hue %.0f, want close to the background's %.0f", grey, h, bgHue)
		}
	}
}

func TestDeriveFollowsLightness(t *testing.T) {
	if p := Derive(RGB("#ffffff")); relativeLuminance(RGB(p.Text)) > 0.1 {
		t.Errorf("text on white is %s, want dark", p.Text)
	}
	if p := Derive(RGB("#000000")); relativeLuminance(RGB(p.Text)) < 0.7 {
		t.Errorf("text on black is %s, want light", p.Text)
	}
}

func TestNamedPalettes(t *testing.T) {
	for _, name := range Names {
		if got := Named(name).Name; got != name {
			t.Errorf("Named(%q).Name = %q", name, got)
		}
	}
	// Fixed palettes are legible on their own backgrounds
	for _, p := range []Palette{Dark, Black, Light} {
		bg := RGB(p.Base)
		for _, accent := range []string{p.Rose, p.Clay, p.Sand, p.Sage, p.Mauve} {
			if c := contrast(RGB(accent), bg); c < 3 {
				t.Errorf("%s: accent %s has a contrast of %.1f against %s", p.Name, accent, c, p.Base)
			}
		}
		if c := contrast(RGB(p.Text), bg); c < 7 {
			t.Errorf("%s: text has a contrast of %.1f", p.Name, c)
		}
	}
}

func TestTerminalPaletteUsesANSIColors(t *testing.T) {
	defer Use(Dark)
	Use(Terminal)
	for _, c := range []lipgloss.TerminalColor{Rose, Clay, Sand, Sage, Mauve, Subtext, Surface} {
		v, ok := c.(lipgloss.Color)
		if !ok || len(v) > 2 {
			t.Errorf("terminal palette color %v isn't an ANSI color number", c)
		}
	}
	if _, ok := Text.(lipgloss.NoColor); !ok {
		t.Errorf("terminal palette text is %v, want the terminal's own text color", Text)
	}
	// Tones switch to the bright variant for tints and glare
	if RoseAccent.Tone.Color(0) != lipgloss.Color("1") || RoseAccent.Tone.Color(1) != lipgloss.Color("9") {
		t.Error("terminal tones don't go from the color to its bright variant")
	}
	if RoseAccent.Tone.Shine(0, 1) != lipgloss.Color("9") {
		t.Error("glare doesn't brighten terminal tones")
	}
}

func TestToneRunsFromShadeToTint(t *testing.T) {
	defer Use(Dark)
	Use(Dark)
	shade := RGB(string(RoseAccent.Tone.Color(0).(lipgloss.Color)))
	tint := RGB(string(RoseAccent.Tone.Color(1).(lipgloss.Color)))
	_, _, ls := shade.Hcl()
	_, _, lt := tint.Hcl()
	if ls >= lt {
		t.Errorf("tone goes from %v to %v, want darker to lighter", shade, tint)
	}
	glare := RGB(string(RoseAccent.Tone.Shine(0.5, 0.5).(lipgloss.Color)))
	plain := RGB(string(RoseAccent.Tone.Color(0.5).(lipgloss.Color)))
	if relativeLuminance(glare) <= relativeLuminance(plain) {
		t.Error("glare doesn't lighten the tone")
	}
}

func TestBackgroundOverride(t *testing.T) {
	t.Setenv("SHELLRECAP_BACKGROUND", "#300a24")
	defer func() { detected = Dark }()
	Detect()
	if got := Named("auto").Base; got != "#300a24" {
		t.Errorf("auto palette background = %s, want the override", got)
	}
	_ = colorful.Color{}
}

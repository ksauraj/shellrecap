// internal/render/sharemenu.go
package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/theme"
)

// ShareMenu is what the share menu shows
type ShareMenu struct {
	Busy    bool // the images are still being created
	Failed  bool
	Empty   bool // there's no history to share
	Frame   int  // animation tick, for the spinner
	Dir     string
	GIF     string
	Poster  string
	GIFSize int64
	Copied  bool // the poster is on the clipboard
	Desktop bool // links can open a browser
	Links   []ShareKey
	Message string // what the last action did
}

// ShareKey is a platform in the share menu and the key that picks it
type ShareKey struct {
	Key, Name string
}

// RenderShareMenu renders the share menu as a card like the other views
func RenderShareMenu(menu ShareMenu, width int) string {
	accent := theme.MauveAccent
	key := lipgloss.NewStyle().Bold(true).Foreground(accent.Color)
	spec := cardSpec{title: "Share your recap", accent: accent, wide: true, body: func(inner int) []string {
		switch {
		case menu.Empty:
			return []string{theme.Normal.Render("There's no shell history to share yet."), "", theme.Dim.Render("esc: close")}
		case menu.Busy:
			return []string{theme.Normal.Render(Spinner(menu.Frame) + " Creating your share images...")}
		case menu.Failed:
			return []string{
				theme.Normal.Render("Couldn't create the images. The details are in shellrecap.log."),
				"", theme.Dim.Render("esc: close"),
			}
		}

		copied := ""
		if menu.Copied {
			copied = " · copied to clipboard"
		}
		lines := []string{
			theme.Dim.Render("Saved to ") + theme.Normal.Render(menu.Dir), "",
			"  " + theme.Normal.Render(fmt.Sprintf("%-24s", menu.GIF)) +
				theme.Dim.Render(fmt.Sprintf("animated · %.1f MB · for X, Discord, Reddit", float64(menu.GIFSize)/1e6)),
			"  " + theme.Normal.Render(fmt.Sprintf("%-24s", menu.Poster)) +
				theme.Dim.Render("poster"+copied+" · for Instagram, LinkedIn"),
			"",
		}
		if !menu.Desktop {
			return append(lines, theme.Dim.Render("Copy the files to your computer to post them."), "",
				theme.Dim.Render("esc: close"))
		}

		var keys []string
		for _, link := range menu.Links {
			keys = append(keys, key.Render(link.Key)+" "+theme.Normal.Render(link.Name))
		}
		keys = append(keys, key.Render("o")+" "+theme.Normal.Render("open folder"), key.Render("esc")+" "+theme.Dim.Render("close"))
		lines = append(lines, "  "+strings.Join(keys, theme.Faint.Render("   ")))
		if menu.Message != "" {
			// Wrapped, since it can be longer than the card is wide
			lines = append(lines, "")
			lines = append(lines, strings.Split(lipgloss.NewStyle().Width(inner).Foreground(theme.Text).Render(menu.Message), "\n")...)
		}
		return lines
	}}
	return grid([]cardSpec{spec}, minInt(width, 96))
}

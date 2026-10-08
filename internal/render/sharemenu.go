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
	Count   int // how many images were saved
	Files   []ShareFile
	Desktop bool // links can open a browser
	Links   []ShareKey
	Message string // what the last action did
}

// ShareFile is a line in the share menu's list of images
type ShareFile struct {
	Name, Note string
}

// ShareKey is a platform in the share menu and the key that picks it
type ShareKey struct {
	Key, Name string
}

// ShareOption is a choice in the customize panel
type ShareOption struct {
	Label string
	Value string // shown for choices with several values
	Note  string
	// Toggle options are on or off
	Toggle, On bool
}

// keyStyle highlights the key that picks an action
func keyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Brand.Color)
}

func keyHints(hints ...[2]string) string {
	var parts []string
	for _, h := range hints {
		parts = append(parts, keyStyle().Render(h[0])+" "+theme.Normal.Render(h[1]))
	}
	return "  " + strings.Join(parts, theme.Faint.Render("   "))
}

// wrapped splits text into lines that fit inner columns
func wrapped(text string, inner int) []string {
	return strings.Split(lipgloss.NewStyle().Width(inner).Foreground(theme.Text).Render(text), "\n")
}

// RenderShareMenu renders the share menu as a card like the other views
func RenderShareMenu(menu ShareMenu, width int) string {
	spec := cardSpec{title: "Share your recap", accent: theme.MauveAccent, wide: true, body: func(inner int) []string {
		switch {
		case menu.Empty:
			return []string{theme.Normal.Render("There's no shell history to share yet."), "", theme.Dim.Render("esc: close")}
		case menu.Busy:
			return []string{theme.Normal.Render(Spinner(menu.Frame) + " Creating your share images..."), "",
				keyHints([2]string{"c", "customize"}, [2]string{"esc", "close"})}
		case menu.Failed:
			return []string{
				theme.Normal.Render("Couldn't create the images. The details are in shellrecap.log."),
				"", keyHints([2]string{"c", "customize"}, [2]string{"esc", "close"}),
			}
		}

		lines := []string{theme.Dim.Render(fmt.Sprintf("Saved %d images to ", menu.Count)) + theme.Normal.Render(menu.Dir), ""}
		nameWidth := 0
		for _, f := range menu.Files {
			nameWidth = maxInt(nameWidth, lipgloss.Width(f.Name))
		}
		for _, f := range menu.Files {
			lines = append(lines, "  "+theme.Normal.Render(f.Name)+strings.Repeat(" ", nameWidth-lipgloss.Width(f.Name)+3)+
				theme.Dim.Render(f.Note))
		}
		lines = append(lines, "")
		if !menu.Desktop {
			return append(lines, theme.Dim.Render("Copy the files to your computer to post them."), "",
				keyHints([2]string{"c", "customize"}, [2]string{"esc", "close"}))
		}

		var hints [][2]string
		for _, link := range menu.Links {
			hints = append(hints, [2]string{link.Key, link.Name})
		}
		hints = append(hints, [2]string{"o", "open folder"}, [2]string{"c", "customize"}, [2]string{"esc", "close"})
		lines = append(lines, keyHints(hints...))
		if menu.Message != "" {
			lines = append(lines, "")
			lines = append(lines, wrapped(menu.Message, inner)...)
		}
		return lines
	}}
	return grid([]cardSpec{spec}, minInt(width, 100))
}

// RenderShareOptions renders the customize panel; cursor is the selected
// option, and the row after the last option is the create button
func RenderShareOptions(options []ShareOption, cursor int, width int) string {
	spec := cardSpec{title: "Customize your share images", accent: theme.MauveAccent, wide: true, body: func(inner int) []string {
		values := make([]string, len(options))
		labelWidth, valueWidth := 0, 0
		for i, o := range options {
			switch {
			case o.Toggle && o.On:
				values[i] = lipgloss.NewStyle().Bold(true).Foreground(theme.Sage).Render("[x] on")
			case o.Toggle:
				values[i] = theme.Faint.Render("[ ] off")
			default:
				values[i] = keyStyle().Render("‹ ") + theme.Bold.Render(o.Value) + keyStyle().Render(" ›")
			}
			labelWidth = maxInt(labelWidth, len(o.Label))
			valueWidth = maxInt(valueWidth, lipgloss.Width(values[i]))
		}
		lines := []string{theme.Dim.Render("Your choices are remembered for next time."), ""}
		for i, o := range options {
			pointer := "  "
			label := theme.Normal.Render(fmt.Sprintf("%-*s", labelWidth, o.Label))
			if i == cursor {
				pointer = keyStyle().Render("> ")
				label = theme.Bold.Render(fmt.Sprintf("%-*s", labelWidth, o.Label))
			}
			value := values[i] + strings.Repeat(" ", valueWidth-lipgloss.Width(values[i]))
			lines = append(lines, pointer+label+"   "+value+"   "+theme.Faint.Render(o.Note))
		}

		create := theme.Normal.Render("[ Create images ]")
		pointer := "  "
		if cursor == len(options) {
			create = lipgloss.NewStyle().Bold(true).Foreground(theme.Base).Background(theme.Brand.Color).Render("[ Create images ]")
			pointer = keyStyle().Render("> ")
		}
		lines = append(lines, "", pointer+create, "",
			keyHints([2]string{"↑↓", "choose"}, [2]string{"space ←→", "change"}, [2]string{"enter", "create"}, [2]string{"esc", "back"}))
		return lines
	}}
	return grid([]cardSpec{spec}, minInt(width, 100))
}

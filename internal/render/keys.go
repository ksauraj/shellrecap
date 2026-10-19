// internal/render/keys.go
package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/theme"
)

// KeysPanel is what the Advanced panel shows
type KeysPanel struct {
	Rows    []KeyRow
	Cursor  int // the selected provider
	Editing bool
	Input   string // the key being pasted, masked
	Typed   int    // how many characters have been pasted
	Message string // what the last action did
	Failed  bool   // the message is an error
	Path    string // where keys are saved
	Setup   KeySetup
}

// KeyRow is a provider in the Advanced panel
type KeyRow struct {
	Name   string
	Source string // where its key comes from, or "" without one
	Masked string // the saved key, masked, when Source is "saved"
	EnvVar string
}

// KeySetup is how to get a provider's API key
type KeySetup struct {
	Name  string
	URL   string
	Steps []string
	Notes []string
}

// numbered wraps text after a "1. " style marker, indenting the lines
// that follow to line up with the text
func numbered(marker, text string, inner int) []string {
	indent := lipgloss.Width(marker)
	lines := wrapped(text, maxInt(inner-indent-1, 10))
	for i := range lines {
		if i == 0 {
			lines[i] = theme.Dim.Render(marker) + lines[i]
		} else {
			lines[i] = strings.Repeat(" ", indent) + lines[i]
		}
	}
	return lines
}

// RenderKeysPanel renders the Advanced panel, where AI API keys are pasted
func RenderKeysPanel(p KeysPanel, width int) string {
	spec := cardSpec{title: "Advanced · AI API keys", accent: theme.MauveAccent, wide: true, body: func(inner int) []string {
		lines := wrapped("AI-written slides need a free Gemini or Groq API key. Gemini is tried first, then Groq. "+
			"A key you paste here is used ahead of the environment and any key built into shellrecap.", inner-1)
		lines = append(lines, "")

		nameWidth := 0
		for _, r := range p.Rows {
			nameWidth = maxInt(nameWidth, lipgloss.Width(r.Name))
		}
		for i, r := range p.Rows {
			pointer, name := "  ", theme.Normal.Render(fmt.Sprintf("%-*s", nameWidth, r.Name))
			if i == p.Cursor {
				pointer, name = keyStyle().Render("> "), theme.Bold.Render(fmt.Sprintf("%-*s", nameWidth, r.Name))
			}
			var status string
			switch {
			case i == p.Cursor && p.Editing:
				field := p.Input + lipgloss.NewStyle().Foreground(theme.Brand.Color).Render("▌")
				if p.Typed == 0 {
					field = theme.Faint.Render("paste your key") + lipgloss.NewStyle().Foreground(theme.Brand.Color).Render("▌")
				}
				status = field
			case r.Source == "saved":
				status = theme.Normal.Render(r.Masked) + "  " + lipgloss.NewStyle().Foreground(theme.Sage).Render("your key")
			case r.Source == "environment":
				status = lipgloss.NewStyle().Foreground(theme.Sage).Render("from $" + r.EnvVar)
			case r.Source == "built in":
				status = theme.Dim.Render("using the shared key built in")
			default:
				status = theme.Faint.Render("not set")
			}
			lines = append(lines, pointer+name+"   "+status)
		}

		if p.Message != "" {
			style := lipgloss.NewStyle().Foreground(theme.Sage)
			if p.Failed {
				style = lipgloss.NewStyle().Foreground(theme.Rose)
			}
			lines = append(lines, "")
			for _, l := range strings.Split(lipgloss.NewStyle().Width(inner-1).Render(p.Message), "\n") {
				lines = append(lines, style.Render(l))
			}
		}

		lines = append(lines, "", theme.Bold.Render("Get a free "+p.Setup.Name+" key"))
		for i, step := range p.Setup.Steps {
			lines = append(lines, numbered(fmt.Sprintf("%d. ", i+1), step, inner)...)
		}
		lines = append(lines, numbered(fmt.Sprintf("%d. ", len(p.Setup.Steps)+1),
			"Back here, select "+p.Setup.Name+", press enter, paste the key with your terminal's paste "+
				"(ctrl+shift+v, cmd+v or a right click) and press enter.", inner)...)
		for _, note := range p.Setup.Notes {
			lines = append(lines, numbered("•  ", note, inner)...)
		}

		if p.Path != "" {
			lines = append(lines, "")
			for _, l := range strings.Split(lipgloss.NewStyle().Width(inner-1).Render("Keys are saved to "+p.Path+
				", which only you can read. Each is only ever sent to its own provider."), "\n") {
				lines = append(lines, theme.Faint.Render(l))
			}
		}
		lines = append(lines, "")
		if p.Editing {
			lines = append(lines, keyHints([2]string{"enter", "save"}, [2]string{"ctrl+u", "clear"}, [2]string{"esc", "cancel"}))
		} else {
			lines = append(lines, keyHints([2]string{"↑↓", "choose"}, [2]string{"enter", "paste a key"},
				[2]string{"o", "open the " + p.Setup.Name + " page"}, [2]string{"d", "remove"}, [2]string{"esc", "close"}))
		}
		return lines
	}}
	return grid([]cardSpec{spec}, minInt(width, 100))
}

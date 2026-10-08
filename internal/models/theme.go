// internal/models/theme.go
package models

import "github.com/ksauraj/shellrecap/internal/theme"

// themeNames are what the footer calls each theme
var themeNames = map[string]string{
	"auto":     "auto, matched to your terminal",
	"dark":     "dark",
	"black":    "pitch black",
	"light":    "light",
	"terminal": "your terminal's own colors",
}

// toastFrames is how long a toast stays up: 2.5 seconds
const toastFrames = 25

// nextTheme switches to the next theme and remembers it
func (m *Model) nextTheme() {
	next := theme.Names[0]
	for i, name := range theme.Names {
		if name == m.settings.Theme {
			next = theme.Names[(i+1)%len(theme.Names)]
		}
	}
	m.settings.Theme = next
	theme.Use(theme.Named(next))
	if err := saveSettings(m.settings); err != nil {
		m.logger.Printf("Error saving settings: %v", err)
	}
	m.toast = "Theme: " + themeNames[next] + " • t: next theme"
	m.toastUntil = m.frame + toastFrames
	m.syncContent()
}

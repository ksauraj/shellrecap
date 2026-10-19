// internal/models/keys.go
package models

import (
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/config"
	"github.com/ksauraj/shellrecap/internal/render"
)

// keysState is the Advanced panel opened with a, where AI API keys are
// pasted in
type keysState struct {
	open    bool
	cursor  int  // the selected provider
	editing bool // a key is being pasted in
	input   []rune
	message string // what the last action did
	failed  bool   // the message is an error
}

// keyProviders are the providers in the panel, in order
var keyProviders = []string{ai.Gemini, ai.Groq}

// Saving keys, swapped out in tests
var (
	loadKeys = config.LoadKeys
	saveKeys = config.SaveKeys
)

// keySetup is how to get each provider's key, shown in the panel
var keySetup = map[string]render.KeySetup{
	ai.Gemini: {
		Name: "Gemini",
		URL:  "https://aistudio.google.com/apikey",
		Steps: []string{
			"You need a Google account. A personal Gmail works; on a work or school account, the admin may have turned AI Studio off.",
			"Open aistudio.google.com/apikey and sign in. Accept the terms the first time.",
			"Click Create API key. AI Studio makes a Google Cloud project for it, so you don't need to set up Google Cloud yourself, or billing for the free tier. If you already use Google Cloud, you can pick one of your projects instead.",
			"Copy the key. It starts with AIza.",
		},
		Notes: []string{
			"The free tier isn't offered in every country. Where it isn't, turn on billing for the project in Google Cloud.",
			"On the free tier, Google may use what's sent to improve its products. shellrecap only sends counts and command names.",
		},
	},
	ai.Groq: {
		Name: "Groq",
		URL:  "https://console.groq.com/keys",
		Steps: []string{
			"Sign up at console.groq.com with your email, Google or GitHub account. No card is needed for the free plan.",
			"Open console.groq.com/keys and click Create API Key. Name it shellrecap.",
			"Copy the key straight away, Groq only shows it once. It starts with gsk_.",
		},
	},
}

// keyPrefixes start each provider's keys, to catch a key pasted into the
// wrong provider
var keyPrefixes = map[string]string{ai.Gemini: "AIza", ai.Groq: "gsk_"}

// applySavedKeys hands the saved keys to the AI providers
func applySavedKeys(k config.Keys) {
	ai.SetSavedKey(ai.Gemini, k.Gemini)
	ai.SetSavedKey(ai.Groq, k.Groq)
}

func setKey(k *config.Keys, provider, key string) {
	if provider == ai.Gemini {
		k.Gemini = key
	} else {
		k.Groq = key
	}
}

// cleanKey trims what terminals and copy buttons add around a pasted key
func cleanKey(s string) string {
	return strings.Trim(strings.TrimSpace(s), `"'`+"`")
}

// checkKey returns why key can't be the provider's key, or ""
func checkKey(provider, key string) string {
	name := keySetup[provider].Name
	switch {
	case key == "":
		return "Nothing was pasted."
	case strings.IndexFunc(key, func(r rune) bool { return unicode.IsSpace(r) || !unicode.IsPrint(r) }) >= 0:
		return "That has spaces in it, so it isn't an API key. Copy just the key and try again."
	case len(key) < 20:
		return "That's too short to be an API key. Copy the whole key and try again."
	}
	for other, prefix := range keyPrefixes {
		if other != provider && strings.HasPrefix(key, prefix) {
			return fmt.Sprintf("That looks like a %s key, not a %s one.", keySetup[other].Name, name)
		}
	}
	return ""
}

func (m *Model) openKeys() {
	m.keys = keysState{open: true}
	m.syncContent()
	m.viewport.GotoTop()
}

// handleKeysKey handles keys while the Advanced panel is open
func (m Model) handleKeysKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	provider := keyProviders[m.keys.cursor]
	name := keySetup[provider].Name

	if m.keys.editing {
		switch msg.Type {
		case tea.KeyEsc:
			m.keys.editing, m.keys.input = false, nil
			m.keys.message = ""
		case tea.KeyEnter:
			key := cleanKey(string(m.keys.input))
			if problem := checkKey(provider, key); problem != "" {
				m.keys.message, m.keys.failed = problem, true
				break
			}
			m.keys.editing, m.keys.input = false, nil
			saved := loadKeys()
			setKey(&saved, provider, key)
			if err := saveKeys(saved); err != nil {
				m.logger.Printf("Error saving API keys: %v", err)
				m.keys.message, m.keys.failed = "Couldn't save the key. The details are in shellrecap.log.", true
				break
			}
			applySavedKeys(saved)
			m.keys.message, m.keys.failed = fmt.Sprintf("Saved your %s key. Writing your AI slides with it now, see the Recap tab.", name), false
			m.syncContent()
			return m, m.loadAISlides(true)
		case tea.KeyBackspace:
			if len(m.keys.input) > 0 {
				m.keys.input = m.keys.input[:len(m.keys.input)-1]
			}
		case tea.KeyCtrlU:
			m.keys.input = nil
		case tea.KeyRunes, tea.KeySpace:
			// A paste arrives as one message with every character
			m.keys.input = append(m.keys.input, msg.Runes...)
			m.keys.message = ""
		}
		m.syncContent()
		return m, nil
	}

	switch msg.String() {
	case "esc", "a":
		m.keys.open = false
	case "up", "k":
		m.keys.cursor = (m.keys.cursor + len(keyProviders) - 1) % len(keyProviders)
		m.keys.message = ""
	case "down", "j", "tab":
		m.keys.cursor = (m.keys.cursor + 1) % len(keyProviders)
		m.keys.message = ""
	case "enter":
		m.keys.editing, m.keys.input, m.keys.message = true, nil, ""
	case "o":
		url := keySetup[provider].URL
		if !isDesktop() || openURL(url) != nil {
			m.keys.message, m.keys.failed = "Open "+url+" in your browser.", false
		} else {
			m.keys.message, m.keys.failed = fmt.Sprintf("Opened the %s page in your browser.", name), false
		}
	case "d", "delete":
		saved := loadKeys()
		if (provider == ai.Gemini && saved.Gemini == "") || (provider == ai.Groq && saved.Groq == "") {
			m.keys.message, m.keys.failed = fmt.Sprintf("There's no saved %s key to remove.", name), false
			break
		}
		setKey(&saved, provider, "")
		if err := saveKeys(saved); err != nil {
			m.logger.Printf("Error saving API keys: %v", err)
			m.keys.message, m.keys.failed = "Couldn't remove the key. The details are in shellrecap.log.", true
			break
		}
		applySavedKeys(saved)
		m.keys.message, m.keys.failed = fmt.Sprintf("Removed your saved %s key.", name), false
	}
	m.syncContent()
	return m, nil
}

// keysPanel describes the Advanced panel for rendering
func (m Model) keysPanel() render.KeysPanel {
	panel := render.KeysPanel{
		Cursor:  m.keys.cursor,
		Editing: m.keys.editing,
		Typed:   len(m.keys.input),
		Message: m.keys.message,
		Failed:  m.keys.failed,
		Setup:   keySetup[keyProviders[m.keys.cursor]],
	}
	if p, err := config.KeysPath(); err == nil {
		panel.Path = p
	}
	saved := loadKeys()
	for _, provider := range keyProviders {
		row := render.KeyRow{Name: keySetup[provider].Name, Source: ai.KeySource(provider), EnvVar: ai.EnvVar(provider)}
		key := saved.Gemini
		if provider == ai.Groq {
			key = saved.Groq
		}
		if row.Source == ai.FromSaved {
			row.Masked = maskKey(key)
		}
		panel.Rows = append(panel.Rows, row)
	}
	if panel.Editing {
		panel.Input = maskKey(string(m.keys.input))
	}
	return panel
}

// maskKey shows enough of a key to recognise it and no more: its first
// and last four characters once it's long enough to keep the rest hidden
func maskKey(key string) string {
	r := []rune(key)
	if len(r) < 16 {
		return strings.Repeat("•", len(r))
	}
	return string(r[:4]) + strings.Repeat("•", minInt(len(r)-8, 16)) + string(r[len(r)-4:])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package models

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/config"
)

// fakeKeys keeps the Advanced panel's keys in memory for a test
func fakeKeys(t *testing.T) *config.Keys {
	t.Helper()
	var stored config.Keys
	oldLoad, oldSave := loadKeys, saveKeys
	loadKeys = func() config.Keys { return stored }
	saveKeys = func(k config.Keys) error { stored = k; return nil }
	t.Cleanup(func() {
		loadKeys, saveKeys = oldLoad, oldSave
		applySavedKeys(config.Keys{})
	})
	return &stored
}

func typeText(t *testing.T, m Model, text string) Model {
	return update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
}

func TestPasteAKeyInTheAdvancedPanel(t *testing.T) {
	stored := fakeKeys(t)
	m := testModel(t, "")
	const key = "gsk_quietly0pasted0groq0key0q"

	m = typeText(t, m, "a")
	if !m.keys.open {
		t.Fatal("a didn't open the Advanced panel")
	}
	view := plain(m.View())
	for _, want := range []string{"Gemini", "Groq", "not set", "aistudio.google.com/apikey", "Google account"} {
		if !strings.Contains(view, want) {
			t.Errorf("the panel doesn't mention %q:\n%s", want, view)
		}
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(plain(m.View()), "console.groq.com/keys") {
		t.Errorf("the Groq steps aren't shown for Groq")
	}
	// The paste has a q in it, which mustn't quit
	m = typeText(t, m, " "+key+"\n")
	if view := plain(m.View()); strings.Contains(view, key) || strings.Contains(view, "0pasted0") {
		t.Errorf("the pasted key is shown in full:\n%s", view)
	}
	var cmd tea.Cmd
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if stored.Groq != key {
		t.Fatalf("saved %+v, want the Groq key trimmed", *stored)
	}
	if ai.KeySource(ai.Groq) != ai.FromSaved || cmd == nil || m.aiStatus != aiPending {
		t.Errorf("the saved key isn't used for AI slides: source %q, status %v", ai.KeySource(ai.Groq), m.aiStatus)
	}
	if view := plain(m.View()); !strings.Contains(view, "gsk_") || !strings.Contains(view, "0q") || !strings.Contains(view, "your key") {
		t.Errorf("the saved key isn't shown masked:\n%s", view)
	}

	m = typeText(t, m, "d")
	if stored.Groq != "" || ai.KeySource(ai.Groq) == ai.FromSaved {
		t.Errorf("d didn't remove the saved key")
	}
}

func TestAdvancedPanelRejectsWrongKeys(t *testing.T) {
	stored := fakeKeys(t)
	m := testModel(t, "")
	m = typeText(t, m, "a")
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	for paste, want := range map[string]string{
		"gsk_this0is0a0groq0key0really":  "looks like a Groq key",
		"short":                          "too short",
		"two words pasted in by mistake": "has spaces",
	} {
		m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlU})
		m = typeText(t, m, paste)
		m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if view := plain(m.View()); !strings.Contains(view, want) {
			t.Errorf("pasting %q doesn't say %q:\n%s", paste, want, view)
		}
	}
	if *stored != (config.Keys{}) {
		t.Errorf("a wrong key was saved: %+v", *stored)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.keys.open {
		t.Error("esc didn't close the panel")
	}
}

func TestMaskKey(t *testing.T) {
	if got := maskKey("AIzaSyD0123456789abcdefWXYZ"); got != "AIza"+strings.Repeat("•", 16)+"WXYZ" {
		t.Errorf("maskKey = %q", got)
	}
	if got := maskKey("short"); got != "•••••" {
		t.Errorf("a short key isn't fully hidden: %q", got)
	}
}

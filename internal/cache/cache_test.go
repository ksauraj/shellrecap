package cache

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ksauraj/shellrecap/internal/ai"
)

func TestSaveAndLoadWrapped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELLRECAP_CACHE_DIR", dir)
	groq := ai.Target{Provider: ai.Groq, Model: "openai/gpt-oss-120b"}
	gemini := ai.Target{Provider: ai.Gemini, Model: "gemini-3.8-flash"}

	if _, ok := LoadWrapped(2026, groq); ok {
		t.Fatal("empty cache returned an entry")
	}

	entry := WrappedEntry{
		Year:      2026,
		Provider:  groq.Provider,
		Model:     groq.Model,
		Summary:   "s",
		Commands:  100,
		CreatedAt: time.Now(),
		Sections:  []ai.Section{{Title: "Cloud Wrangler", Description: "d", Quotes: []string{"q"}}},
	}
	if err := SaveWrapped(entry); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dir, "wrapped-2026-groq-openai_gpt-oss-120b.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no Unix permissions; the cache is in the user's own folder
	if perm := info.Mode().Perm(); perm != 0o600 && runtime.GOOS != "windows" {
		t.Errorf("cache file permissions = %o, want 600", perm)
	}

	got, ok := LoadWrapped(2026, groq)
	if !ok || got.Sections[0].Title != "Cloud Wrangler" || got.Commands != 100 {
		t.Errorf("LoadWrapped = %+v, %v", got, ok)
	}
	if _, ok := LoadWrapped(2025, groq); ok {
		t.Error("entry for 2026 was returned for 2025")
	}
	if _, ok := LoadWrapped(2026, gemini); ok {
		t.Error("Groq's entry was returned for Gemini")
	}
	if _, ok := LoadWrapped(2026, ai.Target{Provider: ai.Groq, Model: "openai/gpt-oss-20b"}); ok {
		t.Error("an entry for one model was returned for another")
	}
}

func TestFreshFor(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	entry := WrappedEntry{Summary: "s", Commands: 1000, CreatedAt: now.Add(-48 * time.Hour)}

	tests := []struct {
		name     string
		summary  string
		commands int
		now      time.Time
		want     bool
	}{
		{"identical stats", "s", 1000, now, true},
		{"identical stats long ago", "s", 1000, now.Add(365 * 24 * time.Hour), true},
		{"small drift", "changed", 1080, now, true},
		{"large drift", "changed", 1200, now, false},
		{"older than a week", "changed", 1001, now.Add(6 * 24 * time.Hour), false},
	}
	for _, tt := range tests {
		if got := entry.FreshFor(tt.summary, tt.commands, tt.now); got != tt.want {
			t.Errorf("%s: FreshFor = %v, want %v", tt.name, got, tt.want)
		}
	}
}

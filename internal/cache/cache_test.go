package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ksauraj/shellrecap/internal/gemini"
)

func TestSaveAndLoadWrapped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELLRECAP_CACHE_DIR", dir)

	if _, ok := LoadWrapped(2026); ok {
		t.Fatal("empty cache returned an entry")
	}

	entry := WrappedEntry{
		Year:      2026,
		Model:     "m",
		Summary:   "s",
		Commands:  100,
		CreatedAt: time.Now(),
		Sections:  []gemini.Section{{Title: "Cloud Wrangler", Description: "d", Quotes: []string{"q"}}},
	}
	if err := SaveWrapped(entry); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dir, "wrapped-2026.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache file permissions = %o, want 600", perm)
	}

	got, ok := LoadWrapped(2026)
	if !ok || got.Sections[0].Title != "Cloud Wrangler" || got.Commands != 100 {
		t.Errorf("LoadWrapped = %+v, %v", got, ok)
	}
	if _, ok := LoadWrapped(2025); ok {
		t.Error("entry for 2026 was returned for 2025")
	}
}

func TestFreshFor(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	entry := WrappedEntry{Model: "m", Summary: "s", Commands: 1000, CreatedAt: now.Add(-48 * time.Hour)}

	tests := []struct {
		name     string
		model    string
		summary  string
		commands int
		now      time.Time
		want     bool
	}{
		{"identical stats", "m", "s", 1000, now, true},
		{"identical stats long ago", "m", "s", 1000, now.Add(365 * 24 * time.Hour), true},
		{"small drift", "m", "changed", 1080, now, true},
		{"large drift", "m", "changed", 1200, now, false},
		{"older than a week", "m", "changed", 1001, now.Add(6 * 24 * time.Hour), false},
		{"different model", "other", "s", 1000, now, false},
	}
	for _, tt := range tests {
		if got := entry.FreshFor(tt.model, tt.summary, tt.commands, tt.now); got != tt.want {
			t.Errorf("%s: FreshFor = %v, want %v", tt.name, got, tt.want)
		}
	}
}

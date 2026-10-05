// internal/cache/cache.go
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ksauraj/shellrecap/internal/gemini"
)

const (
	// maxAge is how long cached AI slides are reused while the stats drift
	maxAge = 7 * 24 * time.Hour
	// maxCommandDrift is how much the command count may change, as a
	// fraction, before cached AI slides are considered stale
	maxCommandDrift = 0.10
)

// WrappedEntry is a cached Gemini response for one year's Wrapped
type WrappedEntry struct {
	Year      int              `json:"year"`
	Model     string           `json:"model"`
	Summary   string           `json:"summary"`
	Commands  int              `json:"commands"`
	CreatedAt time.Time        `json:"created_at"`
	Sections  []gemini.Section `json:"sections"`
}

// Dir returns the cache directory. SHELLRECAP_CACHE_DIR overrides the
// platform default (e.g. ~/.cache/shellrecap on Linux).
func Dir() (string, error) {
	if dir := os.Getenv("SHELLRECAP_CACHE_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "shellrecap"), nil
}

func wrappedPath(year int) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("wrapped-%d.json", year)), nil
}

// LoadWrapped returns the cached AI slides for the given year, if any
func LoadWrapped(year int) (WrappedEntry, bool) {
	path, err := wrappedPath(year)
	if err != nil {
		return WrappedEntry{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return WrappedEntry{}, false
	}
	var entry WrappedEntry
	if err := json.Unmarshal(data, &entry); err != nil || entry.Year != year || len(entry.Sections) == 0 {
		return WrappedEntry{}, false
	}
	return entry, true
}

// SaveWrapped stores AI slides so later runs can skip the API call
func SaveWrapped(entry WrappedEntry) error {
	path, err := wrappedPath(entry.Year)
	if err != nil {
		return err
	}
	// The slides describe the user's habits, so keep them private
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	// Write then rename so a crash never leaves a half-written cache
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// FreshFor reports whether the entry can be reused instead of calling the
// API. Identical stats are always reused; otherwise the entry is reused for
// up to a week as long as the command count hasn't drifted much.
func (e WrappedEntry) FreshFor(model, summary string, commands int, now time.Time) bool {
	if e.Model != model {
		return false
	}
	if e.Summary == summary {
		return true
	}
	if now.Sub(e.CreatedAt) > maxAge || e.Commands <= 0 {
		return false
	}
	drift := float64(commands-e.Commands) / float64(e.Commands)
	return drift >= -maxCommandDrift && drift <= maxCommandDrift
}

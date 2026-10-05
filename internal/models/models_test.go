package models

import (
	"errors"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/cache"
	"github.com/ksauraj/shellrecap/internal/gemini"
)

// TestMain points the cache at a temporary directory so tests never touch
// the real one. Tests that need an isolated cache set their own.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "shellrecap-cache-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("SHELLRECAP_CACHE_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func testData() analyzer.ShellData {
	data := analyzer.InitShellData()
	base := time.Date(2026, 3, 10, 14, 0, 0, 0, time.Local)
	for i, cmd := range []string{"git commit -m x", "git push", "kubectl get pods", "vim main.go", "gti status"} {
		data.Histories["fish"] = append(data.Histories["fish"], analyzer.CommandEntry{
			Command:   cmd,
			Program:   analyzer.ProgramName(cmd),
			Timestamp: base.Add(time.Duration(i) * 25 * time.Hour),
		})
	}
	return data
}

// testModel returns a model with test data loaded, showing the Wrapped tab.
// apiKey controls whether Gemini counts as configured; no request is ever
// made because the tests never run the returned commands.
func testModel(t *testing.T, apiKey string) Model {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", apiKey)
	t.Setenv("GEMINI_MODEL", "test-model")

	m := Model{
		viewport:    viewport.New(80, 26),
		width:       80,
		height:      30,
		loading:     true,
		tabs:        []string{"Overview", "Tech Profile", "Work Patterns", "Tool Usage", "Recap", "Timeline"},
		logger:      log.New(io.Discard, "", 0),
		wrappedYear: 2026,
		autoplay:    true,
	}
	m = update(t, m, testData())
	m.switchTab(4)
	return settle(m)
}

func update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := m.Update(msg)
	return updated.(Model)
}

// settle finishes the reveal animation so numbers show their final values
func settle(m Model) Model {
	m.frame = m.revealStart + revealFrames
	m.syncContent()
	return m
}

func lastSlide(t *testing.T, m Model) string {
	t.Helper()
	m.currentSectionIndex = len(m.slides()) - 1
	m.syncContent()
	return m.View()
}

func TestWrappedWithoutAPIKey(t *testing.T) {
	m := testModel(t, "")
	if m.aiStatus != aiDisabled {
		t.Fatalf("aiStatus = %v, want aiDisabled", m.aiStatus)
	}
	if view := lastSlide(t, m); !strings.Contains(view, "GEMINI_API_KEY") {
		t.Errorf("last slide doesn't explain how to enable AI slides:\n%s", view)
	}
}

func TestWrappedAppendsAISlides(t *testing.T) {
	m := testModel(t, "fake-key")
	if m.aiStatus != aiPending {
		t.Fatalf("aiStatus = %v, want aiPending", m.aiStatus)
	}
	localSlides := len(m.slides())

	m = update(t, m, aiWrappedMsg{id: m.aiRequestID, sections: []gemini.Section{
		{Title: "**Cloud Wrangler** \U0001F680", Description: "You live in kubectl. \u2728", Quotes: []string{"get pods, get life"}},
	}})

	if got := len(m.slides()); got != localSlides+1 {
		t.Fatalf("got %d slides, want %d", got, localSlides+1)
	}
	view := lastSlide(t, m)
	for _, want := range []string{"Cloud Wrangler", "You live in kubectl.", "get pods, get life", "· AI", "just now"} {
		if !strings.Contains(view, want) {
			t.Errorf("AI slide is missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "**") || strings.ContainsAny(view, "\U0001F680\u2728") {
		t.Errorf("markdown or emoji wasn't stripped:\n%s", view)
	}
}

func TestAIResultIsCachedAndReused(t *testing.T) {
	t.Setenv("SHELLRECAP_CACHE_DIR", t.TempDir())
	m := testModel(t, "fake-key")

	updated, cmd := m.Update(aiWrappedMsg{
		id:       m.aiRequestID,
		sections: []gemini.Section{{Title: "Cloud Wrangler", Description: "d"}},
		summary:  m.wrappedStats.Summary(),
		commands: m.wrappedStats.TotalCommands,
	})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("successful AI response wasn't saved")
	}
	cmd() // write the cache

	entry, ok := cache.LoadWrapped(2026)
	if !ok || entry.Sections[0].Title != "Cloud Wrangler" || entry.Model != "test-model" {
		t.Fatalf("cache = %+v, %v", entry, ok)
	}

	// A new session with the same stats uses the cache instead of the API
	next := testModel(t, "fake-key")
	if next.aiStatus != aiDone || !next.aiFromCache || next.aiRequestID != 0 {
		t.Errorf("status = %v, fromCache = %v, requests = %d; want cached slides and no request",
			next.aiStatus, next.aiFromCache, next.aiRequestID)
	}
	if view := lastSlide(t, next); !strings.Contains(view, "Cached from just now · r: regenerate") {
		t.Errorf("cached slide doesn't say it's cached:\n%s", view)
	}
}

func TestRefreshKeyBypassesCache(t *testing.T) {
	t.Setenv("SHELLRECAP_CACHE_DIR", t.TempDir())
	t.Setenv("GEMINI_MODEL", "test-model")
	stats := analyzer.ComputeWrapped(testData(), 2026)
	if err := cache.SaveWrapped(cache.WrappedEntry{
		Year: 2026, Model: "test-model", Summary: stats.Summary(), Commands: stats.TotalCommands,
		CreatedAt: time.Now(), Sections: []gemini.Section{{Title: "Old"}},
	}); err != nil {
		t.Fatal(err)
	}

	m := testModel(t, "fake-key")
	if !m.aiFromCache {
		t.Fatal("fresh cache wasn't used")
	}

	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if !m.loading || !m.refreshing {
		t.Fatal("r didn't start a refresh")
	}
	m = update(t, m, testData())
	if m.aiStatus != aiPending || m.aiRequestID == 0 {
		t.Errorf("refresh didn't request new AI slides: status = %v, requests = %d", m.aiStatus, m.aiRequestID)
	}
}

func TestStaleAIResponseIsIgnored(t *testing.T) {
	m := testModel(t, "fake-key")
	stale := m.aiRequestID
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = update(t, m, testData())

	m = update(t, m, aiWrappedMsg{id: stale, sections: []gemini.Section{{Title: "Stale"}}})
	if m.aiStatus != aiPending || len(m.aiSections) != 0 {
		t.Errorf("response from before the refresh was used: %+v", m.aiSections)
	}
}

func TestAIErrorFallsBackToCache(t *testing.T) {
	t.Setenv("SHELLRECAP_CACHE_DIR", t.TempDir())
	m := testModel(t, "fake-key")
	cache.SaveWrapped(cache.WrappedEntry{
		Year: 2026, Model: "test-model", CreatedAt: time.Now().Add(-3 * time.Hour),
		Sections: []gemini.Section{{Title: "Cached Wrangler"}},
	})

	m = update(t, m, aiWrappedMsg{id: m.aiRequestID, err: errors.New("quota exceeded")})
	view := lastSlide(t, m)
	for _, want := range []string{"Cached Wrangler", "quota exceeded", "showing slides from 3"} {
		if !strings.Contains(view, want) {
			t.Errorf("fallback slide is missing %q:\n%s", want, view)
		}
	}
}

func TestWrappedShowsAIError(t *testing.T) {
	m := testModel(t, "fake-key")
	m = update(t, m, aiWrappedMsg{id: m.aiRequestID, err: errors.New("quota exceeded")})

	if view := lastSlide(t, m); !strings.Contains(view, "quota exceeded") {
		t.Errorf("AI error isn't shown:\n%s", view)
	}
}

func TestSlideNavigationAndAutoplay(t *testing.T) {
	m := testModel(t, "")
	total := len(m.slides())

	m = update(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.currentSectionIndex != total-1 {
		t.Errorf("left from the first slide went to %d, want %d", m.currentSectionIndex, total-1)
	}

	// A stale tick from before the key press must not advance the slide
	m = update(t, m, slideTickMsg{id: m.slideTickID - 1})
	if m.currentSectionIndex != total-1 {
		t.Errorf("stale tick advanced the slide to %d", m.currentSectionIndex)
	}

	m = update(t, m, slideTickMsg{id: m.slideTickID})
	if m.currentSectionIndex != 0 {
		t.Errorf("tick went to slide %d, want 0", m.currentSectionIndex)
	}
}

func TestRevealAnimation(t *testing.T) {
	m := testModel(t, "")
	m.switchTab(0)

	if view := plain(m.View()); !strings.Contains(view, "Total commands:   0") {
		t.Errorf("numbers don't start at zero:\n%s", view)
	}

	// Run the animation until it stops by itself on a static tab
	var cmd tea.Cmd
	for i := 0; i < revealFrames*2; i++ {
		var updated tea.Model
		updated, cmd = m.Update(animTickMsg{})
		m = updated.(Model)
	}
	if cmd != nil || m.animating {
		t.Error("animation kept ticking after the reveal finished")
	}
	if view := plain(m.View()); !strings.Contains(view, "Total commands:   5") {
		t.Errorf("numbers didn't count up to their final value:\n%s", view)
	}
}

func TestNoEmojiOnScreen(t *testing.T) {
	m := testModel(t, "fake-key")
	m = update(t, m, aiWrappedMsg{id: m.aiRequestID, sections: []gemini.Section{{Title: "\U0001F389 Party", Description: "\u2b50 star"}}})

	views := []string{loadingView(m)}
	for tab := range m.tabs {
		m.switchTab(tab)
		views = append(views, settle(m).View())
	}
	m.switchTab(4)
	for i := range m.slides() {
		m.currentSectionIndex = i
		views = append(views, settle(m).View())
	}

	for _, view := range views {
		for _, r := range view {
			if isEmoji(r) {
				t.Fatalf("found emoji %q in:\n%s", r, view)
			}
		}
	}
}

var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// plain strips terminal colors
func plain(s string) string {
	return ansiCodes.ReplaceAllString(s, "")
}

func loadingView(m Model) string {
	m.loading = true
	return m.View()
}

func TestViewFitsTerminal(t *testing.T) {
	m := testModel(t, "")
	m = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 15})

	for tab := range m.tabs {
		m.switchTab(tab)
		lines := strings.Split(settle(m).View(), "\n")
		if len(lines) > 15 {
			t.Errorf("tab %s renders %d lines in a 15 line terminal", m.tabs[tab], len(lines))
		}
		for _, line := range lines {
			if w := lipgloss.Width(line); w > 60 {
				t.Errorf("tab %s renders a %d column line in a 60 column terminal: %q", m.tabs[tab], w, line)
			}
		}
	}
}

func TestCleanAIText(t *testing.T) {
	if got := cleanAIText("\U0001F680 **Cloud**  Wrangler \u2728\n"); got != "Cloud Wrangler" {
		t.Errorf("cleanAIText = %q", got)
	}
}

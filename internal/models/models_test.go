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
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/cache"
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
	return testModelWith(t, Config{}, apiKey, "")
}

var (
	geminiTarget = ai.Target{Provider: ai.Gemini, Model: "test-model"}
	groqTarget   = ai.Target{Provider: ai.Groq, Model: "openai/gpt-oss-120b"}
)

// testModelWith is testModel with command line options and a Groq key
func testModelWith(t *testing.T, cfg Config, geminiKey, groqKey string) Model {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", geminiKey)
	t.Setenv("GEMINI_MODEL", "test-model")
	t.Setenv("GROQ_API_KEY", groqKey)
	t.Setenv("GROQ_MODEL", "")

	m := Model{
		aiOpts:      cfg.AI,
		noCache:     cfg.NoCache,
		viewport:    viewport.New(80, 26),
		width:       80,
		height:      30,
		loading:     true,
		tabs:        []string{"Overview", "Tech Profile", "Work Patterns", "Tool Usage", "Recap"},
		logger:      log.New(io.Discard, "", 0),
		wrappedYear: 2026,
		autoplay:    true,
	}
	m = update(t, m, testData())
	m.switchTab(4)
	return settle(m)
}

func aiResult(target ai.Target, sections ...ai.Section) ai.Result {
	return ai.Result{Target: target, Sections: sections, Duration: 1500 * time.Millisecond}
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

	m = update(t, m, aiWrappedMsg{id: m.aiRequestID, result: aiResult(geminiTarget,
		ai.Section{Title: "**Cloud Wrangler** \U0001F680", Description: "You live in kubectl. \u2728", Quotes: []string{"get pods, get life"}},
	)})

	if got := len(m.slides()); got != localSlides+1 {
		t.Fatalf("got %d slides, want %d", got, localSlides+1)
	}
	view := lastSlide(t, m)
	for _, want := range []string{"Cloud Wrangler", "You live in kubectl.", "get pods, get life", "· AI", "Written by Gemini (test-model) in 1.5s"} {
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
		result:   aiResult(geminiTarget, ai.Section{Title: "Cloud Wrangler", Description: "d"}),
		summary:  m.wrappedStats.Summary(),
		commands: m.wrappedStats.TotalCommands,
	})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("successful AI response wasn't saved")
	}
	cmd() // write the cache

	entry, ok := cache.LoadWrapped(2026, geminiTarget)
	if !ok || entry.Sections[0].Title != "Cloud Wrangler" || entry.Duration != 1.5 {
		t.Fatalf("cache = %+v, %v", entry, ok)
	}

	// A new session with the same stats uses the cache instead of the API
	next := testModel(t, "fake-key")
	if next.aiStatus != aiDone || !next.aiFromCache || next.aiRequestID != 0 {
		t.Errorf("status = %v, fromCache = %v, requests = %d; want cached slides and no request",
			next.aiStatus, next.aiFromCache, next.aiRequestID)
	}
	if view := lastSlide(t, next); !strings.Contains(view, "Written by Gemini (test-model) just now · r: regenerate") {
		t.Errorf("cached slide doesn't say it's cached:\n%s", view)
	}
}

func TestRefreshKeyBypassesCache(t *testing.T) {
	t.Setenv("SHELLRECAP_CACHE_DIR", t.TempDir())
	t.Setenv("GEMINI_MODEL", "test-model")
	stats := analyzer.ComputeWrapped(testData(), 2026)
	if err := cache.SaveWrapped(cache.WrappedEntry{
		Year: 2026, Provider: ai.Gemini, Model: "test-model", Summary: stats.Summary(), Commands: stats.TotalCommands,
		CreatedAt: time.Now(), Sections: []ai.Section{{Title: "Old"}},
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

	m = update(t, m, aiWrappedMsg{id: stale, result: aiResult(geminiTarget, ai.Section{Title: "Stale"})})
	if m.aiStatus != aiPending || len(m.aiSections) != 0 {
		t.Errorf("response from before the refresh was used: %+v", m.aiSections)
	}
}

func TestAIErrorFallsBackToCache(t *testing.T) {
	t.Setenv("SHELLRECAP_CACHE_DIR", t.TempDir())
	m := testModel(t, "fake-key")
	cache.SaveWrapped(cache.WrappedEntry{
		Year: 2026, Provider: ai.Gemini, Model: "test-model", CreatedAt: time.Now().Add(-3 * time.Hour),
		Sections: []ai.Section{{Title: "Cached Wrangler"}},
	})

	m = update(t, m, aiWrappedMsg{id: m.aiRequestID, err: errors.New("quota exceeded")})
	view := lastSlide(t, m)
	for _, want := range []string{"Cached Wrangler", "Written by Gemini (test-model) 3 hours ago"} {
		if !strings.Contains(view, want) {
			t.Errorf("fallback slide is missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "quota exceeded") {
		t.Errorf("the API error is shown on screen:\n%s", view)
	}
}

func TestWrappedShowsAIError(t *testing.T) {
	m := testModel(t, "fake-key")
	m = update(t, m, aiWrappedMsg{id: m.aiRequestID, err: errors.New("gemini API error (HTTP 503): quota exceeded")})

	view := lastSlide(t, m)
	if !strings.Contains(view, "AI slides are unavailable right now. Press r to try again.") {
		t.Errorf("failure isn't explained:\n%s", view)
	}
	if strings.Contains(view, "quota exceeded") || strings.Contains(view, "HTTP") {
		t.Errorf("the API error is shown on screen:\n%s", view)
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

	if view := plain(m.View()); !regexp.MustCompile(`Total commands\s+0\s`).MatchString(view) {
		t.Errorf("numbers don't start at zero:\n%s", view)
	}

	tick := func(m Model, n int) (Model, tea.Cmd) {
		var cmd tea.Cmd
		for i := 0; i < n; i++ {
			var updated tea.Model
			updated, cmd = m.Update(animTickMsg{})
			m = updated.(Model)
		}
		return m, cmd
	}

	// The glare keeps sweeping, so the animation never stops
	m, cmd := tick(m, revealFrames*2)
	if cmd == nil {
		t.Error("animation stopped")
	}
	if view := plain(m.View()); !regexp.MustCompile(`Total commands\s+5\s`).MatchString(view) {
		t.Errorf("numbers didn't count up to their final value:\n%s", view)
	}

}

func TestNoEmojiOnScreen(t *testing.T) {
	m := testModel(t, "fake-key")
	m = update(t, m, aiWrappedMsg{id: m.aiRequestID, result: aiResult(geminiTarget, ai.Section{Title: "\U0001F389 Party", Description: "\u2b50 star"})})

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
			if ai.IsEmoji(r) {
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
	if got := ai.CleanText("\U0001F680 **Cloud**  Wrangler \u2728\n"); got != "Cloud Wrangler" {
		t.Errorf("cleanAIText = %q", got)
	}
}

func TestScrollPercentNeverNaN(t *testing.T) {
	m := testModel(t, "")
	m.switchTab(0)
	// Content exactly one line taller than the viewport used to show NaN%
	m.viewport.SetContent(strings.Repeat("line\n", m.viewport.Height))
	if view := m.View(); strings.Contains(view, "NaN") {
		t.Errorf("footer shows NaN:\n%s", view)
	}
	m.viewport.GotoBottom()
	if view := plain(m.View()); !strings.Contains(view, "100%") {
		t.Errorf("footer doesn't show 100%% at the bottom:\n%s", view)
	}
}

func TestRecapSlidesStartTopLeft(t *testing.T) {
	m := testModel(t, "")
	m = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m.switchTab(4)

	// Like the cards on the other tabs, the slide starts right under the
	// tab bar at the left edge
	content := strings.Split(plain(m.View()), "\n")[chromeHeight-1:]
	if !strings.HasPrefix(content[0], "╭") {
		t.Errorf("slide doesn't start at the top left:\n%s", strings.Join(content[:3], "\n"))
	}
	m.switchTab(0)
	if overview := strings.Split(plain(m.View()), "\n")[chromeHeight-1:]; !strings.HasPrefix(overview[0], "╭") {
		t.Errorf("overview doesn't start at the top left:\n%s", strings.Join(overview[:3], "\n"))
	}
}

func TestFallbackHidesErrors(t *testing.T) {
	m := testModelWith(t, Config{}, "gemini-key", "groq-key")
	result := aiResult(groqTarget, ai.Section{Title: "Fast Fallback"})
	result.Failures = []error{errors.New("Gemini (gemini-3.8-flash) failed: gemini API error (HTTP 503): high demand")}
	m = update(t, m, aiWrappedMsg{id: m.aiRequestID, result: result})

	view := plain(lastSlide(t, m))
	for _, want := range []string{"Fast Fallback", "Written by Groq (openai/gpt-oss-120b) in 1.5s."} {
		if !strings.Contains(view, want) {
			t.Errorf("fallback slide is missing %q:\n%s", want, view)
		}
	}
	for _, leak := range []string{"HTTP", "503", "high demand", "failed"} {
		if strings.Contains(view, leak) {
			t.Errorf("the fallback's error leaks onto the slide (%q):\n%s", leak, view)
		}
	}
}

func TestProviderFlagUsesItsOwnCache(t *testing.T) {
	t.Setenv("SHELLRECAP_CACHE_DIR", t.TempDir())
	t.Setenv("GEMINI_MODEL", "test-model")
	stats := analyzer.ComputeWrapped(testData(), 2026)
	cache.SaveWrapped(cache.WrappedEntry{
		Year: 2026, Provider: ai.Gemini, Model: "test-model", Summary: stats.Summary(), Commands: stats.TotalCommands,
		CreatedAt: time.Now(), Sections: []ai.Section{{Title: "From Gemini"}},
	})

	// The default order picks up Gemini's cached slides...
	if m := testModelWith(t, Config{}, "gemini-key", "groq-key"); !m.aiFromCache || m.aiTarget.Provider != ai.Gemini {
		t.Errorf("default run didn't use Gemini's cache: %+v", m.aiTarget)
	}
	// ...but testing Groq asks Groq instead of showing Gemini's slides
	m := testModelWith(t, Config{AI: ai.Options{Provider: ai.Groq}}, "gemini-key", "groq-key")
	if m.aiFromCache || m.aiStatus != aiPending {
		t.Errorf("--provider groq used another provider's cache: status = %v, target = %+v", m.aiStatus, m.aiTarget)
	}
}

func TestNoCacheAlwaysAsks(t *testing.T) {
	t.Setenv("SHELLRECAP_CACHE_DIR", t.TempDir())
	t.Setenv("GEMINI_MODEL", "test-model")
	stats := analyzer.ComputeWrapped(testData(), 2026)
	cache.SaveWrapped(cache.WrappedEntry{
		Year: 2026, Provider: ai.Gemini, Model: "test-model", Summary: stats.Summary(), Commands: stats.TotalCommands,
		CreatedAt: time.Now(), Sections: []ai.Section{{Title: "Cached"}},
	})

	m := testModelWith(t, Config{NoCache: true}, "gemini-key", "")
	if m.aiFromCache || m.aiStatus != aiPending {
		t.Errorf("--no-cache used the cache: status = %v", m.aiStatus)
	}
}

func TestKeyHintMatchesProvider(t *testing.T) {
	m := testModelWith(t, Config{AI: ai.Options{Provider: ai.Groq}}, "", "")
	if view := lastSlide(t, m); !strings.Contains(view, "Set GROQ_API_KEY") {
		t.Errorf("hint doesn't mention GROQ_API_KEY:\n%s", view)
	}
	m = testModelWith(t, Config{}, "", "")
	if view := lastSlide(t, m); !strings.Contains(view, "GEMINI_API_KEY or GROQ_API_KEY") {
		t.Errorf("hint doesn't mention both keys:\n%s", view)
	}
}

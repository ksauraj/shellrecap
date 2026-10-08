package models

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/config"
	"github.com/ksauraj/shellrecap/internal/share"
	"github.com/ksauraj/shellrecap/internal/theme"
)

// fakeDesktop replaces the share job and the desktop actions, recording
// what they were asked to do
type fakeDesktop struct {
	shares    int
	lastShare config.Share
	saved     []config.Config
	opened    []string
	shown     []string
	copied    []string
	desktop   bool
	openErr   error
}

var testFiles = share.Files{
	Dir: "/home/me/Pictures/shellrecap",
	Outputs: []share.Output{
		{Kind: config.RecapGIF, Name: "Recap", Path: "/home/me/Pictures/shellrecap/shellrecap-2026-recap.gif", Bytes: 700_000},
		{Kind: config.TourGIF, Name: "Tour", Path: "/home/me/Pictures/shellrecap/shellrecap-2026-tour.gif", Bytes: 900_000},
		{Kind: config.Poster, Name: "Poster", Path: "/home/me/Pictures/shellrecap/shellrecap-2026-poster.png", Bytes: 110_000},
		{Kind: config.Tabs, Name: "Overview", Path: "/home/me/Pictures/shellrecap/shellrecap-2026-overview.png"},
		{Kind: config.Tabs, Name: "Tech Profile", Path: "/home/me/Pictures/shellrecap/shellrecap-2026-tech-profile.png"},
	},
	Caption: "My 2026 in the terminal #shellrecap",
	Copied:  true,
}

const (
	recapGIF = "/home/me/Pictures/shellrecap/shellrecap-2026-recap.gif"
	poster   = "/home/me/Pictures/shellrecap/shellrecap-2026-poster.png"
)

func fakeShare(t *testing.T, files share.Files, err error) *fakeDesktop {
	t.Helper()
	f := &fakeDesktop{desktop: true}
	oldRun, oldOpen, oldReveal, oldCopy, oldDesktop, oldSave := runShare, openURL, reveal, copyText, isDesktop, saveSettings
	t.Cleanup(func() {
		runShare, openURL, reveal, copyText, isDesktop, saveSettings = oldRun, oldOpen, oldReveal, oldCopy, oldDesktop, oldSave
	})
	runShare = func(_ ai.Options, s config.Share) (share.Files, error) {
		f.shares++
		f.lastShare = s
		return files, err
	}
	saveSettings = func(c config.Config) error {
		f.saved = append(f.saved, c)
		return nil
	}
	openURL = func(url string) error {
		f.opened = append(f.opened, url)
		return f.openErr
	}
	reveal = func(path string) error {
		f.shown = append(f.shown, path)
		return nil
	}
	copyText = func(text string) error {
		f.copied = append(f.copied, text)
		return nil
	}
	isDesktop = func() bool { return f.desktop }
	return f
}

// pressShare presses s and runs the background job to completion
func pressShare(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if !m.share.open || !m.share.busy || cmd == nil {
		t.Fatalf("s didn't start creating the images: %+v", m.share)
	}
	if view := plain(m.View()); !strings.Contains(view, "Creating your share images") {
		t.Errorf("no progress while creating the images:\n%s", view)
	}
	return update(t, m, cmd())
}

func press(t *testing.T, m Model, key string) Model {
	t.Helper()
	if key == "esc" {
		return update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	}
	return update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func TestShareMenu(t *testing.T) {
	fake := fakeShare(t, testFiles, nil)
	m := pressShare(t, testModel(t, ""))

	view := plain(m.View())
	for _, want := range []string{"Share your recap", "Saved 5 images", "shellrecap-2026-recap.gif", "animated Recap · 0.7 MB",
		"shellrecap-2026-tour.gif", "shellrecap-2026-poster.png", "· copied ·", "+ 2 tab pictures",
		"x X", "b Bluesky", "l LinkedIn", "o open folder", "c customize"} {
		if !strings.Contains(view, want) {
			t.Errorf("share menu is missing %q:\n%s", want, view)
		}
	}
	if fake.shares != 1 {
		t.Errorf("the images were created %d times", fake.shares)
	}

	m = press(t, m, "esc")
	if m.share.open || strings.Contains(plain(m.View()), "Share your recap") {
		t.Error("esc didn't close the share menu")
	}
}

func TestShareToX(t *testing.T) {
	fake := fakeShare(t, testFiles, nil)
	m := press(t, pressShare(t, testModel(t, "")), "x")

	if len(fake.opened) != 1 || !strings.HasPrefix(fake.opened[0], "https://x.com/intent/tweet?text=My%202026") {
		t.Errorf("opened %v, want X's compose page with the caption", fake.opened)
	}
	// X plays GIFs, so that's the file to drag in
	if len(fake.shown) != 1 || fake.shown[0] != recapGIF {
		t.Errorf("showed %v, want the GIF", fake.shown)
	}
	if view := plain(m.View()); !strings.Contains(view, "Paste the poster") {
		t.Errorf("no instructions after opening X:\n%s", view)
	}
}

func TestShareToLinkedIn(t *testing.T) {
	fake := fakeShare(t, testFiles, nil)
	m := press(t, pressShare(t, testModel(t, "")), "l")

	// LinkedIn can't prefill text, so the caption goes on the clipboard
	if len(fake.copied) != 1 || fake.copied[0] != testFiles.Caption {
		t.Errorf("copied %v, want the caption", fake.copied)
	}
	if len(fake.shown) != 1 || fake.shown[0] != poster {
		t.Errorf("showed %v, want the poster", fake.shown)
	}
	view := plain(m.View())
	if !strings.Contains(view, "copied your caption") || strings.Contains(view, "· copied ·") {
		t.Errorf("menu still says the poster is on the clipboard:\n%s", view)
	}
}

func TestShareFailureIsFriendly(t *testing.T) {
	fakeShare(t, share.Files{}, errors.New("exit status 1: open /home/me/Pictures: permission denied"))
	m := pressShare(t, testModel(t, ""))

	view := plain(m.View())
	if !strings.Contains(view, "Couldn't create the images") || strings.Contains(view, "permission denied") {
		t.Errorf("failure isn't explained in plain words:\n%s", view)
	}
}

func TestShareOpenFailure(t *testing.T) {
	fake := fakeShare(t, testFiles, nil)
	fake.openErr = errors.New("xdg-open: not found")
	m := press(t, pressShare(t, testModel(t, "")), "b")

	view := plain(m.View())
	if !strings.Contains(view, "Couldn't open that here") || strings.Contains(view, "xdg-open") {
		t.Errorf("open failure isn't explained in plain words:\n%s", view)
	}
}

func TestShareWithoutDesktop(t *testing.T) {
	fake := fakeShare(t, testFiles, nil)
	fake.desktop = false
	m := press(t, pressShare(t, testModel(t, "")), "x")

	if len(fake.opened) != 0 {
		t.Errorf("tried to open %v without a desktop", fake.opened)
	}
	view := plain(m.View())
	if !strings.Contains(view, "Copy the files to your computer") || strings.Contains(view, "x X") {
		t.Errorf("menu offers platforms without a desktop:\n%s", view)
	}
}

func TestShareWithoutHistory(t *testing.T) {
	fake := fakeShare(t, testFiles, nil)
	m := testModel(t, "")
	m = update(t, m, analyzer.InitShellData())

	m = press(t, m, "s")
	if fake.shares != 0 {
		t.Error("created images of an empty recap")
	}
	if view := plain(m.View()); !strings.Contains(view, "no shell history to share yet") {
		t.Errorf("empty recap isn't explained:\n%s", view)
	}
}

func TestCustomizeShare(t *testing.T) {
	fake := fakeShare(t, testFiles, nil)
	m := pressShare(t, testModel(t, ""))
	m.settings = config.Defaults()

	m = press(t, m, "c")
	view := plain(m.View())
	for _, want := range []string{"Customize your share images", "> Recap GIF", "[x] on", "Slide pictures", "[ ] off",
		"‹ Standard ›", "‹ Portrait ›", "‹ Dark ›", "Create images"} {
		if !strings.Contains(view, want) {
			t.Errorf("customize panel is missing %q:\n%s", want, view)
		}
	}

	// Turn on slide pictures, pick the story shape and the black theme
	down := tea.KeyMsg{Type: tea.KeyDown}
	right := tea.KeyMsg{Type: tea.KeyRight}
	space := tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	for _, key := range []tea.KeyMsg{down, down, down, down, space, down, down, down, right, down, right} {
		m = update(t, m, key)
	}
	s := m.settings.Share
	if !s.Has(config.Slides) || s.Shape != config.Story || s.ImageTheme != "black" {
		t.Fatalf("settings = %+v, want slides on, story shape, black theme", s)
	}
	if len(fake.saved) != 3 {
		t.Errorf("settings were saved %d times, want after each of the 3 changes", len(fake.saved))
	}

	// Create with the new choices
	m.share.cursor = len(shareChoices)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || !m.share.busy {
		t.Fatal("enter on Create didn't start creating the images")
	}
	m = update(t, m, cmd())
	if fake.lastShare.Shape != config.Story || !fake.lastShare.Has(config.Slides) {
		t.Errorf("created with %+v, want the new choices", fake.lastShare)
	}
}

// Customizing while the images are still being created creates them again
// with the new choices once the first run is done
func TestCustomizeWhileBusy(t *testing.T) {
	fake := fakeShare(t, testFiles, nil)
	m := testModel(t, "")
	m.settings = config.Defaults()
	updated, first := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if view := plain(m.View()); !strings.Contains(view, "c customize") {
		t.Errorf("no way to customize while creating the images:\n%s", view)
	}

	m = press(t, m, "c")
	if !m.share.customizing {
		t.Fatal("c didn't open the customize panel while creating the images")
	}
	m.share.cursor = 6 // GIF size
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight})
	m.share.cursor = len(shareChoices)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || !m.share.busy || m.share.customizing {
		t.Fatalf("enter should wait for the run that's going: cmd=%v share=%+v", cmd != nil, m.share)
	}

	// The first run finishes, and its images are made again
	updated, again := m.Update(first())
	m = updated.(Model)
	if again == nil || !m.share.busy {
		t.Fatal("the images weren't created again with the new choices")
	}
	m = update(t, m, again())
	if fake.shares != 2 || fake.lastShare.GIFSize != config.GIFSmall {
		t.Errorf("shares = %d, last = %+v; want 2, the second with small GIFs", fake.shares, fake.lastShare)
	}
	if m.share.busy || !strings.Contains(plain(m.View()), "Saved 5 images") {
		t.Errorf("the second run's images aren't shown:\n%s", plain(m.View()))
	}
}

func TestCustomizeKeepsOneImage(t *testing.T) {
	fakeShare(t, testFiles, nil)
	m := testModel(t, "")
	m.settings = config.Defaults()
	m.settings.Share.Outputs = []string{config.Poster}
	m.change(2, 1) // the poster toggle
	if !m.settings.Share.Has(config.Poster) {
		t.Error("the last image kind was turned off")
	}
}

func TestShareArgs(t *testing.T) {
	s := config.Share{Outputs: []string{config.RecapGIF, config.Poster}, GIFSize: config.GIFSmall, Shape: config.Square,
		ImageTheme: "light", NoAI: true}
	got := strings.Join(shareArgs(ai.Options{Provider: ai.Groq}, s), " ")
	want := "share --json --cached-ai --only recap-gif,poster --gif-size small --shape square --theme light --no-ai --provider groq"
	if got != want {
		t.Errorf("shareArgs = %q\nwant      %q", got, want)
	}
}

func TestThemeKey(t *testing.T) {
	fake := fakeShare(t, testFiles, nil)
	defer theme.Use(theme.Dark)
	m := testModel(t, "")
	m.settings = config.Defaults()

	for _, want := range []string{"dark", "black", "light", "terminal", "auto"} {
		m = press(t, m, "t")
		if m.settings.Theme != want || theme.Current().Name != want {
			t.Fatalf("theme = %s (palette %s), want %s", m.settings.Theme, theme.Current().Name, want)
		}
	}
	if len(fake.saved) != 5 {
		t.Errorf("theme was saved %d times, want 5", len(fake.saved))
	}
	if view := plain(m.View()); !strings.Contains(view, "Theme: auto, matched to your terminal") {
		t.Errorf("no confirmation of the theme:\n%s", view)
	}
}

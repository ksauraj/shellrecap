package models

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/share"
)

// fakeDesktop replaces the share job and the desktop actions, recording
// what they were asked to do
type fakeDesktop struct {
	shares  int
	opened  []string
	shown   []string
	copied  []string
	desktop bool
	openErr error
}

var testFiles = share.Files{
	Dir:      "/home/me/Pictures/shellrecap",
	GIF:      "/home/me/Pictures/shellrecap/shellrecap-2026.gif",
	Poster:   "/home/me/Pictures/shellrecap/shellrecap-2026.png",
	GIFBytes: 700_000,
	Caption:  "My 2026 in the terminal #shellrecap",
	Copied:   true,
}

func fakeShare(t *testing.T, files share.Files, err error) *fakeDesktop {
	t.Helper()
	f := &fakeDesktop{desktop: true}
	oldRun, oldOpen, oldReveal, oldCopy, oldDesktop := runShare, openURL, reveal, copyText, isDesktop
	t.Cleanup(func() {
		runShare, openURL, reveal, copyText, isDesktop = oldRun, oldOpen, oldReveal, oldCopy, oldDesktop
	})
	runShare = func(ai.Options) (share.Files, error) {
		f.shares++
		return files, err
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
	for _, want := range []string{"Share your recap", "shellrecap-2026.gif", "animated · 0.7 MB", "shellrecap-2026.png",
		"copied to clipboard", "x X", "b Bluesky", "l LinkedIn", "o open folder"} {
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
	if len(fake.shown) != 1 || fake.shown[0] != testFiles.GIF {
		t.Errorf("showed %v, want the GIF", fake.shown)
	}
	if view := plain(m.View()); !strings.Contains(view, "Paste your poster") {
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
	if len(fake.shown) != 1 || fake.shown[0] != testFiles.Poster {
		t.Errorf("showed %v, want the poster", fake.shown)
	}
	view := plain(m.View())
	if !strings.Contains(view, "copied your caption") || strings.Contains(view, "copied to clipboard") {
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

// internal/models/share.go
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/render"
	"github.com/ksauraj/shellrecap/internal/share"
)

// shareState is the share menu opened with s
type shareState struct {
	open    bool
	busy    bool // the images are still being created
	failed  bool
	empty   bool // there's no history to share
	files   share.Files
	message string // what the last action did
}

// shareDoneMsg carries the images created in the background
type shareDoneMsg struct {
	files share.Files
	err   error
}

// runShare creates the share images by running `shellrecap share` in the
// background. It's a separate process because rendering the images
// switches lipgloss to a true color dark theme, which mustn't happen under
// the running TUI. Only cached AI slides are used, so nothing is asked of
// the AI twice.
var runShare = func(opts ai.Options) (share.Files, error) {
	exe, err := os.Executable()
	if err != nil {
		return share.Files{}, err
	}
	args := []string{"share", "--json", "--cached-ai"}
	if opts.Provider != "" && opts.Provider != ai.Auto {
		args = append(args, "--provider", opts.Provider)
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	out, err := exec.Command(exe, args...).Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return share.Files{}, fmt.Errorf("%v: %s", err, exitErr.Stderr)
	}
	if err != nil {
		return share.Files{}, err
	}
	var files share.Files
	if err := json.Unmarshal(out, &files); err != nil {
		return share.Files{}, fmt.Errorf("reading the share result: %v", err)
	}
	return files, nil
}

// The desktop actions, swapped out in tests
var (
	openURL   = share.OpenURL
	reveal    = share.Reveal
	copyText  = share.CopyText
	isDesktop = share.Desktop
)

func (m *Model) startShare() tea.Cmd {
	if m.share.busy {
		m.share.open = true
		return nil
	}
	if m.wrappedStats.TotalCommands == 0 {
		m.share = shareState{open: true, empty: true}
		m.syncContent()
		return nil
	}
	m.share = shareState{open: true, busy: true}
	m.syncContent()
	m.viewport.GotoTop()
	opts := m.aiOpts
	return func() tea.Msg {
		files, err := runShare(opts)
		return shareDoneMsg{files: files, err: err}
	}
}

func (m *Model) shareDone(msg shareDoneMsg) {
	m.share.busy = false
	if msg.err != nil {
		m.logger.Printf("Error creating share images: %v", msg.err)
		m.share.failed = true
	} else {
		m.logger.Printf("Saved share images to %s", msg.files.Dir)
		m.share.files = msg.files
	}
	m.syncContent()
}

// handleShareKey handles keys while the share menu is open
func (m Model) handleShareKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "s":
		m.share.open = false
		m.syncContent()
		return m, nil
	}
	if m.share.busy || m.share.failed || m.share.empty || !isDesktop() {
		return m, nil
	}

	files := m.share.files
	paste := share.PasteShortcut()
	if key == "o" {
		m.share.message = m.tryOpen("Opened the folder.", func() error { return reveal(files.GIF) })
		m.syncContent()
		return m, nil
	}
	for _, link := range share.Links(files.Caption) {
		if link.Key != key {
			continue
		}
		// The GIF where the platform plays it, the poster everywhere else
		file, what := files.Poster, "the poster"
		if link.Animated {
			file, what = files.GIF, "the GIF"
		}
		var done string
		switch {
		case !link.Prefilled:
			// Copying the caption replaces the poster on the clipboard
			if copyText(files.Caption) == nil {
				m.share.files.Copied = false
				done = fmt.Sprintf("Opened %s and copied your caption. Paste it with %s, then drag in %s from the folder.",
					link.Name, paste, what)
			} else {
				done = fmt.Sprintf("Opened %s. Drag in %s from the folder.", link.Name, what)
			}
		case files.Copied && link.Animated:
			done = fmt.Sprintf("Opened %s with your caption. Paste your poster with %s, or drag in the GIF from the folder.",
				link.Name, paste)
		case files.Copied:
			done = fmt.Sprintf("Opened %s with your caption. Paste your poster with %s.", link.Name, paste)
		default:
			done = fmt.Sprintf("Opened %s with your caption. Drag in %s from the folder.", link.Name, what)
		}
		m.share.message = m.tryOpen(done, func() error {
			if err := openURL(link.URL); err != nil {
				return err
			}
			return reveal(file)
		})
		m.syncContent()
		return m, nil
	}
	return m, nil
}

// tryOpen runs a desktop action and says how it went
func (m *Model) tryOpen(done string, action func() error) string {
	if err := action(); err != nil {
		m.logger.Printf("Error opening a share target: %v", err)
		return "Couldn't open that here. Run shellrecap share to see the links."
	}
	return done
}

// shareMenu describes the share menu for rendering
func (m Model) shareMenu() render.ShareMenu {
	files := m.share.files
	menu := render.ShareMenu{
		Busy:    m.share.busy,
		Failed:  m.share.failed,
		Empty:   m.share.empty,
		Frame:   m.frame,
		Dir:     share.HomePath(files.Dir),
		GIF:     filepath.Base(files.GIF),
		Poster:  filepath.Base(files.Poster),
		GIFSize: files.GIFBytes,
		Copied:  files.Copied,
		Desktop: isDesktop(),
		Message: m.share.message,
	}
	for _, link := range share.Links(files.Caption) {
		menu.Links = append(menu.Links, render.ShareKey{Key: link.Key, Name: link.Name})
	}
	return menu
}

// internal/models/share.go
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/config"
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
	// customizing is set while the customize panel is open
	customizing bool
	cursor      int
	// again is set when the choices changed while images were being
	// created; they're created again once that run finishes
	again bool
}

// shareDoneMsg carries the images created in the background
type shareDoneMsg struct {
	files share.Files
	err   error
}

// shareArgs are the `shellrecap share` arguments for the given choices.
// Only cached AI slides are used, so nothing is asked of the AI twice.
func shareArgs(opts ai.Options, s config.Share) []string {
	args := []string{"share", "--json", "--cached-ai",
		"--only", strings.Join(s.Outputs, ","), "--gif-size", s.GIFSize, "--shape", s.Shape, "--theme", s.ImageTheme}
	if s.NoAI {
		args = append(args, "--no-ai")
	}
	if opts.Provider != "" && opts.Provider != ai.Auto {
		args = append(args, "--provider", opts.Provider)
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	return args
}

// runShare creates the share images by running `shellrecap share` in the
// background. It's a separate process because rendering the images
// switches lipgloss to true color and the image theme, which mustn't
// happen under the running TUI.
var runShare = func(opts ai.Options, s config.Share) (share.Files, error) {
	exe, err := os.Executable()
	if err != nil {
		return share.Files{}, err
	}
	out, err := exec.Command(exe, shareArgs(opts, s)...).Output()
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

// The desktop actions and saving the settings, swapped out in tests
var (
	openURL      = share.OpenURL
	reveal       = share.Reveal
	copyText     = share.CopyText
	isDesktop    = share.Desktop
	saveSettings = config.Save
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
	opts, settings := m.aiOpts, m.settings.Share
	return func() tea.Msg {
		files, err := runShare(opts, settings)
		return shareDoneMsg{files: files, err: err}
	}
}

func (m *Model) shareDone(msg shareDoneMsg) tea.Cmd {
	m.share.busy = false
	if m.share.again {
		// These images are out of date, so make them with the new choices
		open := m.share.open
		cmd := m.startShare()
		m.share.open = open
		m.syncContent()
		return cmd
	}
	if msg.err != nil {
		m.logger.Printf("Error creating share images: %v", msg.err)
		m.share.failed = true
	} else {
		m.logger.Printf("Saved %d share images to %s", len(msg.files.Outputs), msg.files.Dir)
		m.share.files = msg.files
	}
	m.syncContent()
	return nil
}

// createAgain creates the images with the choices just made. A run that's
// still going finishes first, then its images are replaced.
func (m *Model) createAgain() tea.Cmd {
	m.share.customizing = false
	if m.share.busy {
		m.share.again = true
		m.syncContent()
		return nil
	}
	return m.startShare()
}

// handleShareKey handles keys while the share menu is open
func (m Model) handleShareKey(key string) (tea.Model, tea.Cmd) {
	if m.share.customizing {
		return m.handleCustomizeKey(key)
	}
	switch key {
	case "esc", "s":
		m.share.open = false
		m.syncContent()
		return m, nil
	case "c":
		if !m.share.empty {
			m.share.customizing, m.share.cursor = true, 0
			m.syncContent()
		}
		return m, nil
	}
	if m.share.busy || m.share.failed || m.share.empty || !isDesktop() || len(m.share.files.Outputs) == 0 {
		return m, nil
	}

	files := m.share.files
	// The GIF where the platform plays it, a picture everywhere else
	gif, hasGIF := files.First(config.RecapGIF, config.TourGIF)
	pic, hasPic := files.First(config.Poster, config.Tabs, config.Slides)
	paste := share.PasteShortcut()
	if key == "o" {
		first := files.Outputs[0]
		m.share.message = m.tryOpen("Opened the folder.", func() error { return reveal(first.Path) })
		m.syncContent()
		return m, nil
	}
	for _, link := range share.Links(files.Caption) {
		if link.Key != key {
			continue
		}
		file, what := pic, "the "+strings.ToLower(pic.Name)
		if (link.Animated && hasGIF) || !hasPic {
			file, what = gif, "the GIF"
		}
		var done string
		switch {
		case !link.Prefilled:
			// Copying the caption replaces the picture on the clipboard
			if copyText(files.Caption) == nil {
				m.share.files.Copied = false
				done = fmt.Sprintf("Opened %s and copied your caption. Paste it with %s, then drag in %s from the folder.",
					link.Name, paste, what)
			} else {
				done = fmt.Sprintf("Opened %s. Drag in %s from the folder.", link.Name, what)
			}
		case files.Copied && file.Kind != pic.Kind:
			done = fmt.Sprintf("Opened %s with your caption. Paste %s with %s, or drag in %s from the folder.",
				link.Name, "the "+strings.ToLower(pic.Name), paste, what)
		case files.Copied:
			done = fmt.Sprintf("Opened %s with your caption. Paste %s with %s.", link.Name, what, paste)
		default:
			done = fmt.Sprintf("Opened %s with your caption. Drag in %s from the folder.", link.Name, what)
		}
		m.share.message = m.tryOpen(done, func() error {
			if err := openURL(link.URL); err != nil {
				return err
			}
			return reveal(file.Path)
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

// shareChoices are the options in the customize panel, in order
var shareChoices = []struct {
	label, note string
	output      string   // set for toggles of an image kind
	values      []string // set for choices with several values
	labels      map[string]string
}{
	{label: "Recap GIF", note: "every Recap slide, animated", output: config.RecapGIF},
	{label: "Tour GIF", note: "every tab, then the Recap", output: config.TourGIF},
	{label: "Poster", note: "a one-image summary", output: config.Poster},
	{label: "Tab pictures", note: "Overview, Tech Profile, Work Patterns, Tool Usage", output: config.Tabs},
	{label: "Slide pictures", note: "each Recap slide, for carousels", output: config.Slides},
	{label: "AI slides", note: "include the AI-written slides"},
	{label: "GIF size", values: config.GIFSizes, labels: map[string]string{
		config.GIFStandard: "Standard", config.GIFSmall: "Small"}},
	{label: "Picture shape", values: config.Shapes, labels: map[string]string{
		config.Portrait: "Portrait", config.Story: "Story", config.Square: "Square", config.Landscape: "Landscape"}},
	{label: "Image theme", values: config.ImageThemes, labels: map[string]string{
		"dark": "Dark", "black": "Pitch black", "light": "Light"}},
}

// choiceNotes describe each value of the choices with several values
var choiceNotes = map[string]string{
	config.GIFStandard: "1080x1080 Recap, 1280x960 tour, fits X",
	config.GIFSmall:    "720x720 Recap, 960x720 tour, lighter",
	config.Portrait:    "1080x1350, for Instagram and LinkedIn",
	config.Story:       "1080x1920, for stories",
	config.Square:      "1080x1080",
	config.Landscape:   "1920x1080, for X and LinkedIn",
	"dark":             "neutral greys",
	"black":            "pure black, for OLED screens",
	"light":            "a white background",
}

func (m Model) choiceValue(i int) string {
	s := m.settings.Share
	switch shareChoices[i].label {
	case "GIF size":
		return s.GIFSize
	case "Picture shape":
		return s.Shape
	}
	return s.ImageTheme
}

// shareOptions describes the customize panel for rendering
func (m Model) shareOptions() []render.ShareOption {
	s := m.settings.Share
	var options []render.ShareOption
	for i, c := range shareChoices {
		switch {
		case c.output != "":
			options = append(options, render.ShareOption{Label: c.label, Note: c.note, Toggle: true, On: s.Has(c.output)})
		case c.values == nil:
			options = append(options, render.ShareOption{Label: c.label, Note: c.note, Toggle: true, On: !s.NoAI})
		default:
			v := m.choiceValue(i)
			options = append(options, render.ShareOption{Label: c.label, Value: c.labels[v], Note: choiceNotes[v]})
		}
	}
	return options
}

// change toggles the option at cursor, or moves its value by step
func (m *Model) change(cursor, step int) {
	s := &m.settings.Share
	c := shareChoices[cursor]
	switch {
	case c.output != "":
		if s.Has(c.output) {
			// Keep at least one image
			if len(s.Outputs) > 1 {
				var kept []string
				for _, o := range s.Outputs {
					if o != c.output {
						kept = append(kept, o)
					}
				}
				s.Outputs = kept
			}
		} else {
			// Keep the order the images are offered in
			var outputs []string
			for _, o := range config.Outputs {
				if o == c.output || s.Has(o) {
					outputs = append(outputs, o)
				}
			}
			s.Outputs = outputs
		}
	case c.values == nil:
		s.NoAI = !s.NoAI
	default:
		current := m.choiceValue(cursor)
		i := 0
		for j, v := range c.values {
			if v == current {
				i = j
			}
		}
		next := c.values[((i+step)%len(c.values)+len(c.values))%len(c.values)]
		switch c.label {
		case "GIF size":
			s.GIFSize = next
		case "Picture shape":
			s.Shape = next
		default:
			s.ImageTheme = next
		}
	}
	if err := saveSettings(m.settings); err != nil {
		m.logger.Printf("Error saving settings: %v", err)
	}
}

// handleCustomizeKey handles keys in the customize panel
func (m Model) handleCustomizeKey(key string) (tea.Model, tea.Cmd) {
	create := len(shareChoices)
	switch key {
	case "esc":
		m.share.customizing = false
	case "up", "k":
		m.share.cursor = (m.share.cursor + create) % (create + 1)
	case "down", "j", "tab":
		m.share.cursor = (m.share.cursor + 1) % (create + 1)
	case "left", "h":
		if m.share.cursor < create {
			m.change(m.share.cursor, -1)
		}
	case "right", "l", " ":
		if m.share.cursor < create {
			m.change(m.share.cursor, 1)
		}
	case "enter":
		if m.share.cursor < create && shareChoices[m.share.cursor].values == nil {
			m.change(m.share.cursor, 1)
			break
		}
		return m, m.createAgain()
	}
	m.syncContent()
	return m, nil
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
		Count:   len(files.Outputs),
		Desktop: isDesktop(),
		Message: m.share.message,
	}
	copied, _ := files.First(config.Poster, config.Tabs, config.Slides)
	for _, o := range files.Outputs {
		if o.Kind == config.Tabs || o.Kind == config.Slides {
			continue
		}
		note := share.Describe(o)
		if files.Copied && o.Path == copied.Path {
			note = strings.Replace(note, " · for", " · copied · for", 1)
		}
		menu.Files = append(menu.Files, render.ShareFile{Name: filepath.Base(o.Path), Note: note})
	}
	// Tab and slide pictures are summed up rather than listed
	if n := files.Count(config.Tabs); n > 0 {
		menu.Files = append(menu.Files, render.ShareFile{Name: fmt.Sprintf("+ %d tab pictures", n),
			Note: "Overview, Tech Profile, Work Patterns, Tool Usage"})
	}
	if n := files.Count(config.Slides); n > 0 {
		menu.Files = append(menu.Files, render.ShareFile{Name: fmt.Sprintf("+ %d slide pictures", n),
			Note: "each Recap slide, for carousels"})
	}
	for _, link := range share.Links(files.Caption) {
		menu.Links = append(menu.Links, render.ShareKey{Key: link.Key, Name: link.Name})
	}
	return menu
}

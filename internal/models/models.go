// internal/models/models.go
package models

import (
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/cache"
	"github.com/ksauraj/shellrecap/internal/gemini"
	"github.com/ksauraj/shellrecap/internal/render"
	"github.com/ksauraj/shellrecap/internal/theme"
	"github.com/ksauraj/shellrecap/internal/types"
)

const (
	// chromeHeight is the number of rows used by the header, tabs and footer
	chromeHeight  = 4
	slideInterval = 10 * time.Second
	animInterval  = 100 * time.Millisecond
	// revealFrames is how many animation ticks bars take to grow and
	// numbers take to count up when a view or slide appears
	revealFrames = 8
	// minSplash keeps the loading animation on screen long enough to enjoy
	minSplash = 4 * time.Second

	appVersion = "v1.1.0"
)

type aiStatus int

const (
	aiPending aiStatus = iota
	aiDone
	aiFailed
	aiDisabled
)

// aiWrappedMsg carries the result of the Gemini request
type aiWrappedMsg struct {
	id       int
	sections []gemini.Section
	summary  string
	commands int
	err      error
}

// slideTickMsg advances the Wrapped slides. The id lets manual navigation
// restart the countdown by invalidating ticks that are already scheduled.
type slideTickMsg struct{ id int }

// animTickMsg advances the ASCII animations by one frame
type animTickMsg struct{}

type Model struct {
	viewport            viewport.Model
	width               int
	height              int
	loading             bool
	refreshing          bool // the current load was started with the refresh key
	loadStart           int  // frame the current load started at
	shellData           analyzer.ShellData
	tabs                []string
	activeTab           int
	logger              *log.Logger
	wrappedYear         int
	wrappedStats        analyzer.WrappedStats
	aiSections          []gemini.Section
	aiStatus            aiStatus
	aiErr               error
	aiCreatedAt         time.Time
	aiFromCache         bool
	aiRequestID         int
	currentSectionIndex int
	autoplay            bool
	slideTickID         int
	frame               int
	revealStart         int // frame the current view or slide appeared at
}

func InitialModel() Model {
	logFile, err := os.OpenFile("shellrecap.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	logger := log.New(logFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

	tabs := []string{"Overview", "Tech Profile", "Work Patterns", "Tool Usage", "Recap"}

	// Like any year-in-review, January still looks back at the year before
	now := time.Now()
	wrappedYear := now.Year()
	if now.Month() == time.January {
		wrappedYear--
	}

	return Model{
		viewport:    viewport.New(80, 24-chromeHeight),
		width:       80,
		height:      24,
		loading:     true,
		tabs:        tabs,
		activeTab:   0,
		logger:      logger,
		wrappedYear: wrappedYear,
		autoplay:    true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadShellData(),
		tea.EnterAltScreen,
		// Every view has something moving (the splash, the glare on the
		// bars or the slide art), so the animation ticks run throughout
		animTick(),
	)
}

// loadShellData analyzes the shell history in the background
func loadShellData() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		data := analyzer.AnalyzeShells()
		if wait := minSplash - time.Since(start); wait > 0 {
			time.Sleep(wait)
		}
		return data
	}
}

func animTick() tea.Cmd {
	return tea.Tick(animInterval, func(time.Time) tea.Msg {
		return animTickMsg{}
	})
}

// revealProgress is how far the current view's intro animation has got,
// from 0 to 1
func (m Model) revealProgress() float64 {
	p := float64(m.frame-m.revealStart) / revealFrames
	if p >= 1 {
		return 1
	}
	if p <= 0 {
		return 0
	}
	// Ease out so the numbers slow down as they land
	return 1 - math.Pow(1-p, 3)
}

func (m Model) anim() render.Anim {
	return render.Anim{Progress: m.revealProgress(), Frame: m.frame}
}

// restartReveal replays the reveal animation for the view on screen
func (m *Model) restartReveal() {
	m.revealStart = m.frame
}

func (m Model) onWrappedTab() bool {
	return m.tabs[m.activeTab] == "Recap"
}

func (m *Model) switchTab(tab int) {
	m.activeTab = (tab + len(m.tabs)) % len(m.tabs)
	m.restartReveal()
	m.syncContent()
	m.viewport.GotoTop()
}

func (m *Model) changeSlide(delta int) tea.Cmd {
	total := len(m.slides())
	m.currentSectionIndex = ((m.currentSectionIndex+delta)%total + total) % total
	m.restartReveal()
	m.syncContent()
	return m.scheduleSlideTick()
}

// scheduleSlideTick restarts the autoplay countdown
func (m *Model) scheduleSlideTick() tea.Cmd {
	m.slideTickID++
	if !m.autoplay {
		return nil
	}
	id := m.slideTickID
	return tea.Tick(slideInterval, func(time.Time) tea.Msg {
		return slideTickMsg{id: id}
	})
}

// loadAISlides reuses cached AI slides while they are fresh and only calls
// Gemini when they aren't, or when force is set by the refresh key
func (m *Model) loadAISlides(force bool) tea.Cmd {
	summary := m.wrappedStats.Summary()
	commands := m.wrappedStats.TotalCommands
	entry, cached := cache.LoadWrapped(m.wrappedYear)

	m.aiSections, m.aiErr = nil, nil
	fresh := cached && entry.FreshFor(gemini.Model(), summary, commands, time.Now())
	// Without a key, stale slides are still better than none
	if cached && ((fresh && !force) || !gemini.Available()) {
		m.useCachedAISlides(entry)
		return nil
	}
	if !gemini.Available() {
		m.aiStatus = aiDisabled
		return nil
	}

	m.aiStatus = aiPending
	m.aiRequestID++
	id := m.aiRequestID
	return func() tea.Msg {
		resp, err := gemini.GenerateWrapped(summary)
		return aiWrappedMsg{id: id, sections: resp.Sections, summary: summary, commands: commands, err: err}
	}
}

func (m *Model) useCachedAISlides(entry cache.WrappedEntry) {
	m.aiStatus = aiDone
	m.aiSections = entry.Sections
	m.aiCreatedAt = entry.CreatedAt
	m.aiFromCache = true
}

func saveAISlides(entry cache.WrappedEntry, logger *log.Logger) tea.Cmd {
	return func() tea.Msg {
		if err := cache.SaveWrapped(entry); err != nil {
			logger.Printf("Error caching AI slides: %v", err)
		}
		return nil
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = maxInt(1, msg.Height-chromeHeight)
		m.syncContent()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		if m.loading {
			return m, nil
		}

		switch key := msg.String(); key {
		case "r":
			// Re-read the history and ask Gemini again, skipping the cache
			m.loading, m.refreshing, m.loadStart = true, true, m.frame
			m.aiRequestID++ // ignore any request that is still in flight
			return m, loadShellData()
		case "tab":
			m.switchTab(m.activeTab + 1)
			return m, nil
		case "shift+tab":
			m.switchTab(m.activeTab - 1)
			return m, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if tab := int(key[0] - '1'); tab < len(m.tabs) {
				m.switchTab(tab)
			}
			return m, nil
		case "right", "l", "n":
			if m.onWrappedTab() {
				return m, m.changeSlide(1)
			}
			m.switchTab(m.activeTab + 1)
			return m, nil
		case "left", "h", "p":
			if m.onWrappedTab() {
				return m, m.changeSlide(-1)
			}
			m.switchTab(m.activeTab - 1)
			return m, nil
		case "home", "g":
			m.viewport.GotoTop()
			return m, nil
		case "end", "G":
			m.viewport.GotoBottom()
			return m, nil
		case " ":
			if m.onWrappedTab() {
				m.autoplay = !m.autoplay
				m.syncContent()
				return m, m.scheduleSlideTick()
			}
		}

		// Everything else (arrows, j/k, pgup/pgdn, ...) scrolls the content
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case analyzer.ShellData:
		force := m.refreshing
		m.loading, m.refreshing = false, false
		m.shellData = msg
		m.wrappedStats = analyzer.ComputeWrapped(msg, m.wrappedYear)
		m.currentSectionIndex = 0

		m.restartReveal()
		cmds := []tea.Cmd{m.scheduleSlideTick(), m.loadAISlides(force)}
		m.syncContent()
		return m, tea.Batch(cmds...)

	case aiWrappedMsg:
		if msg.id != m.aiRequestID {
			// Superseded by a refresh
			return m, nil
		}
		if msg.err != nil {
			m.logger.Printf("Error generating wrapped response: %v", msg.err)
			m.aiErr = msg.err
			if entry, ok := cache.LoadWrapped(m.wrappedYear); ok {
				m.useCachedAISlides(entry)
			} else {
				m.aiStatus = aiFailed
			}
			m.syncContent()
			return m, nil
		}

		m.logger.Printf("Generated %d AI sections", len(msg.sections))
		m.aiStatus, m.aiSections, m.aiCreatedAt, m.aiFromCache = aiDone, msg.sections, time.Now(), false
		m.syncContent()
		return m, saveAISlides(cache.WrappedEntry{
			Year:      m.wrappedYear,
			Model:     gemini.Model(),
			Summary:   msg.summary,
			Commands:  msg.commands,
			CreatedAt: m.aiCreatedAt,
			Sections:  msg.sections,
		}, m.logger)

	case slideTickMsg:
		if msg.id != m.slideTickID || !m.autoplay {
			return m, nil
		}
		// Only advance while the slides are on screen
		if m.onWrappedTab() {
			return m, m.changeSlide(1)
		}
		return m, m.scheduleSlideTick()

	case animTickMsg:
		m.frame++
		m.syncContent()
		return m, animTick()

	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
}

// slides returns the locally computed Wrapped slides followed by the AI ones
func (m Model) slides() []types.Slide {
	slides := render.BuildWrappedSlides(m.wrappedStats, m.width, m.anim())

	last := &slides[len(slides)-1]
	switch m.aiStatus {
	case aiPending:
		last.Footer = render.Spinner(m.frame) + " Gemini is writing a few more slides for you..."
	case aiDisabled:
		last.Footer = "Set GEMINI_API_KEY to unlock AI-written slides."
	case aiFailed:
		last.Footer = "AI slides unavailable: " + m.aiErr.Error()
	}

	footer := "Generated by Gemini " + ago(m.aiCreatedAt) + "."
	switch {
	case m.aiFromCache && m.aiErr != nil:
		footer = fmt.Sprintf("Couldn't regenerate (%v), showing slides from %s.", m.aiErr, ago(m.aiCreatedAt))
	case m.aiFromCache && gemini.Available():
		footer = "Cached from " + ago(m.aiCreatedAt) + " · r: regenerate"
	case m.aiFromCache:
		footer = "Cached from " + ago(m.aiCreatedAt) + "."
	}

	for _, section := range m.aiSections {
		var quotes []string
		for _, q := range section.Quotes {
			quotes = append(quotes, cleanAIText(q))
		}
		slides = append(slides, types.Slide{
			Title:  cleanAIText(section.Title),
			Lines:  []string{cleanAIText(section.Description)},
			Quotes: quotes,
			Footer: footer,
			Art:    render.RobotArt(),
			AI:     true,
		})
	}
	return slides
}

// ago describes how long ago t was
func ago(t time.Time) string {
	plural := func(n int, unit string) string {
		if n == 1 {
			return fmt.Sprintf("1 %s ago", unit)
		}
		return fmt.Sprintf("%d %ss ago", n, unit)
	}
	switch d := time.Since(t); {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour")
	default:
		return plural(int(d.Hours()/24), "day")
	}
}

// cleanAIText strips markdown and emoji the model may add despite the prompt
func cleanAIText(text string) string {
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.Map(func(r rune) rune {
		if isEmoji(r) {
			return -1
		}
		return r
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

func isEmoji(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF, // pictographs, emoticons and flags
		r >= 0x2600 && r <= 0x27BF, // miscellaneous symbols and dingbats
		r >= 0x2300 && r <= 0x23FF, // watches, hourglasses and friends
		r >= 0x2B00 && r <= 0x2BFF, // stars and heavy arrows
		r >= 0xFE00 && r <= 0xFE0F, // variation selectors
		r == 0x200D, r == 0x20E3:   // zero width joiner and keycap
		return true
	}
	return false
}

// syncContent re-renders the active tab into the scrollable viewport
func (m *Model) syncContent() {
	if m.loading {
		return
	}

	anim := m.anim()
	var content string
	switch m.tabs[m.activeTab] {
	case "Overview":
		content = render.RenderOverview(m.shellData, m.width, anim)
	case "Tech Profile":
		content = render.RenderTechProfile(m.shellData.Insights.TechnicalProfile, m.width, anim)
	case "Work Patterns":
		content = render.RenderWorkPatterns(m.shellData.Insights.WorkPatterns, m.width, anim)
	case "Tool Usage":
		content = render.RenderToolUsage(m.shellData.Insights.ToolUsage, m.width, anim)
	case "Recap":
		slides := m.slides()
		if m.currentSectionIndex >= len(slides) {
			m.currentSectionIndex = 0
		}
		label := fmt.Sprintf("SHELLRECAP %d", m.wrappedYear)
		if m.wrappedStats.AllTime {
			label = "SHELLRECAP"
		}
		content = render.RenderSlide(slides[m.currentSectionIndex], label,
			m.currentSectionIndex, len(slides), m.autoplay, m.frame-m.revealStart, m.viewport.Width)
	}
	m.viewport.SetContent(content)
}

func (m Model) View() string {
	if m.loading {
		return render.RenderLoading(m.frame-m.loadStart, m.refreshing, m.width, m.height)
	}

	// Header with title and version
	header := lipgloss.NewStyle().Padding(0, 1).Render(
		lipgloss.NewStyle().Bold(true).Foreground(theme.Brand.Tone.Color(0.3)).Render(">_") + " " +
			lipgloss.NewStyle().Bold(true).Foreground(theme.Brand.Color).Render("shellrecap") + " " +
			theme.Faint.Render(appVersion))

	// Render tabs
	tabBar := render.RenderTabs(m.tabs, m.activeTab, m.width)

	// Footer with controls for the current view
	help := "Tab/←→: views • ↑↓/PgUp/PgDn: scroll • r: refresh • q: quit"
	if m.onWrappedTab() {
		help = "←→: slides • Space: pause • Tab: views • r: refresh • q: quit"
	}
	if m.viewport.TotalLineCount() > m.viewport.Height {
		percent := m.viewport.ScrollPercent()
		// The viewport divides by zero when the content is exactly one
		// line taller than the screen
		if math.IsNaN(percent) {
			percent = 0
			if m.viewport.AtBottom() {
				percent = 1
			}
		}
		help = fmt.Sprintf("%3.0f%% • %s", percent*100, help)
	}
	footer := lipgloss.NewStyle().
		Foreground(theme.Overlay).
		Padding(0, 1).
		MaxWidth(m.width).
		Render(help + " • By Ksauraj")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabBar,
		"",
		m.viewport.View(),
		footer,
	)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

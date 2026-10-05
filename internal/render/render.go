// internal/render/render.go
package render

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/theme"
	"github.com/ksauraj/shellrecap/internal/types"
	"github.com/muesli/reflow/truncate"
)

// maxPanelWidth keeps the panels readable on very wide terminals
const maxPanelWidth = 100

// valueStyle highlights the key numbers and facts in a panel
var valueStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.Peach)

// RenderTabs renders the tab bar
func RenderTabs(tabs []string, active int, width int) string {
	var tabsDisplay strings.Builder

	for i, tab := range tabs {
		style := lipgloss.NewStyle().
			Padding(0, 1)

		if i == active {
			style = style.
				Bold(true).
				Background(theme.Mauve).
				Foreground(theme.Base)
			tabsDisplay.WriteString(style.Render(fmt.Sprintf("%d %s", i+1, tab)))
			continue
		}

		tabsDisplay.WriteString(style.Render(theme.Faint.Render(strconv.Itoa(i+1)) + " " + theme.Dim.Render(tab)))
	}

	return fit(tabsDisplay.String(), width)
}

func panelWidth(width int) int {
	if width > maxPanelWidth {
		return maxPanelWidth
	}
	if width < 30 {
		return 30
	}
	return width
}

// innerWidth is the usable text width inside a panel
func innerWidth(width int) int {
	return panelWidth(width) - 2 - 4 // border and padding
}

// panel draws content in a rounded box that fits the terminal width. Lines
// that are too long are truncated rather than wrapped so that every list
// item stays on one row.
func panel(content string, width int) string {
	inner := innerWidth(width)
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for i, line := range lines {
		lines[i] = fit(line, inner)
	}
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.Overlay).
		Padding(1, 2).
		Width(panelWidth(width) - 2).
		Render(strings.Join(lines, "\n"))
}

// fit truncates s to width terminal cells, keeping ANSI colors intact
func fit(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	return truncate.StringWithTail(s, uint(width), "…")
}

// oneLine flattens multi-line commands for single-row display
func oneLine(s string) string {
	return strings.ReplaceAll(s, "\n", " ; ")
}

// title renders a panel title, underlined ASCII style
func title(text string, accent lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Bold(true).Foreground(accent).Render(text) + "\n" +
		theme.Track.Render(strings.Repeat("=", len(text))) + "\n\n"
}

// heading renders a section heading inside a panel
func heading(text string) string {
	return theme.Bold.Render(text) + "\n"
}

// field renders a "label: value" line with the labels padded to line up
func field(label, value string) string {
	return theme.Dim.Render(fmt.Sprintf("%-17s", label+":")) + " " + value + "\n"
}

// empty renders a placeholder for a section without data
func empty(text string) string {
	return "  " + theme.Faint.Render(text) + "\n"
}

// scale returns v scaled by an animation's progress, from 0 to 1
func scale(v int, progress float64) int {
	return int(math.Round(float64(v) * progress))
}

// formatInt formats n with thousands separators
func formatInt(n int) string {
	s := strconv.Itoa(n)
	start := 0
	if n < 0 {
		start = 1
	}
	for i := len(s) - 3; i > start; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// bar draws a horizontal bar whose filled part follows the sunset gradient
func bar(value, max, width int) string {
	if max <= 0 || width <= 0 {
		return ""
	}
	filled := int(math.Round(float64(value) / float64(max) * float64(width)))
	if value > 0 && filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}

	var b strings.Builder
	for i := 0; i < filled; i++ {
		b.WriteString(theme.Fg(theme.Gradient(float64(i) / float64(maxInt(width-1, 1)))).Render("█"))
	}
	b.WriteString(theme.Track.Render(strings.Repeat("░", width-filled)))
	return b.String()
}

// countRows renders aligned "name ███░░ count" rows, scaled to the largest
// count. limit <= 0 shows every row. Bars and counts grow with progress.
func countRows(counts []analyzer.UsageCount, limit int, unit string, width int, progress float64) string {
	if len(counts) == 0 {
		return ""
	}
	if limit > 0 && len(counts) > limit {
		counts = counts[:limit]
	}

	nameWidth, countWidth, max := 0, 0, 0
	for _, c := range counts {
		nameWidth = maxInt(nameWidth, lipgloss.Width(c.Name))
		countWidth = maxInt(countWidth, len(formatInt(c.Count)))
		max = maxInt(max, c.Count)
	}
	if nameWidth > 24 {
		nameWidth = 24
	}
	barWidth := width - 2 - nameWidth - 1 - 1 - countWidth - len(unit)
	if barWidth > 30 {
		barWidth = 30
	}

	var b strings.Builder
	for _, c := range counts {
		name := fit(c.Name, nameWidth)
		fmt.Fprintf(&b, "  %s%s %s %s%s\n",
			theme.Normal.Render(name), strings.Repeat(" ", nameWidth-lipgloss.Width(name)),
			bar(scale(c.Count, progress), max, barWidth),
			theme.Normal.Render(fmt.Sprintf("%*s", countWidth, formatInt(scale(c.Count, progress)))),
			theme.Faint.Render(unit))
	}
	return b.String()
}

var chartBlocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// columnChart draws counts as vertical bars, height rows tall, colored
// along the sunset gradient from left to right. The bars grow with progress.
func columnChart(counts []int, colWidth, gap, height int, progress float64) []string {
	max := 0
	for _, c := range counts {
		max = maxInt(max, c)
	}

	rows := make([]string, height)
	for r := 0; r < height; r++ {
		var b strings.Builder
		rowFromBottom := height - 1 - r
		for i, c := range counts {
			// Eighths of a row filled, counting from the bottom
			level := 0
			if max > 0 {
				level = int(math.Round(float64(c) / float64(max) * float64(height*8) * progress))
			}
			if c > 0 && level == 0 && progress > 0 {
				level = 1
			}
			fill := level - rowFromBottom*8
			if fill < 0 {
				fill = 0
			}
			if fill > 8 {
				fill = 8
			}
			column := theme.Fg(theme.Gradient(float64(i) / float64(maxInt(len(counts)-1, 1))))
			b.WriteString(column.Render(strings.Repeat(chartBlocks[fill], colWidth)))
			if i < len(counts)-1 {
				b.WriteString(strings.Repeat(" ", gap))
			}
		}
		rows[r] = b.String()
	}
	return rows
}

// hourChart draws activity per hour of day with an hour axis underneath
func hourChart(counts [24]int, progress float64) []string {
	rows := columnChart(counts[:], 2, 0, 3, progress)

	axis := []byte(strings.Repeat(" ", 48))
	for _, h := range []int{0, 6, 12, 18} {
		copy(axis[h*2:], fmt.Sprintf("%02d", h))
	}
	copy(axis[46:], "23")
	return append(rows, theme.Faint.Render(string(axis)))
}

// monthChart draws activity per month with month names underneath
func monthChart(counts [12]int, progress float64) []string {
	rows := columnChart(counts[:], 3, 1, 3, progress)

	var labels []string
	for m := time.January; m <= time.December; m++ {
		labels = append(labels, m.String()[:3])
	}
	return append(rows, theme.Faint.Render(strings.Join(labels, " ")))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func RenderOverview(data analyzer.ShellData, width int, progress float64) string {
	inner := innerWidth(width)

	var content strings.Builder
	content.WriteString(title("Shell Usage Overview", theme.Peach))

	shells := analyzer.SortedShells(data)
	if len(shells) == 0 {
		content.WriteString(theme.Faint.Render("No shell history found (looked for bash, zsh and fish history files)") + "\n")
		return panel(content.String(), width)
	}

	all := analyzer.AllEntries(data)
	unique := make(map[string]bool)
	var first, last time.Time
	for _, e := range all {
		unique[e.Command] = true
		if e.Timestamp.IsZero() {
			continue
		}
		if first.IsZero() || e.Timestamp.Before(first) {
			first = e.Timestamp
		}
		if e.Timestamp.After(last) {
			last = e.Timestamp
		}
	}

	content.WriteString(field("Total commands", valueStyle.Render(formatInt(scale(len(all), progress)))))
	content.WriteString(field("Unique commands", valueStyle.Render(formatInt(scale(len(unique), progress)))))
	if !first.IsZero() {
		content.WriteString(field("History span", theme.Normal.Render(first.Format("Jan 2, 2006"))+
			theme.Faint.Render(" → ")+theme.Normal.Render(last.Format("Jan 2, 2006"))))
	}
	content.WriteString("\n")

	content.WriteString(heading("Shells"))
	var shellCounts []analyzer.UsageCount
	for _, shell := range shells {
		shellCounts = append(shellCounts, analyzer.UsageCount{Name: shell, Count: len(data.Histories[shell])})
	}
	content.WriteString(countRows(shellCounts, 0, " commands", inner, progress))
	content.WriteString("\n")

	content.WriteString(heading("Top Commands"))
	content.WriteString(countRows(analyzer.SortedCounts(data.CommonCmds), 10, "", inner, progress))

	// Add shell configuration information
	for _, shell := range shells {
		config, exists := data.ShellConfigs[shell]
		if !exists {
			continue
		}
		content.WriteString("\n" + heading(fmt.Sprintf("%s configuration", shell)))
		content.WriteString("  " + theme.Dim.Render(fmt.Sprintf("%d aliases • %d plugins • %d environment variables",
			len(config.Aliases), len(config.Plugins), len(config.Environment))) + "\n")

		// List up to 5 plugins
		if len(config.Plugins) > 0 {
			var names []string
			for i, plugin := range config.Plugins {
				if i >= 5 {
					break
				}
				names = append(names, theme.Fg(theme.Flamingo).Render(plugin.Name))
			}
			line := "  " + theme.Dim.Render("Plugins: ") + strings.Join(names, theme.Faint.Render(", "))
			if len(config.Plugins) > 5 {
				line += theme.Faint.Render(fmt.Sprintf(" and %d more", len(config.Plugins)-5))
			}
			content.WriteString(line + "\n")
		}

		// List up to 5 aliases, sorted so they don't reshuffle on redraw
		if len(config.Aliases) > 0 {
			names := make([]string, 0, len(config.Aliases))
			for alias := range config.Aliases {
				names = append(names, alias)
			}
			sort.Strings(names)
			for i, alias := range names {
				if i >= 5 {
					content.WriteString(theme.Faint.Render(fmt.Sprintf("  And %d more aliases...", len(names)-5)) + "\n")
					break
				}
				content.WriteString(fmt.Sprintf("  %s %s %s\n", theme.Fg(theme.Peach).Render(alias),
					theme.Faint.Render("→"), theme.Dim.Render(oneLine(config.Aliases[alias]))))
			}
		}
	}

	return panel(content.String(), width)
}

// RenderTechProfile renders the tech profile tab
func RenderTechProfile(profile analyzer.TechProfile, width int, progress float64) string {
	inner := innerWidth(width)

	var content strings.Builder
	content.WriteString(title("Technical Profile", theme.Mauve))

	// Primary Role
	if profile.PrimaryRole != "" {
		content.WriteString(field("Primary Role", lipgloss.NewStyle().Bold(true).Foreground(theme.Mauve).Render(profile.PrimaryRole)))
	} else {
		content.WriteString(field("Primary Role", theme.Faint.Render("Not enough data")))
	}

	// Tech Stack
	if len(profile.TechStack) > 0 {
		content.WriteString(field("Languages", theme.Normal.Render(strings.Join(profile.TechStack, " · "))))
	} else {
		content.WriteString(field("Languages", theme.Faint.Render("No language usage found")))
	}
	content.WriteString("\n")

	// Skill areas
	content.WriteString(heading("Skill Areas"))
	if len(profile.SecondarySkills) > 0 {
		content.WriteString(countRows(profile.SecondarySkills, 0, " cmds", inner, progress))
	} else {
		content.WriteString(empty("No skill data available"))
	}
	content.WriteString("\n")

	// Most used tools
	content.WriteString(heading("Most Used Tech"))
	if len(profile.TopTech) > 0 {
		content.WriteString(countRows(profile.TopTech, 0, " uses", inner, progress))
	} else {
		content.WriteString(empty("No tool usage data available"))
	}

	return panel(content.String(), width)
}

// RenderWorkPatterns renders the work patterns tab
func RenderWorkPatterns(patterns analyzer.WorkPatterns, width int, progress float64) string {
	inner := innerWidth(width)

	var content strings.Builder
	content.WriteString(title("Work Patterns", theme.Yellow))

	// Daily Activity
	content.WriteString(heading("Daily Activity"))
	if len(patterns.PeakHours) > 0 {
		var peaks []string
		for _, hour := range patterns.PeakHours {
			peaks = append(peaks, valueStyle.Render(fmt.Sprintf("%02d:00", hour)))
		}
		content.WriteString("  " + theme.Dim.Render("Peak hours: ") + strings.Join(peaks, theme.Faint.Render(", ")) + "\n\n")
		for _, row := range hourChart(patterns.HourlyActivity, progress) {
			content.WriteString("  " + row + "\n")
		}
	} else {
		content.WriteString(empty("Your history has no timestamps, so activity by hour isn't available"))
	}
	content.WriteString("\n")

	// Productivity Metrics
	content.WriteString(heading("Productivity Metrics"))
	metrics := make([]string, 0, len(patterns.Productivity))
	for metric := range patterns.Productivity {
		metrics = append(metrics, metric)
	}
	sort.Strings(metrics)
	barWidth := minInt(20, inner-2-20-1-7)
	for _, metric := range metrics {
		value := patterns.Productivity[metric] * progress
		content.WriteString(fmt.Sprintf("  %s %s %s\n",
			theme.Normal.Render(fmt.Sprintf("%-20s", metric)),
			bar(int(value*1000), 1000, barWidth),
			theme.Normal.Render(fmt.Sprintf("%5.1f%%", value*100))))
	}
	content.WriteString("\n")

	// Common Workflows
	content.WriteString(heading("Common Workflows"))
	if len(patterns.CommonWorkflows) > 0 {
		for _, workflow := range patterns.CommonWorkflows {
			content.WriteString("  " + theme.Fg(theme.Yellow).Render("•") + " " + theme.Normal.Render(workflow) + "\n")
		}
	} else {
		content.WriteString(empty("No recurring workflows found"))
	}

	return panel(content.String(), width)
}

func RenderToolUsage(usage analyzer.ToolUsage, width int, progress float64) string {
	inner := innerWidth(width)

	var content strings.Builder
	content.WriteString(title("Tool Usage Statistics", theme.Red))

	sections := []struct {
		title string
		usage map[string]int
		empty string
	}{
		{"Editors", usage.Editors, "No editor usage data available"},
		{"Programming Languages", usage.Languages, "No language usage data available"},
		{"Build Tools", usage.BuildTools, "No build tool usage data available"},
		{"DevOps & Cloud", usage.DevOps, "No DevOps tool usage data available"},
	}
	for i, section := range sections {
		if i > 0 {
			content.WriteString("\n")
		}
		content.WriteString(heading(section.title))
		if len(section.usage) > 0 {
			content.WriteString(countRows(analyzer.SortedCounts(section.usage), 8, " uses", inner, progress))
		} else {
			content.WriteString(empty(section.empty))
		}
	}

	return panel(content.String(), width)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RenderTimeline renders the timeline, revealing entries one by one as
// progress goes from 0 to 1
func RenderTimeline(entries []types.TimelineEntry, width int, progress float64) string {
	var content strings.Builder
	content.WriteString(title("Interesting Commands Timeline", theme.Green))

	if len(entries) == 0 {
		content.WriteString(theme.Faint.Render("No interesting commands found") + "\n")
	}
	shown := int(math.Ceil(float64(len(entries)) * progress))
	for i, entry := range entries {
		if i >= shown {
			content.WriteString("\n")
			continue
		}
		when := "unknown date       "
		if !entry.Timestamp.IsZero() {
			when = entry.Timestamp.Format("2006-01-02 15:04:05")
		}
		content.WriteString(fmt.Sprintf("%s  %s  %s\n",
			theme.Faint.Render(when),
			theme.Fg(theme.Mauve).Render(fmt.Sprintf("%-4s", entry.Shell)),
			theme.Normal.Render(oneLine(entry.Command))))
	}

	return panel(content.String(), width)
}

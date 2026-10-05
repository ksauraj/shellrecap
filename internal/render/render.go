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
	"github.com/gookit/color"
	"github.com/ksauraj/k8au-shell-analyzer/internal/analyzer"
	"github.com/ksauraj/k8au-shell-analyzer/internal/types"
	"github.com/muesli/reflow/truncate"
)

// maxPanelWidth keeps the panels readable on very wide terminals
const maxPanelWidth = 100

// RenderTabs renders the tab bar
func RenderTabs(tabs []string, active int, width int) string {
	var tabsDisplay strings.Builder

	for i, tab := range tabs {
		style := lipgloss.NewStyle().
			Padding(0, 1)

		if i == active {
			style = style.
				Bold(true).
				Background(lipgloss.Color("4")).
				Foreground(lipgloss.Color("15"))
		}

		tabsDisplay.WriteString(style.Render(fmt.Sprintf("%d %s", i+1, tab)))
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
func title(text string, c color.Color) string {
	return color.New(c, color.OpBold).Sprint(text) + "\n" +
		color.Gray.Sprint(strings.Repeat("=", len(text))) + "\n\n"
}

// heading renders a section heading inside a panel
func heading(text string) string {
	return color.Bold.Sprint(text) + "\n"
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
	return color.Cyan.Sprint(strings.Repeat("█", filled)) + color.Gray.Sprint(strings.Repeat("░", width-filled))
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
		fmt.Fprintf(&b, "  %s%s %s %*s%s\n",
			name, strings.Repeat(" ", nameWidth-lipgloss.Width(name)),
			bar(scale(c.Count, progress), max, barWidth),
			countWidth, formatInt(scale(c.Count, progress)), unit)
	}
	return b.String()
}

var chartBlocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// columnChart draws counts as vertical bars, height rows tall. The bars
// grow with progress.
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
			b.WriteString(strings.Repeat(chartBlocks[fill], colWidth))
			if i < len(counts)-1 {
				b.WriteString(strings.Repeat(" ", gap))
			}
		}
		rows[r] = color.Cyan.Sprint(b.String())
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
	return append(rows, color.Gray.Sprint(string(axis)))
}

// monthChart draws activity per month with month names underneath
func monthChart(counts [12]int, progress float64) []string {
	rows := columnChart(counts[:], 3, 1, 3, progress)

	var labels []string
	for m := time.January; m <= time.December; m++ {
		labels = append(labels, m.String()[:3])
	}
	return append(rows, color.Gray.Sprint(strings.Join(labels, " ")))
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
	content.WriteString(title("Shell Usage Overview", color.FgGreen))

	shells := analyzer.SortedShells(data)
	if len(shells) == 0 {
		content.WriteString("No shell history found (looked for bash, zsh and fish history files)\n")
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

	content.WriteString(fmt.Sprintf("Total commands:   %s\n", color.Cyan.Sprint(formatInt(scale(len(all), progress)))))
	content.WriteString(fmt.Sprintf("Unique commands:  %s\n", color.Cyan.Sprint(formatInt(scale(len(unique), progress)))))
	if !first.IsZero() {
		content.WriteString(fmt.Sprintf("History span:     %s → %s\n",
			first.Format("Jan 2, 2006"), last.Format("Jan 2, 2006")))
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
		content.WriteString(fmt.Sprintf("  %d aliases • %d plugins • %d environment variables\n",
			len(config.Aliases), len(config.Plugins), len(config.Environment)))

		// List up to 5 plugins
		if len(config.Plugins) > 0 {
			var names []string
			for i, plugin := range config.Plugins {
				if i >= 5 {
					break
				}
				names = append(names, color.Yellow.Sprint(plugin.Name))
			}
			line := "  Plugins: " + strings.Join(names, ", ")
			if len(config.Plugins) > 5 {
				line += fmt.Sprintf(" and %d more", len(config.Plugins)-5)
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
					content.WriteString(fmt.Sprintf("  And %d more aliases...\n", len(names)-5))
					break
				}
				content.WriteString(fmt.Sprintf("  %s → %s\n", color.Yellow.Sprint(alias), oneLine(config.Aliases[alias])))
			}
		}
	}

	return panel(content.String(), width)
}

// RenderTechProfile renders the tech profile tab
func RenderTechProfile(profile analyzer.TechProfile, width int, progress float64) string {
	inner := innerWidth(width)

	var content strings.Builder
	content.WriteString(title("Technical Profile", color.FgGreen))

	// Primary Role
	if profile.PrimaryRole != "" {
		content.WriteString(fmt.Sprintf("Primary Role: %s\n",
			color.Cyan.Sprint(profile.PrimaryRole)))
	} else {
		content.WriteString("Primary Role: Not enough data\n")
	}

	// Tech Stack
	if len(profile.TechStack) > 0 {
		content.WriteString(fmt.Sprintf("Languages:    %s\n\n", strings.Join(profile.TechStack, " · ")))
	} else {
		content.WriteString("Languages:    No language usage found\n\n")
	}

	// Skill areas
	content.WriteString(heading("Skill Areas"))
	if len(profile.SecondarySkills) > 0 {
		content.WriteString(countRows(profile.SecondarySkills, 0, " cmds", inner, progress))
	} else {
		content.WriteString("  No skill data available\n")
	}
	content.WriteString("\n")

	// Most used tools
	content.WriteString(heading("Most Used Tech"))
	if len(profile.TopTech) > 0 {
		content.WriteString(countRows(profile.TopTech, 0, " uses", inner, progress))
	} else {
		content.WriteString("  No tool usage data available\n")
	}

	return panel(content.String(), width)
}

// RenderWorkPatterns renders the work patterns tab
func RenderWorkPatterns(patterns analyzer.WorkPatterns, width int, progress float64) string {
	inner := innerWidth(width)

	var content strings.Builder
	content.WriteString(title("Work Patterns", color.FgYellow))

	// Daily Activity
	content.WriteString(heading("Daily Activity"))
	if len(patterns.PeakHours) > 0 {
		var peaks []string
		for _, hour := range patterns.PeakHours {
			peaks = append(peaks, fmt.Sprintf("%02d:00", hour))
		}
		content.WriteString(fmt.Sprintf("  Peak hours: %s\n\n", strings.Join(peaks, ", ")))
		for _, row := range hourChart(patterns.HourlyActivity, progress) {
			content.WriteString("  " + row + "\n")
		}
	} else {
		content.WriteString("  Your history has no timestamps, so activity by hour isn't available\n")
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
		content.WriteString(fmt.Sprintf("  %-20s %s %5.1f%%\n", metric, bar(int(value*1000), 1000, barWidth), value*100))
	}
	content.WriteString("\n")

	// Common Workflows
	content.WriteString(heading("Common Workflows"))
	if len(patterns.CommonWorkflows) > 0 {
		for _, workflow := range patterns.CommonWorkflows {
			content.WriteString(fmt.Sprintf("  • %s\n", workflow))
		}
	} else {
		content.WriteString("  No recurring workflows found\n")
	}

	return panel(content.String(), width)
}

func RenderToolUsage(usage analyzer.ToolUsage, width int, progress float64) string {
	inner := innerWidth(width)

	var content strings.Builder
	content.WriteString(title("Tool Usage Statistics", color.FgMagenta))

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
			content.WriteString("  " + section.empty + "\n")
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
	content.WriteString(title("Interesting Commands Timeline", color.FgGreen))

	if len(entries) == 0 {
		content.WriteString("No interesting commands found\n")
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
			when,
			color.Yellow.Sprintf("%-4s", entry.Shell),
			color.Cyan.Sprint(oneLine(entry.Command))))
	}

	return panel(content.String(), width)
}

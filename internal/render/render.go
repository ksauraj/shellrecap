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

// RenderTabs renders the tab bar
func RenderTabs(tabs []string, active int, width int) string {
	var tabsDisplay strings.Builder

	for i, tab := range tabs {
		if i == active {
			tabsDisplay.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.Base).
				Background(theme.Brand.Color).Render(fmt.Sprintf(" %d %s ", i+1, tab)))
			continue
		}
		tabsDisplay.WriteString(" " + theme.Faint.Render(strconv.Itoa(i+1)) + " " + theme.Dim.Render(tab) + " ")
	}

	return fit(tabsDisplay.String(), width)
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

// stat renders a "label   value" line with the labels padded to line up
func stat(label, value string) string {
	return theme.Dim.Render(fmt.Sprintf("%-16s", label)) + value
}

// faint renders a placeholder for a card without data
func faint(text string) []string {
	return []string{theme.Faint.Render(text)}
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

func highlight(accent theme.Accent, text string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(accent.Color).Render(text)
}

func RenderOverview(data analyzer.ShellData, width int, anim Anim) string {
	shells := analyzer.SortedShells(data)
	if len(shells) == 0 {
		return grid([]cardSpec{{title: "Overview", accent: theme.MauveAccent, wide: true, body: func(int) []string {
			return faint("No shell history found (looked for bash, zsh and fish history files)")
		}}}, width)
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

	var shellCounts []analyzer.UsageCount
	for _, shell := range shells {
		shellCounts = append(shellCounts, analyzer.UsageCount{Name: shell, Count: len(data.Histories[shell])})
	}
	top := analyzer.TopN(analyzer.SortedCounts(data.CommonCmds), 10)
	layout := newBarLayout(cardInner(halfCardWidth(width)), shellCounts, top)

	return grid([]cardSpec{
		{title: "Summary", accent: theme.MauveAccent, body: func(int) []string {
			lines := []string{
				stat("Total commands", highlight(theme.MauveAccent, formatInt(scale(len(all), anim.Progress)))),
				stat("Unique commands", highlight(theme.MauveAccent, formatInt(scale(len(unique), anim.Progress)))),
				stat("Shells", theme.Normal.Render(strings.Join(shells, ", "))),
			}
			if !first.IsZero() {
				lines = append(lines,
					stat("First command", theme.Normal.Render(first.Format("Jan 2, 2006"))),
					stat("Latest command", theme.Normal.Render(last.Format("Jan 2, 2006"))))
			}
			return lines
		}},
		{title: "Shells", accent: theme.RoseAccent, body: func(int) []string {
			return countRows(shellCounts, layout, theme.RoseAccent.Tone, anim)
		}},
		{title: "Top Commands", accent: theme.SandAccent, body: func(int) []string {
			if len(top) == 0 {
				return faint("No commands found")
			}
			return countRows(top, layout, theme.SandAccent.Tone, anim)
		}},
		{title: "Shell Configuration", accent: theme.SageAccent, body: func(int) []string {
			return configLines(data, shells)
		}},
	}, width)
}

func configLines(data analyzer.ShellData, shells []string) []string {
	var lines []string
	for _, shell := range shells {
		config, exists := data.ShellConfigs[shell]
		if !exists {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, highlight(theme.SageAccent, shell)+"  "+theme.Dim.Render(fmt.Sprintf("%d aliases · %d plugins · %d env vars",
			len(config.Aliases), len(config.Plugins), len(config.Environment))))

		if len(config.Plugins) > 0 {
			var names []string
			for i, plugin := range config.Plugins {
				if i >= 3 {
					break
				}
				names = append(names, plugin.Name)
			}
			line := theme.Faint.Render("plugins ") + theme.Normal.Render(strings.Join(names, ", "))
			if len(config.Plugins) > 3 {
				line += theme.Faint.Render(fmt.Sprintf(" +%d", len(config.Plugins)-3))
			}
			lines = append(lines, line)
		}

		// Sorted so the aliases don't reshuffle on redraw
		names := make([]string, 0, len(config.Aliases))
		for alias := range config.Aliases {
			names = append(names, alias)
		}
		sort.Strings(names)
		for i, alias := range names {
			if i >= 3 {
				lines = append(lines, theme.Faint.Render(fmt.Sprintf("and %d more aliases", len(names)-3)))
				break
			}
			lines = append(lines, lipgloss.NewStyle().Foreground(theme.Sage).Render(alias)+
				theme.Faint.Render(" → ")+theme.Dim.Render(oneLine(config.Aliases[alias])))
		}
	}
	if len(lines) == 0 {
		return faint("No shell configuration found")
	}
	return lines
}

// RenderTechProfile renders the tech profile tab
func RenderTechProfile(profile analyzer.TechProfile, width int, anim Anim) string {
	layout := newBarLayout(cardInner(halfCardWidth(width)), profile.SecondarySkills, profile.TopTech)

	return grid([]cardSpec{
		{title: "Profile", accent: theme.MauveAccent, wide: true, body: func(int) []string {
			role := theme.Faint.Render("Not enough data")
			if profile.PrimaryRole != "" {
				role = highlight(theme.MauveAccent, profile.PrimaryRole)
			}
			languages := theme.Faint.Render("No language usage found")
			if len(profile.TechStack) > 0 {
				languages = theme.Normal.Render(strings.Join(profile.TechStack, " · "))
			}
			return []string{stat("Primary role", role), stat("Languages", languages)}
		}},
		{title: "Skill Areas", accent: theme.RoseAccent, body: func(int) []string {
			if len(profile.SecondarySkills) == 0 {
				return faint("No skill data available")
			}
			return countRows(profile.SecondarySkills, layout, theme.RoseAccent.Tone, anim)
		}},
		{title: "Most Used Tech", accent: theme.SandAccent, body: func(int) []string {
			if len(profile.TopTech) == 0 {
				return faint("No tool usage data available")
			}
			return countRows(profile.TopTech, layout, theme.SandAccent.Tone, anim)
		}},
	}, width)
}

// RenderWorkPatterns renders the work patterns tab
func RenderWorkPatterns(patterns analyzer.WorkPatterns, width int, anim Anim) string {
	return grid([]cardSpec{
		{title: "Daily Activity", accent: theme.MauveAccent, wide: true, body: func(int) []string {
			if len(patterns.PeakHours) == 0 {
				return faint("Your history has no timestamps, so activity by hour isn't available")
			}
			var peaks []string
			for _, hour := range patterns.PeakHours {
				peaks = append(peaks, highlight(theme.MauveAccent, fmt.Sprintf("%02d:00", hour)))
			}
			lines := []string{stat("Peak hours", strings.Join(peaks, theme.Faint.Render(", "))), ""}
			return append(lines, hourChart(patterns.HourlyActivity, theme.MauveAccent.Tone, anim)...)
		}},
		{title: "Productivity", accent: theme.RoseAccent, body: func(inner int) []string {
			metrics := make([]string, 0, len(patterns.Productivity))
			for metric := range patterns.Productivity {
				metrics = append(metrics, metric)
			}
			sort.Strings(metrics)
			barWidth := maxInt(minInt(inner-20-7, 40), 4)
			var lines []string
			for _, metric := range metrics {
				value := patterns.Productivity[metric] * anim.Progress
				lines = append(lines, theme.Normal.Render(fmt.Sprintf("%-19s", metric))+" "+
					bar(int(value*1000), 1000, barWidth, theme.RoseAccent.Tone, anim.Frame)+" "+
					theme.Dim.Render(fmt.Sprintf("%5.1f%%", value*100)))
			}
			return lines
		}},
		{title: "Common Workflows", accent: theme.SandAccent, body: func(int) []string {
			if len(patterns.CommonWorkflows) == 0 {
				return faint("No recurring workflows found")
			}
			var lines []string
			for _, workflow := range patterns.CommonWorkflows {
				lines = append(lines, lipgloss.NewStyle().Foreground(theme.Sand).Render("•")+" "+theme.Normal.Render(workflow))
			}
			return lines
		}},
	}, width)
}

func RenderToolUsage(usage analyzer.ToolUsage, width int, anim Anim) string {
	sections := []struct {
		title  string
		accent theme.Accent
		counts []analyzer.UsageCount
		empty  string
	}{
		{"Editors", theme.MauveAccent, analyzer.TopN(analyzer.SortedCounts(usage.Editors), 8), "No editor usage found"},
		{"Programming Languages", theme.RoseAccent, analyzer.TopN(analyzer.SortedCounts(usage.Languages), 8), "No language usage found"},
		{"Build Tools", theme.SandAccent, analyzer.TopN(analyzer.SortedCounts(usage.BuildTools), 8), "No build tool usage found"},
		{"DevOps & Cloud", theme.SageAccent, analyzer.TopN(analyzer.SortedCounts(usage.DevOps), 8), "No DevOps tool usage found"},
	}

	// One layout for all four cards, so their bars share a grid
	var lists [][]analyzer.UsageCount
	for _, s := range sections {
		lists = append(lists, s.counts)
	}
	layout := newBarLayout(cardInner(halfCardWidth(width)), lists...)

	var specs []cardSpec
	for _, s := range sections {
		s := s
		specs = append(specs, cardSpec{title: s.title, accent: s.accent, body: func(int) []string {
			if len(s.counts) == 0 {
				return faint(s.empty)
			}
			return countRows(s.counts, layout, s.accent.Tone, anim)
		}})
	}
	return grid(specs, width)
}

// RenderTimeline renders the timeline, revealing entries one by one
func RenderTimeline(entries []types.TimelineEntry, width int, anim Anim) string {
	return grid([]cardSpec{{title: "Recent Interesting Commands", accent: theme.SageAccent, wide: true, body: func(int) []string {
		if len(entries) == 0 {
			return faint("No interesting commands found")
		}
		shown := int(math.Ceil(float64(len(entries)) * anim.Progress))
		lines := make([]string, len(entries))
		for i, entry := range entries[:shown] {
			when := "unknown date    "
			if !entry.Timestamp.IsZero() {
				when = entry.Timestamp.Format("2006-01-02 15:04")
			}
			lines[i] = theme.Faint.Render(when) + "  " +
				lipgloss.NewStyle().Foreground(theme.Sage).Render(fmt.Sprintf("%-4s", entry.Shell)) + "  " +
				theme.Normal.Render(oneLine(entry.Command))
		}
		return lines
	}}}, width)
}

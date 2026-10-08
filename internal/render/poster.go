// internal/render/poster.go
package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/theme"
)

// bigDigits spell numbers in the figlet "small" font, like the splash banner
var bigDigits = map[rune][]string{
	'0': {`  __  `, ` /  \ `, `| () |`, ` \__/ `, `      `},
	'1': {` _ `, `/ |`, `| |`, `|_|`, `   `},
	'2': {` ___ `, `|_  )`, ` / / `, `/___|`, `     `},
	'3': {` ____`, `|__ /`, ` |_ \`, `|___/`, `     `},
	'4': {` _ _  `, `| | | `, `|_  _|`, `  |_| `, `      `},
	'5': {` ___ `, `| __|`, `|__ \`, `|___/`, `     `},
	'6': {`  __ `, ` / / `, `/ _ \`, `\___/`, `     `},
	'7': {` ____ `, `|__  |`, `  / / `, ` /_/  `, `      `},
	'8': {` ___ `, `( _ )`, `/ _ \`, `\___/`, `     `},
	'9': {` ___ `, `/ _ \`, `\_, /`, ` /_/ `, `     `},
	',': {`   `, `   `, ` _ `, `( )`, `|/ `},
}

// bigNumber renders n as big ASCII art digits
func bigNumber(n int) []string {
	rows := make([]string, 5)
	for _, ch := range formatInt(n) {
		glyph, ok := bigDigits[ch]
		if !ok {
			continue
		}
		for i := range rows {
			rows[i] += glyph[i]
		}
	}
	return rows
}

// PosterCols is the width of the summary poster, in terminal columns
const PosterCols = 60

// RenderPoster renders a one-image summary of the year for sharing.
// persona is the AI-written persona title, or "" to use the built-in one.
func RenderPoster(s analyzer.WrappedStats, persona string) string {
	accent := theme.MauveAccent
	strong := lipgloss.NewStyle().Bold(true).Foreground(accent.Color)
	full := Anim{Progress: 1}

	var lines []string
	brand := strong.Render(">_ shellrecap")
	year := strong.Render(fmt.Sprint(s.Year))
	if s.AllTime {
		year = strong.Render("all time")
	}
	gap := PosterCols - lipgloss.Width(brand) - lipgloss.Width(year)
	lines = append(lines, brand+strings.Repeat(" ", maxInt(gap, 1))+year, "")

	for i, row := range bigNumber(s.TotalCommands) {
		// Lighter towards the top, like the bars
		lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(accent.Tone.Color(1-float64(i)*0.12)).Render(row))
	}
	period := fmt.Sprintf("commands in %d", s.Year)
	if s.AllTime {
		period = "commands in my shell history"
	}
	if s.HasTimes {
		period += fmt.Sprintf(" · %d active days", s.ActiveDays)
	}
	lines = append(lines, theme.Dim.Render(period), "")

	if persona == "" {
		persona, _ = s.Persona()
	}
	lines = append(lines, theme.Dim.Render("Persona  ")+lipgloss.NewStyle().Bold(true).Foreground(theme.Text).Render(persona), "")

	half := (PosterCols - 2) / 2
	inner := cardInner(half)

	// Top commands next to the time of day
	top := analyzer.TopN(s.TopPrograms, 5)
	topLines := countRows(top, newBarLayout(inner, top), theme.RoseAccent.Tone, full)
	var whenTitle string
	var whenLines []string
	if s.HasTimes {
		whenTitle = "When I code"
		whenLines = append(hourChart(s.HourCounts, inner, 4, theme.SandAccent.Tone, full),
			theme.Dim.Render(fmt.Sprintf("peak %02d:00 · %ss", s.BusiestHour(), s.BusiestWeekday())))
	} else {
		whenTitle = "My stack"
		langs := analyzer.TopN(s.Languages, 5)
		whenLines = countRows(langs, newBarLayout(inner, langs), theme.SandAccent.Tone, full)
	}
	lines = append(lines, posterRow(
		posterCard{"Top commands", theme.RoseAccent, topLines},
		posterCard{whenTitle, theme.SandAccent, whenLines},
	)...)

	// A typo to laugh at next to what's new
	typoLines := []string{theme.Normal.Render("Zero typos."), theme.Dim.Render("Suspiciously perfect.")}
	if len(s.Typos) > 0 {
		t := s.Typos[0]
		typoLines = []string{
			lipgloss.NewStyle().Bold(true).Foreground(theme.Clay).Render(fmt.Sprintf("%q", t.Typed)) +
				theme.Faint.Render(" → ") + theme.Normal.Render(t.Meant),
			theme.Dim.Render(fmt.Sprintf("%d typos caught all year", s.TotalTypos)),
		}
	}
	newTitle, newLines := "New this year", []string{}
	for _, p := range analyzer.TopN(s.NewPrograms, 3) {
		newLines = append(newLines, theme.Normal.Render(p.Name)+theme.Faint.Render(fmt.Sprintf(" · %d runs", p.Count)))
	}
	if len(newLines) == 0 {
		newTitle = "Power moves"
		newLines = []string{
			theme.Normal.Render(fmt.Sprintf("%d sudo", s.SudoCommands)),
			theme.Normal.Render(fmt.Sprintf("%d pipes", s.PipeCommands)),
		}
	}
	lines = append(lines, posterRow(
		posterCard{"Typo of the year", theme.ClayAccent, typoLines},
		posterCard{newTitle, theme.SageAccent, newLines},
	)...)

	lines = append(lines, "", theme.Faint.Render("github.com/ksauraj/shellrecap · #shellrecap"))
	return strings.Join(lines, "\n")
}

type posterCard struct {
	title  string
	accent theme.Accent
	lines  []string
}

// posterRow puts two cards side by side at the same height
func posterRow(left, right posterCard) []string {
	half := (PosterCols - 2) / 2
	height := maxInt(len(left.lines), len(right.lines))
	row := lipgloss.JoinHorizontal(lipgloss.Top,
		card(left.title, left.accent, left.lines, half, height), "  ",
		card(right.title, right.accent, right.lines, half, height))
	return strings.Split(row, "\n")
}

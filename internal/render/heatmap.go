// internal/render/heatmap.go
package render

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/theme"
)

// heatmapLabelWidth is the width of the weekday labels left of the heatmap
const heatmapLabelWidth = 4

// heatmapLevels are the tones of the four activity levels, from quiet to busy
var heatmapLevels = []float64{0.3, 0.5, 0.75, 1}

// heatmapThresholds splits the active days into four levels by quartile,
// like a contribution graph, so one huge day doesn't wash out the rest
func heatmapThresholds(days map[string]int) [3]int {
	var counts []int
	for _, c := range days {
		if c > 0 {
			counts = append(counts, c)
		}
	}
	var t [3]int
	if len(counts) == 0 {
		return t
	}
	sort.Ints(counts)
	for i := range t {
		t[i] = counts[(i+1)*len(counts)/4]
	}
	return t
}

func heatmapLevel(count int, t [3]int) int {
	switch {
	case count <= 0:
		return -1
	case count < t[0]:
		return 0
	case count < t[1]:
		return 1
	case count < t[2]:
		return 2
	default:
		return 3
	}
}

// heatmap draws a contribution-graph calendar of commands per day for the
// year up to end: a column per week, a row per weekday, month names on top
// and a legend underneath. When that doesn't fit in width columns, the
// latest weeks are shown. Cells wash in from the
// top left with the reveal animation and catch the same glare as the bars.
func heatmap(days map[string]int, year int, end time.Time, width int, tone *theme.Tone, anim Anim) []string {
	first := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local)
	last := time.Date(year, time.December, 31, 0, 0, 0, 0, time.Local)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.Local)
	if end.Before(first) {
		end = first
	}
	if end.After(last) {
		end = last
	}
	// Weeks start on Sunday, like the weekday counts
	start := first.AddDate(0, 0, -int(first.Weekday()))
	// The graph ends with the week of end, rather than in empty weeks
	weeks := maxInt(int(end.Sub(start).Hours()/24)/7+1, 1)

	// Leave a gap between cells when the whole year fits with them
	cell := 1
	if heatmapLabelWidth+weeks*2-1 <= width {
		cell = 2
	}
	shown := minInt(weeks, maxInt((width-heatmapLabelWidth+cell-1)/cell, 1))
	firstWeek := weeks - shown

	thresholds := heatmapThresholds(days)
	diagonal := float64(shown + 7)

	// Month names over the week their first day falls in
	months := []byte(strings.Repeat(" ", heatmapLabelWidth+shown*cell))
	free := 0
	for m := time.January; m <= time.December; m++ {
		week := int(time.Date(year, m, 1, 0, 0, 0, 0, time.Local).Sub(start).Hours()/24)/7 - firstWeek
		x := heatmapLabelWidth + week*cell
		if week < 0 || x < free || x+3 > len(months) {
			continue
		}
		copy(months[x:], m.String()[:3])
		free = x + 4
	}
	lines := []string{theme.Faint.Render(strings.TrimRight(string(months), " "))}

	labels := map[time.Weekday]string{time.Monday: "Mon", time.Wednesday: "Wed", time.Friday: "Fri"}
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		var b strings.Builder
		b.WriteString(theme.Faint.Render(fmt.Sprintf("%-*s", heatmapLabelWidth, labels[wd])))
		for w := 0; w < shown; w++ {
			if w > 0 && cell == 2 {
				b.WriteString(" ")
			}
			day := start.AddDate(0, 0, (firstWeek+w)*7+int(wd))
			if day.Before(first) || day.After(end) {
				b.WriteString(" ")
				continue
			}
			level := heatmapLevel(days[day.Format("2006-01-02")], thresholds)
			revealed := float64(w+int(wd)+1)/diagonal <= anim.Progress
			if level < 0 || !revealed {
				b.WriteString(theme.Track.Render("■"))
				continue
			}
			shine := glare(w*cell, anim.Frame) * glareStrength
			b.WriteString(lipgloss.NewStyle().Foreground(tone.Shine(heatmapLevels[level], shine)).Render("■"))
		}
		lines = append(lines, b.String())
	}

	legend := theme.Faint.Render(strings.Repeat(" ", heatmapLabelWidth)+"less ") + theme.Track.Render("■")
	for _, t := range heatmapLevels {
		legend += " " + lipgloss.NewStyle().Foreground(tone.Color(t)).Render("■")
	}
	return append(lines, legend+theme.Faint.Render(" more"))
}

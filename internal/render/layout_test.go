package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/theme"
)

func TestChartsFillTheirWidth(t *testing.T) {
	var hours [24]int
	var months [12]int
	for i := range hours {
		hours[i] = i * 3
	}
	for i := range months {
		months[i] = 12 - i
	}
	anim := Anim{Progress: 1}

	for _, width := range []int{30, 48, 71, 102} {
		for _, line := range hourChart(hours, width, 5, theme.MauveAccent.Tone, anim) {
			if w := lipgloss.Width(line); w != width {
				t.Errorf("hour chart at width %d has a %d column line: %q", width, w, line)
			}
		}
		for _, line := range monthChart(months, width, 4, theme.MauveAccent.Tone, anim) {
			if w := lipgloss.Width(line); w != width {
				t.Errorf("month chart at width %d has a %d column line: %q", width, w, line)
			}
		}
	}
}

func TestColumnWidthsSpreadLeftoverCells(t *testing.T) {
	widths := columnWidths(24, 100, 0)
	total, min, max := 0, widths[0], widths[0]
	for _, w := range widths {
		total += w
		min, max = minInt(min, w), maxInt(max, w)
	}
	if total != 100 || max-min > 1 {
		t.Errorf("columnWidths(24, 100, 0) = %v, want 100 cells spread evenly", widths)
	}
	if axis := hourChart([24]int{}, 100, 1, theme.MauveAccent.Tone, Anim{})[1]; !strings.HasSuffix(axis, "23") {
		t.Errorf("hour axis doesn't end at 23: %q", axis)
	}
}

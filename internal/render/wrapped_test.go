package render

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/theme"
)

func plain(s string) string {
	return ansiCodes.ReplaceAllString(s, "")
}

// barStart is the column the first bar cell of a line starts at, or -1
func barStart(line string) int {
	return strings.Index(plain(line), "■")
}

func TestStackSlideBarsLineUp(t *testing.T) {
	stats := analyzer.WrappedStats{
		Year:          2026,
		TotalCommands: 100,
		Languages:     []analyzer.UsageCount{{Name: "Python", Count: 67}, {Name: "JavaScript", Count: 13}},
		DevOps:        []analyzer.UsageCount{{Name: "kubectl", Count: 823}, {Name: "az", Count: 158}},
	}
	for _, slide := range BuildWrappedSlides(stats, 100, Anim{Progress: 1}) {
		if slide.Title != "Your Stack" {
			continue
		}
		start := -1
		for _, line := range slide.Lines {
			col := barStart(line)
			if col < 0 {
				continue
			}
			if start >= 0 && col != start {
				t.Errorf("bar starts at column %d, want %d: %q", col, start, plain(line))
			}
			start = col
		}
		return
	}
	t.Fatal("no stack slide")
}

func TestHeatmapFitsItsWidth(t *testing.T) {
	days := map[string]int{"2026-01-01": 3, "2026-03-15": 40, "2026-06-30": 12, "2026-10-19": 7}
	end := time.Date(2026, 10, 19, 0, 0, 0, 0, time.Local)
	for _, width := range []int{30, 56, 80, 102, 120} {
		lines := heatmap(days, 2026, end, width, theme.MauveAccent.Tone, Anim{Progress: 1})
		if len(lines) != 9 {
			t.Fatalf("heatmap at width %d has %d lines, want 9", width, len(lines))
		}
		for _, line := range lines {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("heatmap at width %d has a %d column line: %q", width, w, plain(line))
			}
		}
	}
}

func TestDaysInYear(t *testing.T) {
	if d := daysInYear(2026, time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local)); d != 1 {
		t.Errorf("on Jan 1, %d days have gone by, want 1", d)
	}
	if d := daysInYear(2024, time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)); d != 366 {
		t.Errorf("after a leap year, %d days have gone by, want 366", d)
	}
}

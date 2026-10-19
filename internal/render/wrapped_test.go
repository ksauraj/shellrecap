package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ksauraj/shellrecap/internal/analyzer"
)

var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

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

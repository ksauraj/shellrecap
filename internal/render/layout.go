// internal/render/layout.go
package render

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/theme"
)

// Anim carries the animation state into the renderers
type Anim struct {
	Progress float64 // reveal animation, from 0 to 1
	Frame    int     // global animation tick, drives the glare
}

const (
	maxGridWidth = 110
	// twoColumnMin is the narrowest terminal that gets two cards per row
	twoColumnMin = 80
)

func gridWidth(width int) int {
	if width > maxGridWidth {
		return maxGridWidth
	}
	if width < 30 {
		return 30
	}
	return width
}

// halfCardWidth is the width of a card that shares its row, or of every
// card when the terminal is too narrow for two columns
func halfCardWidth(width int) int {
	w := gridWidth(width)
	if w < twoColumnMin {
		return w
	}
	return (w - 2) / 2
}

// cardInner is the usable text width inside a card
func cardInner(cardWidth int) int {
	return cardWidth - 4 // border and padding
}

// cardSpec describes a card in a grid. body receives the card's inner width.
type cardSpec struct {
	title  string
	accent theme.Accent
	wide   bool // takes a whole row
	body   func(inner int) []string
}

// grid lays cards out two per row, or one per row on narrow terminals.
// Cards in a row get the same height so their borders line up.
func grid(specs []cardSpec, width int) string {
	w := gridWidth(width)
	twoColumns := w >= twoColumnMin

	var rows [][]cardSpec
	for i := 0; i < len(specs); {
		if twoColumns && !specs[i].wide && i+1 < len(specs) && !specs[i+1].wide {
			rows = append(rows, specs[i:i+2])
			i += 2
			continue
		}
		rows = append(rows, specs[i:i+1])
		i++
	}

	out := make([]string, 0, len(rows))
	for _, row := range rows {
		cardWidth := w
		if len(row) == 2 {
			cardWidth = halfCardWidth(width)
		}
		bodies := make([][]string, len(row))
		height := 0
		for i, spec := range row {
			bodies[i] = spec.body(cardInner(cardWidth))
			height = maxInt(height, len(bodies[i]))
		}
		cards := make([]string, len(row))
		for i, spec := range row {
			cards[i] = card(spec.title, spec.accent, bodies[i], cardWidth, height)
		}
		if len(cards) == 2 {
			out = append(out, lipgloss.JoinHorizontal(lipgloss.Top, cards[0], strings.Repeat(" ", w-2*cardWidth), cards[1]))
		} else {
			out = append(out, cards[0])
		}
	}
	return strings.Join(out, "\n")
}

// card draws a rounded box with the title set into its top border. Lines
// are truncated to fit and padded to height rows.
func card(title string, accent theme.Accent, lines []string, width, height int) string {
	inner := cardInner(width)
	border := theme.Fg(theme.Border)
	title = fit(title, maxInt(inner-2, 1))
	fill := maxInt(width-5-lipgloss.Width(title), 0)

	var b strings.Builder
	b.WriteString(border.Render("╭─ ") + lipgloss.NewStyle().Bold(true).Foreground(accent.Color).Render(title) +
		border.Render(" "+strings.Repeat("─", fill)+"╮") + "\n")
	// A blank row above and below the content gives it room to breathe
	for i := -1; i <= height; i++ {
		line := ""
		if i >= 0 && i < len(lines) {
			line = fit(lines[i], inner)
		}
		b.WriteString(border.Render("│") + " " + line + strings.Repeat(" ", maxInt(inner-lipgloss.Width(line), 0)) +
			" " + border.Render("│") + "\n")
	}
	b.WriteString(border.Render("╰" + strings.Repeat("─", width-2) + "╯"))
	return b.String()
}

// barLayout sizes the columns of bar rows so that every bar in a grid
// starts and ends at the same column
type barLayout struct {
	nameWidth, countWidth, barWidth int
}

// newBarLayout fits bar rows from all the given lists into inner columns
func newBarLayout(inner int, lists ...[]analyzer.UsageCount) barLayout {
	l := barLayout{nameWidth: 4, countWidth: 1}
	for _, list := range lists {
		for _, c := range list {
			l.nameWidth = maxInt(l.nameWidth, lipgloss.Width(c.Name))
			l.countWidth = maxInt(l.countWidth, len(formatInt(c.Count)))
		}
	}
	l.nameWidth = minInt(l.nameWidth, 18)
	l.barWidth = inner - l.nameWidth - l.countWidth - 2
	l.barWidth = maxInt(minInt(l.barWidth, 40), 4)
	return l
}

// countRows renders "name ▄▄▄▄▄▄ count" rows, scaled to the largest count.
// Bars and counts grow with the reveal animation.
func countRows(counts []analyzer.UsageCount, l barLayout, g *theme.Gradient, anim Anim) []string {
	max := 0
	for _, c := range counts {
		max = maxInt(max, c.Count)
	}
	lines := make([]string, 0, len(counts))
	for _, c := range counts {
		name := fit(c.Name, l.nameWidth)
		lines = append(lines, theme.Normal.Render(name)+strings.Repeat(" ", l.nameWidth-lipgloss.Width(name))+" "+
			bar(scale(c.Count, anim.Progress), max, l.barWidth, g, anim.Frame)+" "+
			theme.Dim.Render(fmt.Sprintf("%*s", l.countWidth, formatInt(scale(c.Count, anim.Progress)))))
	}
	return lines
}

const (
	// glareWidth is how many cells the glare spreads over on each side
	glareWidth = 3.0
	// glarePeriod is the number of ticks between glare sweeps
	glarePeriod = 45
	glareSpeed  = 2.5 // cells per tick
)

// glare returns how strongly the sweeping glare lights up cell x, from 0
// to 1. Every bar uses the same timing, so bars that line up in the grid
// catch the light together.
func glare(x, frame int) float64 {
	pos := float64(frame%glarePeriod)*glareSpeed - glareWidth
	d := math.Abs(float64(x) - pos)
	if d >= glareWidth {
		return 0
	}
	return 1 - d/glareWidth
}

// bar draws a half-height bar filled along the gradient, with half-cell
// precision and a sweeping glare
func bar(value, max, width int, g *theme.Gradient, frame int) string {
	if max <= 0 || width <= 0 {
		return ""
	}
	halves := int(math.Round(float64(value) / float64(max) * float64(width*2)))
	if value > 0 && halves == 0 {
		halves = 1
	}
	halves = minInt(halves, width*2)

	var b strings.Builder
	cells := (halves + 1) / 2
	for i := 0; i < cells; i++ {
		c := g.At(float64(i) / float64(maxInt(width-1, 1)))
		block := "▄"
		if i == cells-1 && halves%2 == 1 {
			block = "▖" // half a cell
		}
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Shine(c, glare(i, frame)*0.75)).Render(block))
	}
	b.WriteString(theme.Track.Render(strings.Repeat("▄", width-cells)))
	return b.String()
}

var chartBlocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// columnChart draws counts as vertical bars, height rows tall, colored
// along the brand gradient with the same sweeping glare as the bars
func columnChart(counts []int, colWidth, gap, height int, anim Anim) []string {
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
				level = int(math.Round(float64(c) / float64(max) * float64(height*8) * anim.Progress))
			}
			if c > 0 && level == 0 && anim.Progress > 0 {
				level = 1
			}
			fill := level - rowFromBottom*8
			if fill < 0 {
				fill = 0
			}
			if fill > 8 {
				fill = 8
			}
			col := theme.Brand.At(float64(i) / float64(maxInt(len(counts)-1, 1)))
			shine := glare(i*(colWidth+gap), anim.Frame) * 0.75
			b.WriteString(lipgloss.NewStyle().Foreground(theme.Shine(col, shine)).
				Render(strings.Repeat(chartBlocks[fill], colWidth)))
			if i < len(counts)-1 {
				b.WriteString(strings.Repeat(" ", gap))
			}
		}
		rows[r] = b.String()
	}
	return rows
}

// hourChart draws activity per hour of day with an hour axis underneath
func hourChart(counts [24]int, anim Anim) []string {
	rows := columnChart(counts[:], 2, 0, 3, anim)

	axis := []byte(strings.Repeat(" ", 48))
	for _, h := range []int{0, 6, 12, 18} {
		copy(axis[h*2:], fmt.Sprintf("%02d", h))
	}
	copy(axis[46:], "23")
	return append(rows, theme.Faint.Render(string(axis)))
}

// monthChart draws activity per month with month names underneath
func monthChart(counts [12]int, anim Anim) []string {
	rows := columnChart(counts[:], 3, 1, 3, anim)

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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

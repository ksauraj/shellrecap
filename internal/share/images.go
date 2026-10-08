// internal/share/images.go
package share

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"strconv"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/render"
	"github.com/ksauraj/shellrecap/internal/theme"
	"github.com/muesli/termenv"
)

// Recap is what gets shared: the year's stats and any AI-written slides
type Recap struct {
	Stats analyzer.WrappedStats
	AI    []ai.Section
}

const (
	// gifSize is the GIF's width and height. Square fits X (at most
	// 1280x1080), Discord, Reddit and Slack.
	gifSize = 1080
	// posterWidth by posterHeight is 4:5, the tallest image Instagram's
	// feed shows uncropped, and also fits LinkedIn, Bluesky and X
	posterWidth, posterHeight = 1080, 1350

	// slideCols is the width the slides are rendered at, in columns
	slideCols = 64
	// maxGIFPixels and maxGIFFrames keep the GIF within X's limits of 300
	// million pixels and 350 frames in total
	maxGIFPixels = 300_000_000
	maxGIFFrames = 350
	// fps is the GIF's frame rate; frameDelay is in hundredths of a second
	fps        = 10
	frameDelay = 100 / fps
	// minSlideFrames is the shortest a slide stays up, so it can be read
	minSlideFrames = 14
	// maxSlideFrames is how long a slide stays up when there's room
	maxSlideFrames = 22
	// margin is the space around the content, in cells
	margin = 2
)

var styleOnce sync.Once

// useShareStyle renders everything in true color on a dark background, so
// the images look the same whatever terminal the command runs in. It
// changes lipgloss's global renderer, so it mustn't run inside the TUI.
func useShareStyle() {
	styleOnce.Do(func() {
		lipgloss.SetColorProfile(termenv.TrueColor)
		lipgloss.SetHasDarkBackground(true)
	})
}

func hexColor(c lipgloss.AdaptiveColor) color.RGBA {
	v, _ := strconv.ParseUint(c.Dark[1:], 16, 32)
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

var (
	background = hexColor(theme.Base)
	foreground = hexColor(theme.Text)
)

// slides returns every Recap slide for sharing, at an animation state.
// Footers are dropped: they explain things only relevant inside the app.
func (r Recap) slides(anim render.Anim) []string {
	built := append(render.BuildWrappedSlides(r.Stats, slideCols, anim), render.AISlides(r.AI, "")...)
	label := fmt.Sprintf("SHELLRECAP %d", r.Stats.Year)
	if r.Stats.AllTime {
		label = "SHELLRECAP"
	}
	cards := make([]string, len(built))
	for i, slide := range built {
		slide.Footer = ""
		cards[i] = render.RenderSlide(slide, label, "", i, len(built), anim.Frame, slideCols)
	}
	return cards
}

// slideTiming spreads the frame budget over the slides: how many frames
// each slide stays up, and how many of those its numbers count up for
func slideTiming(slides int) (perSlide, reveal int) {
	budget := minInt(maxGIFFrames, maxGIFPixels/(gifSize*gifSize))
	perSlide = budget / maxInt(slides, 1)
	if perSlide > maxSlideFrames {
		perSlide = maxSlideFrames
	}
	if perSlide < minSlideFrames {
		perSlide = minSlideFrames
	}
	return perSlide, perSlide * 3 / 8
}

// fitCell returns the largest cell width at which rows rows of cols
// columns, plus margins, fit in width by height pixels
func fitCell(cols, rows, width, height int) int {
	byWidth := width / (cols + 2*margin)
	byHeight := int(float64(height) / (float64(rows+2*margin) * cellAspect))
	return minInt(byWidth, byHeight)
}

// WriteGIF writes the animated Recap: every slide in turn, numbers
// counting up and bars growing, as in the app
func WriteGIF(w io.Writer, r Recap) error {
	useShareStyle()

	// Size the text so the tallest slide fits, with a header and footer
	final := r.slides(render.Anim{Progress: 1})
	if len(final) == 0 {
		return fmt.Errorf("there are no slides to share")
	}
	tallest := 0
	for _, card := range final {
		tallest = maxInt(tallest, len(parseANSI(card, foreground)))
	}
	rend := newRenderer(fitCell(slideCols, tallest+4, gifSize, gifSize))

	header := parseANSI(lipgloss.NewStyle().Bold(true).Foreground(theme.Brand.Tone.Color(0.3)).Render(">_")+" "+
		lipgloss.NewStyle().Bold(true).Foreground(theme.Brand.Color).Render("shellrecap")+
		theme.Faint.Render(fmt.Sprintf(" · my %d in the terminal", r.Stats.Year)), foreground)
	footer := parseANSI(theme.Faint.Render("github.com/ksauraj/shellrecap · #shellrecap"), foreground)
	left := (gifSize - slideCols*rend.cellW) / 2

	perSlide, reveal := slideTiming(len(final))
	frames := perSlide * len(final)
	return encodeGIF(w, frames, func(i int) *image.RGBA {
		slide, tick := i/perSlide, i%perSlide
		p := math.Min(1, float64(tick)/float64(reveal))
		anim := render.Anim{Progress: 1 - math.Pow(1-p, 3), Frame: i}

		card := parseANSI(r.slides(anim)[slide], foreground)
		img := image.NewRGBA(image.Rect(0, 0, gifSize, gifSize))
		fill(img, img.Rect, background)
		rend.draw(img, image.Pt(left, margin*rend.cellH/2), header)
		top := (gifSize - len(card)*rend.cellH) / 2
		rend.draw(img, image.Pt(left, top), card)
		rend.draw(img, image.Pt(left, gifSize-(margin+1)*rend.cellH/2-rend.cellH/2), footer)
		return img
	}, frameDelay)
}

// persona is the AI-written persona title, or "" without AI slides
func (r Recap) persona() string {
	if len(r.AI) == 0 {
		return ""
	}
	// The AI writes the persona slide first
	return ai.CleanText(r.AI[0].Title)
}

// WritePoster writes the one-image summary of the year as a PNG
func WritePoster(w io.Writer, r Recap) error {
	useShareStyle()
	grid := parseANSI(render.RenderPoster(r.Stats, r.persona()), foreground)
	rend := newRenderer(fitCell(render.PosterCols, len(grid), posterWidth, posterHeight))

	img := image.NewRGBA(image.Rect(0, 0, posterWidth, posterHeight))
	fill(img, img.Rect, background)
	left := (posterWidth - render.PosterCols*rend.cellW) / 2
	top := (posterHeight - len(grid)*rend.cellH) / 2
	rend.draw(img, image.Pt(left, top), grid)
	return png.Encode(w, img)
}

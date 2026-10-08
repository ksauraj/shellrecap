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
	"github.com/ksauraj/shellrecap/internal/config"
	"github.com/ksauraj/shellrecap/internal/render"
	"github.com/ksauraj/shellrecap/internal/theme"
	"github.com/muesli/termenv"
)

// Recap is what gets shared: the analyzed history for the tab pictures,
// the year's stats and any AI-written slides
type Recap struct {
	Data  analyzer.ShellData
	Stats analyzer.WrappedStats
	AI    []ai.Section
}

type size struct{ w, h int }

var (
	// The Recap GIF is square and the tour 4:3; at the standard size both
	// fit X, which takes GIFs up to 1280x1080
	recapGIFSizes = map[string]size{config.GIFStandard: {1080, 1080}, config.GIFSmall: {720, 720}}
	tourGIFSizes  = map[string]size{config.GIFStandard: {1280, 960}, config.GIFSmall: {960, 720}}
	// shapes are the picture sizes
	shapes = map[string]size{
		config.Portrait:  {1080, 1350},
		config.Story:     {1080, 1920},
		config.Square:    {1080, 1080},
		config.Landscape: {1920, 1080},
	}
)

const (
	// slideCols is the width the Recap GIF's slides are rendered at
	slideCols = 64
	// tourCols is the width the tour renders the app at
	tourCols = 100
	// maxGIFPixels and maxGIFFrames keep GIFs within X's limits of 300
	// million pixels and 350 frames in total
	maxGIFPixels = 300_000_000
	maxGIFFrames = 350
	// fps is the GIF frame rate; frameDelay is in hundredths of a second
	fps        = 10
	frameDelay = 100 / fps
	// minSlideFrames is the shortest a slide stays up, so it can be read
	minSlideFrames = 14
	// maxSlideFrames is how long a slide stays up when there's room
	maxSlideFrames = 22
	// revealFrames is how long the tour takes to reveal each view
	revealFrames = 8
	// tabHold and slideHold are how long the tour pauses on each view,
	// in hundredths of a second
	tabHold, slideHold = 250, 150
	// noGlare is an animation tick at which the glare is off screen
	noGlare = 44
	// margin is the space around the content, in cells
	margin = 2
)

var styleOnce sync.Once

// useShareStyle renders everything in true color, so the images look the
// same whatever terminal the command runs in. It changes lipgloss's global
// renderer, so it mustn't run inside the TUI.
func useShareStyle() {
	styleOnce.Do(func() {
		lipgloss.SetColorProfile(termenv.TrueColor)
	})
	p := theme.Current()
	if p.ANSI {
		// Images need exact colors
		theme.Use(theme.Dark)
		p = theme.Dark
	}
	background, foreground = hexColor(p.Base), hexColor(p.Text)
}

func hexColor(hex string) color.RGBA {
	v, _ := strconv.ParseUint(hex[1:], 16, 32)
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

// The image's background and default text color, from the theme in use
var background, foreground color.RGBA

// tabs are the dashboard views pictured and toured
var tabs = []struct {
	name, file string
	render     func(r Recap, width int, anim render.Anim) string
}{
	{"Overview", "overview", func(r Recap, w int, a render.Anim) string {
		return render.RenderOverviewForSharing(r.Data, w, a)
	}},
	{"Tech Profile", "tech-profile", func(r Recap, w int, a render.Anim) string {
		return render.RenderTechProfile(r.Data.Insights.TechnicalProfile, w, a)
	}},
	{"Work Patterns", "work-patterns", func(r Recap, w int, a render.Anim) string {
		return render.RenderWorkPatterns(r.Data.Insights.WorkPatterns, w, a)
	}},
	{"Tool Usage", "tool-usage", func(r Recap, w int, a render.Anim) string {
		return render.RenderToolUsage(r.Data.Insights.ToolUsage, w, a)
	}},
}

// tabNames are the tabs as the app shows them
var tabNames = []string{"Overview", "Tech Profile", "Work Patterns", "Tool Usage", "Recap"}

// slides returns every Recap slide for sharing, cols wide, at an animation
// state. Footers are dropped: they explain things only relevant inside
// the app.
func (r Recap) slides(cols int, anim render.Anim) []string {
	built := append(render.BuildWrappedSlides(r.Stats, cols, anim), render.AISlides(r.AI, "")...)
	label := fmt.Sprintf("SHELLRECAP %d", r.Stats.Year)
	if r.Stats.AllTime {
		label = "SHELLRECAP"
	}
	cards := make([]string, len(built))
	for i, slide := range built {
		slide.Footer = ""
		cards[i] = render.RenderSlide(slide, label, "", i, len(built), anim.Frame, cols)
	}
	return cards
}

// persona is the AI-written persona title, or "" without AI slides
func (r Recap) persona() string {
	if len(r.AI) == 0 {
		return ""
	}
	// The AI writes the persona slide first
	return ai.CleanText(r.AI[0].Title)
}

// brand is the line at the top of every picture
func (r Recap) brand() string {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Brand.Tone.Color(0.3)).Render(">_") + " " +
		lipgloss.NewStyle().Bold(true).Foreground(theme.Brand.Color).Render("shellrecap") +
		theme.Faint.Render(fmt.Sprintf(" · my %d in the terminal", r.Stats.Year))
}

func credit() string {
	return theme.Faint.Render("github.com/ksauraj/shellrecap · #shellrecap")
}

// scene is a picture made of terminal output: a header at the top, the
// content and a footer at the bottom
type scene struct {
	header, content, footer string
	// center puts the content in the middle; otherwise it follows the
	// header, as in the app
	center bool
	// block centers the header and content together, for pictures whose
	// content would otherwise leave the bottom half empty
	block bool
}

// rows is how many rows the scene takes, with a blank row on each side of
// the content
func (s scene) rows() int {
	return len(parseANSI(s.header, foreground)) + len(parseANSI(s.content, foreground)) +
		len(parseANSI(s.footer, foreground)) + 2
}

// fitCell returns the largest cell width at which rows rows of cols
// columns, plus margins, fit in sz
func fitCell(cols, rows int, sz size) int {
	byWidth := sz.w / (cols + 2*margin)
	byHeight := int(float64(sz.h) / (float64(rows+2*margin) * cellAspect))
	return minInt(byWidth, byHeight)
}

// paint draws the scene on a new canvas, cols columns wide and centered
func (rend *renderer) paint(sz size, cols int, s scene) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, sz.w, sz.h))
	fill(img, img.Rect, background)
	left := (sz.w - cols*rend.cellW) / 2
	top := margin * rend.cellH / 2

	header := parseANSI(s.header, foreground)
	content := parseANSI(s.content, foreground)
	if s.block {
		top = maxInt(top, (sz.h-(len(header)+1+len(content))*rend.cellH)/2)
	}
	rend.draw(img, image.Pt(left, top), header)
	footer := parseANSI(s.footer, foreground)
	footerTop := sz.h - margin*rend.cellH/2 - len(footer)*rend.cellH
	rend.draw(img, image.Pt(left, footerTop), footer)

	contentTop := top + (len(header)+1)*rend.cellH
	if s.center {
		contentTop = (sz.h - len(content)*rend.cellH) / 2
	}
	rend.draw(img, image.Pt(left, contentTop), content)
	return img
}

// tallest is the most rows any of the scenes take
func tallest(scenes []scene) int {
	rows := 0
	for _, s := range scenes {
		rows = maxInt(rows, s.rows())
	}
	return rows
}

func ease(tick, of int) float64 {
	p := math.Min(1, float64(tick)/float64(of))
	return 1 - math.Pow(1-p, 3)
}

// slideTiming spreads the frame budget of a GIF of sz over the slides:
// how many frames each slide stays up, and how many of those its numbers
// count up for
func slideTiming(slides int, sz size) (perSlide, reveal int) {
	budget := minInt(maxGIFFrames, maxGIFPixels/(sz.w*sz.h))
	perSlide = budget / maxInt(slides, 1)
	if perSlide > maxSlideFrames {
		perSlide = maxSlideFrames
	}
	if perSlide < minSlideFrames {
		perSlide = minSlideFrames
	}
	return perSlide, perSlide * 3 / 8
}

// WriteRecapGIF writes the Recap slides animated: every slide in turn,
// numbers counting up and bars growing, as in the app
func WriteRecapGIF(w io.Writer, r Recap, gifSize string) error {
	useShareStyle()
	sz := recapGIFSizes[gifSize]
	recapScene := func(card string) scene {
		return scene{header: r.brand(), content: card, footer: credit(), center: true}
	}

	final := r.slides(slideCols, render.Anim{Progress: 1})
	if len(final) == 0 {
		return fmt.Errorf("there are no slides to share")
	}
	var scenes []scene
	for _, card := range final {
		scenes = append(scenes, recapScene(card))
	}
	rend := newRenderer(fitCell(slideCols, tallest(scenes), sz))

	perSlide, reveal := slideTiming(len(final), sz)
	return encodeGIF(w, perSlide*len(final), func(i int) (*image.RGBA, int) {
		slide, tick := i/perSlide, i%perSlide
		card := r.slides(slideCols, render.Anim{Progress: ease(tick, reveal), Frame: i})[slide]
		return rend.paint(sz, slideCols, recapScene(card)), frameDelay
	})
}

// appHeader is the app's header and tab bar with one tab active
func (r Recap) appHeader(active, cols int) string {
	return r.brand() + "\n" + render.RenderTabs(tabNames, active, cols)
}

// WriteTourGIF writes a tour of the whole app: each tab revealed in turn,
// then the Recap slides. Every view animates in, then holds on a single
// long frame, which keeps the GIF small and within X's frame limits.
func WriteTourGIF(w io.Writer, r Recap, gifSize string) error {
	useShareStyle()
	sz := tourGIFSizes[gifSize]

	type view struct {
		tab    int
		slide  int
		render func(anim render.Anim) string
	}
	var views []view
	for i, tab := range tabs {
		tab := tab
		views = append(views, view{tab: i, render: func(a render.Anim) string { return tab.render(r, tourCols, a) }})
	}
	recapTab := len(tabNames) - 1
	slides := len(r.slides(tourCols, render.Anim{Progress: 1}))
	for i := 0; i < slides; i++ {
		i := i
		views = append(views, view{tab: recapTab, slide: i, render: func(a render.Anim) string {
			return r.slides(tourCols, a)[i]
		}})
	}

	viewScene := func(v view, a render.Anim) scene {
		return scene{header: r.appHeader(v.tab, tourCols), content: v.render(a), footer: credit()}
	}
	var scenes []scene
	for _, v := range views {
		scenes = append(scenes, viewScene(v, render.Anim{Progress: 1, Frame: noGlare}))
	}
	rend := newRenderer(fitCell(tourCols, tallest(scenes), sz))

	perView := revealFrames + 1
	return encodeGIF(w, perView*len(views), func(i int) (*image.RGBA, int) {
		v, tick := views[i/perView], i%perView
		if tick < revealFrames {
			return rend.paint(sz, tourCols, viewScene(v, render.Anim{Progress: ease(tick, revealFrames-1), Frame: tick})), frameDelay
		}
		hold := tabHold
		if v.tab == recapTab {
			hold = slideHold
		}
		return rend.paint(sz, tourCols, viewScene(v, render.Anim{Progress: 1, Frame: noGlare})), hold
	})
}

// picture renders a scene as a PNG of the given shape
func picture(w io.Writer, sz size, cols int, s scene) error {
	rend := newRenderer(fitCell(cols, s.rows(), sz))
	return png.Encode(w, rend.paint(sz, cols, s))
}

// WritePoster writes the one-image summary of the year as a PNG
func WritePoster(w io.Writer, r Recap, shape string) error {
	useShareStyle()
	return picture(w, shapes[shape], render.PosterCols, scene{content: render.RenderPoster(r.Stats, r.persona()), center: true})
}

// pictureCols is how wide the dashboard is pictured: the narrowest that
// still has two cards a row, or wider for landscape pictures
func pictureCols(shape string) int {
	if shape == config.Landscape {
		return 110
	}
	return 80
}

// WriteTabPicture writes a picture of one dashboard tab as a PNG
func WriteTabPicture(w io.Writer, r Recap, tab int, shape string) error {
	useShareStyle()
	cols := pictureCols(shape)
	content := tabs[tab].render(r, cols, render.Anim{Progress: 1, Frame: noGlare})
	return picture(w, shapes[shape], cols, scene{header: r.appHeader(tab, cols), content: content, footer: credit(), block: true})
}

// WriteSlidePicture writes a picture of one Recap slide as a PNG
func WriteSlidePicture(w io.Writer, r Recap, slide int, shape string) error {
	useShareStyle()
	card := r.slides(slideCols, render.Anim{Progress: 1, Frame: noGlare})[slide]
	return picture(w, shapes[shape], slideCols, scene{header: r.brand(), content: card, footer: credit(), center: true})
}

// SlideCount is how many Recap slides there are to picture
func (r Recap) SlideCount() int {
	return len(render.BuildWrappedSlides(r.Stats, slideCols, render.Anim{Progress: 1})) + len(r.AI)
}

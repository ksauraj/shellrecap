package share

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/render"
)

func uc(name string, count int) analyzer.UsageCount {
	return analyzer.UsageCount{Name: name, Count: count}
}

// testRecap is a full year with every kind of slide, and AI slides
func testRecap() Recap {
	s := analyzer.WrappedStats{
		Year:              2026,
		TotalCommands:     3698,
		UniqueCommands:    3663,
		UniquePrograms:    247,
		Shells:            []analyzer.UsageCount{{Name: "fish", Count: 3698}},
		TopPrograms:       []analyzer.UsageCount{uc("kubectl", 795), uc("git", 659), uc("cd", 226), uc("curl", 222), uc("az", 158)},
		TopGitSubcommands: []analyzer.UsageCount{uc("commit", 295), uc("add", 120), uc("clone", 50)},
		GitCommands:       659,
		Languages:         []analyzer.UsageCount{uc("Python", 67), uc("JavaScript", 13), uc("Go", 4)},
		DevOps:            []analyzer.UsageCount{uc("kubectl", 795), uc("az", 158), uc("docker", 75)},
		Editors:           []analyzer.UsageCount{uc("vim", 75)},
		NewPrograms:       []analyzer.UsageCount{uc("antigravity", 23), uc("claude", 18)},
		HasTimes:          true,
		ActiveDays:        176,
		LongestStreak:     19,
		BusiestDay:        time.Date(2026, 4, 16, 0, 0, 0, 0, time.Local),
		BusiestDayCount:   105,
		SudoCommands:      167,
		PipeCommands:      264,
		LongestCommand:    4863,
		LongestProgram:    "ssh",
		Typos:             []analyzer.Typo{{Typed: "claer", Meant: "clear", Count: 2}, {Typed: "gti", Meant: "git", Count: 1}},
		TotalTypos:        3,
		ClearCommands:     1,
	}
	for h := 9; h < 23; h++ {
		s.HourCounts[h] = 50 + h*10
	}
	for m := 0; m < 10; m++ {
		s.MonthCounts[m] = 300 + m*20
	}
	for d := range s.WeekdayCounts {
		s.WeekdayCounts[d] = 500
	}
	sections := []ai.Section{
		{Title: "The Tuesday Afternoon Architect", Description: "48% of your commands land in the afternoon.", Quotes: []string{"kubectl is just cd with more confidence."}},
		{Title: "Typo Archaeologist", Description: "Between 'gti' and 'claer', your muscle memory fights back.", Quotes: []string{"One clear? Ambition."}},
		{Title: "The Pipe Foreman", Description: "A 4863-character command is a manifesto.", Quotes: []string{"Pipes: because separate commands are lazy."}},
		{Title: "Helm and Hustle", Description: "Expect more helm and fewer typos.", Quotes: []string{"Tuesday: bring snacks."}},
	}
	return Recap{Stats: s, AI: sections}
}

func TestParseANSI(t *testing.T) {
	grid := parseANSI("a\x1b[1;38;2;10;20;30mb\x1b[0mc\n\x1b[48;2;1;2;3m \x1b[3mi", foreground)
	if len(grid) != 2 || len(grid[0]) != 3 || len(grid[1]) != 2 {
		t.Fatalf("grid shape = %d rows, want 2: %+v", len(grid), grid)
	}
	if b := grid[0][1]; b.r != 'b' || !b.bold || b.fg != (color.RGBA{10, 20, 30, 255}) {
		t.Errorf("styled cell = %+v", b)
	}
	if c := grid[0][2]; c.bold || c.fg != foreground {
		t.Errorf("reset didn't clear the style: %+v", c)
	}
	if bg := grid[1][0]; !bg.hasBg || bg.bg != (color.RGBA{1, 2, 3, 255}) {
		t.Errorf("background cell = %+v", bg)
	}
	if !grid[1][1].italic || !grid[1][1].hasBg {
		t.Errorf("italic cell = %+v", grid[1][1])
	}
}

func TestShapes(t *testing.T) {
	r := newRenderer(10)
	img := image.NewRGBA(image.Rect(0, 0, 2*r.cellW, r.cellH))
	red := color.RGBA{255, 0, 0, 255}
	r.draw(img, image.Point{}, [][]cell{{{r: '█', fg: red}, {r: '■', fg: red}}})

	if img.RGBAAt(0, 0) != red || img.RGBAAt(r.cellW-1, r.cellH-1) != red {
		t.Error("a full block doesn't fill its cell")
	}
	// The squares of segmented bars leave a gap so they read as segments
	if img.RGBAAt(r.cellW, r.cellH/2) == red || img.RGBAAt(r.cellW+r.cellW/2, r.cellH/2) != red {
		t.Error("a square isn't drawn with a gap at its edge")
	}
}

// Every character the slides, the poster and the frame use must be drawn as
// a shape or have a glyph in Go Mono
func TestEveryCharacterCanBeDrawn(t *testing.T) {
	useShareStyle()
	r := testRecap()
	text := strings.Join(r.slides(render.Anim{Progress: 1}), "\n") + render.RenderPoster(r.Stats, r.persona()) +
		"·>_ #" + Caption(r)
	rend := newRenderer(15)
	for _, row := range parseANSI(text, foreground) {
		for _, c := range row {
			if !rend.canDraw(c.r) {
				t.Errorf("can't draw %q (U+%04X)", c.r, c.r)
			}
		}
	}
}

// compose replays a decoded GIF frame by frame, as a viewer shows it
func compose(t *testing.T, g *gif.GIF) []*image.RGBA {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	var out []*image.RGBA
	for _, frame := range g.Image {
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		snapshot := image.NewRGBA(canvas.Rect)
		copy(snapshot.Pix, canvas.Pix)
		out = append(out, snapshot)
	}
	return out
}

func TestGIFIsLossless(t *testing.T) {
	// Frames with a changing region, an unchanged frame and a full change
	var frames []*image.RGBA
	for i := 0; i < 4; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 40, 30))
		fill(img, img.Rect, background)
		if i != 2 {
			fill(img, image.Rect(5+i*5, 5, 15+i*5, 20), color.RGBA{uint8(60 * i), 200, 90, 255})
		} else {
			copy(img.Pix, frames[1].Pix)
		}
		frames = append(frames, img)
	}
	frames[3] = image.NewRGBA(frames[3].Rect)
	fill(frames[3], frames[3].Rect, color.RGBA{9, 9, 9, 255})

	var buf bytes.Buffer
	if err := encodeGIF(&buf, len(frames), func(i int) *image.RGBA { return frames[i] }, frameDelay); err != nil {
		t.Fatal(err)
	}
	decoded, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for i, got := range compose(t, decoded) {
		want := frames[i]
		for y := 0; y < want.Rect.Dy(); y++ {
			for x := 0; x < want.Rect.Dx(); x++ {
				if g, w := got.RGBAAt(x, y), want.RGBAAt(x, y); g != w {
					t.Fatalf("frame %d pixel (%d,%d) = %v, want %v", i, x, y, g, w)
				}
			}
		}
	}
	// Unchanged regions aren't stored again
	if b := decoded.Image[1].Bounds(); b.Dx() >= 40 {
		t.Errorf("frame 1 stores %v, want only the changed region", b)
	}
}

func TestGIFFitsPlatformLimits(t *testing.T) {
	var buf bytes.Buffer
	start := time.Now()
	if err := WriteGIF(&buf, testRecap()); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	// Decoding drains the buffer, so measure it first
	size := buf.Len()
	g, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatal(err)
	}
	w, h, frames := g.Config.Width, g.Config.Height, len(g.Image)
	// X: at most 1280x1080, 350 frames, 300 million pixels and 5 MB from mobile
	if w > 1280 || h > 1080 {
		t.Errorf("GIF is %dx%d, over 1280x1080", w, h)
	}
	if frames > 350 || w*h*frames > 300_000_000 {
		t.Errorf("GIF has %d frames, %d pixels in total", frames, w*h*frames)
	}
	if size > 5_000_000 {
		t.Errorf("GIF is %d bytes, over 5 MB", size)
	}
	if g.LoopCount != 0 {
		t.Errorf("GIF loops %d times, want forever", g.LoopCount)
	}
	// 13 slides at a readable pace
	if perSlide := frames / 13; perSlide < minSlideFrames {
		t.Errorf("slides stay up for %d frames, want at least %d", perSlide, minSlideFrames)
	}
	t.Logf("%d frames, %.2f MB, %s", frames, float64(size)/1e6, took)
}

func TestPoster(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePoster(&buf, testRecap()); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != posterWidth || b.Dy() != posterHeight {
		t.Errorf("poster is %v, want %dx%d", b, posterWidth, posterHeight)
	}
	// The corners are background: the content is inside the margins
	if c := color.RGBAModel.Convert(img.At(2, 2)).(color.RGBA); c != background {
		t.Errorf("poster corner is %v, want the background", c)
	}
}

func TestPosterFitsWithoutOptionalData(t *testing.T) {
	r := testRecap()
	r.Stats.Typos, r.Stats.TotalTypos, r.Stats.NewPrograms, r.Stats.HasTimes = nil, 0, nil, false
	r.AI = nil
	for _, line := range strings.Split(render.RenderPoster(r.Stats, ""), "\n") {
		if w := len([]rune(stripANSI(line))); w > render.PosterCols {
			t.Errorf("poster line is %d columns, over %d: %q", w, render.PosterCols, stripANSI(line))
		}
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for _, row := range parseANSI(s, foreground) {
		for _, c := range row {
			b.WriteRune(c.r)
		}
	}
	return b.String()
}

func TestCaption(t *testing.T) {
	r := testRecap()
	caption := Caption(r)
	for _, want := range []string{"My 2026 in the terminal", "3,698 commands", "kubectl was my #1", "The Tuesday Afternoon Architect", "#shellrecap"} {
		if !strings.Contains(caption, want) {
			t.Errorf("caption %q is missing %q", caption, want)
		}
	}
	if strings.Contains(caption, "a The") {
		t.Errorf("caption has a doubled article: %q", caption)
	}
	// Bluesky's 300 characters include the link
	if n := len(caption) + 1 + len(RepoURL); n > 300 {
		t.Errorf("caption with the link is %d characters, over 300", n)
	}

	r.AI = nil
	if caption := Caption(r); !strings.Contains(caption, "Persona: ") {
		t.Errorf("caption without AI slides has no persona: %q", caption)
	}
}

func TestLinks(t *testing.T) {
	links := Links("My year & more #shellrecap")
	byName := map[string]Link{}
	for _, l := range links {
		byName[l.Name] = l
	}
	x := byName["X"]
	if !strings.Contains(x.URL, "text=My%20year%20%26%20more%20%23shellrecap") || !strings.Contains(x.URL, "&url=https%3A%2F%2Fgithub.com") {
		t.Errorf("X link = %s", x.URL)
	}
	if !x.Prefilled || !x.Animated {
		t.Errorf("X link = %+v, want prefilled and animated", x)
	}
	if b := byName["Bluesky"]; !b.Prefilled || b.Animated || !strings.Contains(b.URL, "bsky.app/intent/compose?text=") {
		t.Errorf("Bluesky link = %+v", b)
	}
	if l := byName["LinkedIn"]; l.Prefilled {
		t.Error("LinkedIn can't prefill text, so its caption has to be copied")
	}
}

func TestExport(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "shellrecap")
	files, err := Export(testRecap(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if files.GIF != filepath.Join(dir, "shellrecap-2026.gif") || files.Poster != filepath.Join(dir, "shellrecap-2026.png") {
		t.Errorf("files = %+v", files)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("export left %d files, want just the GIF and the poster", len(entries))
	}
	if info, err := os.Stat(files.GIF); err != nil || info.Size() != files.GIFBytes || info.Mode().Perm() != 0o644 {
		t.Errorf("GIF = %v, %v; want %d bytes, readable by other apps", info, err, files.GIFBytes)
	}
}

func TestDefaultDir(t *testing.T) {
	t.Setenv("SHELLRECAP_SHARE_DIR", "/tmp/somewhere")
	if got := DefaultDir(); got != "/tmp/somewhere" {
		t.Errorf("DefaultDir = %q, want SHELLRECAP_SHARE_DIR", got)
	}
	t.Setenv("SHELLRECAP_SHARE_DIR", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := DefaultDir(); got != filepath.Join(home, "shellrecap") {
		t.Errorf("DefaultDir without Pictures = %q", got)
	}
	os.Mkdir(filepath.Join(home, "Pictures"), 0o755)
	if got := DefaultDir(); got != filepath.Join(home, "Pictures", "shellrecap") {
		t.Errorf("DefaultDir with Pictures = %q", got)
	}
}

func TestSlideTiming(t *testing.T) {
	for _, slides := range []int{1, 9, 13, 30} {
		perSlide, reveal := slideTiming(slides)
		frames := perSlide * slides
		if perSlide < minSlideFrames || perSlide > maxSlideFrames || reveal < 1 || reveal >= perSlide {
			t.Errorf("%d slides: %d frames each, %d to reveal", slides, perSlide, reveal)
		}
		// Within the budget unless that would make slides unreadable
		if frames > maxGIFFrames && perSlide > minSlideFrames {
			t.Errorf("%d slides: %d frames, over the budget", slides, frames)
		}
	}
}

package share

import (
	"bytes"
	"fmt"
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
	"github.com/ksauraj/shellrecap/internal/config"
	"github.com/ksauraj/shellrecap/internal/render"
	"github.com/ksauraj/shellrecap/internal/theme"
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
	return Recap{Data: testData(), Stats: s, AI: sections}
}

// testData is a small analyzed history, for the tab pictures
func testData() analyzer.ShellData {
	data := analyzer.InitShellData()
	start := time.Date(2026, 3, 2, 9, 0, 0, 0, time.Local)
	commands := []string{"git commit -m x", "git push", "kubectl get pods", "vim main.go", "go build ./...",
		"docker ps", "az login", "make test", "python3 app.py", "npm install", "curl -s example.com | jq ."}
	for i := 0; i < 220; i++ {
		cmd := commands[i%len(commands)]
		data.Histories["fish"] = append(data.Histories["fish"], analyzer.CommandEntry{
			Command:   cmd,
			Program:   analyzer.ProgramName(cmd),
			Timestamp: start.Add(time.Duration(i) * 97 * time.Minute),
		})
	}
	return analyzer.Analyze(data)
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
	text := strings.Join(r.slides(slideCols, render.Anim{Progress: 1}), "\n") + render.RenderPoster(r.Stats, r.persona()) +
		r.appHeader(0, 80) + credit() + Caption(r)
	for _, tab := range tabs {
		text += tab.render(r, 80, render.Anim{Progress: 1})
	}
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
	if err := encodeGIF(&buf, len(frames), func(i int) (*image.RGBA, int) { return frames[i], frameDelay }); err != nil {
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

// checkGIF decodes a GIF and checks it fits X: at most 1280x1080, 350
// frames, 300 million pixels in total and 5 MB from mobile
func checkGIF(t *testing.T, name string, data []byte, want size) *gif.GIF {
	t.Helper()
	size := len(data)
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	w, h, frames := g.Config.Width, g.Config.Height, len(g.Image)
	if w != want.w || h != want.h {
		t.Errorf("%s is %dx%d, want %dx%d", name, w, h, want.w, want.h)
	}
	if w > 1280 || h > 1080 {
		t.Errorf("%s is %dx%d, over 1280x1080", name, w, h)
	}
	if frames > 350 || w*h*frames > 300_000_000 {
		t.Errorf("%s has %d frames, %d pixels in total", name, frames, w*h*frames)
	}
	if size > 5_000_000 {
		t.Errorf("%s is %d bytes, over 5 MB", name, size)
	}
	if g.LoopCount != 0 {
		t.Errorf("%s loops %d times, want forever", name, g.LoopCount)
	}
	t.Logf("%s: %dx%d, %d frames, %.2f MB", name, w, h, frames, float64(size)/1e6)
	return g
}

func TestRecapGIF(t *testing.T) {
	for _, gifSize := range config.GIFSizes {
		var buf bytes.Buffer
		if err := WriteRecapGIF(&buf, testRecap(), gifSize); err != nil {
			t.Fatal(err)
		}
		g := checkGIF(t, "recap GIF "+gifSize, buf.Bytes(), recapGIFSizes[gifSize])
		// 13 slides at a readable pace
		if perSlide := len(g.Image) / 13; perSlide < minSlideFrames {
			t.Errorf("slides stay up for %d frames, want at least %d", perSlide, minSlideFrames)
		}
	}
}

func TestTourGIF(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTourGIF(&buf, testRecap(), config.GIFStandard); err != nil {
		t.Fatal(err)
	}
	g := checkGIF(t, "tour GIF", buf.Bytes(), tourGIFSizes[config.GIFStandard])

	// Each of the 4 tabs and 13 slides animates in, then holds on one frame
	views := len(tabs) + 13
	if len(g.Image) != views*(revealFrames+1) {
		t.Errorf("tour has %d frames, want %d", len(g.Image), views*(revealFrames+1))
	}
	total := 0
	for _, d := range g.Delay {
		total += d
	}
	if seconds := float64(total) / 100; seconds < 30 || seconds > 60 {
		t.Errorf("tour lasts %.1fs, want a comfortable read", seconds)
	}
	if hold := g.Delay[revealFrames]; hold != tabHold {
		t.Errorf("first tab holds for %d, want %d", hold, tabHold)
	}
}

func TestPictureShapes(t *testing.T) {
	r := testRecap()
	for _, shape := range config.Shapes {
		var poster, tab, slide bytes.Buffer
		if err := WritePoster(&poster, r, shape); err != nil {
			t.Fatal(err)
		}
		if err := WriteTabPicture(&tab, r, 0, shape); err != nil {
			t.Fatal(err)
		}
		if err := WriteSlidePicture(&slide, r, 1, shape); err != nil {
			t.Fatal(err)
		}
		for name, buf := range map[string]*bytes.Buffer{"poster": &poster, "tab": &tab, "slide": &slide} {
			img, err := png.Decode(buf)
			if err != nil {
				t.Fatalf("%s %s: %v", shape, name, err)
			}
			want := shapes[shape]
			if b := img.Bounds(); b.Dx() != want.w || b.Dy() != want.h {
				t.Errorf("%s %s is %v, want %dx%d", shape, name, b, want.w, want.h)
			}
			// The corners are background: the content is inside the margins
			if c := color.RGBAModel.Convert(img.At(2, 2)).(color.RGBA); c != background {
				t.Errorf("%s %s corner is %v, want the background", shape, name, c)
			}
		}
	}
}

func TestImageThemes(t *testing.T) {
	defer theme.Use(theme.Dark)
	for _, name := range config.ImageThemes {
		dir := t.TempDir()
		opts := config.Defaults().Share
		opts.Outputs, opts.ImageTheme = []string{config.Poster}, name
		files, err := Export(testRecap(), dir, opts)
		if err != nil {
			t.Fatal(err)
		}
		f, _ := os.Open(files.Outputs[0].Path)
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		want := hexColor(theme.Named(name).Base)
		if c := color.RGBAModel.Convert(img.At(2, 2)).(color.RGBA); c != want {
			t.Errorf("%s poster background is %v, want %v", name, c, want)
		}
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
	opts := config.Defaults().Share
	opts.Outputs, opts.GIFSize = config.Outputs, config.GIFSmall
	files, err := Export(testRecap(), dir, opts)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"shellrecap-2026-recap.gif", "shellrecap-2026-tour.gif", "shellrecap-2026-poster.png",
		"shellrecap-2026-overview.png", "shellrecap-2026-tech-profile.png", "shellrecap-2026-work-patterns.png",
		"shellrecap-2026-tool-usage.png"}
	for i := 1; i <= 13; i++ {
		want = append(want, fmt.Sprintf("shellrecap-2026-slide-%02d.png", i))
	}
	if len(files.Outputs) != len(want) {
		t.Fatalf("exported %d files, want %d", len(files.Outputs), len(want))
	}
	for i, o := range files.Outputs {
		if filepath.Base(o.Path) != want[i] {
			t.Errorf("output %d is %s, want %s", i, filepath.Base(o.Path), want[i])
		}
		if info, err := os.Stat(o.Path); err != nil || info.Size() != o.Bytes || info.Mode().Perm() != 0o644 {
			t.Errorf("%s = %v, %v; want %d bytes, readable by other apps", o.Path, info, err, o.Bytes)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != len(want) {
		t.Errorf("export left %d files, want just the images", len(entries))
	}
	if gifOut, ok := files.First(config.RecapGIF); !ok || gifOut.Kind != config.RecapGIF {
		t.Errorf("First(recap) = %+v, %v", gifOut, ok)
	}
	if files.Count(config.Tabs) != 4 {
		t.Errorf("%d tab pictures, want 4", files.Count(config.Tabs))
	}
}

func TestExportOnlyWhatsAsked(t *testing.T) {
	opts := config.Defaults().Share
	opts.Outputs = []string{config.Poster}
	files, err := Export(testRecap(), t.TempDir(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(files.Outputs) != 1 || files.Outputs[0].Kind != config.Poster {
		t.Errorf("outputs = %+v, want just the poster", files.Outputs)
	}
}

func TestValidate(t *testing.T) {
	good := config.Defaults().Share
	if err := validate(good); err != nil {
		t.Errorf("defaults are invalid: %v", err)
	}
	for _, bad := range []func(*config.Share){
		func(s *config.Share) { s.Outputs = []string{"video"} },
		func(s *config.Share) { s.GIFSize = "huge" },
		func(s *config.Share) { s.Shape = "circle" },
		func(s *config.Share) { s.ImageTheme = "terminal" },
	} {
		opts := config.Defaults().Share
		bad(&opts)
		if validate(opts) == nil {
			t.Errorf("accepted %+v", opts)
		}
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
		perSlide, reveal := slideTiming(slides, recapGIFSizes[config.GIFStandard])
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

// Shared pictures go public, so they must never show alias definitions,
// which can hold hostnames, paths or tokens
func TestSharedOverviewHidesAliases(t *testing.T) {
	r := testRecap()
	r.Data.ShellConfigs["fish"] = analyzer.ShellConfig{
		Aliases: map[string]string{"deploy": "ssh admin@prod-db.internal --token=s3cret"},
	}
	text := stripANSI(tabs[0].render(r, 80, render.Anim{Progress: 1}))
	if strings.Contains(text, "prod-db") || strings.Contains(text, "s3cret") || strings.Contains(text, "Shell Configuration") {
		t.Errorf("shared overview shows the shell configuration:\n%s", text)
	}
	if !strings.Contains(text, "Languages") {
		t.Errorf("shared overview is missing the languages card:\n%s", text)
	}
}

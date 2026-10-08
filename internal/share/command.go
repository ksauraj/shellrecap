// internal/share/command.go
package share

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/cache"
	"github.com/ksauraj/shellrecap/internal/config"
	"github.com/ksauraj/shellrecap/internal/theme"
)

// Output is one image Export wrote
type Output struct {
	Kind  string `json:"kind"` // one of the config output kinds
	Name  string `json:"name"` // e.g. "Overview" for a tab picture
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// Files describes the images Export wrote
type Files struct {
	Dir     string   `json:"dir"`
	Outputs []Output `json:"outputs"`
	Caption string   `json:"caption"`
	// Copied is set when a picture was put on the clipboard
	Copied bool `json:"copied"`
}

// First returns the first output of the first kind there is one of
func (f Files) First(kinds ...string) (Output, bool) {
	for _, kind := range kinds {
		for _, o := range f.Outputs {
			if o.Kind == kind {
				return o, true
			}
		}
	}
	return Output{}, false
}

// Count is how many outputs there are of a kind
func (f Files) Count(kind string) int {
	n := 0
	for _, o := range f.Outputs {
		if o.Kind == kind {
			n++
		}
	}
	return n
}

// DefaultDir is SHELLRECAP_SHARE_DIR if set, otherwise
// ~/Pictures/shellrecap, or ~/shellrecap on systems without a Pictures folder
func DefaultDir() string {
	if dir := os.Getenv("SHELLRECAP_SHARE_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	if info, err := os.Stat(filepath.Join(home, "Pictures")); err == nil && info.IsDir() {
		return filepath.Join(home, "Pictures", "shellrecap")
	}
	return filepath.Join(home, "shellrecap")
}

// job writes one output
type job struct {
	output Output
	write  func(io.Writer) error
}

// Export writes the images opts asks for into dir. The GIFs take the
// longest, so they're written alongside the pictures.
func Export(r Recap, dir string, opts config.Share) (Files, error) {
	theme.Use(theme.Named(opts.ImageTheme))
	useShareStyle()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Files{}, err
	}
	prefix := fmt.Sprintf("shellrecap-%d", r.Stats.Year)
	if r.Stats.AllTime {
		prefix = "shellrecap"
	}
	file := func(name string) string { return filepath.Join(dir, prefix+"-"+name) }

	var gifs, pictures []job
	if opts.Has(config.RecapGIF) {
		gifs = append(gifs, job{Output{Kind: config.RecapGIF, Name: "Recap", Path: file("recap.gif")},
			func(w io.Writer) error { return WriteRecapGIF(w, r, opts.GIFSize) }})
	}
	if opts.Has(config.TourGIF) {
		gifs = append(gifs, job{Output{Kind: config.TourGIF, Name: "Tour", Path: file("tour.gif")},
			func(w io.Writer) error { return WriteTourGIF(w, r, opts.GIFSize) }})
	}
	if opts.Has(config.Poster) {
		pictures = append(pictures, job{Output{Kind: config.Poster, Name: "Poster", Path: file("poster.png")},
			func(w io.Writer) error { return WritePoster(w, r, opts.Shape) }})
	}
	if opts.Has(config.Tabs) {
		for i, tab := range tabs {
			i := i
			pictures = append(pictures, job{Output{Kind: config.Tabs, Name: tab.name, Path: file(tab.file + ".png")},
				func(w io.Writer) error { return WriteTabPicture(w, r, i, opts.Shape) }})
		}
	}
	if opts.Has(config.Slides) {
		for i := 0; i < r.SlideCount(); i++ {
			i := i
			pictures = append(pictures, job{Output{Kind: config.Slides, Name: fmt.Sprintf("Slide %d", i+1), Path: file(fmt.Sprintf("slide-%02d.png", i+1))},
				func(w io.Writer) error { return WriteSlidePicture(w, r, i, opts.Shape) }})
		}
	}

	// Each GIF in its own goroutine, and the pictures in one more
	batches := [][]job{pictures}
	for _, g := range gifs {
		batches = append(batches, []job{g})
	}
	errs := make([]error, len(batches))
	var wg sync.WaitGroup
	for b, batch := range batches {
		b, batch := b, batch
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, j := range batch {
				if err := writeFile(j.output.Path, j.write); err != nil {
					errs[b] = fmt.Errorf("writing %s: %w", filepath.Base(j.output.Path), err)
					return
				}
			}
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return Files{}, err
		}
	}

	files := Files{Dir: dir, Caption: Caption(r)}
	for _, j := range append(gifs, pictures...) {
		if info, err := os.Stat(j.output.Path); err == nil {
			j.output.Bytes = info.Size()
		}
		files.Outputs = append(files.Outputs, j.output)
	}
	return files, nil
}

// writeFile writes through a temporary file, so a failure never leaves
// half an image behind
func writeFile(path string, write func(io.Writer) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".shellrecap-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	buf := bufio.NewWriter(tmp)
	if err := write(buf); err != nil {
		tmp.Close()
		return err
	}
	if err := buf.Flush(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// aiSlides finds the AI slides to share: cached ones if there are any, or
// fresh ones unless cachedOnly is set. Sharing goes ahead without them if
// the AI isn't available.
func aiSlides(stats analyzer.WrappedStats, opts ai.Options, cachedOnly bool, status io.Writer) []ai.Section {
	targets := ai.Targets(opts)
	summary := stats.Summary()
	for _, target := range targets {
		if entry, ok := cache.LoadWrapped(stats.Year, target); ok && entry.FreshFor(summary, stats.TotalCommands, time.Now()) {
			return entry.Sections
		}
	}
	// Older slides still beat none
	for _, target := range targets {
		if entry, ok := cache.LoadWrapped(stats.Year, target); ok {
			return entry.Sections
		}
	}
	if cachedOnly || !ai.AnyAvailable(opts) {
		return nil
	}

	fmt.Fprintln(status, "Asking the AI for a few extra slides...")
	result, err := ai.Generate(summary, opts)
	if err != nil {
		fmt.Fprintln(status, "The AI slides are unavailable right now, so sharing without them.")
		return nil
	}
	// Save them, so the app shows the same slides
	_ = cache.SaveWrapped(cache.WrappedEntry{
		Year:      stats.Year,
		Provider:  result.Target.Provider,
		Model:     result.Target.Model,
		Summary:   summary,
		Commands:  stats.TotalCommands,
		CreatedAt: time.Now(),
		Duration:  result.Duration.Seconds(),
		Sections:  result.Sections,
	})
	return result.Sections
}

// Command runs `shellrecap share` and returns its exit code. Its defaults
// are the choices saved from the app's customize panel.
func Command(args []string) int {
	saved := config.Load().Share
	flags := flag.NewFlagSet("share", flag.ContinueOnError)
	out := flags.String("out", DefaultDir(), "folder to save the images in")
	only := flags.String("only", strings.Join(saved.Outputs, ","),
		"images to create, any of: "+strings.Join(config.Outputs, ", "))
	gifSize := flags.String("gif-size", saved.GIFSize, "GIF size: "+strings.Join(config.GIFSizes, " or "))
	shape := flags.String("shape", saved.Shape, "picture shape: "+strings.Join(config.Shapes, ", "))
	imageTheme := flags.String("theme", saved.ImageTheme, "image theme: "+strings.Join(config.ImageThemes, ", "))
	noAI := flags.Bool("no-ai", saved.NoAI, "leave out the AI-written slides")
	cachedAI := flags.Bool("cached-ai", false, "only use AI slides that are already cached, never ask the AI")
	provider := flags.String("provider", ai.Auto, "AI provider for new AI slides: auto, gemini or groq")
	model := flags.String("model", "", "model to use with --provider")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: shellrecap share [flags]\n\n"+
			"Saves your Recap as animated GIFs and pictures, ready to post.\n\nFlags:\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}

	opts := config.Share{GIFSize: *gifSize, Shape: *shape, ImageTheme: *imageTheme, NoAI: *noAI}
	for _, o := range strings.Split(*only, ",") {
		opts.Outputs = append(opts.Outputs, strings.TrimSpace(o))
	}
	if err := validate(opts); err != nil {
		fmt.Fprintln(os.Stderr, "shellrecap share:", err)
		return 2
	}
	aiOpts := ai.Options{Provider: *provider, Model: *model}
	if err := ai.ValidateOptions(aiOpts); err != nil {
		fmt.Fprintln(os.Stderr, "shellrecap share:", err)
		return 2
	}

	status := io.Writer(os.Stdout)
	if *asJSON {
		status = io.Discard
	}
	fmt.Fprintln(status, "Creating your share images...")
	data := analyzer.AnalyzeShells().(analyzer.ShellData)
	r := Recap{Data: data, Stats: analyzer.ComputeWrapped(data, analyzer.RecapYear(time.Now()))}
	if r.Stats.TotalCommands == 0 {
		fmt.Fprintln(os.Stderr, "shellrecap share: there's no shell history to share yet")
		return 1
	}
	if !opts.NoAI {
		r.AI = aiSlides(r.Stats, aiOpts, *cachedAI, status)
	}

	files, err := Export(r, *out, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shellrecap share:", err)
		return 1
	}
	if pic, ok := files.First(config.Poster, config.Tabs, config.Slides); ok && Desktop() && CopyImage(pic.Path) == nil {
		files.Copied = true
	}

	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(files)
		return 0
	}
	printFiles(os.Stdout, files)
	return 0
}

// validate checks share options given on the command line
func validate(opts config.Share) error {
	valid := func(v string, choices []string) bool {
		for _, c := range choices {
			if v == c {
				return true
			}
		}
		return false
	}
	for _, o := range opts.Outputs {
		if !valid(o, config.Outputs) {
			return fmt.Errorf("unknown image %q (use %s)", o, strings.Join(config.Outputs, ", "))
		}
	}
	switch {
	case !valid(opts.GIFSize, config.GIFSizes):
		return fmt.Errorf("unknown GIF size %q (use %s)", opts.GIFSize, strings.Join(config.GIFSizes, " or "))
	case !valid(opts.Shape, config.Shapes):
		return fmt.Errorf("unknown shape %q (use %s)", opts.Shape, strings.Join(config.Shapes, ", "))
	case !valid(opts.ImageTheme, config.ImageThemes):
		return fmt.Errorf("unknown theme %q (use %s)", opts.ImageTheme, strings.Join(config.ImageThemes, ", "))
	}
	return nil
}

// HomePath shortens paths in the home folder to start with ~
func HomePath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}

// Describe says what an output is and where it's best posted
func Describe(o Output) string {
	switch o.Kind {
	case config.RecapGIF:
		return fmt.Sprintf("animated Recap · %.1f MB · for X, Discord, Reddit", float64(o.Bytes)/1e6)
	case config.TourGIF:
		return fmt.Sprintf("animated tour of the app · %.1f MB", float64(o.Bytes)/1e6)
	case config.Poster:
		return "summary poster · for Instagram, LinkedIn, Bluesky"
	case config.Tabs:
		return "the " + o.Name + " tab"
	}
	return o.Name
}

func printFiles(w io.Writer, files Files) {
	fmt.Fprintf(w, "\nSaved %d images to %s\n", len(files.Outputs), HomePath(files.Dir))
	for _, o := range files.Outputs {
		fmt.Fprintf(w, "  %-34s %s\n", filepath.Base(o.Path), Describe(o))
	}
	if files.Copied {
		pic, _ := files.First(config.Poster, config.Tabs, config.Slides)
		fmt.Fprintf(w, "%s is on your clipboard, ready to paste with %s.\n", filepath.Base(pic.Path), PasteShortcut())
	}
	fmt.Fprintf(w, "\nCaption:\n  %s\n\nPost it:\n", files.Caption)
	for _, link := range Links(files.Caption) {
		note := ""
		if !link.Prefilled {
			note = "  (paste the caption)"
		}
		fmt.Fprintf(w, "  %-9s %s%s\n", link.Name, link.URL, note)
	}
}

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
	"time"

	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/cache"
)

// Files describes the images Export wrote
type Files struct {
	Dir      string `json:"dir"`
	GIF      string `json:"gif"`
	Poster   string `json:"poster"`
	GIFBytes int64  `json:"gif_bytes"`
	Caption  string `json:"caption"`
	// Copied is set when the poster was put on the clipboard
	Copied bool `json:"copied"`
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

// Export writes the animated GIF and the poster for r into dir
func Export(r Recap, dir string) (Files, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Files{}, err
	}
	name := fmt.Sprintf("shellrecap-%d", r.Stats.Year)
	if r.Stats.AllTime {
		name = "shellrecap"
	}
	files := Files{
		Dir:     dir,
		GIF:     filepath.Join(dir, name+".gif"),
		Poster:  filepath.Join(dir, name+".png"),
		Caption: Caption(r),
	}
	if err := writeFile(files.Poster, func(w io.Writer) error { return WritePoster(w, r) }); err != nil {
		return Files{}, fmt.Errorf("writing the poster: %w", err)
	}
	if err := writeFile(files.GIF, func(w io.Writer) error { return WriteGIF(w, r) }); err != nil {
		return Files{}, fmt.Errorf("writing the GIF: %w", err)
	}
	if info, err := os.Stat(files.GIF); err == nil {
		files.GIFBytes = info.Size()
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

// Command runs `shellrecap share` and returns its exit code
func Command(args []string) int {
	flags := flag.NewFlagSet("share", flag.ContinueOnError)
	out := flags.String("out", DefaultDir(), "folder to save the images in")
	noAI := flags.Bool("no-ai", false, "leave out the AI-written slides")
	cachedAI := flags.Bool("cached-ai", false, "only use AI slides that are already cached, never ask the AI")
	provider := flags.String("provider", ai.Auto, "AI provider for new AI slides: auto, gemini or groq")
	model := flags.String("model", "", "model to use with --provider")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: shellrecap share [flags]\n\n"+
			"Saves your Recap as an animated GIF and a summary poster, ready to post.\n\nFlags:\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	opts := ai.Options{Provider: *provider, Model: *model}
	if err := ai.ValidateOptions(opts); err != nil {
		fmt.Fprintln(os.Stderr, "shellrecap share:", err)
		return 2
	}

	status := io.Writer(os.Stdout)
	if *asJSON {
		status = io.Discard
	}
	fmt.Fprintln(status, "Creating your share images...")
	data := analyzer.AnalyzeShells().(analyzer.ShellData)
	r := Recap{Stats: analyzer.ComputeWrapped(data, analyzer.RecapYear(time.Now()))}
	if r.Stats.TotalCommands == 0 {
		fmt.Fprintln(os.Stderr, "shellrecap share: there's no shell history to share yet")
		return 1
	}
	if !*noAI {
		r.AI = aiSlides(r.Stats, opts, *cachedAI, status)
	}

	files, err := Export(r, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shellrecap share:", err)
		return 1
	}
	if Desktop() && CopyImage(files.Poster) == nil {
		files.Copied = true
	}

	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(files)
		return 0
	}
	printFiles(os.Stdout, files)
	return 0
}

// HomePath shortens paths in the home folder to start with ~
func HomePath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}

func printFiles(w io.Writer, files Files) {
	fmt.Fprintf(w, "\nSaved your recap to %s\n", HomePath(files.Dir))
	fmt.Fprintf(w, "  %-22s animated, %.1f MB   for X, Discord, Reddit and Slack\n",
		filepath.Base(files.GIF), float64(files.GIFBytes)/1e6)
	fmt.Fprintf(w, "  %-22s poster              for Instagram, LinkedIn and Bluesky\n", filepath.Base(files.Poster))
	if files.Copied {
		fmt.Fprintf(w, "The poster is on your clipboard, ready to paste with %s.\n", PasteShortcut())
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

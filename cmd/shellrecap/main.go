// cmd/shellrecap/main.go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/models"
	"github.com/ksauraj/shellrecap/internal/share"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "share" {
		os.Exit(share.Command(os.Args[2:]))
	}

	var cfg models.Config
	flag.StringVar(&cfg.AI.Provider, "provider", ai.Auto,
		"AI provider for the Recap slides: auto (Gemini, falling back to Groq), gemini or groq")
	flag.StringVar(&cfg.AI.Model, "model", "",
		"model to use with --provider, e.g. gemini-3.8-flash or openai/gpt-oss-20b")
	flag.BoolVar(&cfg.NoCache, "no-cache", false, "always ask the AI for fresh slides instead of using cached ones")
	version := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: shellrecap [flags]\n"+
			"       shellrecap share [flags]   save your Recap as a GIF and a poster to post\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nEnvironment:\n"+
			"  GEMINI_API_KEY, GROQ_API_KEY   API keys for the AI slides\n"+
			"  GEMINI_MODEL, GROQ_MODEL       default models when --model isn't given\n"+
			"  SHELLRECAP_CACHE_DIR           where AI slides are cached\n"+
			"  SHELLRECAP_SHARE_DIR           where share images are saved\n")
	}
	flag.Parse()

	if *version {
		fmt.Println("shellrecap", models.Version)
		return
	}
	if err := ai.ValidateOptions(cfg.AI); err != nil {
		fmt.Fprintln(os.Stderr, "shellrecap:", err)
		os.Exit(2)
	}

	// Ask the terminal for its background color now, before the TUI takes
	// over the input, so the theme can pick its light or dark palette
	lipgloss.HasDarkBackground()

	p := tea.NewProgram(models.InitialModel(cfg),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

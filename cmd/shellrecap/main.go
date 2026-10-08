// cmd/shellrecap/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/config"
	"github.com/ksauraj/shellrecap/internal/models"
	"github.com/ksauraj/shellrecap/internal/share"
	"github.com/ksauraj/shellrecap/internal/theme"
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
	cfg.Settings = config.Load()
	themeFlag := flag.String("theme", "", "color theme: auto (matches your terminal), dark, black, light or terminal")
	version := flag.Bool("version", false, "print the version and exit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: shellrecap [flags]\n"+
			"       shellrecap share [flags]   save your recap as GIFs and pictures to post\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nEnvironment:\n"+
			"  GEMINI_API_KEY, GROQ_API_KEY   API keys for the AI slides\n"+
			"  GEMINI_MODEL, GROQ_MODEL       default models when --model isn't given\n"+
			"  SHELLRECAP_THEME               color theme, like --theme\n"+
			"  SHELLRECAP_BACKGROUND          your terminal's background, like #300a24, for the auto theme\n"+
			"  SHELLRECAP_CACHE_DIR           where AI slides are cached\n"+
			"  SHELLRECAP_SHARE_DIR           where share images are saved\n"+
			"  SHELLRECAP_CONFIG_DIR          where your theme and share choices are saved\n")
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

	// The theme: --theme, then SHELLRECAP_THEME, then the remembered one
	for _, choice := range []string{*themeFlag, os.Getenv("SHELLRECAP_THEME")} {
		if choice == "" {
			continue
		}
		if !validTheme(choice) {
			fmt.Fprintf(os.Stderr, "shellrecap: unknown theme %q (use %s)\n", choice, strings.Join(theme.Names, ", "))
			os.Exit(2)
		}
		cfg.Settings.Theme = choice
		break
	}
	// Ask the terminal for its background color now, before the TUI takes
	// over the input, so the auto theme can match it
	theme.Detect()
	theme.Use(theme.Named(cfg.Settings.Theme))

	p := tea.NewProgram(models.InitialModel(cfg),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())

	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func validTheme(name string) bool {
	for _, t := range theme.Names {
		if t == name {
			return true
		}
	}
	return false
}

// internal/ai/ai.go

// Package ai writes the AI slides of the Recap. It tries Gemini first and
// falls back to Groq, unless a provider is chosen explicitly.
package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Section is one AI-written slide
type Section struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Quotes      []string `json:"quotes,omitempty"`
}

// Provider names, as accepted by the --provider flag
const (
	Auto   = "auto"
	Gemini = "gemini"
	Groq   = "groq"
)

// Options choose the provider and model, normally from command line flags
type Options struct {
	Provider string // Auto (Gemini, then Groq), Gemini or Groq
	Model    string // overrides the provider's default model
}

// Target is a provider and model to ask for slides
type Target struct {
	Provider string
	Model    string
}

func (t Target) String() string {
	return fmt.Sprintf("%s (%s)", t.Name(), t.Model)
}

// Name is the provider's display name
func (t Target) Name() string {
	return displayNames[t.Provider]
}

var displayNames = map[string]string{Gemini: "Gemini", Groq: "Groq"}

type provider interface {
	// key returns the API key, or "" when none is configured
	key() string
	defaultModel() string
	generate(model, system, user string) (string, error)
}

var providers = map[string]provider{
	Gemini: geminiProvider{},
	Groq:   groqProvider{},
}

// ErrNoAPIKey is returned when none of the providers to try has an API key
var ErrNoAPIKey = errors.New("no AI provider API key configured")

// requestTimeout bounds each provider, so a hanging one doesn't hold up
// the fallback for long
const requestTimeout = 40 * time.Second

// Targets lists the providers and models to try, in order
func Targets(opts Options) []Target {
	names := []string{Gemini, Groq}
	if opts.Provider != "" && opts.Provider != Auto {
		names = []string{opts.Provider}
	}
	targets := make([]Target, 0, len(names))
	for _, name := range names {
		model := opts.Model
		if model == "" {
			model = os.Getenv(strings.ToUpper(name) + "_MODEL")
		}
		if model == "" {
			model = providers[name].defaultModel()
		}
		targets = append(targets, Target{Provider: name, Model: model})
	}
	return targets
}

// Available reports whether t's provider has an API key
func Available(t Target) bool {
	return providers[t.Provider].key() != ""
}

// AnyAvailable reports whether any provider in opts has an API key
func AnyAvailable(opts Options) bool {
	for _, t := range Targets(opts) {
		if Available(t) {
			return true
		}
	}
	return false
}

// KeyHint tells the user which environment variable unlocks AI slides
func KeyHint(opts Options) string {
	switch opts.Provider {
	case Gemini:
		return "Set GEMINI_API_KEY to unlock AI-written slides."
	case Groq:
		return "Set GROQ_API_KEY to unlock AI-written slides."
	}
	return "Set GEMINI_API_KEY or GROQ_API_KEY to unlock AI-written slides."
}

// ValidateOptions checks options given on the command line
func ValidateOptions(opts Options) error {
	switch opts.Provider {
	case "", Auto, Gemini, Groq:
	default:
		return fmt.Errorf("unknown provider %q (use auto, gemini or groq)", opts.Provider)
	}
	if opts.Model != "" && (opts.Provider == "" || opts.Provider == Auto) {
		return errors.New("--model needs --provider gemini or --provider groq")
	}
	return nil
}

// Result is a successful generation
type Result struct {
	Target   Target
	Sections []Section
	Duration time.Duration
	// Failures holds the errors of providers that were tried first
	Failures []error
}

// Generate asks each available provider in turn for slides about the
// summarized stats and returns the first success
func Generate(summary string, opts Options) (Result, error) {
	var failures []error
	tried := false
	for _, target := range Targets(opts) {
		if !Available(target) {
			continue
		}
		tried = true
		start := time.Now()
		text, err := providers[target.Provider].generate(target.Model, systemPrompt, "Here are my shell stats:\n\n"+summary)
		var sections []Section
		if err == nil {
			sections, err = parseSections(text)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", displayNames[target.Provider], err))
			continue
		}
		return Result{Target: target, Sections: sections, Duration: time.Since(start), Failures: failures}, nil
	}
	if !tried {
		return Result{}, ErrNoAPIKey
	}
	msgs := make([]string, len(failures))
	for i, f := range failures {
		msgs[i] = f.Error()
	}
	return Result{}, errors.New(strings.Join(msgs, "; "))
}

const systemPrompt = `You write the AI slides of shellrecap, a Spotify-Wrapped-style recap of a developer's year in the terminal. The slides are shown in a terminal UI, so keep them short and punchy.

Write exactly 4 slides, in this order:
1. Persona: invent a creative terminal persona title for them (2-4 words) and justify it with their stats.
2. Roast: a good-natured roast of their habits (typos, clear, sudo, late nights, favourite commands).
3. Superpower: what they are clearly great at, and the evidence for it.
4. Forecast: playful predictions for their next year in the terminal.

Rules:
- Only use numbers that appear in the stats. Never invent numbers, dates or tools.
- Speak directly to the user as "you".
- title: at most 40 characters.
- description: 1-3 sentences, at most 280 characters.
- quotes: 1-2 witty one-liners per slide, each at most 90 characters, not attributed to anyone.
- Plain ASCII text only: no emoji, no markdown, no asterisks, no hashtags.

Respond with JSON only, in this shape:
{"sections": [{"title": "...", "description": "...", "quotes": ["..."]}]}`

// slidesSchema is the JSON schema of the response, in the strict form that
// Groq's structured outputs require
var slidesSchema = map[string]interface{}{
	"type": "object",
	"properties": map[string]interface{}{
		"sections": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":       map[string]interface{}{"type": "string"},
					"description": map[string]interface{}{"type": "string"},
					"quotes": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required":             []string{"title", "description", "quotes"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"sections"},
	"additionalProperties": false,
}

func parseSections(text string) ([]Section, error) {
	var resp struct {
		Sections []Section `json:"sections"`
	}
	if err := json.Unmarshal([]byte(stripCodeFence(text)), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse the slides as JSON: %v", err)
	}
	if len(resp.Sections) == 0 {
		return nil, errors.New("the response had no slides")
	}
	return resp.Sections, nil
}

// stripCodeFence removes a ```json fence in case the model adds one anyway
func stripCodeFence(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
	}
	return strings.TrimSpace(text)
}

// apiError extracts the message from an OpenAI or Gemini style error body
func apiError(provider string, status int, body []byte) error {
	var parsed struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error != nil && parsed.Error.Message != "" {
		return fmt.Errorf("%s API error (HTTP %d): %s", provider, status, parsed.Error.Message)
	}
	return fmt.Errorf("%s API error (HTTP %d)", provider, status)
}

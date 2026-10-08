// internal/config/config.go

// Package config remembers the user's choices between runs: the theme and
// how the Recap is shared.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// The images sharing can create
const (
	RecapGIF = "recap-gif" // the Recap slides, animated
	TourGIF  = "tour-gif"  // every tab, then the Recap, animated
	Poster   = "poster"    // a one-image summary
	Tabs     = "tabs"      // a picture of each dashboard tab
	Slides   = "slides"    // a picture of each Recap slide
)

// Outputs lists every image kind, in the order they're offered
var Outputs = []string{RecapGIF, TourGIF, Poster, Tabs, Slides}

// GIF sizes
const (
	GIFStandard = "standard" // 1080x1080 Recap, 1280x960 tour: fits X
	GIFSmall    = "small"    // 720x720 Recap, 960x720 tour: lighter
)

// Image shapes for the pictures
const (
	Portrait  = "portrait"  // 1080x1350, 4:5: Instagram and LinkedIn feeds
	Story     = "story"     // 1080x1920, 9:16: stories
	Square    = "square"    // 1080x1080
	Landscape = "landscape" // 1920x1080, 16:9
)

// GIFSizes and Shapes list the choices, in the order they're offered
var (
	GIFSizes    = []string{GIFStandard, GIFSmall}
	Shapes      = []string{Portrait, Story, Square, Landscape}
	ImageThemes = []string{"dark", "black", "light"}
)

// Share holds how the Recap is shared
type Share struct {
	Outputs    []string `json:"outputs"`
	GIFSize    string   `json:"gif_size"`
	Shape      string   `json:"shape"`
	ImageTheme string   `json:"image_theme"`
	NoAI       bool     `json:"no_ai,omitempty"`
}

// Has reports whether an image kind is turned on
func (s Share) Has(output string) bool {
	for _, o := range s.Outputs {
		if o == output {
			return true
		}
	}
	return false
}

// Config is everything remembered between runs
type Config struct {
	Theme string `json:"theme,omitempty"`
	Share Share  `json:"share"`
}

// Defaults are used for anything that isn't set
func Defaults() Config {
	return Config{
		Theme: "auto",
		Share: Share{
			Outputs:    []string{RecapGIF, TourGIF, Poster, Tabs},
			GIFSize:    GIFStandard,
			Shape:      Portrait,
			ImageTheme: "dark",
		},
	}
}

func oneOf(v string, valid []string, fallback string) string {
	for _, ok := range valid {
		if v == ok {
			return v
		}
	}
	return fallback
}

// normalize replaces missing or unknown values with the defaults
func (c Config) normalize() Config {
	d := Defaults()
	c.Theme = oneOf(c.Theme, []string{"auto", "dark", "black", "light", "terminal"}, d.Theme)
	var outputs []string
	for _, o := range c.Share.Outputs {
		if oneOf(o, Outputs, "") != "" {
			outputs = append(outputs, o)
		}
	}
	if len(outputs) == 0 {
		outputs = d.Share.Outputs
	}
	c.Share.Outputs = outputs
	c.Share.GIFSize = oneOf(c.Share.GIFSize, GIFSizes, d.Share.GIFSize)
	c.Share.Shape = oneOf(c.Share.Shape, Shapes, d.Share.Shape)
	c.Share.ImageTheme = oneOf(c.Share.ImageTheme, ImageThemes, d.Share.ImageTheme)
	return c
}

// path is the config file: SHELLRECAP_CONFIG_DIR/config.json, or the
// platform's config folder (e.g. ~/.config/shellrecap on Linux)
func path() (string, error) {
	dir := os.Getenv("SHELLRECAP_CONFIG_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "shellrecap")
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config, falling back to the defaults for anything missing
func Load() Config {
	p, err := path()
	if err != nil {
		return Defaults()
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Defaults()
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Defaults()
	}
	return c.normalize()
}

// Save writes the config
func Save(c Config) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c.normalize(), "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

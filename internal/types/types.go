// internal/types/types.go
package types

import "time"

// TimelineEntry represents a single entry in the timeline
type TimelineEntry struct {
	Timestamp time.Time
	Command   string
	Shell     string
}

// Slide is a single card of the Wrapped view
type Slide struct {
	Title    string
	Headline string   // the big stat or statement of the slide
	Lines    []string // supporting details, may contain pre-rendered charts
	Quotes   []string
	Footer   string   // small print shown under the slide
	Art      []string // ASCII animation frames shown next to the title
	AI       bool     // written by Gemini rather than computed locally
}

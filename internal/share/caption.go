// internal/share/caption.go
package share

import (
	"fmt"
	"net/url"
	"strings"
)

// RepoURL is linked from the caption and the images
const RepoURL = "https://github.com/ksauraj/shellrecap"

// Caption is the text posted with the images. It stays well under X's 280
// characters (which count the link as 23) and Bluesky's 300.
func Caption(r Recap) string {
	s := r.Stats
	var b strings.Builder
	if s.AllTime {
		fmt.Fprintf(&b, "My shell history, recapped: %s commands", commas(s.TotalCommands))
	} else {
		fmt.Fprintf(&b, "My %d in the terminal: %s commands", s.Year, commas(s.TotalCommands))
	}
	if len(s.TopPrograms) > 0 {
		fmt.Fprintf(&b, ", and %s was my #1", s.TopPrograms[0].Name)
	}
	persona := r.persona()
	if persona == "" {
		persona, _ = s.Persona()
	}
	// AI personas often start with "The", so no article here
	fmt.Fprintf(&b, ". Persona: %s. #shellrecap", persona)
	return b.String()
}

func commas(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// Link is a way to share the images on a platform
type Link struct {
	Key  string // key that picks it in the share menu
	Name string
	URL  string
	// Prefilled is set when the URL fills in the caption; otherwise the
	// caption has to be pasted
	Prefilled bool
	// Animated is set when the platform plays uploaded GIFs
	Animated bool
}

// queryEscape escapes s for a query string, with spaces as %20, which
// every compose page understands
func queryEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// Links lists the platforms in the share menu, with compose pages that
// open with the caption where the platform supports that
func Links(caption string) []Link {
	return []Link{
		{
			Key: "x", Name: "X", Prefilled: true, Animated: true,
			URL: "https://x.com/intent/tweet?text=" + queryEscape(caption) + "&url=" + queryEscape(RepoURL),
		},
		{
			// Bluesky shows uploaded GIFs as still images
			Key: "b", Name: "Bluesky", Prefilled: true,
			URL: "https://bsky.app/intent/compose?text=" + queryEscape(caption+" "+RepoURL),
		},
		{
			// LinkedIn's share links can't fill in text
			Key: "l", Name: "LinkedIn",
			URL: "https://www.linkedin.com/feed/?shareActive=true",
		},
	}
}

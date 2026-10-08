// internal/ai/clean.go
package ai

import "strings"

// typography maps the curly quotes, dashes and special spaces models like
// to use to plain ASCII
var typography = strings.NewReplacer(
	"\u2018", "'", "\u2019", "'", "\u201c", `"`, "\u201d", `"`,
	"\u2010", "-", "\u2011", "-", "\u2013", "-", "\u2014", " - ",
	"\u2026", "...", "\u00a0", " ", "\u202f", " ",
)

// CleanText strips markdown, emoji and fancy typography the model may add
// despite the prompt
func CleanText(text string) string {
	text = typography.Replace(text)
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.Map(func(r rune) rune {
		if IsEmoji(r) {
			return -1
		}
		return r
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

// IsEmoji reports whether r is an emoji or emoji modifier
func IsEmoji(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF, // pictographs, emoticons and flags
		r >= 0x2600 && r <= 0x27BF, // miscellaneous symbols and dingbats
		r >= 0x2300 && r <= 0x23FF, // watches, hourglasses and friends
		r >= 0x2B00 && r <= 0x2BFF, // stars and heavy arrows
		r >= 0xFE00 && r <= 0xFE0F, // variation selectors
		r == 0x200D, r == 0x20E3:   // zero width joiner and keycap
		return true
	}
	return false
}

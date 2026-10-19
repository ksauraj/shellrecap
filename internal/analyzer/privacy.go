// internal/analyzer/privacy.go
package analyzer

import (
	"strings"
	"unicode"
)

// secretPrefixes start well-known API keys and tokens, lowercased
var secretPrefixes = []string{
	"sk-", "sk_", "pk_", "rk_", "ghp_", "gho_", "ghs_", "ghu_", "github_pat_", "glpat-", "gsk_", "aiza",
	"akia", "asia", "xox", "hf_", "npm_", "pypi-", "shpat_", "sq0", "eyj", "ya29.",
}

// scriptExtensions mark someone's own scripts, whose names can say more
// about their work than they'd like to share
var scriptExtensions = []string{
	".sh", ".bash", ".zsh", ".fish", ".py", ".rb", ".pl", ".php", ".js", ".mjs", ".ts", ".ps1", ".psm1",
	".bat", ".cmd", ".exp", ".lua", ".r", ".jar",
}

// hostSuffixes end host names that ended up where a program name goes
var hostSuffixes = []string{
	".com", ".net", ".org", ".io", ".dev", ".app", ".ai", ".co", ".local", ".lan", ".internal", ".corp",
	".home", ".cloud",
}

// PublicName reports whether a program, subcommand or typo name is safe to
// send to an AI provider. History can hold anything typed at a prompt: a
// pasted token, a host name, a path to a client's project or a script
// named after one. Only names that look like ordinary commands pass.
func PublicName(name string) bool {
	if name == "" || len(name) > 24 || !programNamePattern.MatchString(name) {
		return false
	}
	lower := strings.ToLower(name)
	for _, p := range secretPrefixes {
		if strings.HasPrefix(lower, p) {
			return false
		}
	}
	for _, ext := range scriptExtensions {
		if strings.HasSuffix(lower, ext) {
			return false
		}
	}
	for _, suffix := range hostSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return false
		}
	}
	// Host names like db1.prod.example
	if strings.Count(name, ".") >= 2 {
		return false
	}

	digits, upper, lowerCase := 0, 0, 0
	for _, r := range name {
		switch {
		case unicode.IsDigit(r):
			digits++
		case unicode.IsUpper(r):
			upper++
		case unicode.IsLower(r):
			lowerCase++
		}
	}
	// IDs, phone numbers, dates and the like
	if digits >= 5 {
		return false
	}
	// Random looking strings: tokens and hashes mix case and digits
	if len(name) >= 12 && digits > 0 && upper > 0 && lowerCase > 0 {
		return false
	}
	return true
}

// publicCounts keeps the counts whose names are safe to send
func publicCounts(counts []UsageCount) []UsageCount {
	var public []UsageCount
	for _, c := range counts {
		if PublicName(c.Name) {
			public = append(public, c)
		}
	}
	return public
}

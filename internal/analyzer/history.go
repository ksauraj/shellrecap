// internal/analyzer/history.go
package analyzer

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// readHistory reads and parses the history file of the given shell
func readHistory(shell, path string) ([]CommandEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []CommandEntry
	switch shell {
	case "fish":
		entries, err = parseFishHistory(file)
	case "zsh":
		entries, err = parseZshHistory(file)
	default:
		entries, err = parseBashHistory(file)
	}

	for i := range entries {
		entries[i].Program = ProgramName(entries[i].Command)
		entries[i].Categories = categorizeCommand(entries[i].Program)
	}
	return entries, err
}

func newScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	// Pasted scripts can produce very long history lines
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return scanner
}

// parseFishHistory parses fish's YAML-like history format, where each entry
// is a "- cmd: <command>" line followed by indented "when: <timestamp>" and
// "paths:" lines
func parseFishHistory(r io.Reader) ([]CommandEntry, error) {
	var entries []CommandEntry
	scanner := newScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "- cmd: "):
			cmd := unescapeFish(strings.TrimPrefix(line, "- cmd: "))
			entries = append(entries, CommandEntry{Command: strings.TrimSpace(cmd)})
		case strings.HasPrefix(line, "  when: ") && len(entries) > 0:
			if ts, err := strconv.ParseInt(strings.TrimPrefix(line, "  when: "), 10, 64); err == nil {
				entries[len(entries)-1].Timestamp = time.Unix(ts, 0)
			}
		}
	}

	return dropEmpty(entries), scanner.Err()
}

// unescapeFish reverses fish's history escaping of backslashes and newlines
func unescapeFish(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseZshHistory parses both the plain and the EXTENDED_HISTORY format
// (": <timestamp>:<duration>;<command>"). Multi-line commands are stored
// with a trailing backslash on every line except the last.
func parseZshHistory(r io.Reader) ([]CommandEntry, error) {
	var entries []CommandEntry
	scanner := newScanner(r)
	continuing := false

	for scanner.Scan() {
		line := unmetafyZsh(scanner.Bytes())

		if continuing && len(entries) > 0 {
			last := &entries[len(entries)-1]
			last.Command += "\n" + strings.TrimSuffix(line, `\`)
			continuing = strings.HasSuffix(line, `\`)
			continue
		}

		entry := CommandEntry{Command: line}
		if strings.HasPrefix(line, ": ") {
			if semi := strings.IndexByte(line, ';'); semi > 0 {
				meta := strings.SplitN(line[2:semi], ":", 2)
				if ts, err := strconv.ParseInt(strings.TrimSpace(meta[0]), 10, 64); err == nil {
					entry.Timestamp = time.Unix(ts, 0)
					entry.Command = line[semi+1:]
				}
			}
		}

		continuing = strings.HasSuffix(entry.Command, `\`)
		entry.Command = strings.TrimSuffix(entry.Command, `\`)
		entries = append(entries, entry)
	}

	for i := range entries {
		entries[i].Command = strings.TrimSpace(entries[i].Command)
	}
	return dropEmpty(entries), scanner.Err()
}

// unmetafyZsh decodes zsh's "metafied" history encoding, where bytes that
// are special to zsh are stored as 0x83 followed by the byte XOR 32
func unmetafyZsh(line []byte) string {
	const meta = 0x83
	if bytes.IndexByte(line, meta) < 0 {
		return string(line)
	}
	out := make([]byte, 0, len(line))
	for i := 0; i < len(line); i++ {
		if line[i] == meta && i+1 < len(line) {
			i++
			out = append(out, line[i]^32)
			continue
		}
		out = append(out, line[i])
	}
	return string(out)
}

// parseBashHistory parses bash history, picking up the "#<timestamp>" lines
// that bash writes before each command when HISTTIMEFORMAT is set
func parseBashHistory(r io.Reader) ([]CommandEntry, error) {
	var entries []CommandEntry
	var pending time.Time
	scanner := newScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			if ts, err := strconv.ParseInt(line[1:], 10, 64); err == nil {
				pending = time.Unix(ts, 0)
			}
			// Either a timestamp or a pasted comment, neither is a command
			continue
		}
		entries = append(entries, CommandEntry{Command: line, Timestamp: pending})
		pending = time.Time{}
	}

	return dropEmpty(entries), scanner.Err()
}

func dropEmpty(entries []CommandEntry) []CommandEntry {
	kept := entries[:0]
	for _, e := range entries {
		if e.Command != "" {
			kept = append(kept, e)
		}
	}
	return kept
}

// commandPrefixes are wrappers that run another program, so the program that
// follows them is the interesting one
var commandPrefixes = map[string]bool{
	"sudo": true, "doas": true, "env": true, "time": true, "nohup": true,
	"exec": true, "command": true, "builtin": true, "nice": true, "caffeinate": true,
}

// ProgramName extracts the program being run from a command line, skipping
// wrappers like sudo, environment assignments and directory prefixes
func ProgramName(cmd string) string {
	// A commented out command doesn't run anything
	if strings.HasPrefix(strings.TrimSpace(cmd), "#") {
		return ""
	}
	for _, field := range strings.Fields(cmd) {
		if commandPrefixes[field] || strings.HasPrefix(field, "-") {
			continue
		}
		// FOO=bar cmd
		if eq := strings.IndexByte(field, '='); eq > 0 && !strings.ContainsAny(field[:eq], "/.") {
			continue
		}
		field = strings.Trim(field, `"'(`)
		if field == "" {
			continue
		}
		// /usr/bin/git -> git, but keep ./script.sh recognisable
		if strings.Contains(field, "/") && !strings.HasPrefix(field, "./") {
			field = filepath.Base(field)
		}
		return field
	}
	return ""
}

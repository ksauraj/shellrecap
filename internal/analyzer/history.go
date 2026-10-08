// internal/analyzer/history.go
package analyzer

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// readHistory reads and parses the history file of the given shell.
// ignoreCase is set for shells on Windows, where Git and git both run git.
func readHistory(shell, path string, ignoreCase bool) ([]CommandEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	r := skipBOM(file)

	var entries []CommandEntry
	switch shell {
	case "fish":
		entries, err = parseFishHistory(r)
	case "zsh":
		entries, err = parseZshHistory(r)
	case "powershell":
		entries, err = parsePowerShellHistory(r)
	case "cmd":
		entries, err = parseClinkHistory(r)
	case "nu":
		entries, err = parseNuHistory(r)
	default:
		entries, err = parseBashHistory(r)
	}

	for i := range entries {
		entries[i].Program = ProgramName(entries[i].Command)
	}
	if ignoreCase {
		foldPrograms(entries)
	}
	for i := range entries {
		entries[i].Categories = categorizeCommand(entries[i].Program)
	}
	return entries, err
}

// skipBOM skips the byte order mark Windows editors put at the start of
// UTF-8 files
func skipBOM(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	if b, err := br.Peek(3); err == nil && bytes.Equal(b, []byte{0xef, 0xbb, 0xbf}) {
		br.Discard(3)
	}
	return br
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

// parsePowerShellHistory parses the history PSReadLine saves for PowerShell:
// a command per line, where a command spanning several lines ends each of
// them but the last with a backtick. It has no timestamps.
func parsePowerShellHistory(r io.Reader) ([]CommandEntry, error) {
	var entries []CommandEntry
	var command strings.Builder
	scanner := newScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(line, "`") {
			command.WriteString(line[:len(line)-1])
			command.WriteByte('\n')
			continue
		}
		command.WriteString(line)
		entries = append(entries, CommandEntry{Command: strings.TrimSpace(command.String())})
		command.Reset()
	}
	if command.Len() > 0 {
		entries = append(entries, CommandEntry{Command: strings.TrimSpace(command.String())})
	}
	return dropEmpty(entries), scanner.Err()
}

// parseClinkHistory parses the history Clink saves for cmd. When Clink's
// history.time_stamp setting is on, a "|<tab>time=<unix time>" line comes
// before each command. Other lines starting with "|" are commands that
// were deleted, or a tag Clink uses to share the file between windows.
func parseClinkHistory(r io.Reader) ([]CommandEntry, error) {
	var entries []CommandEntry
	var pending time.Time
	scanner := newScanner(r)

	for scanner.Scan() {
		// Clink also ends lines with a NUL or a lone carriage return
		for _, line := range strings.FieldsFunc(scanner.Text(), func(r rune) bool { return r == 0 || r == '\r' }) {
			switch {
			case strings.HasPrefix(line, "|\ttime="):
				if ts, err := strconv.ParseInt(strings.TrimSpace(line[len("|\ttime="):]), 10, 64); err == nil {
					pending = time.Unix(ts, 0)
				}
			case strings.HasPrefix(line, "|"):
				pending = time.Time{}
			default:
				entries = append(entries, CommandEntry{Command: strings.TrimSpace(line), Timestamp: pending})
				pending = time.Time{}
			}
		}
	}
	return dropEmpty(entries), scanner.Err()
}

// parseNuHistory parses Nushell's plain text history, which writes the line
// breaks within a command as "<\n>". It has no timestamps.
func parseNuHistory(r io.Reader) ([]CommandEntry, error) {
	var entries []CommandEntry
	scanner := newScanner(r)
	for scanner.Scan() {
		command := strings.ReplaceAll(scanner.Text(), `<\n>`, "\n")
		entries = append(entries, CommandEntry{Command: strings.TrimSpace(command)})
	}
	return dropEmpty(entries), scanner.Err()
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
// follows them is the interesting one. "&" is PowerShell's call operator
// and "call" is cmd's.
var commandPrefixes = map[string]bool{
	"sudo": true, "doas": true, "env": true, "time": true, "nohup": true,
	"exec": true, "command": true, "builtin": true, "nice": true, "caffeinate": true,
	"&": true, "call": true,
}

// assignments are PowerShell's assignment operators, as in $x = Get-Item
var assignments = toSet("=", "+=", "-=", "*=", "/=", "??=")

// ProgramName extracts the program being run from a command line, skipping
// wrappers like sudo, environment assignments and directory prefixes
func ProgramName(cmd string) string {
	// A commented out command doesn't run anything
	if strings.HasPrefix(strings.TrimSpace(cmd), "#") {
		return ""
	}
	fields := commandWords(cmd, 64)
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if commandPrefixes[field] || strings.HasPrefix(field, "-") {
			continue
		}
		// $x = Get-Item in PowerShell
		if strings.HasPrefix(field, "$") && i+1 < len(fields) && assignments[fields[i+1]] {
			i++
			continue
		}
		// FOO=bar cmd
		if eq := strings.IndexByte(field, '='); eq > 0 && !strings.ContainsAny(field[:eq], "/.") {
			continue
		}
		// & "C:\app.exe" and cmd's @echo off can be written without a space
		field = strings.TrimLeft(field, "&@(")
		if field == "" {
			continue
		}
		return programBase(field)
	}
	return ""
}

// commandWords splits up to max words off the start of a command, keeping
// quoted words like "C:\Program Files\app.exe" together
func commandWords(cmd string, max int) []string {
	var words []string
	var word strings.Builder
	inWord := false
	var quote rune
	for _, r := range cmd {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inWord = r, true
		case unicode.IsSpace(r):
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
				if len(words) == max {
					return words
				}
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words
}

// windowsExtensions are the extensions Windows runs programs from, which
// are usually left off but sometimes typed
var windowsExtensions = []string{".exe", ".cmd", ".bat"}

// programBase turns a program's path into its name: /usr/bin/git and
// C:\Program Files\Git\cmd\git.exe both become git. Scripts run from the
// current folder, like ./build.sh or .\build.ps1, are kept as they are.
func programBase(field string) string {
	if strings.HasPrefix(field, "./") || strings.HasPrefix(field, `.\`) {
		return field
	}
	field = strings.TrimRight(field, `/\`)
	if i := strings.LastIndexAny(field, `/\`); i >= 0 {
		field = field[i+1:]
	}
	lower := strings.ToLower(field)
	for _, ext := range windowsExtensions {
		if len(field) > len(ext) && strings.HasSuffix(lower, ext) {
			return field[:len(field)-len(ext)]
		}
	}
	return field
}

// cmdletVerbs are the verbs PowerShell commands start with, as in
// Get-ChildItem
var cmdletVerbs = toSet("add", "approve", "assert", "backup", "block", "build", "checkpoint", "clear", "close",
	"compare", "complete", "compress", "confirm", "connect", "convert", "convertfrom", "convertto", "copy", "debug",
	"deny", "deploy", "disable", "disconnect", "dismount", "edit", "enable", "enter", "exit", "expand", "export",
	"find", "foreach", "format", "get", "grant", "group", "hide", "import", "initialize", "install", "invoke",
	"join", "limit", "lock", "measure", "merge", "mount", "move", "new", "open", "optimize", "out", "ping", "pop",
	"protect", "publish", "push", "read", "receive", "redo", "register", "remove", "rename", "repair", "request",
	"reset", "resize", "resolve", "restart", "restore", "resume", "revoke", "save", "search", "select", "send",
	"set", "show", "skip", "sort", "split", "start", "step", "stop", "submit", "suspend", "switch", "sync", "tee",
	"test", "trace", "unblock", "undo", "uninstall", "unlock", "unprotect", "unpublish", "unregister", "update",
	"use", "wait", "watch", "where", "write")

// isCmdlet reports whether a program is a PowerShell command, like
// Get-ChildItem or Invoke-WebRequest
func isCmdlet(program string) bool {
	dash := strings.IndexByte(program, '-')
	return dash > 0 && dash < len(program)-1 && cmdletVerbs[strings.ToLower(program[:dash])] &&
		!strings.ContainsAny(program, "./\\")
}

// foldPrograms evens out the program names of a shell that ignores case.
// Programs are written in lower case, like they are everywhere else, and
// PowerShell commands the way they're typed most, like Get-ChildItem.
// Expressions and drive changes like C: aren't programs.
func foldPrograms(entries []CommandEntry) {
	spellings := make(map[string]map[string]int)
	for i := range entries {
		p := entries[i].Program
		if strings.HasPrefix(p, "$") || strings.HasPrefix(p, "[") || strings.HasPrefix(p, "{") ||
			(len(p) == 2 && p[1] == ':') {
			entries[i].Program = ""
			continue
		}
		if isCmdlet(p) {
			key := strings.ToLower(p)
			if spellings[key] == nil {
				spellings[key] = make(map[string]int)
			}
			spellings[key][p]++
		}
	}
	best := make(map[string]string)
	for key, counts := range spellings {
		for spelling, n := range counts {
			b := best[key]
			if b == "" || n > counts[b] || (n == counts[b] && spelling < b) {
				best[key] = spelling
			}
		}
	}
	for i := range entries {
		if p := entries[i].Program; isCmdlet(p) {
			entries[i].Program = best[strings.ToLower(p)]
		} else {
			entries[i].Program = strings.ToLower(p)
		}
	}
}

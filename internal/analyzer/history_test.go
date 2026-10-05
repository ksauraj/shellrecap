package analyzer

import (
	"strings"
	"testing"
	"time"
)

func TestParseFishHistory(t *testing.T) {
	input := `- cmd: git commit -m "fix\nbug" C:\\dir
  when: 1767225600
  paths:
    - foo
- cmd: ls -la
  when: 1767229200
`
	entries, err := parseFishHistory(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Command != "git commit -m \"fix\nbug\" C:\\dir" {
		t.Errorf("command = %q", entries[0].Command)
	}
	if !entries[0].Timestamp.Equal(time.Unix(1767225600, 0)) {
		t.Errorf("timestamp = %v", entries[0].Timestamp)
	}
	if entries[1].Command != "ls -la" {
		t.Errorf("command = %q", entries[1].Command)
	}
}

func TestParseZshHistory(t *testing.T) {
	input := ": 1767225600:0;docker ps -a\n" +
		": 1767225601:3;for i in 1 2; do\\\necho $i\\\ndone\n" +
		"plain command\n"
	entries, err := parseZshHistory(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %q", len(entries), entries)
	}
	if entries[0].Command != "docker ps -a" || entries[0].Timestamp.Unix() != 1767225600 {
		t.Errorf("entry 0 = %+v", entries[0])
	}
	if entries[1].Command != "for i in 1 2; do\necho $i\ndone" {
		t.Errorf("multi-line command = %q", entries[1].Command)
	}
	if entries[2].Command != "plain command" || !entries[2].Timestamp.IsZero() {
		t.Errorf("entry 2 = %+v", entries[2])
	}
}

func TestParseBashHistory(t *testing.T) {
	input := "#1767225600\ngit status\n# a pasted comment\nmake build\n"
	entries, err := parseBashHistory(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Timestamp.Unix() != 1767225600 {
		t.Errorf("timestamp = %v", entries[0].Timestamp)
	}
	if !entries[1].Timestamp.IsZero() {
		t.Errorf("untimed entry got timestamp %v", entries[1].Timestamp)
	}
}

func TestProgramName(t *testing.T) {
	tests := map[string]string{
		"git status":                    "git",
		"sudo apt install foo":          "apt",
		"FOO=bar BAZ=1 npm run build":   "npm",
		"/usr/bin/python3 script.py":    "python3",
		"./gradlew assembleDebug":       "./gradlew",
		"sudo -E env PATH=/x docker ps": "docker",
		"time make -j8":                 "make",
		"":                              "",
	}
	for cmd, want := range tests {
		if got := ProgramName(cmd); got != want {
			t.Errorf("ProgramName(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestSubcommand(t *testing.T) {
	tests := map[string]string{
		"git commit -m wip":      "commit",
		"git -C repo push":       "push",
		"sudo docker compose up": "compose",
		"ls -la":                 "",
		"git ./weird":            "",
		"kubectl -n prod get po": "get",
	}
	for cmd, want := range tests {
		entry := CommandEntry{Command: cmd, Program: ProgramName(cmd)}
		if got := Subcommand(entry); got != want {
			t.Errorf("Subcommand(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestToolUsageCountsPrograms(t *testing.T) {
	// Regression test: every tool used to be credited with every command
	var entries []CommandEntry
	for _, cmd := range []string{"vim a.go", "go build", "go test ./...", "docker ps", "ls", "cd /tmp"} {
		entries = append(entries, CommandEntry{Command: cmd, Program: ProgramName(cmd)})
	}
	usage := analyzeToolUsage(entries)
	if usage.Editors["vim"] != 1 {
		t.Errorf("vim = %d, want 1", usage.Editors["vim"])
	}
	if usage.Languages["Go"] != 2 {
		t.Errorf("Go = %d, want 2", usage.Languages["Go"])
	}
	if usage.DevOps["docker"] != 1 {
		t.Errorf("docker = %d, want 1", usage.DevOps["docker"])
	}
	if len(usage.Languages) != 1 {
		t.Errorf("languages = %v, want only Go", usage.Languages)
	}
}

func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"gti", "git", 1},
		{"claer", "clear", 1},
		{"sl", "ls", 1},
		{"docker", "docker", 0},
		{"abc", "xyz", 3},
	}
	for _, tt := range tests {
		if got := editDistance(tt.a, tt.b); got != tt.want {
			t.Errorf("editDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestLongestStreak(t *testing.T) {
	days := map[string]int{"2026-01-01": 1, "2026-01-02": 1, "2026-01-03": 1, "2026-02-10": 1}
	if got := longestStreak(days); got != 3 {
		t.Errorf("longestStreak = %d, want 3", got)
	}
}

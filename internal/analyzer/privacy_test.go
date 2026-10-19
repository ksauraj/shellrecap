package analyzer

import (
	"strings"
	"testing"
)

func TestPublicName(t *testing.T) {
	for _, name := range []string{"git", "kubectl", "python3.11", "docker-compose", "nvim", "az", "g++",
		"Get-ChildItem", "terraform", "commit", "gti", "claer", "node", "go"} {
		if !PublicName(name) {
			t.Errorf("%q should be safe to send", name)
		}
	}
	for _, name := range []string{
		"", "ghp_x1y2z3", "sk-proj-abc", "gsk_abcdef", "AIzaSyD-thing", "AKIAIOSFODNN7", "eyJhbGciOi",
		"xoxb-123-456", "deploy-acme.sh", "payroll_export.py", "backup.ps1", "db1.prod.example",
		"acme.com", "build.internal", "9876543210", "build-20260419", "aB3dE5fG7hJ9kL",
		"a-very-long-name-for-a-command", "user@host", "/home/me/bin/tool", "C:\\tools\\x.exe", "FOO=bar",
	} {
		if PublicName(name) {
			t.Errorf("%q shouldn't be sent", name)
		}
	}
}

func TestSummaryLeavesOutPrivateNames(t *testing.T) {
	s := WrappedStats{
		Year:          2026,
		TotalCommands: 500,
		Shells:        []UsageCount{{Name: "bash (WSL)", Count: 500}},
		TopPrograms: []UsageCount{{Name: "git", Count: 200}, {Name: "deploy-acme-prod.sh", Count: 90},
			{Name: "ghp_16C7e42F292c6912E7710c838347Ae178B4a", Count: 1}, {Name: "kubectl", Count: 80},
			{Name: "db1.prod.acme", Count: 5}},
		TopGitSubcommands: []UsageCount{{Name: "commit", Count: 50}},
		GitCommands:       200,
		NewPrograms:       []UsageCount{{Name: "acme-internal-cli.py", Count: 4}},
		Typos:             []Typo{{Typed: "gti", Meant: "git", Count: 3}, {Typed: "sk-live-x9", Meant: "ls", Count: 1}},
		TotalTypos:        6,
	}
	summary := s.Summary()
	for _, private := range []string{"acme", "ghp_", "db1", "sk-live"} {
		if strings.Contains(summary, private) {
			t.Errorf("the summary sent to the AI contains %q:\n%s", private, summary)
		}
	}
	for _, public := range []string{"git (200)", "kubectl (80)", "commit (50)", "bash (WSL) (500)", `"gti" instead of "git"`, "Typos (5 total)"} {
		if !strings.Contains(summary, public) {
			t.Errorf("the summary is missing %q:\n%s", public, summary)
		}
	}
	if len(s.TopPrograms) != 5 {
		t.Error("Summary changed the stats it was given")
	}
}

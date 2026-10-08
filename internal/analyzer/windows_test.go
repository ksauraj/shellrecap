package analyzer

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ksauraj/shellrecap/internal/platform"
)

func commands(entries []CommandEntry) []string {
	var cmds []string
	for _, e := range entries {
		cmds = append(cmds, e.Command)
	}
	return cmds
}

func TestParsePowerShellHistory(t *testing.T) {
	// PSReadLine writes CRLF on Windows, and a backtick before each line
	// break inside a command
	input := "git status\r\nGet-ChildItem |`\n  Where-Object Length -gt 1kb\r\n\r\ncode .\r\nfunction hi {`\n  'hi'`\n}\r\n"
	entries, err := parsePowerShellHistory(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"git status", "Get-ChildItem |\n  Where-Object Length -gt 1kb", "code .", "function hi {\n  'hi'\n}"}
	if got := commands(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
	for _, e := range entries {
		if !e.Timestamp.IsZero() {
			t.Errorf("%q has a timestamp, but PowerShell doesn't save any", e.Command)
		}
	}
}

func TestParseClinkHistory(t *testing.T) {
	input := "|CTAG_1767225600_123_456_0\r\n" +
		"|\ttime=1767225600\r\ngit status\r\n" +
		"dir /s\r\n" + // saved before timestamps were turned on
		"|\ttime=1767225700\r\n|eleted command\r\n" + // a deleted command and its timestamp
		"|\ttime=1767225800\r\ncls\r\n"
	entries, err := parseClinkHistory(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := commands(entries), []string{"git status", "dir /s", "cls"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if entries[0].Timestamp.Unix() != 1767225600 || !entries[1].Timestamp.IsZero() || entries[2].Timestamp.Unix() != 1767225800 {
		t.Errorf("timestamps = %v, %v, %v", entries[0].Timestamp, entries[1].Timestamp, entries[2].Timestamp)
	}
}

func TestParseNuHistory(t *testing.T) {
	input := "ls | where size > 1kb\ndef greet [] {<\\n>  print hi<\\n>}\n\ncargo build\n"
	entries, err := parseNuHistory(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ls | where size > 1kb", "def greet [] {\n  print hi\n}", "cargo build"}
	if got := commands(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWindowsProgramNames(t *testing.T) {
	tests := map[string]string{
		`git.exe status`: "git",
		`& "C:\Program Files\Git\cmd\git.exe" log`: "git",
		`&"C:\tools\rg.exe" TODO`:                  "rg",
		`C:\Python312\python.exe -m pip install x`: "python",
		`.\build.ps1 -Release`:                     `.\build.ps1`,
		`$files = Get-ChildItem *.go`:              "Get-ChildItem",
		`npm.cmd run dev`:                          "npm",
		`@echo off`:                                "echo",
		`call build.bat`:                           "build",
		`FOO="a b" make`:                           "make",
		`"/opt/my app/bin/tool" --version`:         "tool",
		`explorer.exe .`:                           "explorer",
		`sudo apt update`:                          "apt",
	}
	for cmd, want := range tests {
		if got := ProgramName(cmd); got != want {
			t.Errorf("ProgramName(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestFoldPrograms(t *testing.T) {
	var entries []CommandEntry
	for _, cmd := range []string{"Git status", "git push", "GIT log", "Get-ChildItem", "Get-ChildItem -Recurse",
		"get-childitem", "$x.Count", "C:", "cd..", "Code ."} {
		entries = append(entries, CommandEntry{Command: cmd, Program: ProgramName(cmd)})
	}
	foldPrograms(entries)
	var got []string
	for _, e := range entries {
		got = append(got, e.Program)
	}
	want := []string{"git", "git", "git", "Get-ChildItem", "Get-ChildItem", "Get-ChildItem", "", "", "cd..", "code"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestSubcommandOfWindowsPrograms(t *testing.T) {
	for cmd, want := range map[string]string{
		"git.exe commit -m wip":                     "commit",
		"winget install Git.Git":                    "install",
		`& "C:\Program Files\Git\cmd\git.exe" pull`: "pull",
	} {
		entry := CommandEntry{Command: cmd, Program: ProgramName(cmd)}
		if got := Subcommand(entry); got != want {
			t.Errorf("Subcommand(%q) = %q, want %q", cmd, got, want)
		}
	}
}

// write creates a file and the folders it's in
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func labels(data ShellData) []string {
	var names []string
	for label := range data.Histories {
		names = append(names, label)
	}
	sort.Strings(names)
	return names
}

// windowsHome lays out a Windows user's profile folder with the history of
// PowerShell (two hosts), cmd with Clink, Git Bash and Nushell
func windowsHome(t *testing.T, home string) {
	ps := filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "PowerShell", "PSReadLine")
	write(t, filepath.Join(ps, "ConsoleHost_history.txt"), "\ufeffgit status\r\nwinget upgrade --all\r\nsl C:\\code\r\n")
	write(t, filepath.Join(ps, "Visual Studio Code Host_history.txt"), "dotnet build\r\n")
	write(t, filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
		"Import-Module posh-git\r\noh-my-posh init pwsh | Invoke-Expression\r\nSet-Alias -Name g -Value git\r\n"+
			"function gs { git status }\r\n$env:EDITOR = \"code\"\r\n# Set-Alias old thing\r\n")
	clink := filepath.Join(home, "AppData", "Local", "clink")
	write(t, filepath.Join(clink, "clink_history"), "|\ttime=1767225600\r\nipconfig /all\r\n")
	write(t, filepath.Join(clink, "fzf.lua"), "-- a Clink script\n")
	write(t, filepath.Join(home, ".bash_history"), "ls -la\nMake\n")
	write(t, filepath.Join(home, "AppData", "Roaming", "nushell", "history.txt"), "ls | sort-by size\n")
	write(t, filepath.Join(home, "AppData", "Roaming", "nushell", "config.nu"), "alias ll = ls -l\n$env.EDITOR = \"hx\"\n")
}

func TestSourcesOnLinux(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".bash_history"), "git status\n")
	write(t, filepath.Join(home, ".zsh_history"), ": 1767225600:0;ls\n")
	write(t, filepath.Join(home, "data", "fish", "fish_history"), "- cmd: make\n  when: 1767225600\n")
	// pwsh follows XDG_DATA_HOME, like fish
	write(t, filepath.Join(home, "data", "powershell", "PSReadLine", "ConsoleHost_history.txt"), "Get-Process\n")
	write(t, filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1"), "Set-Alias k kubectl\n")
	write(t, filepath.Join(home, ".config", "nushell", "history.txt"), "ls\n")

	s := system{goos: "linux", home: home, configDir: filepath.Join(home, ".config"),
		getenv: env(map[string]string{"XDG_DATA_HOME": filepath.Join(home, "data")})}
	data := Load(s.sources())
	if got, want := labels(data), []string{"bash", "fish", "nu", "powershell", "zsh"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("shells = %q, want %q", got, want)
	}
	// Nushell's folder is found twice on Linux, but read once
	if n := len(data.Histories["nu"]); n != 1 {
		t.Errorf("nu has %d commands, want 1", n)
	}
	if data.ShellConfigs["powershell"].Aliases["k"] != "kubectl" {
		t.Errorf("PowerShell aliases = %v", data.ShellConfigs["powershell"].Aliases)
	}
}

func TestSourcesOnWindows(t *testing.T) {
	home := t.TempDir()
	windowsHome(t, home)
	distro := t.TempDir()
	write(t, filepath.Join(distro, ".bash_history"), "sudo apt update\n")
	write(t, filepath.Join(distro, ".zsh_history"), ": 1767225600:0;docker ps\n")

	s := system{goos: "windows", home: home, getenv: env(nil), documents: []string{filepath.Join(home, "Documents")},
		distros: []platform.Distro{{Name: "Ubuntu", Home: distro}}}
	data := Load(s.sources())
	want := []string{"bash", "bash (WSL)", "cmd", "nu", "powershell", "zsh (WSL)"}
	if got := labels(data); !reflect.DeepEqual(got, want) {
		t.Fatalf("shells = %q, want %q", got, want)
	}

	// Both PowerShell hosts' history, with the BOM gone
	ps := data.Histories["powershell"]
	if len(ps) != 4 || ps[0].Command != "git status" {
		t.Errorf("PowerShell history = %q", commands(ps))
	}
	config := data.ShellConfigs["powershell"]
	if config.Aliases["g"] != "git" || config.Aliases["gs"] != "git status" || config.Environment["EDITOR"] != "code" {
		t.Errorf("PowerShell profile: aliases %v, env %v", config.Aliases, config.Environment)
	}
	if _, ok := config.Aliases["old"]; ok {
		t.Error("a commented out alias was read")
	}
	var plugins []string
	for _, p := range config.Plugins {
		plugins = append(plugins, p.Name)
	}
	if !reflect.DeepEqual(plugins, []string{"posh-git", "oh-my-posh"}) {
		t.Errorf("PowerShell plugins = %q", plugins)
	}

	cmd := data.Histories["cmd"]
	if len(cmd) != 1 || cmd[0].Program != "ipconfig" || cmd[0].Timestamp.Unix() != 1767225600 {
		t.Errorf("cmd history = %+v", cmd)
	}
	if p := data.ShellConfigs["cmd"].Plugins; len(p) != 1 || p[0].Name != "fzf" {
		t.Errorf("Clink scripts = %+v", p)
	}
	// Windows ignores case, so Git Bash's Make ran make
	if bash := data.Histories["bash"]; len(bash) != 2 || bash[1].Program != "make" {
		t.Errorf("Git Bash history = %+v", bash)
	}
	if data.ShellConfigs["nu"].Aliases["ll"] != "ls -l" || data.ShellConfigs["nu"].Environment["EDITOR"] != "hx" {
		t.Errorf("Nushell config = %+v", data.ShellConfigs["nu"])
	}
}

func TestSourcesInWSL(t *testing.T) {
	root := t.TempDir() // stands in for /mnt/c
	winHome := filepath.Join(root, "Users", "Name")
	windowsHome(t, winHome)
	write(t, filepath.Join(root, "msys64", "home", "Name", ".bash_history"), "pacman -Syu\n")
	home := t.TempDir()
	write(t, filepath.Join(home, ".bash_history"), "git pull\n")

	s := system{goos: "linux", wsl: true, home: home, configDir: filepath.Join(home, ".config"), getenv: env(nil),
		windowsHome: winHome, documents: []string{filepath.Join(winHome, "Documents")}}
	data := Load(s.sources())
	want := []string{"bash", "bash (Windows)", "cmd", "nu (Windows)", "powershell"}
	if got := labels(data); !reflect.DeepEqual(got, want) {
		t.Fatalf("shells = %q, want %q", got, want)
	}
	// Git Bash and MSYS2 are both bash on Windows
	if got := commands(data.Histories["bash (Windows)"]); !reflect.DeepEqual(got, []string{"ls -la", "Make", "pacman -Syu"}) {
		t.Errorf("Windows bash = %q", got)
	}
}

// Every history file is read once, even when two paths lead to it
func TestSourcesReadEachFileOnce(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".zsh_history"), "ls\n")
	if err := os.Symlink(filepath.Join(home, ".zsh_history"), filepath.Join(home, ".histfile")); err != nil {
		t.Skip("can't make symlinks here:", err)
	}
	s := system{goos: "linux", home: home, configDir: filepath.Join(home, ".config"), getenv: env(nil)}
	if n := len(Load(s.sources()).Histories["zsh"]); n != 1 {
		t.Errorf("zsh has %d commands, want 1", n)
	}
}

// PowerShell's sl is Set-Location, not a typo of ls
func TestPowerShellAliasesAreNotTypos(t *testing.T) {
	data := InitShellData()
	for i := 0; i < 100; i++ {
		data.Histories["powershell"] = append(data.Histories["powershell"], CommandEntry{Command: "ls", Program: "ls"})
	}
	for i := 0; i < 5; i++ {
		data.Histories["powershell"] = append(data.Histories["powershell"], CommandEntry{Command: "sl ..", Program: "sl"})
	}
	stats := ComputeWrapped(Analyze(data), 2026)
	for _, typo := range stats.Typos {
		if typo.Typed == "sl" {
			t.Errorf("sl was counted as a typo of %s", typo.Meant)
		}
	}
}

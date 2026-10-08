// internal/analyzer/sources.go
package analyzer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ksauraj/shellrecap/internal/platform"
)

// Source is a shell's history and the configuration its aliases are
// defined in
type Source struct {
	Shell  string   // the history format: bash, zsh, fish, powershell, cmd or nu
	Label  string   // the name it's shown by, like "bash" or "bash (WSL)"
	Paths  []string // history files; every one that exists is read
	Config []string // configuration files
	Home   string   // the home folder plugins are looked for in, for bash, zsh and fish
	// IgnoreCase is set for shells on Windows, where Git, git and GIT all
	// run git
	IgnoreCase bool
}

// system is what finding the history needs to know about this machine
type system struct {
	goos      string
	home      string
	getenv    func(string) string
	configDir string // where Nushell keeps its files: os.UserConfigDir
	// wsl is set for Linux running under WSL
	wsl bool
	// windowsHome is the Windows user's profile folder, on Windows or from
	// inside WSL, and documents where Windows keeps their Documents
	windowsHome string
	documents   []string
	// distros are the WSL distros, when running on Windows
	distros []platform.Distro
}

// thisSystem describes the machine shellrecap runs on
func thisSystem() system {
	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()
	s := system{goos: runtime.GOOS, home: home, getenv: os.Getenv, configDir: configDir, wsl: platform.IsWSL()}
	if s.goos == "windows" || s.wsl {
		s.windowsHome = platform.WindowsHome()
		s.documents = platform.Documents()
	}
	if s.goos == "windows" {
		s.distros = platform.WSLDistros()
	}
	return s
}

// sources lists everywhere this machine may keep shell history
func (s system) sources() []Source {
	if s.goos == "windows" {
		sources := s.windowsSources(s.home, "")
		for _, home := range s.unixHomesOnWindows(s.home) {
			sources = append(sources, onWindows(unixSources(home, "", noEnv))...)
		}
		for _, d := range s.distros {
			sources = append(sources, unixSources(d.Home, " (WSL)", noEnv)...)
		}
		return sources
	}

	sources := unixSources(s.home, "", s.getenv)
	sources = append(sources, s.crossPlatformSources()...)
	// From inside WSL, the history of the Windows side too
	if s.wsl && s.windowsHome != "" {
		sources = append(sources, s.windowsSources(s.windowsHome, " (Windows)")...)
		for _, home := range s.unixHomesOnWindows(s.windowsHome) {
			sources = append(sources, onWindows(unixSources(home, " (Windows)", noEnv))...)
		}
	}
	return sources
}

func noEnv(string) string { return "" }

// onWindows marks sources as running on Windows
func onWindows(sources []Source) []Source {
	for i := range sources {
		sources[i].IgnoreCase = true
	}
	return sources
}

// xdg is an XDG base folder: the variable's value, or its default
func xdg(getenv func(string) string, variable, home, fallback string) string {
	if dir := getenv(variable); filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(home, fallback)
}

// unixSources are the shells that keep their history in the home folder:
// bash, zsh and fish on Linux and macOS, in WSL, and in Git Bash
func unixSources(home, suffix string, getenv func(string) string) []Source {
	in := func(names ...string) []string {
		var paths []string
		for _, name := range names {
			paths = append(paths, filepath.Join(home, name))
		}
		return paths
	}
	zsh := Source{Shell: "zsh", Label: "zsh" + suffix, Home: home,
		// .histfile is where zsh's first-run setup puts it
		Paths:  in(".zsh_history", ".histfile"),
		Config: in(".zshrc", ".zsh_plugins", ".zprofile")}
	if dir := getenv("ZDOTDIR"); filepath.IsAbs(dir) {
		zsh.Paths = append(zsh.Paths, filepath.Join(dir, ".zsh_history"))
		zsh.Config = append(zsh.Config, filepath.Join(dir, ".zshrc"))
	}
	fishData := xdg(getenv, "XDG_DATA_HOME", home, filepath.Join(".local", "share"))
	fishConfig := xdg(getenv, "XDG_CONFIG_HOME", home, ".config")
	return []Source{
		{Shell: "bash", Label: "bash" + suffix, Home: home,
			Paths:  in(".bash_history"),
			Config: in(".bashrc", ".bash_profile", ".bash_aliases")},
		zsh,
		{Shell: "fish", Label: "fish" + suffix, Home: home,
			Paths:  []string{filepath.Join(fishData, "fish", "fish_history")},
			Config: []string{filepath.Join(fishConfig, "fish", "config.fish")}},
	}
}

// crossPlatformSources are PowerShell and Nushell on Linux and macOS
func (s system) crossPlatformSources() []Source {
	data := xdg(s.getenv, "XDG_DATA_HOME", s.home, filepath.Join(".local", "share"))
	config := xdg(s.getenv, "XDG_CONFIG_HOME", s.home, ".config")
	powershell := filepath.Join(config, "powershell")
	return []Source{
		// PowerShell ignores case on Linux and macOS too
		{Shell: "powershell", Label: "powershell", IgnoreCase: true,
			Paths:  psReadLineFiles(filepath.Join(data, "powershell", "PSReadLine")),
			Config: []string{filepath.Join(powershell, "Microsoft.PowerShell_profile.ps1"), filepath.Join(powershell, "profile.ps1")}},
		// Nushell uses ~/Library/Application Support on macOS, unless
		// XDG_CONFIG_HOME is set
		nuSource(s.configDir, ""),
		nuSource(config, ""),
	}
}

// windowsSources are the shells of a Windows user: PowerShell, cmd with
// Clink, and Nushell. suffix marks the ones that also run on Linux, when
// they're seen from WSL.
func (s system) windowsSources(home, suffix string) []Source {
	appData := filepath.Join(home, "AppData", "Roaming")
	localAppData := filepath.Join(home, "AppData", "Local")
	if s.goos == "windows" {
		if dir := s.getenv("APPDATA"); dir != "" {
			appData = dir
		}
		if dir := s.getenv("LOCALAPPDATA"); dir != "" {
			localAppData = dir
		}
	}

	// Windows PowerShell and PowerShell 7 share their history, and each
	// has its own profile
	var profiles []string
	for _, docs := range s.documents {
		for _, dir := range []string{"PowerShell", "WindowsPowerShell"} {
			profiles = append(profiles, filepath.Join(docs, dir, "Microsoft.PowerShell_profile.ps1"),
				filepath.Join(docs, dir, "profile.ps1"))
		}
	}

	// cmd forgets its history when it closes, but Clink saves it
	clinkDirs := []string{filepath.Join(localAppData, "clink")}
	if dir := s.getenv("CLINK_PROFILE"); s.goos == "windows" && dir != "" {
		clinkDirs = append([]string{dir}, clinkDirs...)
	}
	cmd := Source{Shell: "cmd", Label: "cmd"}
	for _, dir := range clinkDirs {
		// clink_history since Clink 1.0, .history before
		cmd.Paths = append(cmd.Paths, filepath.Join(dir, "clink_history"), filepath.Join(dir, ".history"))
		// Clink's scripts are its plugins
		scripts, _ := filepath.Glob(filepath.Join(dir, "*.lua"))
		cmd.Config = append(cmd.Config, scripts...)
	}

	return onWindows([]Source{
		{Shell: "powershell", Label: "powershell",
			Paths:  psReadLineFiles(filepath.Join(appData, "Microsoft", "Windows", "PowerShell", "PSReadLine")),
			Config: profiles},
		cmd,
		nuSource(appData, suffix),
	})
}

// unixHomesOnWindows are the home folders of bash and other Linux shells
// on Windows: the user's own, which Git Bash uses, and MSYS2's and Cygwin's
func (s system) unixHomesOnWindows(home string) []string {
	user := filepath.Base(home)
	root := filepath.VolumeName(home) + `\`
	if s.goos != "windows" {
		// /mnt/c/Users/Name from WSL
		root = filepath.Dir(filepath.Dir(home))
	}
	homes := []string{
		home,
		filepath.Join(root, "msys64", "home", user),
		filepath.Join(root, "cygwin64", "home", user),
		filepath.Join(root, "cygwin", "home", user),
	}
	if s.goos == "windows" {
		if dir := s.getenv("HOME"); filepath.IsAbs(dir) && !strings.EqualFold(filepath.Clean(dir), filepath.Clean(home)) {
			homes = append(homes, dir)
		}
	}
	return homes
}

// psReadLineFiles are the history files PSReadLine keeps in dir, one for
// each program hosting PowerShell: the console, VS Code and so on
func psReadLineFiles(dir string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, "*_history.txt"))
	return files
}

func nuSource(configDir, suffix string) Source {
	dir := filepath.Join(configDir, "nushell")
	return Source{Shell: "nu", Label: "nu" + suffix,
		Paths:  []string{filepath.Join(dir, "history.txt")},
		Config: []string{filepath.Join(dir, "config.nu"), filepath.Join(dir, "env.nu")}}
}

// Load reads the history of every source that has some, merging sources
// with the same label. A file found through two paths is read once.
func Load(sources []Source) ShellData {
	data := InitShellData()
	var read []os.FileInfo
	for _, src := range sources {
		found := false
		for _, path := range src.Paths {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() || sameFile(info, read) {
				continue
			}
			read = append(read, info)
			if history, err := readHistory(src.Shell, path, src.IgnoreCase); err == nil && len(history) > 0 {
				data.Histories[src.Label] = append(data.Histories[src.Label], history...)
				found = true
			}
		}
		if found {
			config := analyzeShellConfig(src)
			if existing, ok := data.ShellConfigs[src.Label]; ok {
				config = mergeConfigs(existing, config)
			}
			data.ShellConfigs[src.Label] = config
		}
	}
	return data
}

func sameFile(info os.FileInfo, seen []os.FileInfo) bool {
	for _, s := range seen {
		if os.SameFile(info, s) {
			return true
		}
	}
	return false
}

func mergeConfigs(a, b ShellConfig) ShellConfig {
	for k, v := range b.ConfigFiles {
		a.ConfigFiles[k] = v
	}
	for k, v := range b.Aliases {
		a.Aliases[k] = v
	}
	for k, v := range b.Environment {
		a.Environment[k] = v
	}
	a.Plugins = append(a.Plugins, b.Plugins...)
	return a
}

// HistoryTip is advice for when no history was found, if there's any
func HistoryTip() string {
	if runtime.GOOS == "windows" {
		return "cmd forgets its history when it closes. Install Clink (chrisant996.github.io/clink) to keep it."
	}
	return ""
}

// SupportedShells names the shells shellrecap reads on this system, for
// saying where it looked
func SupportedShells() string {
	if runtime.GOOS == "windows" {
		return "PowerShell, cmd with Clink, Git Bash, Nushell and WSL"
	}
	if platform.IsWSL() {
		return "bash, zsh, fish, PowerShell and Nushell, in WSL and on Windows"
	}
	return "bash, zsh, fish, PowerShell and Nushell"
}

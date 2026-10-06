// internal/analyzer/shell_analysis.go
package analyzer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func AnalyzeShells() tea.Msg {
	data := InitShellData()

	// Read shell histories
	shellPaths := map[string]string{
		"bash": "~/.bash_history",
		"zsh":  "~/.zsh_history",
		"fish": "~/.local/share/fish/fish_history",
	}

	for shell, path := range shellPaths {
		expandedPath := expandPath(path)
		if history, err := readHistory(shell, expandedPath); err == nil && len(history) > 0 {
			data.Histories[shell] = history
			data.ShellConfigs[shell] = analyzeShellConfigs(shell)
		}
	}

	// Analyze all shells together so one shell doesn't overwrite another
	allEntries := AllEntries(data)
	analyzeCommands(allEntries, &data)
	data.Insights.ToolUsage = analyzeToolUsage(allEntries)

	return data
}

// AllEntries returns the history entries of every shell
func AllEntries(data ShellData) []CommandEntry {
	var all []CommandEntry
	for _, shell := range SortedShells(data) {
		all = append(all, data.Histories[shell]...)
	}
	return all
}

// SortedShells returns the shells with history, most used first
func SortedShells(data ShellData) []string {
	shells := make([]string, 0, len(data.Histories))
	for shell := range data.Histories {
		shells = append(shells, shell)
	}
	sort.Slice(shells, func(i, j int) bool {
		a, b := len(data.Histories[shells[i]]), len(data.Histories[shells[j]])
		if a != b {
			return a > b
		}
		return shells[i] < shells[j]
	})
	return shells
}

func toSet(items ...string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// languagePrograms maps executables to the language their use indicates
var languagePrograms = map[string]string{
	"python": "Python", "python3": "Python", "python2": "Python", "pip": "Python", "pip3": "Python",
	"pipx": "Python", "uv": "Python", "poetry": "Python", "ipython": "Python", "jupyter": "Python", "pytest": "Python",
	"node": "JavaScript", "npm": "JavaScript", "npx": "JavaScript", "yarn": "JavaScript", "pnpm": "JavaScript",
	"bun": "JavaScript", "deno": "JavaScript", "tsc": "TypeScript", "ts-node": "TypeScript",
	"go": "Go", "gofmt": "Go",
	"cargo": "Rust", "rustc": "Rust", "rustup": "Rust",
	"java": "Java", "javac": "Java", "mvn": "Java", "gradle": "Java", "gradlew": "Java", "./gradlew": "Java",
	"kotlin": "Kotlin", "kotlinc": "Kotlin",
	"ruby": "Ruby", "gem": "Ruby", "bundle": "Ruby", "rails": "Ruby", "irb": "Ruby", "rake": "Ruby",
	"php": "PHP", "composer": "PHP",
	"gcc": "C/C++", "g++": "C/C++", "clang": "C/C++", "clang++": "C/C++", "cc": "C/C++", "cmake": "C/C++",
	"dotnet": "C#", "swift": "Swift", "swiftc": "Swift", "lua": "Lua", "perl": "Perl",
	"R": "R", "Rscript": "R", "julia": "Julia",
	"ghc": "Haskell", "ghci": "Haskell", "stack": "Haskell", "cabal": "Haskell",
	"elixir": "Elixir", "mix": "Elixir", "iex": "Elixir", "erl": "Erlang",
	"zig": "Zig", "nim": "Nim", "dart": "Dart", "flutter": "Dart",
	"scala": "Scala", "sbt": "Scala", "ocaml": "OCaml", "dune": "OCaml",
}

var editorPrograms = toSet("vim", "vi", "nvim", "emacs", "emacsclient", "code", "codium", "cursor", "zed",
	"nano", "micro", "hx", "helix", "subl", "kak", "gedit", "kate", "pico")

var buildToolPrograms = toSet("make", "cmake", "ninja", "meson", "bazel", "mvn", "gradle", "gradlew", "./gradlew",
	"ant", "sbt", "npm", "yarn", "pnpm", "bun", "pip", "pip3", "poetry", "uv", "cargo", "composer", "bundle", "mix")

var devopsPrograms = toSet("docker", "docker-compose", "podman", "kubectl", "k9s", "kubectx", "kubens", "helm",
	"minikube", "kind", "k3s", "oc", "eksctl", "terraform", "tofu", "terragrunt", "pulumi", "ansible",
	"ansible-playbook", "vagrant", "packer", "aws", "az", "gcloud", "gsutil", "doctl", "flyctl", "fly",
	"vercel", "netlify", "heroku", "wrangler")

// skillDomains groups programs into broader areas of expertise
var skillDomains = []struct {
	name     string
	programs map[string]bool
}{
	{"Version Control", toSet("git", "gh", "glab", "svn", "hg", "lazygit", "tig")},
	{"Containers & K8s", toSet("docker", "docker-compose", "podman", "kubectl", "k9s", "kubectx",
		"kubens", "helm", "minikube", "kind", "k3s", "oc", "eksctl")},
	{"Cloud CLIs", toSet("aws", "az", "gcloud", "gsutil", "doctl", "flyctl", "fly", "vercel", "netlify",
		"heroku", "wrangler")},
	{"Infra as Code", toSet("terraform", "tofu", "terragrunt", "pulumi", "ansible",
		"ansible-playbook", "vagrant", "packer")},
	{"System Admin", toSet("systemctl", "journalctl", "service", "apt", "apt-get", "apt-fast", "dpkg",
		"dnf", "yum", "pacman", "yay", "paru", "brew", "snap", "flatpak", "zypper", "mount", "umount", "chmod",
		"chown", "useradd", "usermod", "crontab", "dmesg", "lsblk", "fdisk", "htop", "top", "btop", "ps",
		"kill", "pkill", "killall", "df", "du", "free")},
	{"Networking", toSet("ssh", "scp", "rsync", "sftp", "curl", "wget", "ping", "traceroute", "dig",
		"nslookup", "nc", "ncat", "nmap", "ip", "ifconfig", "netstat", "ss", "iptables", "ufw", "tailscale", "wg")},
	{"Databases", toSet("mysql", "psql", "pg_dump", "mongo", "mongosh", "redis-cli", "sqlite3", "clickhouse-client")},
	{"Android & Mobile", toSet("adb", "fastboot", "flutter", "emulator", "scrcpy", "apktool", "sdkmanager")},
}

// subcommandTools are programs whose first argument is a subcommand worth
// tracking, e.g. "git commit" or "docker compose"
var subcommandTools = toSet("git", "gh", "docker", "docker-compose", "podman", "kubectl", "helm", "npm", "yarn",
	"pnpm", "bun", "cargo", "go", "az", "aws", "gcloud", "terraform", "tofu", "systemctl", "apt", "apt-get",
	"brew", "pip", "pip3", "uv", "poetry", "adb", "fastboot", "snap", "flatpak", "dnf", "pacman", "make")

var subcommandPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// flagsWithValue are global flags that take a separate value, like
// "git -C dir" or "kubectl -n namespace"
var flagsWithValue = toSet("-C", "-c", "-n", "--namespace", "-f", "--file", "--context", "--kubeconfig",
	"-H", "--host", "-p", "--project-name", "--git-dir", "--work-tree", "--profile", "--region")

// Subcommand returns the subcommand of a command like "git commit -m x",
// or "" when the program isn't one with subcommands
func Subcommand(entry CommandEntry) string {
	if !subcommandTools[entry.Program] {
		return ""
	}
	fields := strings.Fields(entry.Command)
	for i, field := range fields {
		if filepath.Base(field) != entry.Program {
			continue
		}
		args := fields[i+1:]
		for j := 0; j < len(args); j++ {
			arg := args[j]
			if strings.HasPrefix(arg, "-") {
				// Skip global flags like "git -C dir"
				if flagsWithValue[arg] {
					j++
				}
				continue
			}
			if subcommandPattern.MatchString(arg) {
				return arg
			}
			return ""
		}
		return ""
	}
	return ""
}

func categorizeCommand(program string) []string {
	categories := []string{}
	patterns := map[string][]string{
		"development": {"git", "docker", "npm", "go", "python", "python3"},
		"system":      {"sudo", "systemctl", "ps", "top"},
		"file":        {"ls", "cd", "cp", "mv", "rm"},
	}

	for category, programs := range patterns {
		for _, p := range programs {
			if program == p {
				categories = append(categories, category)
				break
			}
		}
	}

	return categories
}

func analyzeCommands(entries []CommandEntry, data *ShellData) {
	// Initialize maps for analysis
	langUsage := make(map[string]int)
	techUsage := make(map[string]int)
	domainUsage := make(map[string]int)
	workflows := make(map[string]int)
	timeOfDay := make(map[int]int)
	commandPatterns := make(map[string]int)

	// Analyze each command
	for _, entry := range entries {
		program := entry.Program
		if program == "" {
			continue
		}
		data.CommonCmds[program]++

		// Entries without a timestamp would all land on midnight
		if !entry.Timestamp.IsZero() {
			timeOfDay[entry.Timestamp.Hour()]++
			data.Insights.WorkPatterns.HourlyActivity[entry.Timestamp.Hour()]++
		}

		lang, isLang := languagePrograms[program]
		if isLang {
			langUsage[lang]++
		}
		isTech := isLang || devopsPrograms[program] || editorPrograms[program] || buildToolPrograms[program]
		for _, domain := range skillDomains {
			if domain.programs[program] {
				domainUsage[domain.name]++
				isTech = isTech || domain.name == "Version Control" || domain.name == "Databases"
			}
		}
		if isTech {
			techUsage[program]++
		}
		if sub := Subcommand(entry); sub != "" {
			workflows[program+" "+sub]++
		}

		// Analyze command patterns
		analyzeCommandPattern(entry.Command, commandPatterns)
	}

	// Update TechnicalProfile
	techProfile := &data.Insights.TechnicalProfile
	languages := SortedCounts(langUsage)

	techProfile.TechStack = make([]string, 0)
	for _, lang := range languages {
		if lang.Count >= 3 && len(techProfile.TechStack) < 8 {
			techProfile.TechStack = append(techProfile.TechStack, lang.Name)
		}
	}

	techProfile.SecondarySkills = make([]UsageCount, 0)
	for _, domain := range SortedCounts(domainUsage) {
		if domain.Count >= 10 {
			techProfile.SecondarySkills = append(techProfile.SecondarySkills, domain)
		}
	}

	techProfile.TopTech = TopN(SortedCounts(techUsage), 10)
	techProfile.PrimaryRole = primaryRole(languages, domainUsage)

	// Update WorkPatterns
	patterns := &data.Insights.WorkPatterns
	patterns.PeakHours = getPeakHours(timeOfDay)
	patterns.CommonWorkflows = make([]string, 0)
	for _, wf := range TopN(SortedCounts(workflows), 6) {
		patterns.CommonWorkflows = append(patterns.CommonWorkflows, fmt.Sprintf("%s (%d×)", wf.Name, wf.Count))
	}

	// Calculate productivity metrics based on command complexity and variety
	patterns.Productivity = calculateProductivityMetrics(entries, commandPatterns)
}

// primaryRole picks the role that best explains the bulk of the user's commands
func primaryRole(languages []UsageCount, domainUsage map[string]int) string {
	type candidate struct {
		role  string
		count int
	}
	candidates := []candidate{
		{"DevOps & Cloud Engineer", domainUsage["Containers & K8s"] +
			domainUsage["Cloud CLIs"] + domainUsage["Infra as Code"]},
		{"System Administrator", domainUsage["System Admin"]},
		{"Android Developer", domainUsage["Android & Mobile"]},
	}
	if len(languages) > 0 {
		total := 0
		for _, lang := range languages {
			total += lang.Count
		}
		// Listed first so it wins ties
		candidates = append([]candidate{{languages[0].Name + " Developer", total}}, candidates...)
	}

	best := candidate{}
	for _, c := range candidates {
		if c.count > best.count {
			best = c
		}
	}
	if best.count < 10 {
		return ""
	}
	return best.role
}

func analyzeToolUsage(entries []CommandEntry) ToolUsage {
	toolUsage := ToolUsage{
		Editors:    make(map[string]int),
		Languages:  make(map[string]int),
		BuildTools: make(map[string]int),
		DevOps:     make(map[string]int),
	}

	// Count by the program actually being run, not by substring matches
	for _, entry := range entries {
		program := entry.Program
		if lang, ok := languagePrograms[program]; ok {
			toolUsage.Languages[lang]++
		}
		if editorPrograms[program] {
			toolUsage.Editors[program]++
		}
		if buildToolPrograms[program] {
			toolUsage.BuildTools[program]++
		}
		if devopsPrograms[program] {
			toolUsage.DevOps[program]++
		}
	}

	return toolUsage
}

// commandPatternMap defines common command patterns
var commandPatternMap = map[string]*regexp.Regexp{
	"git_workflow": regexp.MustCompile(`git (commit|push|pull|merge)`),
	"build":        regexp.MustCompile(`(make|build|compile)`),
	"deploy":       regexp.MustCompile(`(deploy|kubectl|docker)`),
	"test":         regexp.MustCompile(`test|spec|pytest`),
}

func analyzeCommandPattern(cmd string, patterns map[string]int) {
	for pattern, regex := range commandPatternMap {
		if regex.MatchString(cmd) {
			patterns[pattern]++
		}
	}
}

func getPeakHours(timeOfDay map[int]int) []int {
	type hourCount struct {
		hour  int
		count int
	}

	var hours []hourCount
	for h, c := range timeOfDay {
		hours = append(hours, hourCount{h, c})
	}

	sort.Slice(hours, func(i, j int) bool {
		if hours[i].count != hours[j].count {
			return hours[i].count > hours[j].count
		}
		return hours[i].hour < hours[j].hour
	})

	// Return top 3 peak hours
	var peaks []int
	for i := 0; i < len(hours) && i < 3; i++ {
		peaks = append(peaks, hours[i].hour)
	}
	return peaks
}

func calculateProductivityMetrics(entries []CommandEntry, patterns map[string]int) map[string]float64 {
	metrics := make(map[string]float64)
	totalCommands := len(entries)

	if totalCommands == 0 {
		return metrics
	}

	// Command variety score
	uniqueCommands := make(map[string]bool)
	pipelines := 0
	for _, entry := range entries {
		uniqueCommands[entry.Command] = true
		if strings.Contains(entry.Command, "|") {
			pipelines++
		}
	}
	metrics["Command Variety"] = float64(len(uniqueCommands)) / float64(totalCommands)
	metrics["Pipeline Usage"] = float64(pipelines) / float64(totalCommands)

	// Workflow complexity score
	workflowScore := float64(patterns["git_workflow"]+patterns["build"]+
		patterns["deploy"]+patterns["test"]) / float64(totalCommands)
	metrics["Workflow Complexity"] = workflowScore

	return metrics
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func analyzeShellConfigs(shell string) ShellConfig {
	configPaths := map[string][]string{
		"bash": {
			"~/.bashrc",
			"~/.bash_profile",
			"~/.bash_aliases",
		},
		"zsh": {
			"~/.zshrc",
			"~/.zsh_plugins",
			"~/.zprofile",
		},
		"fish": {
			"~/.config/fish/config.fish",
			"~/.config/fish/functions",
			"~/.config/fish/conf.d",
		},
	}

	config := ShellConfig{
		ConfigFiles: make(map[string]ConfigInfo),
		Aliases:     make(map[string]string),
		Environment: make(map[string]string),
		Plugins:     make([]PluginInfo, 0),
	}

	// Read and analyze config files
	for _, paths := range configPaths[shell] {
		expandedPath := expandPath(paths)
		if info, err := os.Stat(expandedPath); err == nil {
			content, _ := os.ReadFile(expandedPath)
			config.ConfigFiles[paths] = ConfigInfo{
				Path:     expandedPath,
				Modified: info.ModTime(),
				Content:  string(content),
			}

			// Parse the config file
			parseShellConfig(string(content), &config)
		}
	}

	// Detect plugins based on shell type
	detectPlugins(shell, &config)

	return config
}

func parseShellConfig(content string, config *ShellConfig) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		// Parse aliases
		if strings.HasPrefix(line, "alias ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "alias "), "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
				config.Aliases[name] = value
			}
		}

		// Parse environment variables
		if strings.HasPrefix(line, "export ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "export "), "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
				config.Environment[name] = value
			}
		}
	}
}

func detectPlugins(shell string, config *ShellConfig) {
	switch shell {
	case "zsh":
		detectZshPlugins(config)
	case "fish":
		detectFishPlugins(config)
	case "bash":
		detectBashPlugins(config)
	}
}

func detectZshPlugins(config *ShellConfig) {
	// Check for Oh My Zsh plugins
	omzPath := expandPath("~/.oh-my-zsh")
	if info, err := os.Stat(omzPath); err == nil && info.IsDir() {
		pluginsPath := filepath.Join(omzPath, "plugins")
		if pluginsDir, err := os.ReadDir(pluginsPath); err == nil {
			for _, pluginDir := range pluginsDir {
				if pluginDir.IsDir() {
					config.Plugins = append(config.Plugins, PluginInfo{
						Name:        pluginDir.Name(),
						Source:      filepath.Join(pluginsPath, pluginDir.Name()),
						LastUpdated: info.ModTime(),
					})
				}
			}
		}
	}

	// Check for other plugin managers (Antigen, Zinit, Zplug, etc.)
	pluginManagers := []string{
		"~/.antigen",
		"~/.zinit",
		"~/.zplug",
	}

	for _, manager := range pluginManagers {
		path := expandPath(manager)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			config.Plugins = append(config.Plugins, PluginInfo{
				Name:        filepath.Base(manager),
				Source:      path,
				LastUpdated: info.ModTime(),
			})
		}
	}
}

func detectFishPlugins(config *ShellConfig) {
	fishPluginPath := expandPath("~/.config/fish/conf.d")
	if files, err := os.ReadDir(fishPluginPath); err == nil {
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".fish") {
				info, _ := file.Info()
				config.Plugins = append(config.Plugins, PluginInfo{
					Name:        strings.TrimSuffix(file.Name(), ".fish"),
					Source:      filepath.Join(fishPluginPath, file.Name()),
					LastUpdated: info.ModTime(),
				})
			}
		}
	}
}

func detectBashPlugins(config *ShellConfig) {
	// Check for common bash plugin managers and extensions
	bashPluginPaths := []string{
		"~/.bash_it",
		"~/.local/share/bash-completion",
	}

	for _, path := range bashPluginPaths {
		expandedPath := expandPath(path)
		if info, err := os.Stat(expandedPath); err == nil && info.IsDir() {
			config.Plugins = append(config.Plugins, PluginInfo{
				Name:        filepath.Base(path),
				Source:      expandedPath,
				LastUpdated: info.ModTime(),
			})
		}
	}
}

func analyzeCommandComplexity(data *ShellData) float64 {
	var totalCommands, complexCommands float64

	for _, history := range data.Histories {
		for _, entry := range history {
			totalCommands++

			// Count pipes and redirections
			if strings.Contains(entry.Command, "|") ||
				strings.Contains(entry.Command, ">") ||
				strings.Contains(entry.Command, "<") {
				complexCommands++
			}

			// Count commands with multiple arguments
			if len(strings.Fields(entry.Command)) > 2 {
				complexCommands += 0.5
			}
		}
	}

	if totalCommands == 0 {
		return 0
	}

	return (complexCommands / totalCommands) * 100
}

func generateRecommendations(data *ShellData) []string {
	recommendations := []string{}

	// Analyze shell configuration
	for shell, config := range data.ShellConfigs {
		if len(config.Aliases) < 5 {
			recommendations = append(recommendations,
				fmt.Sprintf("Consider adding more aliases to your %s configuration to improve productivity", shell))
		}

		if len(config.Plugins) < 3 {
			recommendations = append(recommendations,
				fmt.Sprintf("Explore popular %s plugins to enhance your shell experience", shell))
		}
	}

	return recommendations
}

func generateWorkflowTips(data *ShellData) []string {
	tips := []string{}

	// Analyze command patterns
	commonPatterns := analyzeCommandPatterns(data)
	for pattern, count := range commonPatterns {
		if count > 10 {
			tips = append(tips, fmt.Sprintf(
				"You frequently use '%s'. Consider creating an alias for this pattern", pattern))
		}
	}

	return tips
}

func analyzeCommandPatterns(data *ShellData) map[string]int {
	patterns := make(map[string]int)

	for _, history := range data.Histories {
		for _, entry := range history {
			// Look for common command sequences
			parts := strings.Fields(entry.Command)
			if len(parts) > 1 {
				pattern := strings.Join(parts[:2], " ")
				patterns[pattern]++
			}
		}
	}

	return patterns
}

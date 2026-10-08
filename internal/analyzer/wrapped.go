// internal/analyzer/wrapped.go
package analyzer

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// WrappedStats holds the year-in-review numbers behind the Wrapped slides
type WrappedStats struct {
	Year int
	// AllTime is set when the history has no timestamps for Year, so the
	// stats cover the whole history and time-of-day stats are unavailable
	AllTime bool

	TotalCommands  int
	UniqueCommands int
	UniquePrograms int
	Shells         []UsageCount
	UntimedShells  []string // shells whose history has no timestamps

	TopPrograms       []UsageCount
	TopGitSubcommands []UsageCount
	GitCommands       int
	Languages         []UsageCount
	DevOps            []UsageCount
	Editors           []UsageCount
	NewPrograms       []UsageCount // first used during Year

	HasTimes        bool
	HourCounts      [24]int
	WeekdayCounts   [7]int
	MonthCounts     [12]int
	ActiveDays      int
	LongestStreak   int
	BusiestDay      time.Time
	BusiestDayCount int

	SudoCommands   int
	PipeCommands   int
	LongestCommand int
	LongestProgram string
	Typos          []Typo
	TotalTypos     int
	ClearCommands  int
	ExitCommands   int
}

// Typo is a mistyped program name and what was probably meant
type Typo struct {
	Typed string
	Meant string
	Count int
}

// shellBuiltins are commands that never show up in PATH
var shellBuiltins = toSet("cd", "pwd", "echo", "export", "set", "unset", "source", ".", "alias", "unalias",
	"exit", "history", "type", "which", "function", "functions", "funced", "funcsave", "abbr", "fg", "bg",
	"jobs", "read", "eval", "exec", "return", "test", "[", "builtin", "command", "ulimit", "umask", "wait",
	"disown", "pushd", "popd", "dirs", "shopt", "setopt", "unsetopt", "bindkey", "complete", "if", "for",
	"while", "begin", "end", "and", "or", "not", "else", "case", "switch", "true", "false", "printf",
	"local", "declare", "typeset", "readonly", "hash", "rehash", "z", "fish_config", "fish_add_path",
	"conda", "nvm", "pyenv", "rbenv", "sdk", "time", "help", "string", "math", "contains", "status")

var programNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._+-]*$`)

// knownTypos are typos common enough to recognise even without a close match
var knownTypos = map[string]string{
	"sl": "ls", "cd..": "cd ..", "gti": "git", "claer": "clear", "clera": "clear", "clea": "clear",
	"exot": "exit", "eixt": "exit", "sduo": "sudo", "suod": "sudo", "pythoon": "python", "vmi": "vim",
	"nivm": "nvim", "emasc": "emacs", "got": "git", "gut": "git", "dokcer": "docker", "cta": "cat",
}

// RecapYear is the year to recap at the given time. Like any
// year-in-review, January still looks back at the year before.
func RecapYear(now time.Time) int {
	if now.Month() == time.January {
		return now.Year() - 1
	}
	return now.Year()
}

// ComputeWrapped computes the year-in-review stats for the given year. When
// none of the history has timestamps in that year it falls back to all-time
// stats.
func ComputeWrapped(data ShellData, year int) WrappedStats {
	stats := WrappedStats{Year: year}
	all := AllEntries(data)

	var entries []CommandEntry
	firstSeen := make(map[string]time.Time)
	for _, e := range all {
		if e.Timestamp.IsZero() {
			continue
		}
		if first, ok := firstSeen[e.Program]; !ok || e.Timestamp.Before(first) {
			firstSeen[e.Program] = e.Timestamp
		}
		if e.Timestamp.Year() == year {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		stats.AllTime = true
		entries = all
	}

	shellCounts := make(map[string]int)
	for _, shell := range SortedShells(data) {
		timed := false
		for _, e := range data.Histories[shell] {
			timed = timed || !e.Timestamp.IsZero()
			if stats.AllTime || (!e.Timestamp.IsZero() && e.Timestamp.Year() == year) {
				shellCounts[shell]++
			}
		}
		if !timed {
			stats.UntimedShells = append(stats.UntimedShells, shell)
		}
	}
	stats.Shells = SortedCounts(shellCounts)

	programs := make(map[string]int)
	gitSubs := make(map[string]int)
	langs := make(map[string]int)
	devops := make(map[string]int)
	editors := make(map[string]int)
	unique := make(map[string]bool)
	days := make(map[string]int)

	for _, e := range entries {
		stats.TotalCommands++
		unique[e.Command] = true
		if e.Program != "" {
			programs[e.Program]++
		}

		if lang, ok := languagePrograms[e.Program]; ok {
			langs[lang]++
		}
		if devopsPrograms[e.Program] {
			devops[e.Program]++
		}
		if editorPrograms[e.Program] {
			editors[e.Program]++
		}
		if e.Program == "git" {
			stats.GitCommands++
			if sub := Subcommand(e); sub != "" {
				gitSubs[sub]++
			}
		}

		fields := strings.Fields(e.Command)
		if len(fields) > 0 && (fields[0] == "sudo" || fields[0] == "doas") {
			stats.SudoCommands++
		}
		if strings.Contains(e.Command, "|") {
			stats.PipeCommands++
		}
		if n := len(e.Command); n > stats.LongestCommand {
			stats.LongestCommand = n
			stats.LongestProgram = e.Program
		}
		switch e.Program {
		case "clear", "cls":
			stats.ClearCommands++
		case "exit", "logout":
			stats.ExitCommands++
		}

		if !e.Timestamp.IsZero() {
			t := e.Timestamp.Local()
			stats.HourCounts[t.Hour()]++
			stats.WeekdayCounts[t.Weekday()]++
			stats.MonthCounts[t.Month()-1]++
			days[t.Format("2006-01-02")]++
		}
	}

	stats.UniqueCommands = len(unique)
	stats.UniquePrograms = len(programs)
	stats.TopPrograms = TopN(SortedCounts(programs), 10)
	stats.TopGitSubcommands = TopN(SortedCounts(gitSubs), 5)
	stats.Languages = TopN(SortedCounts(langs), 5)
	stats.DevOps = TopN(SortedCounts(devops), 5)
	stats.Editors = TopN(SortedCounts(editors), 3)

	// Imported or synced histories can stamp everything with one time, which
	// makes time-of-day stats meaningless
	stats.HasTimes = len(days) > 1
	stats.ActiveDays = len(days)
	for day, count := range days {
		if count > stats.BusiestDayCount {
			stats.BusiestDayCount = count
			stats.BusiestDay, _ = time.ParseInLocation("2006-01-02", day, time.Local)
		}
	}
	stats.LongestStreak = longestStreak(days)

	if !stats.AllTime {
		newPrograms := make(map[string]int)
		for program, count := range programs {
			if first, ok := firstSeen[program]; ok && first.Year() == year && count >= 3 && isRealProgram(program) {
				newPrograms[program] = count
			}
		}
		stats.NewPrograms = TopN(SortedCounts(newPrograms), 5)
	}

	stats.Typos, stats.TotalTypos = findTypos(programs, data)
	return stats
}

func longestStreak(days map[string]int) int {
	longest := 0
	for day := range days {
		t, err := time.Parse("2006-01-02", day)
		if err != nil {
			continue
		}
		// Only count from the first day of each streak
		if _, ok := days[t.AddDate(0, 0, -1).Format("2006-01-02")]; ok {
			continue
		}
		streak := 1
		for {
			t = t.AddDate(0, 0, 1)
			if _, ok := days[t.Format("2006-01-02")]; !ok {
				break
			}
			streak++
		}
		if streak > longest {
			longest = streak
		}
	}
	return longest
}

func isRealProgram(program string) bool {
	if shellBuiltins[program] {
		return true
	}
	if strings.Contains(program, "/") {
		return false
	}
	_, err := exec.LookPath(program)
	return err == nil
}

// findTypos finds program names that don't exist but are one edit away from
// a program the user runs much more often
func findTypos(programs map[string]int, data ShellData) ([]Typo, int) {
	known := make(map[string]bool)
	for _, config := range data.ShellConfigs {
		for alias := range config.Aliases {
			known[alias] = true
		}
	}
	// fish functions behave like commands but aren't in PATH
	if files, err := os.ReadDir(expandPath("~/.config/fish/functions")); err == nil {
		for _, f := range files {
			known[strings.TrimSuffix(f.Name(), ".fish")] = true
		}
	}

	// Candidates for what was meant: frequently used, real programs
	var targets []UsageCount
	for _, p := range TopN(SortedCounts(programs), 60) {
		if p.Count >= 5 && isRealProgram(p.Name) {
			targets = append(targets, p)
		}
	}

	var typos []Typo
	total := 0
	for program, count := range programs {
		if known[program] || !programNamePattern.MatchString(program) {
			continue
		}
		meant, ok := knownTypos[program]
		if !ok {
			// Two-letter names are one edit away from far too much
			if len(program) < 3 {
				continue
			}
			for _, t := range targets {
				if t.Count >= count*10 && editDistance(program, t.Name) == 1 {
					meant = t.Name
					break
				}
			}
			// Checked last since it searches PATH
			if meant == "" || isRealProgram(program) {
				continue
			}
		}
		typos = append(typos, Typo{Typed: program, Meant: meant, Count: count})
		total += count
	}

	sort.Slice(typos, func(i, j int) bool {
		if typos[i].Count != typos[j].Count {
			return typos[i].Count > typos[j].Count
		}
		return typos[i].Typed < typos[j].Typed
	})
	if len(typos) > 5 {
		typos = typos[:5]
	}
	return typos, total
}

// editDistance is the optimal string alignment distance, which counts a
// swap of two adjacent letters ("gti" -> "git") as a single edit
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	d := make([][]int, len(ra)+1)
	for i := range d {
		d[i] = make([]int, len(rb)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = minInt(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = minInt(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(ra)][len(rb)]
}

// minInt avoids the min builtin, which needs Go 1.21 (CI builds with 1.20)
func minInt(first int, rest ...int) int {
	for _, v := range rest {
		if v < first {
			first = v
		}
	}
	return first
}

// Persona sums up the year in a short label
func (s WrappedStats) Persona() (string, string) {
	timeLabel, timeDesc := "", ""
	if s.HasTimes && s.TotalCommands > 0 {
		night, morning := 0, 0
		for h, c := range s.HourCounts {
			if h >= 22 || h < 4 {
				night += c
			}
			if h >= 5 && h < 9 {
				morning += c
			}
		}
		switch {
		case float64(night)/float64(s.TotalCommands) > 0.25:
			timeLabel, timeDesc = "Night Owl", "you do your best work after dark"
		case float64(morning)/float64(s.TotalCommands) > 0.2:
			timeLabel, timeDesc = "Early Bird", "your terminal is up before most people's coffee"
		}
	}

	toolLabel, toolDesc := "Terminal Explorer", "you roam across a little bit of everything"
	top := make(map[string]bool)
	for _, p := range TopN(s.TopPrograms, 3) {
		top[p.Name] = true
	}
	switch {
	case top["az"] || top["aws"] || top["gcloud"]:
		toolLabel, toolDesc = "Cloud Wrangler", "you herd cloud resources from the command line"
	case top["docker"] || top["kubectl"] || top["podman"] || top["helm"]:
		toolLabel, toolDesc = "Container Captain", "if it runs, you've put it in a container"
	case top["git"]:
		toolLabel, toolDesc = "Git Gardener", "you tend your branches daily"
	case top["vim"] || top["nvim"] || top["emacs"]:
		toolLabel, toolDesc = "Editor Purist", "you live inside your editor"
	case top["ssh"]:
		toolLabel, toolDesc = "Remote Operator", "half your life is spent on other machines"
	case s.TotalCommands > 0 && float64(s.PipeCommands)/float64(s.TotalCommands) > 0.1:
		toolLabel, toolDesc = "Pipe Wizard", "you chain commands like spells"
	}

	if timeLabel == "" {
		return toolLabel, strings.ToUpper(toolDesc[:1]) + toolDesc[1:] + "."
	}
	return timeLabel + " · " + toolLabel,
		strings.ToUpper(timeDesc[:1]) + timeDesc[1:] + ", and " + toolDesc + "."
}

// BusiestHour returns the hour of day with the most commands
func (s WrappedStats) BusiestHour() int {
	best := 0
	for h, c := range s.HourCounts {
		if c > s.HourCounts[best] {
			best = h
		}
	}
	return best
}

// BusiestWeekday returns the day of week with the most commands
func (s WrappedStats) BusiestWeekday() time.Weekday {
	best := 0
	for d, c := range s.WeekdayCounts {
		if c > s.WeekdayCounts[best] {
			best = d
		}
	}
	return time.Weekday(best)
}

// Summary describes the stats for the AI prompt. It only includes program
// names and counts, never full command lines, which may contain secrets.
func (s WrappedStats) Summary() string {
	var b strings.Builder
	period := fmt.Sprintf("the year %d", s.Year)
	if s.AllTime {
		period = "their whole shell history (no timestamps available)"
	}
	fmt.Fprintf(&b, "Period: %s\n", period)
	fmt.Fprintf(&b, "Total commands: %d (%d unique command lines, %d different programs)\n",
		s.TotalCommands, s.UniqueCommands, s.UniquePrograms)
	fmt.Fprintf(&b, "Shells: %s\n", formatCounts(s.Shells))
	fmt.Fprintf(&b, "Top programs: %s\n", formatCounts(s.TopPrograms))
	if s.GitCommands > 0 {
		fmt.Fprintf(&b, "Git commands: %d, top subcommands: %s\n", s.GitCommands, formatCounts(s.TopGitSubcommands))
	}
	if len(s.Languages) > 0 {
		fmt.Fprintf(&b, "Languages (by tool usage): %s\n", formatCounts(s.Languages))
	}
	if len(s.DevOps) > 0 {
		fmt.Fprintf(&b, "DevOps/cloud tools: %s\n", formatCounts(s.DevOps))
	}
	if len(s.Editors) > 0 {
		fmt.Fprintf(&b, "Editors: %s\n", formatCounts(s.Editors))
	}
	if len(s.NewPrograms) > 0 {
		fmt.Fprintf(&b, "Programs first used this year: %s\n", formatCounts(s.NewPrograms))
	}
	if s.HasTimes {
		peak := s.BusiestHour()
		fmt.Fprintf(&b, "Busiest hour: %02d:00 (%s), busiest weekday: %s\n", peak, partOfDay(peak), s.BusiestWeekday())
		// Spelled out, because models misread raw hourly counts
		fmt.Fprintf(&b, "Share of commands by time of day: %s\n", s.timeOfDayShares())
		fmt.Fprintf(&b, "Active days: %d, longest streak of consecutive active days: %d\n", s.ActiveDays, s.LongestStreak)
	}
	fmt.Fprintf(&b, "sudo commands: %d, commands with pipes: %d, longest command: %d characters\n",
		s.SudoCommands, s.PipeCommands, s.LongestCommand)
	fmt.Fprintf(&b, "Times they ran clear: %d\n", s.ClearCommands)
	if len(s.Typos) > 0 {
		var typos []string
		for _, t := range s.Typos {
			typos = append(typos, fmt.Sprintf("%q instead of %q (%dx)", t.Typed, t.Meant, t.Count))
		}
		fmt.Fprintf(&b, "Typos (%d total): %s\n", s.TotalTypos, strings.Join(typos, ", "))
	}
	return b.String()
}

// timeOfDay splits the day into named parts, as [from, to) hours
var timeOfDay = []struct {
	name     string
	from, to int
}{
	{"morning (05-12)", 5, 12},
	{"afternoon (12-17)", 12, 17},
	{"evening (17-22)", 17, 22},
	{"night (22-05)", 22, 29},
}

func partOfDay(hour int) string {
	for _, part := range timeOfDay {
		if (hour >= part.from && hour < part.to) || (hour+24 >= part.from && hour+24 < part.to) {
			return strings.Fields(part.name)[0]
		}
	}
	return ""
}

func (s WrappedStats) timeOfDayShares() string {
	var parts []string
	for _, part := range timeOfDay {
		count := 0
		for h := part.from; h < part.to; h++ {
			count += s.HourCounts[h%24]
		}
		parts = append(parts, fmt.Sprintf("%s %d%%", part.name, count*100/maxInt(s.TotalCommands, 1)))
	}
	return strings.Join(parts, ", ")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatCounts(counts []UsageCount) string {
	parts := make([]string, 0, len(counts))
	for _, c := range counts {
		parts = append(parts, fmt.Sprintf("%s (%d)", c.Name, c.Count))
	}
	return strings.Join(parts, ", ")
}

// internal/analyzer/analyzer.go
package analyzer

import (
	"sort"
	"time"
)

// ShellData contains all the analyzed shell data
type ShellData struct {
	Histories    map[string][]CommandEntry
	CommonCmds   map[string]int
	TimePatterns map[string]int
	Insights     DetailedInsights
	ShellConfigs map[string]ShellConfig
}

// CommandEntry represents a single command entry in the shell history
type CommandEntry struct {
	Command    string
	Program    string    // the executable being run, e.g. "git" for "sudo git pull"
	Timestamp  time.Time // zero when the history format has no timestamps
	Count      int
	Categories []string
}

// UsageCount is a name with the number of times it was used
type UsageCount struct {
	Name  string
	Count int
}

// DetailedInsights contains detailed insights about the user's shell usage
type DetailedInsights struct {
	TechnicalProfile TechProfile
	WorkPatterns     WorkPatterns
	ToolUsage        ToolUsage
}

// TechProfile contains technical profile information
type TechProfile struct {
	PrimaryRole     string
	SecondarySkills []UsageCount // skill areas by number of commands
	TechStack       []string
	TopTech         []UsageCount
}

// WorkPatterns contains work pattern information
type WorkPatterns struct {
	PeakHours       []int
	HourlyActivity  [24]int // commands per hour of day, for timestamped history only
	CommonWorkflows []string
	Productivity    map[string]float64
}

// ToolUsage contains tool usage statistics
type ToolUsage struct {
	Editors    map[string]int
	Languages  map[string]int
	BuildTools map[string]int
	DevOps     map[string]int
}

// ShellConfig contains shell configuration information
type ShellConfig struct {
	ConfigFiles map[string]ConfigInfo
	Plugins     []PluginInfo
	Aliases     map[string]string
	Environment map[string]string
}

// ConfigInfo contains information about a configuration file
type ConfigInfo struct {
	Path     string
	Modified time.Time
	Content  string
}

// PluginInfo contains information about a plugin
type PluginInfo struct {
	Name        string
	Source      string
	LastUpdated time.Time
}

// InitShellData initializes an empty ShellData structure
func InitShellData() ShellData {
	return ShellData{
		Histories:    make(map[string][]CommandEntry),
		CommonCmds:   make(map[string]int),
		TimePatterns: make(map[string]int),
		Insights: DetailedInsights{
			WorkPatterns: WorkPatterns{
				Productivity: make(map[string]float64),
			},
			ToolUsage: ToolUsage{
				Editors:    make(map[string]int),
				Languages:  make(map[string]int),
				BuildTools: make(map[string]int),
				DevOps:     make(map[string]int),
			},
		},
		ShellConfigs: make(map[string]ShellConfig),
	}
}

// SortedCounts converts a usage map into a slice sorted by count, highest first
func SortedCounts(usage map[string]int) []UsageCount {
	counts := make([]UsageCount, 0, len(usage))
	for name, count := range usage {
		counts = append(counts, UsageCount{name, count})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].Count != counts[j].Count {
			return counts[i].Count > counts[j].Count
		}
		return counts[i].Name < counts[j].Name
	})
	return counts
}

// TopN returns at most the first n counts
func TopN(counts []UsageCount, n int) []UsageCount {
	if len(counts) > n {
		return counts[:n]
	}
	return counts
}

// internal/render/wrapped.go
package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/k8au-shell-analyzer/internal/analyzer"
	"github.com/ksauraj/k8au-shell-analyzer/internal/types"
)

// slideColors gives every slide its own accent color
var slideColors = []lipgloss.Color{"205", "86", "214", "141", "39", "203", "120", "177", "220"}

// slideWidth is the card width for a terminal of the given width
func slideWidth(width int) int {
	w := width - 4
	if w > 72 {
		w = 72
	}
	if w < 36 {
		w = 36
	}
	return w
}

// slideInner is the usable text width inside a slide card
func slideInner(width int) int {
	return slideWidth(width) - 2 - 6 // border and padding
}

func splitLines(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

// BuildWrappedSlides turns the year's stats into Wrapped slides sized for a
// terminal of the given width. Numbers count up and charts grow as progress
// goes from 0 to 1.
func BuildWrappedSlides(s analyzer.WrappedStats, width int, progress float64) []types.Slide {
	inner := slideInner(width)
	n := func(v int) string { return formatInt(scale(v, progress)) }

	if s.TotalCommands == 0 {
		return []types.Slide{{
			Title:    "Shell Wrapped",
			Headline: "Nothing to wrap yet",
			Lines:    []string{"No shell history was found for bash, zsh or fish."},
			Art:      typingArt("history"),
		}}
	}

	var slides []types.Slide

	// Intro
	intro := types.Slide{
		Title:    fmt.Sprintf("Your %d in the Terminal", s.Year),
		Headline: fmt.Sprintf("%s commands", n(s.TotalCommands)),
		Art:      typingArt("wrapped"),
	}
	if s.AllTime {
		intro.Title = "Your Shell History, Wrapped"
		intro.Footer = fmt.Sprintf("None of your history is timestamped in %d, so this covers everything found.", s.Year)
	} else if len(s.UntimedShells) > 0 {
		intro.Footer = fmt.Sprintf("Your %s history has no timestamps, so it isn't counted here.",
			strings.Join(s.UntimedShells, " and "))
	}
	if s.HasTimes {
		intro.Lines = append(intro.Lines, fmt.Sprintf("typed across %s active days", n(s.ActiveDays)))
	}
	intro.Lines = append(intro.Lines,
		fmt.Sprintf("%s unique command lines · %s different programs", n(s.UniqueCommands), n(s.UniquePrograms)),
		"", "Shells:")
	intro.Lines = append(intro.Lines, splitLines(countRows(s.Shells, 0, "", inner, progress))...)
	slides = append(slides, intro)

	// Top commands
	if len(s.TopPrograms) > 0 {
		top := s.TopPrograms[0]
		slide := types.Slide{
			Title:    "Your Top Commands",
			Headline: fmt.Sprintf("%s was your #1 command", top.Name),
			Lines:    splitLines(countRows(s.TopPrograms, 5, "", inner, progress)),
			Art:      trophyArt,
		}
		if s.HasTimes && s.ActiveDays > 0 {
			perDay := (top.Count + s.ActiveDays/2) / s.ActiveDays
			slide.Quotes = []string{fmt.Sprintf("%s runs. That's about %d for every day you showed up.",
				formatInt(top.Count), perDay)}
		} else {
			slide.Quotes = []string{fmt.Sprintf("You ran it %s times. It's basically muscle memory now.",
				formatInt(top.Count))}
		}
		slides = append(slides, slide)
	}

	// Time of day
	if s.HasTimes {
		peak := s.BusiestHour()
		night := 0
		for h, c := range s.HourCounts {
			if h >= 22 || h < 4 {
				night += c
			}
		}
		slide := types.Slide{
			Title:    "When You're in the Zone",
			Headline: fmt.Sprintf("Peak hour: %02d:00", peak),
			Lines:    hourChart(s.HourCounts, progress),
			Art:      clockArt,
		}
		slide.Lines = append(slide.Lines, "",
			fmt.Sprintf("Busiest weekday: %s", s.BusiestWeekday()),
			fmt.Sprintf("Late-night commands (10pm-4am): %s (%s%%)", n(night), n(night*100/s.TotalCommands)))
		slide.Quotes = []string{peakHourQuip(peak)}
		slides = append(slides, slide)
	}

	// Month by month
	if s.HasTimes && !s.AllTime {
		busiestMonth := 0
		for m, c := range s.MonthCounts {
			if c > s.MonthCounts[busiestMonth] {
				busiestMonth = m
			}
		}
		slide := types.Slide{
			Title:    "Your Year in Motion",
			Headline: fmt.Sprintf("Busiest month: %s", time.Month(busiestMonth+1)),
			Lines:    monthChart(s.MonthCounts, progress),
			Art:      calendarArt,
		}
		slide.Lines = append(slide.Lines, "",
			fmt.Sprintf("Biggest day: %s with %s commands", s.BusiestDay.Format("Mon, Jan 2"), n(s.BusiestDayCount)),
			fmt.Sprintf("Longest streak: %s days in a row", n(s.LongestStreak)))
		slides = append(slides, slide)
	}

	// Git
	if s.GitCommands > 0 {
		slide := types.Slide{
			Title:    "Your Git Story",
			Headline: fmt.Sprintf("%s git commands", n(s.GitCommands)),
			Lines:    splitLines(countRows(s.TopGitSubcommands, 5, "", inner, progress)),
			Art:      gitArt,
		}
		commits := 0
		for _, sub := range s.TopGitSubcommands {
			if sub.Name == "commit" {
				commits = sub.Count
			}
		}
		if commits > 0 {
			slide.Quotes = []string{fmt.Sprintf("%s commits. Hopefully not all of them called \"fix\".", formatInt(commits))}
		} else {
			slide.Quotes = []string{"You read more history than you write."}
		}
		slides = append(slides, slide)
	}

	// Stack
	if len(s.Languages)+len(s.DevOps)+len(s.Editors) > 0 {
		slide := types.Slide{Title: "Your Stack", Art: stackArt}
		var names []string
		for _, c := range analyzer.TopN(s.Languages, 3) {
			names = append(names, c.Name)
		}
		if len(names) == 0 {
			for _, c := range analyzer.TopN(s.DevOps, 3) {
				names = append(names, c.Name)
			}
		}
		slide.Headline = strings.Join(names, " · ")
		if len(s.Languages) > 0 {
			slide.Lines = append(slide.Lines, "Languages:")
			slide.Lines = append(slide.Lines, splitLines(countRows(s.Languages, 3, "", inner, progress))...)
		}
		if len(s.DevOps) > 0 {
			slide.Lines = append(slide.Lines, "DevOps & Cloud:")
			slide.Lines = append(slide.Lines, splitLines(countRows(s.DevOps, 3, "", inner, progress))...)
		}
		if len(s.Editors) > 0 {
			slide.Lines = append(slide.Lines, "",
				fmt.Sprintf("Editor of choice: %s (%s launches)", s.Editors[0].Name, n(s.Editors[0].Count)))
		}
		slides = append(slides, slide)
	}

	// New tools
	if len(s.NewPrograms) > 0 {
		slides = append(slides, types.Slide{
			Title:    "New This Year",
			Headline: "Fresh additions to your toolbox",
			Lines:    splitLines(countRows(s.NewPrograms, 5, "", inner, progress)),
			Quotes:   []string{fmt.Sprintf("Welcome to the family, %s.", s.NewPrograms[0].Name)},
			Art:      newArt,
		})
	}

	// Habits and typos
	habits := types.Slide{
		Title: "Power Moves & Oopsies",
		Lines: []string{
			fmt.Sprintf("sudo     %s times you had to ask nicely", n(s.SudoCommands)),
			fmt.Sprintf("pipes    %s commands chained with |", n(s.PipeCommands)),
			fmt.Sprintf("longest  %s characters (%s)", n(s.LongestCommand), s.LongestProgram),
			fmt.Sprintf("clear    %s fresh starts", n(s.ClearCommands)),
		},
		Art: typoArt("claer", "clear"),
	}
	if s.TotalTypos > 0 {
		habits.Headline = fmt.Sprintf("%s typos caught", n(s.TotalTypos))
		habits.Lines = append(habits.Lines, "", "Typo hall of fame:")
		for _, t := range s.Typos {
			habits.Lines = append(habits.Lines, fmt.Sprintf("  %s → %s (%d×)", t.Typed, t.Meant, t.Count))
		}
		habits.Quotes = []string{fmt.Sprintf("%q... so close to %q.", s.Typos[0].Typed, s.Typos[0].Meant)}
		habits.Art = typoArt(s.Typos[0].Typed, s.Typos[0].Meant)
	} else {
		habits.Headline = "Zero typos. Suspiciously perfect."
	}
	slides = append(slides, habits)

	// Persona
	label, description := s.Persona()
	slides = append(slides, types.Slide{
		Title:    "Your Terminal Persona",
		Headline: label,
		Lines:    []string{description},
		Art:      faceArt,
	})

	return slides
}

func peakHourQuip(hour int) string {
	switch {
	case hour < 5:
		return "Sleep is just a background process for you."
	case hour < 12:
		return "Morning commits hit different."
	case hour < 18:
		return "Peak focus, right after lunch."
	case hour < 22:
		return "Evenings are for shipping."
	default:
		return "The best bugs come out at night."
	}
}

// RenderSlide renders one Wrapped slide centered in a width x height area.
// frame counts animation ticks since the slide appeared and drives its art.
func RenderSlide(slide types.Slide, label string, index, total int, autoplay bool, frame, width, height int) string {
	accent := slideColors[index%len(slideColors)]
	inner := slideInner(width)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// Story-style progress bar
	var progress string
	if segment := (inner - (total - 1)) / total; segment >= 1 {
		var segments []string
		for i := 0; i < total; i++ {
			style := muted
			if i <= index {
				style = lipgloss.NewStyle().Foreground(accent)
			}
			segments = append(segments, style.Render(strings.Repeat("━", segment)))
		}
		progress = strings.Join(segments, " ")
	}

	if slide.AI {
		label += " · AI"
	}
	counter := fmt.Sprintf("%d/%d · auto", index+1, total)
	if !autoplay {
		counter = fmt.Sprintf("%d/%d · paused", index+1, total)
	}
	gap := inner - lipgloss.Width(label) - lipgloss.Width(counter)
	if gap < 1 {
		gap = 1
	}
	top := muted.Render(label + strings.Repeat(" ", gap) + counter)

	head := lipgloss.NewStyle().Bold(true).Foreground(accent).Render(slide.Title)
	if slide.Headline != "" {
		head += "\n\n" + lipgloss.NewStyle().Bold(true).Render(slide.Headline)
	}
	// The art sits to the right of the title when there's room for both
	if len(slide.Art) > 0 {
		art := artFrame(slide.Art, frame/ArtTicks)
		if textWidth := inner - lipgloss.Width(art) - 2; textWidth >= 28 {
			head = lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Width(textWidth).Render(head), "  ",
				lipgloss.NewStyle().Foreground(accent).Render(art))
		}
	}

	parts := []string{progress, top, "", head}
	if len(slide.Lines) > 0 {
		parts = append(parts, "")
		parts = append(parts, slide.Lines...)
	}
	if len(slide.Quotes) > 0 {
		parts = append(parts, "")
		quote := lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("250"))
		for _, q := range slide.Quotes {
			parts = append(parts, quote.Render("“"+q+"”"))
		}
	}
	if slide.Footer != "" {
		parts = append(parts, "", muted.Render(slide.Footer))
	}

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 3).
		Width(slideWidth(width) - 2).
		Render(strings.Join(parts, "\n"))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card)
}

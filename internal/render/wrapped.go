// internal/render/wrapped.go
package render

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ksauraj/shellrecap/internal/ai"
	"github.com/ksauraj/shellrecap/internal/analyzer"
	"github.com/ksauraj/shellrecap/internal/theme"
	"github.com/ksauraj/shellrecap/internal/types"
)

// slideWidth is the card width for a terminal of the given width, the
// same as a full-width card on the other tabs
func slideWidth(width int) int {
	return maxInt(gridWidth(width), 36)
}

// slideInner is the usable text width inside a slide card
func slideInner(width int) int {
	return slideWidth(width) - 2 - 6 // border and padding
}

// BuildWrappedSlides turns the year's stats into Wrapped slides sized for a
// terminal of the given width. Numbers count up and charts grow with the
// reveal animation.
func BuildWrappedSlides(s analyzer.WrappedStats, width int, anim Anim) []types.Slide {
	inner := slideInner(width)
	n := func(v int) string { return formatInt(scale(v, anim.Progress)) }

	var slides []types.Slide
	// tone is the tonal range of the accent of the slide being built
	tone := func() *theme.Tone {
		return theme.Accents[len(slides)%len(theme.Accents)].Tone
	}
	// rows draws bars in the accent of the slide being built
	rowsIn := func(counts []analyzer.UsageCount, layout barLayout) []string {
		lines := countRows(counts, layout, tone(), anim)
		for i := range lines {
			lines[i] = "  " + lines[i]
		}
		return lines
	}
	rows := func(counts []analyzer.UsageCount, limit int) []string {
		counts = analyzer.TopN(counts, limit)
		return rowsIn(counts, newBarLayout(inner-2, counts))
	}

	if s.TotalCommands == 0 {
		return []types.Slide{{
			Title:    "shellrecap",
			Headline: "Nothing to recap yet",
			Lines:    noHistoryLines(),
			Art:      typingArt("history"),
		}}
	}

	// Intro
	intro := types.Slide{
		Title:    fmt.Sprintf("Your %d in the Terminal", s.Year),
		Headline: fmt.Sprintf("%s commands", n(s.TotalCommands)),
		Art:      typingArt("shellrecap"),
	}
	if s.AllTime {
		intro.Title = "Your Shell History, Recapped"
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
	intro.Lines = append(intro.Lines, rows(s.Shells, len(s.Shells))...)
	slides = append(slides, intro)

	// Top commands
	if len(s.TopPrograms) > 0 {
		top := s.TopPrograms[0]
		slide := types.Slide{
			Title:    "Your Top Commands",
			Headline: fmt.Sprintf("%s was your #1 command", top.Name),
			Lines:    rows(s.TopPrograms, 5),
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
			Lines:    hourChart(s.HourCounts, inner, 4, tone(), anim),
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
			Lines:    monthChart(s.MonthCounts, inner, 4, tone(), anim),
			Art:      calendarArt,
		}
		slide.Lines = append(slide.Lines, "",
			fmt.Sprintf("Biggest day: %s with %s commands", s.BusiestDay.Format("Mon, Jan 2"), n(s.BusiestDayCount)))
		slides = append(slides, slide)

		// Every day of the year, as a contribution graph
		days := daysInYear(s.Year, time.Now())
		heat := types.Slide{
			Title:    "Your Year in Squares",
			Headline: fmt.Sprintf("Active on %s of %s days", n(s.ActiveDays), formatInt(days)),
			Lines:    heatmap(s.DayCounts, s.Year, time.Now(), inner, tone(), anim),
			Art:      squaresArt,
		}
		heat.Lines = append(heat.Lines, "",
			fmt.Sprintf("Longest streak: %s days in a row", n(s.LongestStreak)))
		heat.Quotes = []string{streakQuip(s.ActiveDays, days)}
		slides = append(slides, heat)
	}

	// Git
	if s.GitCommands > 0 {
		slide := types.Slide{
			Title:    "Your Git Story",
			Headline: fmt.Sprintf("%s git commands", n(s.GitCommands)),
			Lines:    rows(s.TopGitSubcommands, 5),
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
		// One layout for both lists, so their bars start at the same column
		languages, devops := analyzer.TopN(s.Languages, 3), analyzer.TopN(s.DevOps, 3)
		layout := newBarLayout(inner-2, languages, devops)
		if len(languages) > 0 {
			slide.Lines = append(slide.Lines, "Languages:")
			slide.Lines = append(slide.Lines, rowsIn(languages, layout)...)
		}
		if len(devops) > 0 {
			slide.Lines = append(slide.Lines, "DevOps & Cloud:")
			slide.Lines = append(slide.Lines, rowsIn(devops, layout)...)
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
			Lines:    rows(s.NewPrograms, 5),
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

// AISlides turns AI-written sections into slides, each with the footer
func AISlides(sections []ai.Section, footer string) []types.Slide {
	slides := make([]types.Slide, 0, len(sections))
	for _, section := range sections {
		var quotes []string
		for _, q := range section.Quotes {
			quotes = append(quotes, ai.CleanText(q))
		}
		slides = append(slides, types.Slide{
			Title:  ai.CleanText(section.Title),
			Lines:  []string{ai.CleanText(section.Description)},
			Quotes: quotes,
			Footer: footer,
			Art:    RobotArt(),
			AI:     true,
		})
	}
	return slides
}

// daysInYear is how many days of year have gone by at now, all of them
// once the year is over
func daysInYear(year int, now time.Time) int {
	first := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.Local)
	if today := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local); today.Before(end) {
		end = today
	}
	return maxInt(int(end.Sub(first).Hours()/24+0.5), 1)
}

func streakQuip(active, days int) string {
	switch share := active * 100 / maxInt(days, 1); {
	case share >= 80:
		return "Weekends? Never heard of them."
	case share >= 50:
		return "More color than grey. Consistency looks good on you."
	case share >= 20:
		return "You show up when it counts."
	default:
		return "Quality over quantity, right?"
	}
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

// SlideAnim is how far a slide's animations have got
type SlideAnim struct {
	// Frame counts animation ticks since the slide appeared and drives
	// its art
	Frame int
	// Intro is how far the slide's entrance has got, from 0 to 1: the
	// title types itself, the headline glides in, the lines light up one
	// after another and the quote fades in
	Intro float64
	// Played is how much of its time on screen the slide has used up,
	// from 0 to 1, which fills its segment of the progress bar
	Played float64
}

// Settled is a slide with every animation finished
func Settled(frame int) SlideAnim {
	return SlideAnim{Frame: frame, Intro: 1, Played: 1}
}

// ansiCodes match the color codes lipgloss writes
var ansiCodes = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stage maps the part of the entrance between from and to onto 0 to 1
func stage(intro, from, to float64) float64 {
	return math.Max(0, math.Min(1, (intro-from)/(to-from)))
}

// fadeIn renders text that fades in from the background as t goes from 0
// to 1, ending in the given style. Faded text is drawn without its own
// colors, so it keeps its size and the slide doesn't jump around.
func fadeIn(text string, t float64, style lipgloss.Style) string {
	switch {
	case t >= 1:
		return style.Render(text)
	case t >= 0.5:
		return theme.Faint.Render(ansiCodes.ReplaceAllString(text, ""))
	default:
		return theme.Track.Render(ansiCodes.ReplaceAllString(text, ""))
	}
}

// RenderSlide renders one Recap slide, anchored at the top left like the
// cards on the other tabs. status follows the slide counter, e.g. "auto".
func RenderSlide(slide types.Slide, label, status string, index, total, width int, anim SlideAnim) string {
	accent := theme.Accents[index%len(theme.Accents)].Color
	inner := slideInner(width)
	muted := theme.Faint

	// Story-style progress bar, the current segment filling up as the
	// slide plays
	var progress string
	if segment := (inner - (total - 1)) / total; segment >= 1 {
		var segments []string
		for i := 0; i < total; i++ {
			filled := 0
			switch {
			case i < index:
				filled = segment
			case i == index:
				filled = maxInt(int(math.Round(anim.Played*float64(segment))), 1)
			}
			filled = minInt(filled, segment)
			segments = append(segments, lipgloss.NewStyle().Foreground(accent).Render(strings.Repeat("━", filled))+
				theme.Track.Render(strings.Repeat("━", segment-filled)))
		}
		progress = strings.Join(segments, " ")
	}

	if slide.AI {
		label += " · AI"
	}
	counter := fmt.Sprintf("%d/%d", index+1, total)
	if status != "" {
		counter += " · " + status
	}
	gap := inner - lipgloss.Width(label) - lipgloss.Width(counter)
	if gap < 1 {
		gap = 1
	}
	top := muted.Render(label + strings.Repeat(" ", gap) + counter)

	// The art sits to the right of the title when there's room for both
	var art string
	textWidth := inner
	if len(slide.Art) > 0 {
		art = artFrame(slide.Art, anim.Frame/ArtTicks)
		if w := inner - lipgloss.Width(art) - 2; w >= 28 {
			textWidth = w
		} else {
			art = ""
		}
	}

	// The title types itself, with a cursor while it's typing
	title := []rune(slide.Title)
	typed := int(math.Round(stage(anim.Intro, 0, 0.3) * float64(len(title))))
	head := lipgloss.NewStyle().Bold(true).Foreground(accent).Render(string(title[:typed]))
	if typed < len(title) {
		head += lipgloss.NewStyle().Foreground(accent).Render("▌")
	}
	// The headline glides in from the right as it fades in
	if slide.Headline != "" {
		t := stage(anim.Intro, 0.15, 0.45)
		glide := minInt(int(math.Round((1-t)*6)), maxInt(textWidth-lipgloss.Width(slide.Headline), 0))
		head += "\n\n" + strings.Repeat(" ", glide) + fadeIn(slide.Headline, t, theme.Bold)
	}
	if art != "" {
		head = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(textWidth).Render(head), "  ",
			lipgloss.NewStyle().Foreground(accent).Render(art))
	}

	parts := []string{progress, top, "", head}
	if len(slide.Lines) > 0 {
		parts = append(parts, "")
		// Lines light up one after another
		for i, line := range slide.Lines {
			at := 0.25 + 0.35*float64(i)/float64(len(slide.Lines))
			parts = append(parts, fadeIn(line, stage(anim.Intro, at, at+0.08), theme.Normal))
		}
	}
	if len(slide.Quotes) > 0 {
		parts = append(parts, "")
		quote := lipgloss.NewStyle().Italic(true).Foreground(theme.Subtext)
		t := stage(anim.Intro, 0.75, 1)
		for _, q := range slide.Quotes {
			parts = append(parts, fadeIn("“"+q+"”", t, quote))
		}
	}
	if slide.Footer != "" {
		parts = append(parts, "", muted.Render(slide.Footer))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(1, 3).
		Width(slideWidth(width) - 2).
		Render(strings.Join(parts, "\n"))
}

// noHistoryLines say where shellrecap looked for history
func noHistoryLines() []string {
	lines := []string{"No shell history was found. Looked for " + analyzer.SupportedShells() + "."}
	if tip := analyzer.HistoryTip(); tip != "" {
		lines = append(lines, tip)
	}
	return lines
}

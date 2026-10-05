// internal/render/art.go
package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Every animation here is plain ASCII. An animation is a list of frames,
// each a multi-line string; frames are padded to a common size when drawn.

// ArtTicks is how many animation ticks each art frame stays on screen
const ArtTicks = 3

// Spinner returns a classic ASCII spinner for the given tick
func Spinner(frame int) string {
	return string(`|/-\`[frame%4])
}

// bannerLetters spell shellrecap in the figlet "small" font
var bannerLetters = func() [][]string {
	glyphs := map[rune][]string{
		's': {`    `, ` ___`, `(_-<`, `/__/`, `    `},
		'h': {` _    `, `| |_  `, `| ' \ `, `|_||_|`, `      `},
		'e': {`     `, ` ___ `, `/ -_)`, `\___|`, `     `},
		'l': {` _ `, `| |`, `| |`, `|_|`, `   `},
		'r': {`     `, ` _ _ `, `| '_|`, `|_|  `, `     `},
		'c': {`    `, ` __ `, `/ _|`, `\__|`, `    `},
		'a': {`      `, ` __ _ `, `/ _` + "`" + ` |`, `\__,_|`, `      `},
		'p': {`      `, ` _ __ `, `| '_ \`, `| .__/`, `|_|   `},
	}
	var letters [][]string
	for _, ch := range "shellrecap" {
		letters = append(letters, glyphs[ch])
	}
	return letters
}()

// bannerWave is the gradient that sweeps across the banner
var bannerWave = []lipgloss.Color{"24", "31", "38", "45", "51", "87", "123", "87", "51", "45", "38", "31"}

var loadingMessages = []string{
	"reading your shell history",
	"counting commands",
	"looking for your favourite tools",
	"spotting typos",
	"working out your peak hours",
}

// RenderLoading renders the animated splash shown while history is analyzed
func RenderLoading(frame int, refreshing bool, width, height int) string {
	lines := make([]string, len(bannerLetters[0]))
	for i := range lines {
		var row []string
		for _, letter := range bannerLetters {
			row = append(row, letter[i])
		}
		lines[i] = strings.Join(row, "")
	}

	// Color each column by a gradient that moves with the frame
	var banner strings.Builder
	for i, line := range lines {
		if i > 0 {
			banner.WriteString("\n")
		}
		for col, ch := range line {
			c := bannerWave[((col-frame*2)%len(bannerWave)+len(bannerWave))%len(bannerWave)]
			banner.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(string(ch)))
		}
	}

	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render("your year in the terminal")

	// A block bouncing back and forth
	const track, block = 24, 6
	pos := frame % (2 * (track - block))
	if pos > track-block {
		pos = 2*(track-block) - pos
	}
	bar := "[" + strings.Repeat(" ", pos) + strings.Repeat("=", block) + strings.Repeat(" ", track-block-pos) + "]"

	message := loadingMessages[(frame/6)%len(loadingMessages)]
	if refreshing && frame < 6 {
		message = "refreshing everything, skipping the cache"
	}
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).
		Render(Spinner(frame) + " " + message + strings.Repeat(".", frame/2%4))

	content := lipgloss.JoinVertical(lipgloss.Center,
		banner.String(), "", subtitle, "", lipgloss.NewStyle().Foreground(lipgloss.Color("45")).Render(bar), "", status)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// artFrame picks frame i of an animation, padded to the size of the
// largest frame so the art doesn't jitter
func artFrame(frames []string, i int) string {
	width, height := 0, 0
	for _, f := range frames {
		lines := strings.Split(f, "\n")
		height = maxInt(height, len(lines))
		for _, l := range lines {
			width = maxInt(width, len(l))
		}
	}
	lines := strings.Split(frames[i%len(frames)], "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	for j := range lines {
		lines[j] += strings.Repeat(" ", width-len(lines[j]))
	}
	return strings.Join(lines, "\n")
}

func repeatFrame(frame string, n int) []string {
	frames := make([]string, n)
	for i := range frames {
		frames[i] = frame
	}
	return frames
}

// terminalBox draws text inside a tiny terminal window
func terminalBox(text string, width int) string {
	return "." + strings.Repeat("-", width+2) + ".\n" +
		"| " + text + strings.Repeat(" ", width-len(text)) + " |\n" +
		"'" + strings.Repeat("-", width+2) + "'"
}

// typingArt types a command at a prompt, then blinks the cursor
func typingArt(command string) []string {
	width := len(command) + 3
	var frames []string
	for i := 0; i <= len(command); i++ {
		frames = append(frames, terminalBox("$ "+command[:i]+"_", width))
	}
	for i := 0; i < 4; i++ {
		frames = append(frames, terminalBox("$ "+command+" ", width), terminalBox("$ "+command+"_", width))
	}
	return frames
}

// typoArt types a typo, notices, backspaces and fixes it
func typoArt(typed, meant string) []string {
	width := maxInt(len(typed), len(meant)) + 3
	var frames []string
	show := func(text string, n int) {
		frames = append(frames, repeatFrame(terminalBox("$ "+text+"_", width), n)...)
	}
	for i := 0; i <= len(typed); i++ {
		show(typed[:i], 1)
	}
	show(typed, 4) // the moment of realisation

	prefix := 0
	for prefix < len(typed) && prefix < len(meant) && typed[prefix] == meant[prefix] {
		prefix++
	}
	for i := len(typed) - 1; i >= prefix; i-- {
		show(typed[:i], 1)
	}
	for i := prefix + 1; i <= len(meant); i++ {
		show(meant[:i], 1)
	}
	show(meant, 6)
	return frames
}

// sparkleArt twinkles stars at the given positions around a base picture
func sparkleArt(base []string, positions [][2]int) []string {
	var frames []string
	for _, p := range positions {
		lines := make([]string, len(base))
		copy(lines, base)
		if p[0] >= 0 {
			row := []byte(lines[p[0]])
			row[p[1]] = '*'
			lines[p[0]] = string(row)
		}
		frames = append(frames, strings.Join(lines, "\n"), strings.Join(lines, "\n"))
	}
	return frames
}

// revealArt draws a picture column by column, then holds it
func revealArt(picture string) []string {
	lines := strings.Split(picture, "\n")
	width := 0
	for _, l := range lines {
		width = maxInt(width, len(l))
	}
	var frames []string
	for c := 0; c <= width; c++ {
		cut := make([]string, len(lines))
		for i, l := range lines {
			cut[i] = l[:minInt(c, len(l))]
		}
		frames = append(frames, strings.Join(cut, "\n"))
	}
	return append(frames, repeatFrame(picture, 8)...)
}

// marqueeArt scrolls a pattern through a small window, like an activity graph
func marqueeArt(pattern string, width int) []string {
	var frames []string
	loop := pattern + pattern
	for i := 0; i < len(pattern); i++ {
		frames = append(frames, terminalBox(loop[i:i+width], width))
	}
	return frames
}

var (
	trophyArt = sparkleArt(
		[]string{`  '\_1_/'  `, `    |_|    `, `   [___]   `},
		[][2]int{{0, 0}, {2, 10}, {-1, 0}, {0, 10}, {2, 0}, {-1, 0}},
	)

	clockArt = func() []string {
		var frames []string
		for _, hand := range []string{"|", "/", "-", `\`} {
			frames = append(frames, " .---.\n(  "+hand+"  )\n '---'")
		}
		return frames
	}()

	calendarArt = marqueeArt("#:.##:#..#:##.:#.#:.", 10)

	gitArt = revealArt("o--o--o--o\n    \\   /\n     o-o")

	stackArt = func() []string {
		rows := []string{"  [##]  ", " [####] ", "[######]"}
		var frames []string
		for n := 0; n <= len(rows); n++ {
			visible := make([]string, len(rows))
			for i := range rows {
				if i >= len(rows)-n {
					visible[i] = rows[i]
				}
			}
			frames = append(frames, repeatFrame(strings.Join(visible, "\n"), 2)...)
		}
		return append(frames, repeatFrame(strings.Join(rows, "\n"), 6)...)
	}()

	newArt = []string{
		" *     .\n  [NEW]\n .     *",
		" .     *\n  [NEW]\n *     .",
	}

	faceArt = append(append(append(
		repeatFrame(" .---.\n( o o )\n '-v-'", 8),
		" .---.\n( - - )\n '-v-'"),
		repeatFrame(" .---.\n( o o )\n '-v-'", 3)...),
		repeatFrame(" .---.\n( ^ ^ )\n '-v-'", 4)...)

	robotArt = []string{
		" [o_o]\n /|_|\\\n  / \\",
		" [o_o]\n \\|_|/\n  / \\",
		" [o_o]\n /|_|\\\n  / \\",
		" [-_-]\n /|_|\\\n  / \\",
	}
)

// RobotArt is the animation shown on AI-written slides
func RobotArt() []string {
	return robotArt
}

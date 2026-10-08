// internal/share/term.go

// Package share turns the Recap into images people can post: an animated
// GIF of the slides and a summary poster. It renders the same styled
// terminal output the TUI shows, so what people share is what they saw.
package share

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// cell is one character of terminal output with its style
type cell struct {
	r            rune
	fg, bg       color.RGBA
	hasBg        bool
	bold, italic bool
}

// parseANSI turns styled terminal output into rows of cells. It handles the
// SGR codes lipgloss emits with a true color profile.
func parseANSI(s string, defaultFg color.RGBA) [][]cell {
	var grid [][]cell
	fg, bg, hasBg, bold, italic := defaultFg, color.RGBA{}, false, false, false
	for _, line := range strings.Split(s, "\n") {
		var row []cell
		for i := 0; i < len(line); {
			if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
				end := strings.IndexByte(line[i:], 'm')
				if end < 0 {
					break
				}
				codes := strings.Split(line[i+2:i+end], ";")
				for j := 0; j < len(codes); j++ {
					n, _ := strconv.Atoi(codes[j])
					switch {
					case n == 0:
						fg, hasBg, bold, italic = defaultFg, false, false, false
					case n == 1:
						bold = true
					case n == 22:
						bold = false
					case n == 3:
						italic = true
					case n == 23:
						italic = false
					case n == 39:
						fg = defaultFg
					case n == 49:
						hasBg = false
					case (n == 38 || n == 48) && j+4 < len(codes) && codes[j+1] == "2":
						c := color.RGBA{atou8(codes[j+2]), atou8(codes[j+3]), atou8(codes[j+4]), 255}
						if n == 38 {
							fg = c
						} else {
							bg, hasBg = c, true
						}
						j += 4
					}
				}
				i += end + 1
				continue
			}
			r, size := utf8.DecodeRuneInString(line[i:])
			row = append(row, cell{r: r, fg: fg, bg: bg, hasBg: hasBg, bold: bold, italic: italic})
			i += size
		}
		grid = append(grid, row)
	}
	return grid
}

func atou8(s string) uint8 {
	n, _ := strconv.Atoi(s)
	return uint8(n)
}

// width is the widest row of a grid, in cells
func width(grid [][]cell) int {
	w := 0
	for _, row := range grid {
		if len(row) > w {
			w = len(row)
		}
	}
	return w
}

// renderer draws cell grids with the Go Mono font. Box drawing and block
// characters are drawn as shapes instead of glyphs, so lines join up and
// bars stay crisp at any size.
type renderer struct {
	cellW, cellH int
	ascent       int
	fonts        [3]*sfnt.Font
	faces        [3]font.Face // regular, bold, italic
	glyphs       map[glyphKey]*image.Alpha
}

type glyphKey struct {
	r    rune
	face int
}

// cellAspect is a terminal cell's height relative to its width
const cellAspect = 2.2

// newRenderer returns a renderer whose cells are cellW pixels wide
func newRenderer(cellW int) *renderer {
	r := &renderer{glyphs: map[glyphKey]*image.Alpha{}, cellW: cellW, cellH: int(float64(cellW) * cellAspect)}
	// Go Mono glyphs are 0.6em wide
	size := float64(cellW) / 0.6
	for i, ttf := range [][]byte{gomono.TTF, gomonobold.TTF, gomonoitalic.TTF} {
		f, err := opentype.Parse(ttf)
		if err != nil {
			panic(err) // the fonts are compiled in, so this can't fail at runtime
		}
		face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			panic(err)
		}
		r.fonts[i], r.faces[i] = f, face
	}
	m := r.faces[0].Metrics()
	r.ascent = m.Ascent.Ceil() + (r.cellH-m.Height.Ceil())/2
	return r
}

// drawnAsShape lists the characters drawn as shapes instead of glyphs
var drawnAsShape = map[rune]bool{
	'─': true, '━': true, '│': true, '╭': true, '╮': true, '╰': true, '╯': true, '■': true,
	'▁': true, '▂': true, '▃': true, '▄': true, '▅': true, '▆': true, '▇': true, '█': true,
}

// canDraw reports whether r is drawn as a shape or has a glyph in Go Mono
func (r *renderer) canDraw(ch rune) bool {
	if ch == ' ' || drawnAsShape[ch] {
		return true
	}
	var buf sfnt.Buffer
	idx, err := r.fonts[0].GlyphIndex(&buf, ch)
	return err == nil && idx != 0
}

func (r *renderer) glyph(ch rune, face int) *image.Alpha {
	k := glyphKey{ch, face}
	if g, ok := r.glyphs[k]; ok {
		return g
	}
	g := image.NewAlpha(image.Rect(0, 0, r.cellW, r.cellH))
	d := font.Drawer{Dst: g, Src: image.Opaque, Face: r.faces[face], Dot: fixed.P(0, r.ascent)}
	d.DrawString(string(ch))
	r.glyphs[k] = g
	return g
}

func fill(dst *image.RGBA, rect image.Rectangle, c color.RGBA) {
	draw.Draw(dst, rect, image.NewUniform(c), image.Point{}, draw.Src)
}

// arc draws a quarter of an ellipse centered at (cx, cy), from angle a0 to
// a1, for rounded box corners
func arc(dst *image.RGBA, cx, cy, rx, ry, thick int, a0, a1 float64, c color.RGBA) {
	steps := 4 * (rx + ry)
	for i := 0; i <= steps; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(steps)
		x := cx + int(math.Round(float64(rx)*math.Cos(a)))
		y := cy + int(math.Round(float64(ry)*math.Sin(a)))
		fill(dst, image.Rect(x-thick/2, y-thick/2, x-thick/2+thick, y-thick/2+thick), c)
	}
}

// draw paints the grid with its top left corner at origin
func (r *renderer) draw(dst *image.RGBA, origin image.Point, grid [][]cell) {
	w, h := r.cellW, r.cellH
	t := maxInt(1, w/8) // line thickness
	for y, row := range grid {
		for x, c := range row {
			px, py := origin.X+x*w, origin.Y+y*h
			rect := image.Rect(px, py, px+w, py+h)
			if c.hasBg {
				fill(dst, rect, c.bg)
			}
			mx, my := px+w/2, py+h/2
			lineTop := my - t/2
			lineLeft := mx - t/2
			switch c.r {
			case ' ':
			case '─':
				fill(dst, image.Rect(px, lineTop, px+w, lineTop+t), c.fg)
			case '━':
				fill(dst, image.Rect(px, my-t, px+w, my+t), c.fg)
			case '│':
				fill(dst, image.Rect(lineLeft, py, lineLeft+t, py+h), c.fg)
			case '╭':
				arc(dst, px+w, py+h, w/2, h/2, t, math.Pi, 1.5*math.Pi, c.fg)
			case '╮':
				arc(dst, px, py+h, w/2, h/2, t, 1.5*math.Pi, 2*math.Pi, c.fg)
			case '╰':
				arc(dst, px+w, py, w/2, h/2, t, 0.5*math.Pi, math.Pi, c.fg)
			case '╯':
				arc(dst, px, py, w/2, h/2, t, 0, 0.5*math.Pi, c.fg)
			case '■':
				// Squares with a gap between them, like btop's meters
				gap := maxInt(2, w/5)
				side := w - gap
				fill(dst, image.Rect(px+gap/2, my-side/2, px+gap/2+side, my-side/2+side), c.fg)
			case '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█':
				eighths := int(c.r-'▁') + 1
				fill(dst, image.Rect(px, py+h-h*eighths/8, px+w, py+h), c.fg)
			default:
				face := 0
				if c.bold {
					face = 1
				} else if c.italic {
					face = 2
				}
				draw.DrawMask(dst, rect, image.NewUniform(c.fg), image.Point{}, r.glyph(c.r, face), image.Point{}, draw.Over)
			}
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

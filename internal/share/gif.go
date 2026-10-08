// internal/share/gif.go
package share

import (
	"image"
	"image/color"
	"image/gif"
	"io"
	"sort"
)

// transparent marks pixels a frame leaves as they were in the frame before
var transparent = color.RGBA{}

// encodeGIF writes n frames as an animated GIF that loops forever.
// render(i) renders frame i and says how long it stays up, in hundredths
// of a second; only the previous frame is kept in memory. Every frame after the first only stores the
// rectangle that changed, with unchanged pixels left transparent, and gets
// its own palette holding the exact colors of that rectangle (or the most
// common ones if there are too many), which keeps files small without
// shifting colors.
func encodeGIF(w io.Writer, n int, render func(i int) (*image.RGBA, int)) error {
	anim := &gif.GIF{}
	var prev *image.RGBA
	for i := 0; i < n; i++ {
		frame, delay := render(i)
		if prev == nil {
			anim.Config = image.Config{Width: frame.Rect.Dx(), Height: frame.Rect.Dy()}
		}
		rect := frame.Rect
		if prev != nil {
			rect = changedRect(prev, frame)
			if rect.Empty() {
				// Frames can't be empty, so repeat a single transparent pixel
				rect = image.Rect(0, 0, 1, 1)
			}
		}
		anim.Image = append(anim.Image, paletted(frame, prev, rect))
		anim.Delay = append(anim.Delay, delay)
		anim.Disposal = append(anim.Disposal, gif.DisposalNone)
		prev = frame
	}
	return gif.EncodeAll(w, anim)
}

func rgbaAt(img *image.RGBA, x, y int) color.RGBA {
	o := img.PixOffset(x, y)
	return color.RGBA{img.Pix[o], img.Pix[o+1], img.Pix[o+2], 255}
}

// paletted converts rect of frame to a paletted image. Pixels that match
// prev become transparent; prev is nil for the first frame.
func paletted(frame, prev *image.RGBA, rect image.Rectangle) *image.Paletted {
	counts := map[color.RGBA]int{}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := rgbaAt(frame, x, y)
			if prev != nil && rgbaAt(prev, x, y) == c {
				continue
			}
			counts[c]++
		}
	}

	var pal color.Palette
	if prev != nil {
		pal = append(pal, transparent)
	}
	colors := make([]color.RGBA, 0, len(counts))
	for c := range counts {
		colors = append(colors, c)
	}
	// Most common first, so a reduced palette keeps what matters most
	sort.Slice(colors, func(i, j int) bool {
		if counts[colors[i]] != counts[colors[j]] {
			return counts[colors[i]] > counts[colors[j]]
		}
		a, b := colors[i], colors[j]
		return uint32(a.R)<<16|uint32(a.G)<<8|uint32(a.B) < uint32(b.R)<<16|uint32(b.G)<<8|uint32(b.B)
	})
	for _, c := range colors {
		if len(pal) == 256 {
			break
		}
		pal = append(pal, c)
	}
	if len(pal) == 0 {
		pal = append(pal, transparent)
	}

	opaque := pal
	if prev != nil {
		opaque = pal[1:]
	}
	index := make(map[color.RGBA]uint8, len(counts))
	for i, c := range pal {
		index[c.(color.RGBA)] = uint8(i)
	}
	lookup := func(c color.RGBA) uint8 {
		if i, ok := index[c]; ok {
			return i
		}
		// Only when the rectangle has more than 256 colors
		i := uint8(opaque.Index(c))
		if prev != nil {
			i++
		}
		index[c] = i
		return i
	}

	img := image.NewPaletted(rect, pal)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := rgbaAt(frame, x, y)
			if prev != nil && rgbaAt(prev, x, y) == c {
				img.SetColorIndex(x, y, 0)
				continue
			}
			img.SetColorIndex(x, y, lookup(c))
		}
	}
	return img
}

// changedRect returns the smallest rectangle holding every pixel that
// differs between a and b
func changedRect(a, b *image.RGBA) image.Rectangle {
	minX, minY, maxX, maxY := b.Rect.Max.X, b.Rect.Max.Y, -1, -1
	for y := b.Rect.Min.Y; y < b.Rect.Max.Y; y++ {
		rowA := a.Pix[a.PixOffset(0, y) : a.PixOffset(0, y)+a.Stride]
		rowB := b.Pix[b.PixOffset(0, y) : b.PixOffset(0, y)+b.Stride]
		if string(rowA) == string(rowB) {
			continue
		}
		for x := b.Rect.Min.X; x < b.Rect.Max.X; x++ {
			o := (x - b.Rect.Min.X) * 4
			if rowA[o] != rowB[o] || rowA[o+1] != rowB[o+1] || rowA[o+2] != rowB[o+2] {
				minX, maxX = minInt(minX, x), maxInt(maxX, x)
				minY, maxY = minInt(minY, y), maxInt(maxY, y)
			}
		}
	}
	if maxX < 0 {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

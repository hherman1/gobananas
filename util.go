package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"image/color"
	"math"
)


var (
	emptyImage    = ebiten.NewImage(3, 3)
	emptySubImage = emptyImage.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
)

func init() {
	emptyImage.Fill(color.White)
}

// Utillity for drawing lines with a transformation
func drawline(img *ebiten.Image, srcx, srcy, dstx, dsty float64, thickness float64, geom Mx, c color.Color) {
	x1, y1 := geom.Apply(srcx, srcy)
	x2, y2 := geom.Apply(dstx, dsty)

	length := math.Hypot(x2-x1, y2-y1)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(length, thickness)
	op.GeoM.Rotate(math.Atan2(y2-y1, x2-x1))
	op.GeoM.Translate(x1, y1)
	op.ColorM.Scale(colorToScale(c))
	// Filter must be 'nearest' filter (default).
	// Linear filtering would make edges blurred.
	img.DrawImage(emptySubImage, op)
}

func colorToScale(clr color.Color) (float64, float64, float64, float64) {
	cr, cg, cb, ca := clr.RGBA()
	if ca == 0 {
		return 0, 0, 0, 0
	}
	return float64(cr) / float64(ca), float64(cg) / float64(ca), float64(cb) / float64(ca), float64(ca) / 0xffff
}

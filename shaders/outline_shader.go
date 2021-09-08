//go:build ignore
// +build ignore

package shaders

var Time float
var Cursor vec2
var ScreenSize vec2
var Vx float
var Vy float
var ScreenPixels vec2

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	return vec4(position.x/ScreenPixels.x, 0, 0, 1)
}
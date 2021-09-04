//go:build ignore
// +build ignore

package main

var Time float
var Cursor vec2
var ScreenSize vec2
var Vx float
var Vy float
var ScreenPixels vec2

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	//xfac := 1 - log2(abs(Vx))
	return vec4(position.x / ScreenPixels.x, position.y / ScreenPixels.y, 0, 1)
	//lightpos := vec3(Cursor, 50)
	//lightdir := normalize(lightpos - position.xyz)
	//normal := normalize(imageSrc1UnsafeAt(texCoord) - 0.5)
	//const ambient = 0.25
	//diffuse := 0.75 * max(0.0, dot(normal.xyz, lightdir))
	//return imageSrc0UnsafeAt(texCoord) * (ambient + diffuse)
}
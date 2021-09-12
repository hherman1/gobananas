package resources

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"image/png"
	"io"
	"strings"
)

// Sample rate for all our audio
const SampleRate = 44100

//go:embed shaders
var shadersFS embed.FS
var shaders = map[string]*ebiten.Shader{}

//go:embed resources
var resources embed.FS

// Caches images loaded from disk. Not thread safe.
var imgs = map[string]*ebiten.Image{}

// Loads an image from the given resource path (resource/*), reusing it if previously loaded. Not thread safe.
func Image(path string) (*ebiten.Image, error) {
	if img, ok := imgs[path]; ok {
		return img, nil
	}

	b, err := resources.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	switch {
	case strings.HasSuffix(path, ".png"):
		img, err := png.Decode(bytes.NewBuffer(b))
		if err != nil {
			return nil, fmt.Errorf("decode png: %w", err)
		}
		eimg := ebiten.NewImageFromImage(img)
		imgs[path] = eimg
		return eimg, nil
	}
	return nil, errors.New("unrecognized format")
}

// Loads an image from the given shader path (shaders/*), reusing it if previously loaded. Not thread safe.
func Shader(path string) (*ebiten.Shader, error) {
	if s, ok := shaders[path]; ok {
		return s, nil
	}
	b, err := shadersFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	shader, err := ebiten.NewShader(b)
	if err != nil {
		return nil, fmt.Errorf("loading shader: %w", err)
	}
	shaders[path] = shader
	return shader, nil
}

// Loads and decodes audio file from the resources directory
func Audio(path string) ([]byte, error) {
	f, err := resources.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	stream, err := wav.DecodeWithSampleRate(SampleRate, f.(io.ReadSeeker))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	a, err := io.ReadAll(stream)
	if err != nil {
		return nil, fmt.Errorf("read stream: %w", err)
	}
	return a, nil
}

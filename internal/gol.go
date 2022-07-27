package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
)

type GameOfLife struct {
	width, height int
	data          [][]bool // current state
	back_buffer   [][]bool // future state
}

func NewGameOfLife(width, height int) (*GameOfLife, error) {
	game := GameOfLife{width, height, nil, nil}
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid size")
	}
	game.data, game.back_buffer = make([][]bool, height), make([][]bool, height)
	for i := 0; i < height; i++ {
		game.data[i], game.back_buffer[i] = make([]bool, width), make([]bool, width)
	}

	return &game, nil
}

type GameOfLifeSave struct {
	Width  int
	Height int
	Cells  []string
}

func LoadFromJSON(file string) (*GameOfLife, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	sav := GameOfLifeSave{}
	err = json.Unmarshal(data, &sav)
	if err != nil {
		return nil, err
	}

	game, err := NewGameOfLife(sav.Width, sav.Height)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(sav.Cells); i++ {
		line := sav.Cells[i]
		for j := 0; j < len(line); j++ {
			if line[j] == ' ' {
				game.Set(j, i, false)
			} else {
				game.Set(j, i, true)
			}
		}
	}

	return game, nil
}

func LoadFromImage(img image.Image) (*GameOfLife, error) {
	game, err := NewGameOfLife(img.Bounds().Dx(), img.Bounds().Dy())

	for i := 0; i < img.Bounds().Dy(); i++ {
		for j := 0; j < img.Bounds().Dx(); j++ {
			val := color.GrayModel.Convert(img.At(j, i)).(color.Gray).Y
			if val < 16 { // really black color
				game.Set(j, i, true)
			}
		}
		fmt.Println()
	}
	return game, err
}

func LoadFromJPEG(file string) (*GameOfLife, error) {
	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	img, err := jpeg.Decode(fp)
	if err != nil {
		return nil, err
	}

	return LoadFromImage(img)
}

func LoadFromPNG(file string) (*GameOfLife, error) {
	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	img, err := png.Decode(fp)
	if err != nil {
		return nil, err
	}

	return LoadFromImage(img)
}

// data is lines with data
// key is the alive cell value (make sure key is lowercase)
func LoadFromText(data string, key byte) (*GameOfLife, error) {
	width, height := 0, 0

	lines := strings.Split(data, "\n")
	height = len(lines)
	for i := 0; i < height; i++ {
		lineWidth := len(lines[i])
		if lineWidth > width {
			width = lineWidth
		}
	}

	game, err := NewGameOfLife(width, height)
	if err != nil {
		return nil, err
	}

	for i := 0; i < height; i++ {
		line := strings.ToLower(lines[i])
		for j := 0; j < len(line); j++ {
			if line[j] == key {
				game.Set(j, i, true)
			} else {
				game.Set(j, i, false)
			}
		}
	}

	return game, nil
}

func (g *GameOfLife) Set(x, y int, val bool) error {
	if x < 0 || x >= g.width {
		return errors.New("out of bounds")
	}
	if y < 0 || y >= g.height {
		return errors.New("out of bounds")
	}
	g.data[y][x], g.back_buffer[y][x] = val, val
	return nil
}

func (g *GameOfLife) At(x, y int) bool {
	if x < 0 || x >= g.width {
		return false
	}
	if y < 0 || y >= g.height {
		return false
	}
	return g.data[y][x]
}

func (g *GameOfLife) Flush() {
	for i := 0; i < g.height; i++ {
		for j := 0; j < g.width; j++ {
			g.data[i][j] = g.back_buffer[i][j]
		}
	}
}

func (g *GameOfLife) CellValue(x, y int) int {
	sum := 0
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if j == 1 && i == 1 {
				continue
			}
			if g.At(j+x-1, i+y-1) {
				sum++
			}
		}
	}
	return sum
}

func (g *GameOfLife) GetState() [][]bool {
	return g.data
}

// update state of data
func (g *GameOfLife) Update() {
	for i := 0; i < g.height; i++ {
		for j := 0; j < g.width; j++ {
			cnt := g.CellValue(j, i)
			if g.At(j, i) {
				if cnt < 2 || cnt > 3 {
					g.back_buffer[i][j] = false
				}
			} else {
				if cnt == 3 {
					g.back_buffer[i][j] = true
				}
			}
		}
	}

	g.Flush()
}

func (g *GameOfLife) GetWidth() int {
	return g.width
}

func (g *GameOfLife) GetHeight() int {
	return g.height
}

func DrawRect(x0, y0, x1, y1 int, col color.Color, img *image.Paletted) {
	for i := y0; i <= y1; i++ {
		for j := x0; j <= x1; j++ {
			img.Set(j, i, col)
		}
	}
}

func (g *GameOfLife) Image(scale int) *image.Paletted {
	palette := []color.Color{
		color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, g.width*scale, g.height*scale), palette)
	for i := 0; i < g.height; i++ {
		for j := 0; j < g.width; j++ {
			px, py := j*scale, i*scale
			col := color.White
			if g.At(j, i) {
				col = color.Black
			}
			DrawRect(px, py, px+scale, py+scale, col, img)
		}
	}
	return img
}

func (g *GameOfLife) Text() string {
	buffer := bytes.Buffer{}
	for i := 0; i < g.height; i++ {
		for j := 0; j < g.width; j++ {
			if g.At(j, i) {
				buffer.WriteString("o")
			} else {
				buffer.WriteString(" ")
			}
		}
		buffer.WriteString("\n")
	}
	return buffer.String()
}

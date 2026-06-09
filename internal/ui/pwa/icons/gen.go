//go:build ignore

// Генератор иконок PWA для onebase (этап 45). Рисует молнию (символ ⚡ из
// шапки) светло-голубым на тёмном фоне темы. Фон заливает всю канву —
// для maskable-иконки платформа сама применит маску/скругление.
//
// Запуск (go.exe не в PATH): go run gen.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

var (
	bg   = color.RGBA{0x1e, 0x29, 0x3b, 0xff} // #1e293b — theme_color
	bolt = color.RGBA{0x7d, 0xd3, 0xfc, 0xff} // #7dd3fc — акцент шапки
)

// Контур молнии в нормализованных координатах (0..1, y вниз), в пределах
// безопасной зоны maskable (~0.15..0.85).
var poly = [][2]float64{
	{0.55, 0.15},
	{0.32, 0.52},
	{0.47, 0.52},
	{0.43, 0.85},
	{0.70, 0.46},
	{0.55, 0.46},
	{0.62, 0.15},
}

func inPoly(px, py float64) bool {
	in := false
	n := len(poly)
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := poly[i][0], poly[i][1]
		xj, yj := poly[j][0], poly[j][1]
		if (yi > py) != (yj > py) && px < (xj-xi)*(py-yi)/(yj-yi)+xi {
			in = !in
		}
		j = i
	}
	return in
}

func render(size int, path string) error {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		ny := (float64(y) + 0.5) / float64(size)
		for x := 0; x < size; x++ {
			nx := (float64(x) + 0.5) / float64(size)
			if inPoly(nx, ny) {
				img.Set(x, y, bolt)
			} else {
				img.Set(x, y, bg)
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func main() {
	if err := render(192, "icon-192.png"); err != nil {
		panic(err)
	}
	if err := render(512, "icon-512.png"); err != nil {
		panic(err)
	}
}

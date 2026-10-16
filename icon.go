// icon.go — generate icon pakai Go stdlib (tanpa dependency eksternal).
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// 5x7 pixel font sederhana untuk huruf A-Z.
var pixelFont = map[rune][7]uint8{
	'A': {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'B': {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
	'C': {0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E},
	'D': {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
	'E': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
	'F': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10},
	'G': {0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0F},
	'H': {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'I': {0x0E, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'J': {0x07, 0x02, 0x02, 0x02, 0x02, 0x12, 0x0C},
	'K': {0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11},
	'L': {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
	'M': {0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11},
	'N': {0x11, 0x19, 0x19, 0x15, 0x13, 0x13, 0x11},
	'O': {0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'P': {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
	'Q': {0x0E, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0D},
	'R': {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
	'S': {0x0F, 0x10, 0x10, 0x0E, 0x01, 0x01, 0x1E},
	'T': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'U': {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'V': {0x11, 0x11, 0x11, 0x11, 0x11, 0x0A, 0x04},
	'W': {0x11, 0x11, 0x11, 0x15, 0x15, 0x15, 0x0A},
	'X': {0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11},
	'Y': {0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04},
	'Z': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1F},
	'0': {0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E},
	'1': {0x04, 0x0C, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'2': {0x0E, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1F},
	'3': {0x0E, 0x11, 0x01, 0x06, 0x01, 0x11, 0x0E},
	'4': {0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02},
	'5': {0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E},
	'6': {0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E},
	'7': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	'8': {0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E},
	'9': {0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x0C},
}

// colorFromName — warna stabil dari nama app.
func colorFromName(name string) color.RGBA {
	var h uint32 = 5381
	for _, c := range name {
		h = h*33 + uint32(c)
	}
	r := uint8(80 + (h>>0)&0x7F)
	g := uint8(80 + (h>>8)&0x7F)
	b := uint8(80 + (h>>16)&0x7F)
	return color.RGBA{r, g, b, 255}
}

// firstInitial — huruf pertama (bukan spasi).
func firstInitial(name string) rune {
	for _, r := range name {
		if r != ' ' {
			if r >= 'a' && r <= 'z' {
				r = r - 32
			}
			return r
		}
	}
	return 'N'
}

// drawLetter — gambar huruf pakai pixel pattern.
func drawLetter(img *image.RGBA, letter rune, centerX, centerY, pixelSize int, col color.RGBA) {
	pat, ok := pixelFont[letter]
	if !ok {
		return
	}
	patW := 5
	patH := 7
	offX := centerX - (patW*pixelSize)/2
	offY := centerY - (patH*pixelSize)/2

	for y := 0; y < patH; y++ {
		row := pat[y]
		for x := 0; x < patW; x++ {
			if row&(1<<uint(patW-1-x)) != 0 {
				for dy := 0; dy < pixelSize; dy++ {
					for dx := 0; dx < pixelSize; dx++ {
						img.Set(offX+x*pixelSize+dx, offY+y*pixelSize+dy, col)
					}
				}
			}
		}
	}
}

// makeDefaultIcon — kotak warna + huruf inisial putih.
func makeDefaultIcon(appName string, size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	bg := colorFromName(appName)
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	letter := firstInitial(appName)
	pixelSize := size / 8
	if pixelSize < 1 {
		pixelSize = 1
	}
	drawLetter(img, letter, size/2, size/2, pixelSize, color.RGBA{255, 255, 255, 255})
	return img
}

// resizeIcon — resize pakai CatmullRom.
func resizeIcon(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// padToSquare — taruh di tengah canvas.
func padToSquare(content image.Image, canvasSize, contentSize int) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, canvasSize, canvasSize))
	if contentSize <= 0 {
		contentSize = canvasSize * 2 / 3
	}
	offset := (canvasSize - contentSize) / 2
	rect := image.Rect(offset, offset, offset+contentSize, offset+contentSize)
	xdraw.CatmullRom.Scale(canvas, rect, content, content.Bounds(), draw.Over, nil)
	return canvas
}

// loadIconImage — baca PNG atau JPEG.
func loadIconImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	low := strings.ToLower(path)
	if strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg") {
		return jpeg.Decode(f)
	}
	return png.Decode(f)
}

// savePNG — tulis PNG.
func savePNG(img image.Image, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// generateIconsGo — versi Go native dari generateIcons.
func generateIconsGo(cfg apkConfig) error {
	sizes := map[string]int{
		"mipmap-mdpi":    48,
		"mipmap-hdpi":    72,
		"mipmap-xhdpi":   96,
		"mipmap-xxhdpi":  144,
		"mipmap-xxxhdpi": 192,
	}
	adaptiveSizes := map[string]int{
		"mipmap-mdpi":    108,
		"mipmap-hdpi":    162,
		"mipmap-xhdpi":   216,
		"mipmap-xxhdpi":  324,
		"mipmap-xxxhdpi": 432,
	}

	var src image.Image
	var err error

	if cfg.IconPath != "" {
		src, err = loadIconImage(cfg.IconPath)
		if err != nil {
			return err
		}
	} else {
		src = makeDefaultIcon(cfg.AppName, 512)
	}

	// Legacy icons
	for dir, size := range sizes {
		resized := resizeIcon(src, size, size)
		outPath := filepath.Join(cfg.WorkDir, "res", dir, "ic_launcher.png")
		if err := savePNG(resized, outPath); err != nil {
			return err
		}
	}

	// Adaptive foreground
	for dir, size := range adaptiveSizes {
		padded := padToSquare(src, size, 0)
		outPath := filepath.Join(cfg.WorkDir, "res", dir, "ic_launcher_foreground.png")
		if err := savePNG(padded, outPath); err != nil {
			return err
		}
	}

	// Adaptive background
	bgColor := colorFromName(cfg.AppName)
	for dir, size := range adaptiveSizes {
		bg := image.NewRGBA(image.Rect(0, 0, size, size))
		draw.Draw(bg, bg.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)
		outPath := filepath.Join(cfg.WorkDir, "res", dir, "ic_launcher_background.png")
		if err := savePNG(bg, outPath); err != nil {
			return err
		}
	}

	return nil
}

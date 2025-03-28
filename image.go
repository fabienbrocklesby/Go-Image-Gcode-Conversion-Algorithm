package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

func LoadImage(filePath string) (image.Image, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var img image.Image
	switch ext {
	case ".svg":
		return loadSVG(data)
	case ".png":
		img, err = png.Decode(bytes.NewReader(data))
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(bytes.NewReader(data))
	default:
		return nil, errors.New("unsupported image format: " + ext)
	}

	if err != nil {
		return nil, err
	}

	return processRasterImage(img), nil
}

func loadSVG(data []byte) (image.Image, error) {
	svgIcon, err := oksvg.ReadIconStream(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	viewBoxW := float64(svgIcon.ViewBox.W)
	viewBoxH := float64(svgIcon.ViewBox.H)

	svgIcon.SetTarget(0, 0, viewBoxW, viewBoxH)
	width := int(viewBoxW)
	height := int(viewBoxH)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	scanner := rasterx.NewScannerGV(width, height, img, img.Bounds())
	scanner.SetClip(img.Bounds())
	raster := rasterx.NewDasher(width, height, scanner)

	svgIcon.Draw(raster, 1.0)
	return processSVGImage(img), nil
}

func processSVGImage(img *image.RGBA) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	result := image.NewRGBA(bounds)
	draw.Draw(result, bounds, &image.Uniform{color.White}, image.Point{}, draw.Src)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelColor := img.RGBAAt(x, y)
			if pixelColor.R < 240 && pixelColor.G < 240 && pixelColor.B < 240 {
				neighbors := 0

				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						nx, ny := x+dx, y+dy
						if nx >= 0 && nx < width && ny >= 0 && ny < height {
							nc := img.RGBAAt(nx, ny)
							if nc.R > 240 && nc.G > 240 && nc.B > 240 {
								neighbors++
							}
						}
					}
				}

				if neighbors > 0 || isSolidRegion(img, x, y, width, height) {
					result.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
				}
			}
		}
	}

	return result
}

func isSolidRegion(img *image.RGBA, x, y, width, height int) bool {
	blackCount := 0
	totalCount := 0

	for dy := -5; dy <= 5; dy++ {
		for dx := -5; dx <= 5; dx++ {
			nx, ny := x+dx, y+dy
			if nx >= 0 && nx < width && ny >= 0 && ny < height {
				totalCount++
				nc := img.RGBAAt(nx, ny)
				if nc.R < 100 && nc.G < 100 && nc.B < 100 {
					blackCount++
				}
			}
		}
	}

	return blackCount > int(float64(totalCount)*0.75)
}

func processRasterImage(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewGray(bounds)

	var totalCount int
	colorHistogram := make(map[uint32]int)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			if a < 0x8000 {
				continue
			}

			r = r * 0xffff / a
			g = g * 0xffff / a
			b = b * 0xffff / a

			colorKey := (r>>12)<<16 | (g>>12)<<8 | (b >> 12)
			colorHistogram[uint32(colorKey)]++
			totalCount++
		}
	}

	dominantColors := findDominantColors(colorHistogram, 3)
	if len(dominantColors) == 0 {
		return dst
	}

	dominantColor := dominantColors[0].color
	secondaryColor := uint32(0)
	if len(dominantColors) > 1 {
		secondaryColor = dominantColors[1].color
	}

	hasDistinctGroups := hasDistinctColorGroups(dominantColors, totalCount)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			if a == 0 {
				dst.SetGray(x, y, color.Gray{255})
				continue
			}

			r = r * 0xffff / a
			g = g * 0xffff / a
			b = b * 0xffff / a

			colorKey := (r>>12)<<16 | (g>>12)<<8 | (b >> 12)

			var brightness uint8 = 255

			if hasDistinctGroups {
				if isColorSimilar(uint32(colorKey), dominantColor) {
					brightness = 0
				} else if secondaryColor != 0 && isColorSimilar(uint32(colorKey), secondaryColor) {
					brightness = 127
				}
			} else {
				gray := uint8(((299*r + 587*g + 114*b) / 1000) >> 8)
				if gray < 128 {
					brightness = 0
				}
			}

			dst.SetGray(x, y, color.Gray{brightness})
		}
	}

	return dst
}

func findDominantColors(histogram map[uint32]int, count int) []struct {
	color uint32
	count int
} {
	colors := make([]struct {
		color uint32
		count int
	}, 0, len(histogram))

	for color, count := range histogram {
		colors = append(colors, struct {
			color uint32
			count int
		}{color, count})
	}

	for i := 0; i < len(colors)-1; i++ {
		for j := i + 1; j < len(colors); j++ {
			if colors[j].count > colors[i].count {
				colors[i], colors[j] = colors[j], colors[i]
			}
		}
	}

	if len(colors) > count {
		return colors[:count]
	}
	return colors
}

func hasDistinctColorGroups(dominantColors []struct {
	color uint32
	count int
}, totalPixels int) bool {
	if len(dominantColors) < 2 {
		return false
	}

	topTwoPercentage := float64(dominantColors[0].count+dominantColors[1].count) / float64(totalPixels)
	return topTwoPercentage > 0.7
}

func isColorSimilar(c1, c2 uint32) bool {
	r1, g1, b1 := (c1>>16)&0xFF, (c1>>8)&0xFF, c1&0xFF
	r2, g2, b2 := (c2>>16)&0xFF, (c2>>8)&0xFF, c2&0xFF

	rDiff := abs(int(r1) - int(r2))
	gDiff := abs(int(g1) - int(g2))
	bDiff := abs(int(b1) - int(b2))

	return rDiff < 3 && gDiff < 3 && bDiff < 3
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

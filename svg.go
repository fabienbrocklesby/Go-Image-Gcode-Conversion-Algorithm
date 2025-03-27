package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"os"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

func LoadSVG(filePath string) (image.Image, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

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
	return img, nil
}

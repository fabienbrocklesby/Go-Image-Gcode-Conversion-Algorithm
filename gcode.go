package main

import (
	"fmt"
	"image"
	"math"
	"strings"
)

func ConvertToGCode(img image.Image, targetWidth, targetHeight, offset float64, threshold uint8) (string, error) {
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()

	scaleX := targetWidth / float64(imgWidth)
	scaleY := targetHeight / float64(imgHeight)

	var sb strings.Builder
	sb.WriteString("G21\n")
	sb.WriteString("G90\n")
	sb.WriteString("M5\n")
	sb.WriteString("G0 F2000\n")
	sb.WriteString("G1 F1000\n")

	paths := extractOutlinePaths(img, threshold)

	for _, path := range paths {
		sb.WriteString("M5\n")
		firstPoint := true

		for _, point := range path.points {
			x := offset + float64(point.x)*scaleX
			y := offset + float64(point.y)*scaleY

			if firstPoint {
				sb.WriteString(fmt.Sprintf("G0 X%.3f Y%.3f\n", x, y))
				sb.WriteString("M3 S1000\n")
				firstPoint = false
			} else {
				sb.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f\n", x, y))
			}
		}
	}

	sb.WriteString("M5\n")
	sb.WriteString("G0 X0 Y0\n")

	return sb.String(), nil
}

type Point struct {
	x, y int
}

type Path struct {
	points []Point
}

func extractOutlinePaths(img image.Image, threshold uint8) []Path {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	visited := make([][]bool, height)
	for i := range visited {
		visited[i] = make([]bool, width)
	}

	var paths []Path

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if visited[y][x] {
				continue
			}

			gray := getGrayscale(img, bounds, x, y)
			if gray >= 230 {
				visited[y][x] = true
				continue
			}

			if isEdgePixel(img, bounds, x, y) {
				path := tracePath(img, bounds, x, y, visited)
				if len(path.points) > 5 {
					paths = append(paths, path)
				}
			}

			visited[y][x] = true
		}
	}

	return paths
}

func isEdgePixel(img image.Image, bounds image.Rectangle, x, y int) bool {
	gray := getGrayscale(img, bounds, x, y)
	if gray >= 230 {
		return false
	}

	directions := []struct{ dx, dy int }{
		{-1, 0}, {1, 0}, {0, -1}, {0, 1},
		{-1, -1}, {-1, 1}, {1, -1}, {1, 1},
	}

	for _, dir := range directions {
		nx, ny := x+dir.dx, y+dir.dy
		if nx < 0 || ny < 0 || nx >= bounds.Dx() || ny >= bounds.Dy() {
			return true
		}

		neighborGray := getGrayscale(img, bounds, nx, ny)
		if neighborGray >= 230 {
			return true
		}
	}

	return false
}

func tracePath(img image.Image, bounds image.Rectangle, startX, startY int, visited [][]bool) Path {
	path := Path{
		points: []Point{{startX, startY}},
	}

	visited[startY][startX] = true

	directions := []struct{ dx, dy int }{
		{-1, 0}, {1, 0}, {0, -1}, {0, 1},
		{-1, -1}, {-1, 1}, {1, -1}, {1, 1},
	}

	x, y := startX, startY
	foundNext := true

	for foundNext {
		foundNext = false
		bestDistance := math.MaxFloat64
		var nextX, nextY int

		for _, dir := range directions {
			nx, ny := x+dir.dx, y+dir.dy
			if nx < 0 || ny < 0 || nx >= bounds.Dx() || ny >= bounds.Dy() {
				continue
			}

			if visited[ny][nx] {
				continue
			}

			nGray := getGrayscale(img, bounds, nx, ny)
			if nGray < 230 && isEdgePixel(img, bounds, nx, ny) {
				dist := math.Hypot(float64(nx-x), float64(ny-y))
				if dist < bestDistance {
					bestDistance = dist
					nextX, nextY = nx, ny
					foundNext = true
				}
			}
		}

		if foundNext {
			x, y = nextX, nextY
			path.points = append(path.points, Point{x, y})
			visited[y][x] = true
		}
	}

	return path
}

func getGrayscale(img image.Image, bounds image.Rectangle, x, y int) int {
	r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
	r8 := uint8(r >> 8)
	g8 := uint8(g >> 8)
	b8 := uint8(b >> 8)
	return (299*int(r8) + 587*int(g8) + 114*int(b8)) / 1000
}

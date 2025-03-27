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
	sb.WriteString("G21\nG90\nM5\nG0 F3000\nG1 F1500\n")

	outlines := extractOutlinePaths(img, threshold)
	fillAreas := extractFillRegions(img, threshold)

	for _, path := range outlines {
		if len(path.points) < 5 {
			continue
		}

		sb.WriteString("M5\n")
		firstPoint := true
		simplifiedPath := simplifyPath(path.points, 1.0)

		for _, point := range simplifiedPath {
			x := offset + float64(point.x)*scaleX
			y := offset + float64(point.y)*scaleY

			if firstPoint {
				sb.WriteString(fmt.Sprintf("G0 X%.3f Y%.3f\nM3 S1000\n", x, y))
				firstPoint = false
			} else {
				sb.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f\n", x, y))
			}
		}
	}

	for _, region := range fillAreas {
		if len(region.points) < 200 {
			continue
		}

		minX, minY, maxX, maxY := getBoundingBox(region.points)
		fillOptimizedZigZag(minX, minY, maxX, maxY, region.points, offset, scaleX, scaleY, &sb)
	}

	sb.WriteString("M5\nG0 X0 Y0\n")
	return sb.String(), nil
}

type Point struct {
	x, y int
}

type Path struct {
	points []Point
}

func simplifyPath(points []Point, tolerance float64) []Point {
	if len(points) < 3 {
		return points
	}

	result := []Point{points[0]}
	prev := points[0]

	for i := 1; i < len(points); i++ {
		current := points[i]
		if math.Abs(float64(current.x-prev.x)) > tolerance || math.Abs(float64(current.y-prev.y)) > tolerance {
			result = append(result, current)
			prev = current
		}
	}

	if len(result) > 1 && (result[len(result)-1].x != points[len(points)-1].x || result[len(result)-1].y != points[len(points)-1].y) {
		result = append(result, points[len(points)-1])
	}

	return result
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
				paths = append(paths, path)
			}

			visited[y][x] = true
		}
	}

	return paths
}

func extractFillRegions(img image.Image, threshold uint8) []Path {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	visited := make([][]bool, height)
	for i := range visited {
		visited[i] = make([]bool, width)
	}

	var regions []Path

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

			if !isEdgePixel(img, bounds, x, y) {
				region := floodFill(img, bounds, x, y, visited)
				regions = append(regions, region)
			}

			visited[y][x] = true
		}
	}

	return regions
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

func floodFill(img image.Image, bounds image.Rectangle, startX, startY int, visited [][]bool) Path {
	region := Path{
		points: []Point{{startX, startY}},
	}

	queue := []Point{{startX, startY}}
	visited[startY][startX] = true

	directions := []struct{ dx, dy int }{
		{-1, 0}, {1, 0}, {0, -1}, {0, 1},
	}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, dir := range directions {
			nx, ny := curr.x+dir.dx, curr.y+dir.dy
			if nx < 0 || ny < 0 || nx >= bounds.Dx() || ny >= bounds.Dy() {
				continue
			}

			if visited[ny][nx] {
				continue
			}

			gray := getGrayscale(img, bounds, nx, ny)
			if gray < 230 {
				region.points = append(region.points, Point{nx, ny})
				queue = append(queue, Point{nx, ny})
				visited[ny][nx] = true
			}
		}
	}

	return region
}

func getBoundingBox(points []Point) (int, int, int, int) {
	if len(points) == 0 {
		return 0, 0, 0, 0
	}

	minX, minY := points[0].x, points[0].y
	maxX, maxY := points[0].x, points[0].y

	for _, p := range points {
		if p.x < minX {
			minX = p.x
		}
		if p.y < minY {
			minY = p.y
		}
		if p.x > maxX {
			maxX = p.x
		}
		if p.y > maxY {
			maxY = p.y
		}
	}

	return minX, minY, maxX, maxY
}

func fillOptimizedZigZag(minX, minY, maxX, maxY int, points []Point, offset, scaleX, scaleY float64, sb *strings.Builder) {
	pointMap := make(map[int]map[int]bool)
	for _, p := range points {
		if _, ok := pointMap[p.y]; !ok {
			pointMap[p.y] = make(map[int]bool)
		}
		pointMap[p.y][p.x] = true
	}

	lineSpacing := 3

	for y := minY; y <= maxY; y += lineSpacing {
		fromRight := (y-minY)%2 == 1
		var segments []struct{ startX, endX int }

		startSegment := -1

		if fromRight {
			for x := maxX; x >= minX; x-- {
				if pointMap[y] != nil && pointMap[y][x] {
					if startSegment == -1 {
						startSegment = x
					}
				} else if startSegment != -1 {
					segments = append(segments, struct{ startX, endX int }{x + 1, startSegment})
					startSegment = -1
				}
			}
			if startSegment != -1 {
				segments = append(segments, struct{ startX, endX int }{minX, startSegment})
			}
		} else {
			for x := minX; x <= maxX; x++ {
				if pointMap[y] != nil && pointMap[y][x] {
					if startSegment == -1 {
						startSegment = x
					}
				} else if startSegment != -1 {
					segments = append(segments, struct{ startX, endX int }{startSegment, x - 1})
					startSegment = -1
				}
			}
			if startSegment != -1 {
				segments = append(segments, struct{ startX, endX int }{startSegment, maxX})
			}
		}

		for _, seg := range segments {
			if seg.endX-seg.startX < 3 {
				continue
			}

			startX := offset + float64(seg.startX)*scaleX
			startY := offset + float64(y)*scaleY
			endX := offset + float64(seg.endX)*scaleX

			sb.WriteString(fmt.Sprintf("G0 X%.3f Y%.3f\nM3 S1000\n", startX, startY))
			sb.WriteString(fmt.Sprintf("G1 X%.3f Y%.3f\n", endX, startY))
			sb.WriteString("M5\n")
		}
	}
}

func getGrayscale(img image.Image, bounds image.Rectangle, x, y int) int {
	r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
	r8 := uint8(r >> 8)
	g8 := uint8(g >> 8)
	b8 := uint8(b >> 8)
	return (299*int(r8) + 587*int(g8) + 114*int(b8)) / 1000
}

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	inputFile := flag.String("input", "", "Path to the input SVG file")
	outputFile := flag.String("output", "output.gcode", "Path to output G-code file")
	width := flag.Float64("width", 100.0, "Target engraving width (mm)")
	height := flag.Float64("height", 100.0, "Target engraving height (mm)")
	offset := flag.Float64("offset", 0.0, "Offset (mm) to apply to both X and Y")
	threshold := flag.Uint("threshold", 128, "Grayscale threshold for engraving (0-255)")
	flag.Parse()

	if *inputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	img, err := LoadSVG(*inputFile)
	if err != nil {
		log.Fatalf("failed to load SVG: %v", err)
	}

	gcode, err := ConvertToGCode(img, *width, *height, *offset, uint8(*threshold))
	if err != nil {
		log.Fatalf("failed to convert image to G-code: %v", err)
	}

	if err = os.WriteFile(*outputFile, []byte(gcode), 0644); err != nil {
		log.Fatalf("failed to write output file: %v", err)
	}

	fmt.Printf("G-code successfully written to %s\n", *outputFile)
}

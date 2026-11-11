package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImagePlacement(t *testing.T) {
	tests := []struct {
		fit     string
		want    rect
		imageW  float64
		imageH  float64
		targetW float64
		targetH float64
	}{
		{"contain", rect{0, 50, 200, 100}, 400, 200, 200, 200},
		{"cover", rect{-100, 0, 400, 200}, 400, 200, 200, 200},
		{"stretch", rect{0, 0, 200, 200}, 400, 200, 200, 200},
	}
	for _, tt := range tests {
		got, err := imagePlacement(tt.imageW, tt.imageH, tt.targetW, tt.targetH, tt.fit)
		if err != nil || !sameRect(got, tt.want) {
			t.Errorf("%s: got %#v, err %v; want %#v", tt.fit, got, err, tt.want)
		}
	}
}

func TestImagePlacementZoomInterpolatesContainAndCover(t *testing.T) {
	contain, err := imagePlacementZoom(400, 200, 200, 200, 0)
	if err != nil || !sameRect(contain, rect{0, 50, 200, 100}) {
		t.Fatalf("contain endpoint: %#v, %v", contain, err)
	}
	cover, err := imagePlacementZoom(400, 200, 200, 200, 1)
	if err != nil || !sameRect(cover, rect{-100, 0, 400, 200}) {
		t.Fatalf("cover endpoint: %#v, %v", cover, err)
	}
}

func TestRunCreatesExpectedNumberOfPages(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "source.png")
	output := filepath.Join(dir, "poster.pdf")
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	f, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	err = run(options{input: input, output: output, width: 410, height: 580, fit: "contain", orientation: "portrait"})
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if pages := strings.Count(string(pdf), "/Type /Page\n"); pages != 4 {
		t.Fatalf("generated %d page objects; want 4", pages)
	}
}

func TestRunSkipsUncheckedTilesAndAddsMap(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "source.png")
	output := filepath.Join(dir, "poster.pdf")
	f, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 40, 20))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	err = run(options{input: input, output: output, width: 410, height: 580, fit: "contain", orientation: "portrait", printMap: true, skipTiles: map[tileKey]bool{{col: 0, row: 0}: true}})
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	// Four visible image tiles, minus one skipped tile, plus the map page.
	if pages := strings.Count(string(pdf), "/Type /Page\n"); pages != 4 {
		t.Fatalf("generated %d page objects; want 4", pages)
	}
}

func TestVisibleTilesSkipsBlankColumns(t *testing.T) {
	// A tall image, centred in a wide target canvas, touches only the middle
	// A4 column. The two blank columns must not turn into PDF pages.
	dest, err := imagePlacement(100, 300, 630, 297, "contain")
	if err != nil {
		t.Fatal(err)
	}
	tiles := visibleTiles(3, 1, pageSize{width: 210, height: 297}, 630, 297, dest, nil)
	if len(tiles) != 1 {
		t.Fatalf("generated %d visible tiles; want 1", len(tiles))
	}
	if !closeEnough(tiles[0].bounds.x, 210) {
		t.Fatalf("visible tile starts at %.2fmm; want the middle column at 210mm", tiles[0].bounds.x)
	}
}

func sameRect(a, b rect) bool {
	return closeEnough(a.x, b.x) && closeEnough(a.y, b.y) && closeEnough(a.width, b.width) && closeEnough(a.height, b.height)
}

func closeEnough(a, b float64) bool { return math.Abs(a-b) < 0.000001 }

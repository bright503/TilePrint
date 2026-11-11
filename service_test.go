package main

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sourceForTest(t *testing.T) (*PosterService, LayoutRequest) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 40, 20))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	s := &PosterService{}
	source, err := s.loadSource(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(source.DataURL, "data:image/png;base64,") {
		t.Fatal("missing preview image")
	}
	return s, LayoutRequest{SourceID: source.ID, Width: 420, Height: 594, Orientation: "portrait", PrintMap: true, Skipped: []TileID{{0, 0}}}
}

func TestPreviewMatchesExport(t *testing.T) {
	for _, orientation := range []string{"portrait", "landscape", "auto"} {
		for _, zoom := range []float64{0, .5, 1} {
			s, req := sourceForTest(t)
			req.Orientation, req.Zoom, req.OffsetX, req.OffsetY = orientation, zoom, 27, -15
			opt, layout, err := s.prepare(req)
			if err != nil {
				t.Fatal(err)
			}
			dest, err := imagePlacementForOptions(40, 20, opt)
			if err != nil || layout.Image != previewRect(dest) {
				t.Fatal("preview image differs from PDF geometry")
			}
			opt.output = filepath.Join(t.TempDir(), "poster.pdf")
			if err := exportAtomic(opt); err != nil {
				t.Fatal(err)
			}
			pdf, err := os.ReadFile(opt.output)
			if err != nil {
				t.Fatal(err)
			}
			if pages := strings.Count(string(pdf), "/Type /Page\n"); pages != layout.Printed+1 {
				t.Fatalf("%s zoom %v: PDF has %d pages, preview says %d + map", orientation, zoom, pages, layout.Printed)
			}
			for _, tile := range layout.Tiles {
				if tile.Col == 0 && tile.Row == 0 && !tile.Skipped {
					t.Fatal("skipped tile was not marked")
				}
			}
		}
	}
}
func TestPreviewRejectsInvalidAndStaleRequests(t *testing.T) {
	s, base := sourceForTest(t)
	cases := []LayoutRequest{base, base, base, base, base}
	cases[0].Width = math.NaN()
	cases[1].OffsetX = math.Inf(1)
	cases[2].Height = 1e12
	cases[3].SourceID++
	cases[4].Zoom = 2
	for i, req := range cases {
		if _, err := s.Preview(req); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}
func TestAtomicExportPreservesDestinationOnFailure(t *testing.T) {
	s, req := sourceForTest(t)
	opt, _, err := s.prepare(req)
	if err != nil {
		t.Fatal(err)
	}
	opt.output = filepath.Join(t.TempDir(), "existing.pdf")
	if err := os.WriteFile(opt.output, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	opt.input = "does-not-exist.png"
	if err := exportAtomic(opt); err == nil {
		t.Fatal("expected failure")
	}
	data, err := os.ReadFile(opt.output)
	if err != nil || string(data) != "keep" {
		t.Fatal("destination modified on failed export")
	}
	files, err := os.ReadDir(filepath.Dir(opt.output))
	if err != nil || len(files) != 1 {
		t.Fatal("temporary file leaked")
	}
}

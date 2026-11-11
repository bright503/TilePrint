package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sync"
)

// LayoutRequest is shared by preview and export; all geometry is in millimetres.
type LayoutRequest struct {
	SourceID    uint64   `json:"sourceID"`
	Width       float64  `json:"width"`
	Height      float64  `json:"height"`
	Zoom        float64  `json:"zoom"`
	OffsetX     float64  `json:"offsetX"`
	OffsetY     float64  `json:"offsetY"`
	Orientation string   `json:"orientation"`
	PrintMap    bool     `json:"printMap"`
	Skipped     []TileID `json:"skipped"`
}
type TileID struct {
	Col int `json:"col"`
	Row int `json:"row"`
}
type PreviewRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
type PreviewTile struct {
	TileID
	Bounds  PreviewRect `json:"bounds"`
	Skipped bool        `json:"skipped"`
}
type PreviewLayout struct {
	Image   PreviewRect   `json:"image"`
	Page    PreviewRect   `json:"page"`
	Tiles   []PreviewTile `json:"tiles"`
	Printed int           `json:"printed"`
}
type SourceImage struct {
	ID      uint64 `json:"id"`
	Name    string `json:"name"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	DataURL string `json:"dataURL"`
}

// PosterService protects source state because MyGo runs calls concurrently.
type PosterService struct {
	mu            sync.Mutex
	sourceID      uint64
	input         string
	width, height int
}

func (s *PosterService) loadSource(path string) (*SourceImage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	var data bytes.Buffer
	// Encode the first frame as PNG so GIF preview agrees with PDF rendering.
	if err := png.Encode(&data, img); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sourceID++
	s.input, s.width, s.height = path, img.Bounds().Dx(), img.Bounds().Dy()
	return &SourceImage{ID: s.sourceID, Name: filepath.Base(path), Width: s.width, Height: s.height, DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes())}, nil
}
func previewRect(r rect) PreviewRect { return PreviewRect{r.x, r.y, r.width, r.height} }
func (s *PosterService) prepare(req LayoutRequest) (options, PreviewLayout, error) {
	s.mu.Lock()
	input, id, w, h := s.input, s.sourceID, s.width, s.height
	s.mu.Unlock()
	if input == "" || id != req.SourceID {
		return options{}, PreviewLayout{}, errors.New("请重新选择图片")
	}
	skipped := make(map[tileKey]bool, len(req.Skipped))
	for _, t := range req.Skipped {
		skipped[tileKey{t.Col, t.Row}] = true
	}
	opt := options{input: input, width: req.Width, height: req.Height, zoom: req.Zoom, useZoom: true, offsetX: req.OffsetX, offsetY: req.OffsetY, orientation: req.Orientation, printMap: req.PrintMap, skipTiles: skipped}
	if err := validate(opt); err != nil {
		return options{}, PreviewLayout{}, err
	}
	dest, err := imagePlacementForOptions(float64(w), float64(h), opt)
	if err != nil {
		return options{}, PreviewLayout{}, err
	}
	page, err := choosePage(opt)
	if err != nil {
		return options{}, PreviewLayout{}, err
	}
	tiles := visibleTiles(int(math.Ceil(opt.width/page.width)), int(math.Ceil(opt.height/page.height)), page, opt.width, opt.height, dest, skipped)
	layout := PreviewLayout{Image: previewRect(dest), Page: PreviewRect{Width: page.width, Height: page.height}, Tiles: make([]PreviewTile, 0, len(tiles))}
	for _, t := range tiles {
		layout.Tiles = append(layout.Tiles, PreviewTile{TileID: TileID{t.col, t.row}, Bounds: previewRect(t.bounds), Skipped: t.skipped})
		if !t.skipped {
			layout.Printed++
		}
	}
	return opt, layout, nil
}
func (s *PosterService) Preview(req LayoutRequest) (PreviewLayout, error) {
	_, layout, err := s.prepare(req)
	return layout, err
}

// Render to a sibling temporary file, preserving the destination on failure.
func exportAtomic(opt options) error {
	input, err := filepath.Abs(opt.input)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(opt.output)
	if err != nil {
		return err
	}
	if input == output {
		return errors.New("输出文件不能覆盖原图片")
	}
	f, err := os.CreateTemp(filepath.Dir(output), ".tileprint-*.pdf")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err := f.Close(); err != nil {
		return err
	}
	opt.output = temp
	if err := run(opt); err != nil {
		return err
	}
	return os.Rename(temp, output)
}

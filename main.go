// tileprint creates a multi-page A4 PDF that can be printed and assembled
// into a poster at an exact physical size.
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

const (
	a4Short = 210.0 // millimetres
	a4Long  = 297.0
)

type options struct {
	input, output     string
	width, height     float64
	fit, orientation  string
	zoom              float64
	useZoom, printMap bool
	offsetX, offsetY  float64
	skipTiles         map[tileKey]bool
}

type pageSize struct{ width, height float64 }

type rect struct{ x, y, width, height float64 }

type tile struct {
	col, row int
	bounds   rect
	visible  rect
	skipped  bool
}

type tileKey struct{ col, row int }

func main() {
	opt := options{}
	var gui bool
	flag.StringVar(&opt.input, "in", "", "input image (PNG, JPEG, or GIF)")
	flag.StringVar(&opt.output, "out", "poster.pdf", "output PDF path")
	flag.Float64Var(&opt.width, "width", 0, "finished poster width in millimetres")
	flag.Float64Var(&opt.height, "height", 0, "finished poster height in millimetres")
	flag.StringVar(&opt.fit, "fit", "contain", "image fit: contain, cover, or stretch")
	flag.Float64Var(&opt.zoom, "zoom", -1, "relative zoom from contain (0) to cover (1)")
	flag.Float64Var(&opt.offsetX, "offset-x", 0, "horizontal image offset in millimetres")
	flag.Float64Var(&opt.offsetY, "offset-y", 0, "vertical image offset in millimetres")
	flag.BoolVar(&opt.printMap, "map", false, "add a first-page tile assembly map")
	flag.StringVar(&opt.orientation, "orientation", "portrait", "A4 orientation: portrait, landscape, or auto")
	flag.BoolVar(&gui, "gui", false, "open the graphical interface")
	flag.Parse()
	opt.useZoom = opt.zoom >= 0
	if gui || opt.input == "" {
		startGUI()
		return
	}

	if err := run(opt); err != nil {
		fmt.Fprintf(os.Stderr, "tileprint: %v\n", err)
		os.Exit(1)
	}
}

func run(opt options) error {
	if err := validate(opt); err != nil {
		return err
	}

	file, err := os.Open(opt.input)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return fmt.Errorf("read image dimensions: %w", err)
	}
	if config.Width < 1 || config.Height < 1 {
		return errors.New("image has no pixels")
	}

	page, err := choosePage(opt)
	if err != nil {
		return err
	}
	cols := int(math.Ceil(opt.width / page.width))
	rows := int(math.Ceil(opt.height / page.height))
	dest, err := imagePlacementForOptions(float64(config.Width), float64(config.Height), opt)
	if err != nil {
		return err
	}
	tiles := visibleTiles(cols, rows, page, opt.width, opt.height, dest, opt.skipTiles)

	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr: "mm",
		Size:    gofpdf.SizeType{Wd: page.width, Ht: page.height},
	})
	pdf.SetTitle("TilePrint poster", false)
	pdf.SetAuthor("TilePrint", false)
	pdf.SetCreator("TilePrint", false)
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.RegisterImageOptions(opt.input, gofpdf.ImageOptions{ImageType: imageType(opt.input), ReadDpi: false})
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("load input image: %w", err)
	}

	if opt.printMap {
		paintMap(pdf, opt.input, page, opt.width, opt.height, dest, tiles)
	}
	printed := 0
	for _, tile := range tiles {
		if tile.skipped {
			continue
		}
		pdf.AddPage()
		paintTile(pdf, opt.input, tile, dest)
		printed++
	}
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("render PDF: %w", err)
	}
	if err := pdf.OutputFileAndClose(opt.output); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Printf("Created %s: %d image A4 pages (from a %d columns × %d rows layout).\n", opt.output, printed, cols, rows)
	fmt.Printf("Target size: %.1f × %.1f mm. Print at 100%% / Actual size. Blank areas are not drawn.\n", opt.width, opt.height)
	return nil
}

func validate(opt options) error {
	if opt.input == "" {
		return errors.New("-in is required")
	}
	if opt.width <= 0 || opt.height <= 0 {
		return errors.New("-width and -height must be greater than zero")
	}
	if !opt.useZoom && !oneOf(strings.ToLower(opt.fit), "contain", "cover", "stretch") {
		return errors.New("-fit must be contain, cover, or stretch")
	}
	if !oneOf(strings.ToLower(opt.orientation), "portrait", "landscape", "auto") {
		return errors.New("-orientation must be portrait, landscape, or auto")
	}
	if opt.useZoom && (opt.zoom < 0 || opt.zoom > 1) {
		return errors.New("-zoom must be between 0 (contain) and 1 (cover)")
	}
	if _, err := os.Stat(opt.input); err != nil {
		return fmt.Errorf("input image: %w", err)
	}
	if imageType(opt.input) == "" {
		return errors.New("input must be a PNG, JPEG, or GIF image")
	}
	return nil
}

func imagePlacementForOptions(sourceW, sourceH float64, opt options) (rect, error) {
	var dest rect
	var err error
	if opt.useZoom {
		dest, err = imagePlacementZoom(sourceW, sourceH, opt.width, opt.height, opt.zoom)
	} else {
		dest, err = imagePlacement(sourceW, sourceH, opt.width, opt.height, opt.fit)
	}
	dest.x += opt.offsetX
	dest.y += opt.offsetY
	return dest, err
}

func imagePlacementZoom(sourceW, sourceH, targetW, targetH, zoom float64) (rect, error) {
	if sourceW <= 0 || sourceH <= 0 || zoom < 0 || zoom > 1 {
		return rect{}, errors.New("invalid image zoom")
	}
	contain := math.Min(targetW/sourceW, targetH/sourceH)
	cover := math.Max(targetW/sourceW, targetH/sourceH)
	scale := contain + (cover-contain)*zoom
	w, h := sourceW*scale, sourceH*scale
	return rect{x: (targetW - w) / 2, y: (targetH - h) / 2, width: w, height: h}, nil
}

func choosePage(opt options) (pageSize, error) {
	portrait := pageSize{a4Short, a4Long}
	landscape := pageSize{a4Long, a4Short}
	switch strings.ToLower(opt.orientation) {
	case "portrait":
		return portrait, nil
	case "landscape":
		return landscape, nil
	case "auto":
		pPages := pageCount(opt.width, opt.height, portrait)
		lPages := pageCount(opt.width, opt.height, landscape)
		if lPages < pPages {
			return landscape, nil
		}
		return portrait, nil
	default:
		return pageSize{}, errors.New("invalid orientation")
	}
}

func pageCount(width, height float64, page pageSize) int {
	return int(math.Ceil(width/page.width)) * int(math.Ceil(height/page.height))
}

func imagePlacement(sourceW, sourceH, targetW, targetH float64, fit string) (rect, error) {
	if sourceW <= 0 || sourceH <= 0 {
		return rect{}, errors.New("invalid source image size")
	}
	if fit == "stretch" {
		return rect{width: targetW, height: targetH}, nil
	}
	scaleX, scaleY := targetW/sourceW, targetH/sourceH
	scale := math.Min(scaleX, scaleY)
	if fit == "cover" {
		scale = math.Max(scaleX, scaleY)
	}
	w, h := sourceW*scale, sourceH*scale
	return rect{x: (targetW - w) / 2, y: (targetH - h) / 2, width: w, height: h}, nil
}

func visibleTiles(cols, rows int, page pageSize, targetW, targetH float64, dest rect, skipped map[tileKey]bool) []tile {
	target := rect{width: targetW, height: targetH}
	tiles := make([]tile, 0, cols*rows)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			bounds := rect{
				x:      float64(col) * page.width,
				y:      float64(row) * page.height,
				width:  math.Min(page.width, targetW-float64(col)*page.width),
				height: math.Min(page.height, targetH-float64(row)*page.height),
			}
			visible := intersect(intersect(bounds, target), dest)
			if visible.width > 0 && visible.height > 0 {
				key := tileKey{col: col, row: row}
				tiles = append(tiles, tile{col: col, row: row, bounds: bounds, visible: visible, skipped: skipped[key]})
			}
		}
	}
	return tiles
}

func paintMap(pdf *gofpdf.Fpdf, imageName string, page pageSize, targetW, targetH float64, dest rect, tiles []tile) {
	pdf.AddPage()
	const margin = 12.0
	scale := math.Min((page.width-2*margin)/targetW, (page.height-2*margin-12)/targetH)
	w, h := targetW*scale, targetH*scale
	x, y := (page.width-w)/2, 18.0+(page.height-30-h)/2
	pdf.ClipRect(x, y, w, h, false)
	pdf.ImageOptions(imageName, x+dest.x*scale, y+dest.y*scale, dest.width*scale, dest.height*scale, false, gofpdf.ImageOptions{AllowNegativePosition: true}, 0, "")
	pdf.ClipEnd()
	pdf.SetFont("Arial", "B", 13)
	pdf.Text(margin, 10, "Tile assembly map")
	pdf.SetDrawColor(20, 24, 31)
	pdf.Rect(x, y, w, h, "D")
	for _, tile := range tiles {
		tx, ty := x+tile.bounds.x*scale, y+tile.bounds.y*scale
		tw, th := tile.bounds.width*scale, tile.bounds.height*scale
		pdf.SetDrawColor(37, 99, 235)
		pdf.Rect(tx, ty, tw, th, "D")
		if tile.skipped {
			pdf.SetDrawColor(220, 38, 38)
			pdf.Line(tx, ty, tx+tw, ty+th)
			pdf.Line(tx+tw, ty, tx, ty+th)
		}
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("Arial", "B", 8)
		pdf.Text(tx+2, ty+5, fmt.Sprintf("%d,%d", tile.col+1, tile.row+1))
	}
	pdf.SetTextColor(0, 0, 0)
}

func intersect(a, b rect) rect {
	x1, y1 := math.Max(a.x, b.x), math.Max(a.y, b.y)
	x2, y2 := math.Min(a.x+a.width, b.x+b.width), math.Min(a.y+a.height, b.y+b.height)
	if x2 <= x1 || y2 <= y1 {
		return rect{}
	}
	return rect{x: x1, y: y1, width: x2 - x1, height: y2 - y1}
}

func paintTile(pdf *gofpdf.Fpdf, imageName string, tile tile, dest rect) {
	// The clipping rectangle makes every page a window onto the one, scaled
	// poster image. Empty areas are never painted into the PDF.
	pdf.ClipRect(tile.visible.x-tile.bounds.x, tile.visible.y-tile.bounds.y, tile.visible.width, tile.visible.height, false)
	pdf.ImageOptions(imageName, dest.x-tile.bounds.x, dest.y-tile.bounds.y, dest.width, dest.height, false, gofpdf.ImageOptions{AllowNegativePosition: true}, 0, "")
	pdf.ClipEnd()
}

func imageType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "JPG"
	case ".png":
		return "PNG"
	case ".gif":
		return "GIF"
	default:
		return ""
	}
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

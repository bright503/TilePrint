package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

const maxPreviewGridLines = 128

// startGUI opens the desktop workflow. Command-line generation remains
// available whenever -in is supplied.
func startGUI() {
	application := app.NewWithID("com.tileprint.app")
	window := application.NewWindow("TilePrint · A4 拼图打印")
	window.Resize(fyne.NewSize(1120, 720))

	preview := newPosterPreview()
	width := widget.NewEntry()
	width.SetText("600")
	height := widget.NewEntry()
	height.SetText("900")
	zoom := widget.NewSlider(0, 1)
	zoom.Step = 0.01
	zoomLabel := widget.NewLabel("Contain  0%  →  Cover")
	orientation := widget.NewSelect([]string{"portrait", "landscape", "auto"}, nil)
	orientation.SetSelected("portrait")
	printMap := widget.NewCheck("第一页打印拼接地图", nil)
	offsetX, offsetY := 0.0, 0.0
	offsetLabel := widget.NewLabel("偏移：X 0 mm，Y 0 mm")
	skippedTiles := map[tileKey]bool{}
	tileChecks := container.NewVBox()
	selectedPath := ""
	selectedImage := image.Image(nil)
	fileName := widget.NewLabel("尚未选择图片")
	fileName.Wrapping = fyne.TextWrapBreak
	status := widget.NewLabel("选择图片并填写目标尺寸后，即可导出 PDF。")
	status.Wrapping = fyne.TextWrapWord

	var updatePreview func()
	rebuildTileChecks := func(w, h float64) {
		if selectedImage == nil {
			tileChecks.RemoveAll()
			return
		}
		dest, err := imagePlacementForOptions(float64(selectedImage.Bounds().Dx()), float64(selectedImage.Bounds().Dy()), options{width: w, height: h, useZoom: true, zoom: zoom.Value, offsetX: offsetX, offsetY: offsetY})
		if err != nil {
			return
		}
		page, _ := choosePage(options{width: w, height: h, orientation: orientation.Selected})
		tiles := visibleTiles(int(math.Ceil(w/page.width)), int(math.Ceil(h/page.height)), page, w, h, dest, skippedTiles)
		tileChecks.RemoveAll()
		for _, tile := range tiles {
			key := tileKey{col: tile.col, row: tile.row}
			check := widget.NewCheck(fmt.Sprintf("不打印图块 %d,%d", tile.col+1, tile.row+1), nil)
			check.SetChecked(tile.skipped)
			check.OnChanged = func(disabled bool) {
				skippedTiles[key] = disabled
				updatePreview()
			}
			tileChecks.Add(check)
		}
	}
	updatePreview = func() {
		w, h, err := dimensionsFromEntries(width, height)
		if err != nil {
			preview.Set(selectedImage, 0, 0, 0, 0, 0, orientation.Selected, skippedTiles)
			return
		}
		offsetLabel.SetText(fmt.Sprintf("偏移：X %.0f mm，Y %.0f mm", offsetX, offsetY))
		zoomLabel.SetText(fmt.Sprintf("缩放：Contain  %.0f%%  Cover", zoom.Value*100))
		preview.Set(selectedImage, w, h, zoom.Value, offsetX, offsetY, orientation.Selected, skippedTiles)
		rebuildTileChecks(w, h)
	}
	preview.onTileToggled = func() { updatePreview() }
	width.OnChanged = func(string) { updatePreview() }
	height.OnChanged = func(string) { updatePreview() }
	zoom.OnChanged = func(float64) { updatePreview() }
	orientation.OnChanged = func(string) { updatePreview() }

	chooseImage := widget.NewButton("选择图片…", func() {
		picker := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			if reader == nil {
				return
			}
			path := reader.URI().Path()
			_ = reader.Close()
			img, err := loadImage(path)
			if err != nil {
				dialog.ShowError(fmt.Errorf("无法读取图片：%w", err), window)
				return
			}
			selectedPath, selectedImage = path, img
			fileName.SetText(filepath.Base(path))
			status.SetText(fmt.Sprintf("已载入 %s（%d × %d 像素）", filepath.Base(path), img.Bounds().Dx(), img.Bounds().Dy()))
			updatePreview()
		}, window)
		picker.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg", ".gif"}))
		picker.Show()
	})

	export := widget.NewButton("导出 A4 PDF…", func() {
		if selectedPath == "" {
			dialog.ShowInformation("请先选择图片", "请选择一张 PNG、JPEG 或 GIF 图片。", window)
			return
		}
		w, h, err := dimensionsFromEntries(width, height)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}
		if orientation.Selected == "" {
			dialog.ShowError(fmt.Errorf("请选择 A4 方向"), window)
			return
		}
		saver := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			if writer == nil {
				return
			}
			output := writer.URI().Path()
			if err := writer.Close(); err != nil {
				dialog.ShowError(err, window)
				return
			}
			if err := run(options{input: selectedPath, output: output, width: w, height: h, orientation: orientation.Selected, useZoom: true, zoom: zoom.Value, offsetX: offsetX, offsetY: offsetY, skipTiles: skippedTiles, printMap: printMap.Checked}); err != nil {
				dialog.ShowError(err, window)
				return
			}
			status.SetText("PDF 已导出：" + output)
			dialog.ShowInformation("导出完成", "请在打印对话框中选择“实际大小 / 100%”。", window)
		}, window)
		saver.SetFileName("poster.pdf")
		saver.SetFilter(storage.NewExtensionFileFilter([]string{".pdf"}))
		saver.Show()
	})

	settings := widget.NewForm(
		widget.NewFormItem("输入图片", container.NewVBox(chooseImage, fileName)),
		widget.NewFormItem("目标宽度（mm）", width),
		widget.NewFormItem("目标高度（mm）", height),
		widget.NewFormItem("等比缩放", container.NewVBox(zoom, zoomLabel)),
		widget.NewFormItem("移动裁切", container.NewVBox(offsetLabel, container.NewGridWithColumns(3,
			widget.NewButton("←", func() { offsetX -= 5; updatePreview() }),
			widget.NewButton("↑", func() { offsetY -= 5; updatePreview() }),
			widget.NewButton("→", func() { offsetX += 5; updatePreview() }),
			widget.NewButton("重置", func() { offsetX, offsetY = 0, 0; updatePreview() }),
			widget.NewButton("↓", func() { offsetY += 5; updatePreview() }),
			widget.NewLabel("每次 5 mm"),
		))),
		widget.NewFormItem("A4 方向", orientation),
		widget.NewFormItem("打印选项", printMap),
		widget.NewFormItem("取消打印图块", container.NewVScroll(tileChecks)),
	)
	left := container.NewBorder(
		widget.NewLabelWithStyle("导出设置", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVBox(status, export), nil, nil,
		container.NewVScroll(settings),
	)
	right := container.NewBorder(
		widget.NewLabelWithStyle("预览：黑框为目标尺寸，蓝线为 A4 裁切线", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil, preview,
	)
	split := container.NewHSplit(left, right)
	split.Offset = 0.34
	window.SetContent(split)
	updatePreview()
	window.ShowAndRun()
}

func dimensionsFromEntries(width, height *widget.Entry) (float64, float64, error) {
	w, err := strconv.ParseFloat(width.Text, 64)
	if err != nil || w <= 0 {
		return 0, 0, fmt.Errorf("目标宽度必须是大于 0 的数字（单位：mm）")
	}
	h, err := strconv.ParseFloat(height.Text, 64)
	if err != nil || h <= 0 {
		return 0, 0, fmt.Errorf("目标高度必须是大于 0 的数字（单位：mm）")
	}
	return w, h, nil
}

func loadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	return img, err
}

// posterPreview is a scale-accurate on-screen composition. Its outer black
// rectangle is the requested finished size; blue lines show A4 page cuts.
type posterPreview struct {
	widget.BaseWidget
	source                    image.Image
	width, height             float64
	zoom, offsetX, offsetY    float64
	orientation               string
	background, target, frame *canvas.Rectangle
	imageView                 *canvas.Image
	imageLayer                *fyne.Container
	imageClip                 *container.Clip
	caption                   *canvas.Text
	lines                     []*canvas.Line
	skippedOverlays           []*canvas.Rectangle
	objects                   []fyne.CanvasObject
	tiles                     []tile
	skipped                   map[tileKey]bool
	targetPosition            fyne.Position
	previewScale              float64
	onTileToggled             func()
}

func newPosterPreview() *posterPreview {
	p := &posterPreview{
		orientation: "portrait",
		background:  canvas.NewRectangle(color.NRGBA{R: 245, G: 247, B: 250, A: 255}),
		target:      canvas.NewRectangle(color.NRGBA{R: 255, G: 255, B: 255, A: 255}),
		frame:       canvas.NewRectangle(color.Transparent),
		imageView:   canvas.NewImageFromImage(nil),
		caption:     canvas.NewText("请输入目标尺寸", color.NRGBA{R: 55, G: 65, B: 81, A: 255}),
	}
	p.frame.StrokeColor = color.NRGBA{R: 20, G: 24, B: 31, A: 255}
	p.frame.StrokeWidth = 2
	p.imageView.FillMode = canvas.ImageFillContain
	p.imageView.ScaleMode = canvas.ImageScaleSmooth
	p.imageLayer = container.NewWithoutLayout(p.imageView)
	p.imageClip = container.NewClip(p.imageLayer)
	p.objects = []fyne.CanvasObject{p.background, p.target, p.imageClip, p.frame, p.caption}
	for range maxPreviewGridLines {
		overlay := canvas.NewRectangle(color.NRGBA{R: 220, G: 38, B: 38, A: 90})
		overlay.StrokeColor = color.NRGBA{R: 185, G: 28, B: 28, A: 255}
		overlay.StrokeWidth = 1
		overlay.Hide()
		p.skippedOverlays = append(p.skippedOverlays, overlay)
		p.objects = append(p.objects, overlay)
	}
	for range maxPreviewGridLines {
		line := canvas.NewLine(color.NRGBA{R: 37, G: 99, B: 235, A: 255})
		line.StrokeWidth = 1
		line.Hide()
		p.lines = append(p.lines, line)
		p.objects = append(p.objects, line)
	}
	p.ExtendBaseWidget(p)
	return p
}

func (p *posterPreview) Set(source image.Image, width, height, zoom, offsetX, offsetY float64, orientation string, skipped map[tileKey]bool) {
	p.source, p.width, p.height = source, width, height
	p.zoom, p.offsetX, p.offsetY, p.orientation = zoom, offsetX, offsetY, orientation
	p.skipped = skipped
	p.updateTiles()
	p.Refresh()
}

func (p *posterPreview) updateTiles() {
	p.tiles = nil
	if p.source == nil || p.width <= 0 || p.height <= 0 {
		return
	}
	dest, err := imagePlacementForOptions(float64(p.source.Bounds().Dx()), float64(p.source.Bounds().Dy()), options{width: p.width, height: p.height, useZoom: true, zoom: p.zoom, offsetX: p.offsetX, offsetY: p.offsetY})
	if err != nil {
		return
	}
	page, err := choosePage(options{width: p.width, height: p.height, orientation: p.orientation})
	if err != nil {
		return
	}
	p.tiles = visibleTiles(int(math.Ceil(p.width/page.width)), int(math.Ceil(p.height/page.height)), page, p.width, p.height, dest, p.skipped)
}

// Tapped lets the preview itself act as the tile selector.
func (p *posterPreview) Tapped(event *fyne.PointEvent) {
	if p.previewScale <= 0 || p.skipped == nil {
		return
	}
	x := float64(event.Position.X-p.targetPosition.X) / p.previewScale
	y := float64(event.Position.Y-p.targetPosition.Y) / p.previewScale
	for _, tile := range p.tiles {
		if x >= tile.visible.x && x <= tile.visible.x+tile.visible.width && y >= tile.visible.y && y <= tile.visible.y+tile.visible.height {
			key := tileKey{col: tile.col, row: tile.row}
			p.skipped[key] = !p.skipped[key]
			p.updateTiles()
			if p.onTileToggled != nil {
				p.onTileToggled()
			} else {
				p.Refresh()
			}
			return
		}
	}
}

func (p *posterPreview) CreateRenderer() fyne.WidgetRenderer {
	return &posterPreviewRenderer{preview: p}
}

type posterPreviewRenderer struct{ preview *posterPreview }

func (r *posterPreviewRenderer) Destroy() {}

func (r *posterPreviewRenderer) Layout(size fyne.Size) {
	p := r.preview
	p.background.Resize(size)
	if p.width <= 0 || p.height <= 0 {
		p.caption.Text = "请输入有效的目标尺寸"
		p.caption.Move(fyne.NewPos(16, 16))
		p.caption.Show()
		p.target.Hide()
		p.frame.Hide()
		p.imageClip.Hide()
		hideOverlays(p.skippedOverlays)
		p.imageView.Hide()
		hideLines(p.lines)
		return
	}

	const padding, captionHeight = 18, 28
	availableW := float64(size.Width) - padding*2
	availableH := float64(size.Height) - padding*2 - captionHeight
	if availableW <= 0 || availableH <= 0 {
		return
	}
	scale := min(availableW/p.width, availableH/p.height)
	previewW, previewH := p.width*scale, p.height*scale
	x := (float64(size.Width) - previewW) / 2
	y := captionHeight + padding + (availableH-previewH)/2
	position := fyne.NewPos(float32(x), float32(y))
	previewSize := fyne.NewSize(float32(previewW), float32(previewH))
	p.targetPosition, p.previewScale = position, scale

	p.target.Move(position)
	p.target.Resize(previewSize)
	p.target.Show()
	p.imageView.Image = p.source
	p.imageView.FillMode = canvas.ImageFillStretch
	p.imageClip.Move(position)
	p.imageClip.Resize(previewSize)
	if p.source == nil {
		p.imageView.Hide()
		p.imageClip.Hide()
	} else {
		dest, err := imagePlacementForOptions(float64(p.source.Bounds().Dx()), float64(p.source.Bounds().Dy()), options{width: p.width, height: p.height, useZoom: true, zoom: p.zoom, offsetX: p.offsetX, offsetY: p.offsetY})
		if err == nil {
			p.imageView.Move(fyne.NewPos(float32(dest.x*scale), float32(dest.y*scale)))
			p.imageView.Resize(fyne.NewSize(float32(dest.width*scale), float32(dest.height*scale)))
		}
		p.imageView.Show()
		p.imageClip.Show()
	}
	p.frame.Move(position)
	p.frame.Resize(previewSize)
	p.frame.Show()

	page, err := choosePage(options{width: p.width, height: p.height, orientation: p.orientation})
	if err != nil {
		page = pageSize{width: a4Short, height: a4Long}
	}
	cols := int(math.Ceil(p.width / page.width))
	rows := int(math.Ceil(p.height / page.height))
	p.caption.Text = fmt.Sprintf("目标 %.1f × %.1f mm · A4 %s · %d × %d 页", p.width, p.height, p.orientation, cols, rows)
	p.caption.Move(fyne.NewPos(float32(x), 8))
	p.caption.Refresh()
	p.caption.Show()
	overlayIndex := 0
	for _, tile := range p.tiles {
		if !tile.skipped || overlayIndex >= len(p.skippedOverlays) {
			continue
		}
		overlay := p.skippedOverlays[overlayIndex]
		overlay.Move(fyne.NewPos(float32(x+tile.visible.x*scale), float32(y+tile.visible.y*scale)))
		overlay.Resize(fyne.NewSize(float32(tile.visible.width*scale), float32(tile.visible.height*scale)))
		overlay.Show()
		overlayIndex++
	}
	for ; overlayIndex < len(p.skippedOverlays); overlayIndex++ {
		p.skippedOverlays[overlayIndex].Hide()
	}

	lineIndex := 0
	for col := 1; col < cols && lineIndex < len(p.lines); col++ {
		line := p.lines[lineIndex]
		line.Position1 = fyne.NewPos(float32(x+float64(col)*page.width*scale), float32(y))
		line.Position2 = fyne.NewPos(float32(x+float64(col)*page.width*scale), float32(y+previewH))
		line.Show()
		lineIndex++
	}
	for row := 1; row < rows && lineIndex < len(p.lines); row++ {
		line := p.lines[lineIndex]
		line.Position1 = fyne.NewPos(float32(x), float32(y+float64(row)*page.height*scale))
		line.Position2 = fyne.NewPos(float32(x+previewW), float32(y+float64(row)*page.height*scale))
		line.Show()
		lineIndex++
	}
	for ; lineIndex < len(p.lines); lineIndex++ {
		p.lines[lineIndex].Hide()
	}
}

func (r *posterPreviewRenderer) MinSize() fyne.Size           { return fyne.NewSize(420, 360) }
func (r *posterPreviewRenderer) Objects() []fyne.CanvasObject { return r.preview.objects }
func (r *posterPreviewRenderer) Refresh()                     { r.Layout(r.preview.Size()) }

func hideLines(lines []*canvas.Line) {
	for _, line := range lines {
		line.Hide()
	}
}

func hideOverlays(overlays []*canvas.Rectangle) {
	for _, overlay := range overlays {
		overlay.Hide()
	}
}

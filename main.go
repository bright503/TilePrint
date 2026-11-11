package main

import (
	"flag"
	"fmt"
	"os"
)

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

# TilePrint

TilePrint turns one image into a PDF made of edge-to-edge A4 tiles. Print the PDF at **Actual size / 100%** and join the sheets to make a poster at the requested physical size.

## Desktop interface

Requires Go **1.27.3 or later** (Go can download the toolchain automatically).
The desktop interface uses [MyGo](https://github.com/egoist/mygo) v0.8.3 and the
system WebView. On macOS, WKWebView is included. Windows needs WebView2 Runtime;
Linux needs GTK 3 and WebKitGTK 4.1 (`libwebkit2gtk-4.1-0` on Debian/Ubuntu).

Run the project without arguments to open the MyGo desktop interface:

```sh
go run .
```

Choose an input image, enter the target width and height in millimetres, then choose **导出 A4 PDF…**. The live preview shows the finished-size frame in black and the A4 cut positions in blue. It uses the same image-fit and orientation settings as the exported PDF.

The zoom slider continuously interpolates from `contain` to `cover`; the arrow buttons move the image in 5 mm increments so you can decide which parts are cropped. Click a visible A4 area directly in the preview to toggle its printing state; skipped blocks appear as a transparent red overlay. Click them again to restore printing. Enable **第一页打印拼接地图** to add an assembly map before the image tiles; skipped tiles are crossed out on that map.

The HTML/CSS/JavaScript in `web/` is embedded into the executable with `go:embed`;
no Node, Bun, frontend build, or local HTTP server is needed. The Canvas preview
requests image placement and visible tiles from Go, using the same geometry as
the PDF exporter. File selection and saving use native system dialogs. Export
writes a temporary file first, then replaces the chosen destination on success.

Build a local executable:

```sh
go build -o build/tileprint .
```

The app uses `resources/icon.png` for its Dock/window icon. To package a macOS
app bundle with the Finder icon:

```sh
go run github.com/egoist/mygo/cmd/mygo@v0.8.3 build -skip-dmg
open build/darwin-arm64/TilePrint.app
```

`mygo.json` defines the app identity and icon; the build uses local ad-hoc signing.
The source prompt for the generated icon is in `resources/icon-prompt.txt`.

Run regression tests:

```sh
go test ./...
```

`main.go` contains the CLI entry point, `poster.go` the layout/PDF engine,
`service.go` the shared preview/export service, and `gui.go` the MyGo integration.
The layout is limited to 10,000 grid cells to keep preview/export responsive.

## Quick start

The command-line mode remains available when `-in` is supplied:

```sh
go run . \
  -in artwork.png \
  -out poster.pdf \
  -width 900 \
  -height 600
```

All dimensions are millimetres. The command above produces a 900 × 600 mm finished poster.

## Options

```text
-in             input PNG, JPEG, or GIF image (required)
-out            output PDF path (default "poster.pdf")
-width          finished poster width in mm (required)
-height         finished poster height in mm (required)
-fit            contain (default), cover, or stretch
-zoom           interpolate contain (0) to cover (1); overrides -fit
-offset-x       horizontal image offset in mm (default 0)
-offset-y       vertical image offset in mm (default 0)
-map            add a first-page assembly map
-gui            open the desktop interface
-orientation    portrait (default), landscape, or auto
```

`contain` keeps the complete image and centres it inside the requested finished size. `cover` fills the finished size and crops excess image. `stretch` ignores the source aspect ratio. With `contain`, blank areas are not painted and any A4 tile that contains no part of the image is omitted from the PDF.

There are no added page margins, trim guides, or labels. TilePrint places image pixels at the A4 edges. This requires a printer with borderless A4 support; a printer's hardware margin cannot be removed by a PDF.

## Printing

Disable the print dialog's “Fit to page” / “Scale to fit” setting. Choose **Actual size**, **100%**, or the equivalent.

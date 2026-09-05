# TilePrint

TilePrint turns one image into a PDF made of edge-to-edge A4 tiles. Print the PDF at **Actual size / 100%** and join the sheets to make a poster at the requested physical size.

## Desktop interface

Run the project without arguments to open the Fyne desktop interface:

```sh
go run .
```

Choose an input image, enter the target width and height in millimetres, then choose **导出 A4 PDF…**. The live preview shows the finished-size frame in black and the A4 cut positions in blue. It uses the same image-fit and orientation settings as the exported PDF.

The zoom slider continuously interpolates from `contain` to `cover`; the arrow buttons move the image in 5 mm increments so you can decide which parts are cropped. Click a visible A4 area directly in the preview to toggle its printing state; skipped blocks appear as a transparent red overlay and are also listed under **取消打印图块**. Enable **第一页打印拼接地图** to add an assembly map before the image tiles; skipped tiles are crossed out on that map.

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
-orientation    portrait (default), landscape, or auto
```

`contain` keeps the complete image and centres it inside the requested finished size. `cover` fills the finished size and crops excess image. `stretch` ignores the source aspect ratio. With `contain`, blank areas are not painted and any A4 tile that contains no part of the image is omitted from the PDF.

There are no added page margins, trim guides, or labels. TilePrint places image pixels at the A4 edges. This requires a printer with borderless A4 support; a printer's hardware margin cannot be removed by a PDF.

## Printing

Disable the print dialog's “Fit to page” / “Scale to fit” setting. Choose **Actual size**, **100%**, or the equivalent.

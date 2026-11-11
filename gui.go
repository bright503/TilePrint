package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

//go:embed web/*
var frontend embed.FS

//go:embed resources/icon.png
var appIcon []byte

func startGUI() {
	web, err := fs.Sub(frontend, "web")
	if err != nil {
		log.Fatal(err)
	}
	mygo.SetFrontend(web)
	mygo.App.SetName("TilePrint")
	mygo.Bind(&PosterService{})
	mygo.App.WhenReady(func() {
		window := mygo.NewWindow(mygo.WindowOptions{Title: "TilePrint · A4 拼图打印", URL: "/", Width: 1120, Height: 780, MinWidth: 820, MinHeight: 600})
		if runtime.GOOS == "darwin" {
			if err := mygo.App.Dock.SetIcon(appIcon); err != nil {
				log.Printf("set Dock icon: %v", err)
			}
		} else {
			if err := window.SetIcon(appIcon); err != nil {
				log.Printf("set window icon: %v", err)
			}
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// OpenImage only reads a file chosen in the native file dialog.
func (s *PosterService) OpenImage(ctx context.Context) (*SourceImage, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: mygo.CallerWindow(ctx), Title: "选择图片",
		Filters: []mygo.FileFilter{{Name: "图片", Extensions: []string{"png", "jpg", "jpeg", "gif"}}},
	})
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	return s.loadSource(paths[0])
}

// Export uses the same layout request as Preview. Canceling creates no file.
func (s *PosterService) Export(ctx context.Context, req LayoutRequest) (string, error) {
	opt, layout, err := s.prepare(req)
	if err != nil {
		return "", err
	}
	if layout.Printed == 0 {
		return "", fmt.Errorf("没有可打印的图块，请调整图片位置或恢复图块")
	}
	output, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent: mygo.CallerWindow(ctx), DefaultPath: "poster.pdf",
		Filters: []mygo.FileFilter{{Name: "PDF", Extensions: []string{"pdf"}}},
	})
	if err != nil || output == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(output), ".pdf") {
		output += ".pdf"
	}
	opt.output = output
	if err := exportAtomic(opt); err != nil {
		return "", err
	}
	return output, nil
}

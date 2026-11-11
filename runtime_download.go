package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

// The same bounded downloader is used for the signed Microsoft bootstrapper.
func downloadRuntime(ctx context.Context, client *http.Client, url, path string) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载 WebView2 失败：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 WebView2 失败：HTTP %d", response.StatusCode)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(path)
		}
	}()
	n, copyErr := io.Copy(f, io.LimitReader(response.Body, 16<<20+1))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n == 0 || n > 16<<20 {
		return fmt.Errorf("WebView2 下载文件大小异常")
	}
	return nil
}

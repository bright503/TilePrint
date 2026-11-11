package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDownload(t *testing.T) {
	for _, code := range []int{200, 404, 503} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code); w.Write([]byte("installer")) }))
		path := filepath.Join(t.TempDir(), "runtime.exe")
		err := downloadRuntime(context.Background(), server.Client(), server.URL, path)
		server.Close()
		if code == 200 {
			data, readErr := os.ReadFile(path)
			if err != nil || readErr != nil || string(data) != "installer" {
				t.Fatalf("bad download: %v, %v", err, readErr)
			}
		} else {
			if err == nil {
				t.Fatal("HTTP error accepted")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("failed download left executable")
			}
		}
	}
}
func TestRuntimeDownloadCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "runtime.exe")
	if err := downloadRuntime(ctx, http.DefaultClient, "https://example.invalid", path); err == nil {
		t.Fatal("canceled download succeeded")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("canceled download left executable")
	}
}

func TestRuntimeDownloadRejectsIncompletePayload(t *testing.T) {
	for _, kind := range []string{"empty", "oversized", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch kind {
				case "oversized":
					w.Write([]byte(strings.Repeat("x", (16<<20)+1)))
				case "truncated":
					w.Header().Set("Content-Length", "1000")
					w.Write([]byte("short"))
				}
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "runtime.exe")
			if err := downloadRuntime(context.Background(), server.Client(), server.URL, path); err == nil {
				t.Fatal("invalid payload accepted")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("invalid executable left on disk")
			}
		})
	}
}

//go:build ignore

// Run from the project root: go run scripts/package-windows.go [-offline].
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	offline := flag.Bool("offline", false, "bundle the Microsoft Evergreen x64 standalone installer")
	payload := flag.String("runtime-installer", "", "local Microsoft x64 standalone installer (implies -offline)")
	flag.Parse()
	if err := build(*offline || *payload != "", *payload); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func build(offline bool, payload string) error {
	// On Windows the MyGo CLI downloads NSIS automatically. Cross-packaging
	// on macOS/Linux requires NSIS installed, otherwise no setup is produced.
	if os.PathSeparator != '\\' {
		if _, err := exec.LookPath("makensis"); err != nil {
			return fmt.Errorf("需要 NSIS (makensis) 制作安装包；macOS 可运行 brew install nsis。可先使用 MyGo build -skip-installer 仅生成 exe")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	temp, err := os.MkdirTemp("", "tileprint-package-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if offline {
		if payload == "" {
			payload = filepath.Join(temp, "MicrosoftEdgeWebView2RuntimeInstallerX64.exe")
			fmt.Println("Downloading Microsoft Evergreen x64 standalone installer…")
			req, err := http.NewRequestWithContext(ctx, "GET", "https://go.microsoft.com/fwlink/?LinkId=2099617", nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			if resp.StatusCode != 200 {
				resp.Body.Close()
				return fmt.Errorf("runtime download HTTP %d", resp.StatusCode)
			}
			f, err := os.Create(payload)
			if err != nil {
				resp.Body.Close()
				return err
			}
			n, copyErr := io.Copy(f, io.LimitReader(resp.Body, (1<<30)+1))
			resp.Body.Close()
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if n == 0 || n > 1<<30 {
				return fmt.Errorf("invalid runtime installer size")
			}
		}
		payload, err = filepath.Abs(payload)
		if err != nil {
			return err
		}
		info, err := os.Stat(payload)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("runtime installer is not a regular file")
		}
	}
	// Separate outputs so online/offline artifacts never overwrite each other.
	mode := "online"
	if offline {
		mode = "offline"
	}
	out := filepath.Join("build", "windows-"+mode)
	cmd := exec.CommandContext(ctx, "go", "run", "github.com/egoist/mygo/cmd/mygo@v0.8.3", "build", "-platform", "windows/amd64", "-o", out)
	cmd.Env = append(os.Environ(), "TILEPRINT_WEBVIEW2_OFFLINE="+payload)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	setups, err := filepath.Glob(filepath.Join(out, "windows-amd64", "tileprint-setup-*.exe"))
	if err != nil {
		return err
	}
	if len(setups) == 0 {
		return fmt.Errorf("未生成安装包，请安装 NSIS 后重试")
	}
	fmt.Println("Installer ready:", setups[0])
	return nil
}

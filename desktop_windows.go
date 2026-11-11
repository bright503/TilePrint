package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/egoist/mygo"
)

const bootstrapURL = "https://go.microsoft.com/fwlink/p/?LinkId=2124703"
const runtimeWebsite = "https://developer.microsoft.com/microsoft-edge/webview2/"

var messageBox = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")

func nativeMessage(text string, flags uintptr) uintptr {
	title, _ := syscall.UTF16PtrFromString("TilePrint")
	body, _ := syscall.UTF16PtrFromString(text)
	result, _, _ := messageBox.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), flags)
	return result
}
func runtimeLog() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "TilePrint", "logs", "startup.log")
}
func initDesktopLog() {
	path := runtimeLog()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, f))
	}
}

// These internal installer commands run before flags and never start a window.
// Exit 2 means missing runtime; all other failures return 1.
func runtimeCommand() {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "--check-webview2":
		if mygo.WebViewAvailable() {
			os.Exit(0)
		}
		os.Exit(2)
	case "--install-webview2":
		initDesktopLog()
		payload := ""
		if len(os.Args) > 2 {
			payload = os.Args[2]
		}
		if err := installWebView(payload, true); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
func installWebView(payload string, silent bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if payload == "" {
		dir, err := os.MkdirTemp("", "tileprint-webview2-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		payload = filepath.Join(dir, "MicrosoftEdgeWebview2Setup.exe")
		log.Print("Downloading WebView2 bootstrapper")
		if err := downloadRuntime(ctx, http.DefaultClient, bootstrapURL, payload); err != nil {
			return err
		}
	}
	absolute, err := filepath.Abs(payload)
	if err != nil {
		return err
	}
	// Verify trust and publisher before executing either a download or bundled file.
	// The path travels through the environment, never PowerShell source text.
	script := `$ErrorActionPreference='Stop'; $s=Get-AuthenticodeSignature -LiteralPath $env:TILEPRINT_RUNTIME_INSTALLER; if ($s.Status -ne 'Valid' -or $s.SignerCertificate.Subject -notmatch '(^|,\s*)CN=Microsoft Corporation(,|$)') { throw 'Invalid Microsoft signature' }`
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	verify := exec.CommandContext(ctx, powershell, "-NoProfile", "-NonInteractive", "-Command", script)
	verify.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	verify.Env = append(os.Environ(), "TILEPRINT_RUNTIME_INSTALLER="+absolute)
	if output, err := verify.CombinedOutput(); err != nil {
		return fmt.Errorf("微软安装程序签名验证失败：%w；%s", err, output)
	}
	args := []string{"/install"}
	if silent {
		args = append([]string{"/silent"}, args...)
	}
	cmd := exec.CommandContext(ctx, absolute, args...)
	if silent {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	log.Print("Installing WebView2 Runtime")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("WebView2 installer: %v; %s", err, output)
	}
	// Some installers return a nonzero 'already installed' result. Availability
	// after installation is the condition that matters to TilePrint.
	if mygo.WebViewAvailable() {
		log.Print("WebView2 Runtime available")
		return nil
	}
	if ctx.Err() != nil {
		return fmt.Errorf("安装超时，请使用离线安装包：%w", ctx.Err())
	}
	return fmt.Errorf("WebView2 安装未完成（%v）。请检查网络、系统策略，或使用离线安装包", err)
}
func desktopReady() bool {
	if os.Getenv("MYGO_GENERATE") != "" {
		return true
	}
	initDesktopLog()
	if mygo.WebViewAvailable() {
		return true
	}
	for {
		choice := nativeMessage("TilePrint 需要 Microsoft WebView2 运行库。\n\n选择“是”自动下载并安装（可能需要几分钟）；\n选择“否”打开微软下载页面；\n选择“取消”退出。\n\n无网络的电脑请使用 TilePrint 离线安装包。", 0x23)
		if choice == 7 { // No
			_ = exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "rundll32.exe"), "url.dll,FileProtocolHandler", runtimeWebsite).Start()
			return false
		}
		if choice != 6 {
			return false
		} // Yes
		if err := installWebView("", false); err == nil {
			return true
		} else {
			log.Print(err)
			if nativeMessage("WebView2 安装失败。\n\n"+err.Error()+"\n\n详细日志："+runtimeLog()+"\n\n请检查网络后重试，或使用离线安装包。", 0x15) != 4 {
				return false
			}
		}
	}
}
func desktopError(err error) {
	log.Print(err)
	nativeMessage("TilePrint 界面启动失败。\n\n请尝试重新打开应用；若仍然失败，请安装或修复 Microsoft WebView2 运行库。\n\n"+err.Error()+"\n\n日志："+runtimeLog(), 0x10)
}

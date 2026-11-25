package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// nativeWebView2Viewer is the standalone Windows viewer shared by installer builders.
// The top-level window belongs to the generated application; WebView2 renders the HTML.
const nativeWebView2Viewer = `package main

import (
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

const appName = __APPNAME__

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	procMsgBox = user32.NewProc("MessageBoxW")
)

func messageBox(title, text string, flags uintptr) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	procMsgBox.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags)
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		messageBox(appName, "Gagal menentukan lokasi aplikasi: "+err.Error(), 0x10)
		return
	}
	htmlPath := filepath.Join(filepath.Dir(exe), "index.html")
	if _, err := os.Stat(htmlPath); err != nil {
		messageBox(appName, "index.html tidak ditemukan di:\n"+filepath.Dir(exe), 0x10)
		return
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title: appName,
			Width: 1200,
			Height: 800,
			Center: true,
		},
	})
	if w == nil {
		messageBox(appName, "Gagal membuat WebView2. Pastikan Microsoft Edge WebView2 Runtime terpasang.", 0x10)
		return
	}
	defer w.Destroy()
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(htmlPath)}
	w.Navigate(u.String())
	w.Run()
}
`

func renderNativeWebView2Viewer(appName string) string {
	return strings.ReplaceAll(nativeWebView2Viewer, "__APPNAME__", strconv.Quote(appName))
}

// prepareWebView2Module resolves the Windows-only WebView2 binding in generated
// build workspaces before cross-compilation.
func prepareWebView2Module(dir string) error {
	cmd := exec.Command("go", "get", "github.com/jchv/go-webview2@latest")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resolve WebView2 dependency: %w\n%s", err, out)
	}
	return nil
}

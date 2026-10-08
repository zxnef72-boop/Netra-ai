// clipboard.go — deteksi tool clipboard per-OS + copy ke clipboard
package main

import (
	"os/exec"
	"runtime"
	"strings"
)

// detectClipboardTool — pilih command yang ada di sistem.
// Return (cmd, args). cmd="" kalo gak ada yang ketemu.
func detectClipboardTool() (string, []string) {
	// Android Termux
	if _, err := exec.LookPath("termux-clipboard-set"); err == nil {
		return "termux-clipboard-set", nil
	}
	// macOS
	if _, err := exec.LookPath("pbcopy"); err == nil {
		return "pbcopy", nil
	}
	// Windows
	if runtime.GOOS == "windows" {
		return "clip", nil
	}
	// Linux Wayland
	if _, err := exec.LookPath("wl-copy"); err == nil {
		return "wl-copy", nil
	}
	// Linux X11 (xclip)
	if _, err := exec.LookPath("xclip"); err == nil {
		return "xclip", []string{"-selection", "clipboard"}
	}
	// Linux X11 (xsel)
	if _, err := exec.LookPath("xsel"); err == nil {
		return "xsel", []string{"--clipboard", "--input"}
	}
	return "", nil
}

// copyToClipboard — kirim string ke clipboard. Return error kalo gagal.
func copyToClipboard(text string) error {
	cmd, args := detectClipboardTool()
	if cmd == "" {
		return &clipboardError{}
	}
	c := exec.Command(cmd, args...)
	c.Stdin = strings.NewReader(text)
	return c.Run()
}

type clipboardError struct{}

func (e *clipboardError) Error() string {
	return "clipboard tool gak ketemu (install: termux-clipboard-set / xclip / wl-copy / pbcopy)"
}

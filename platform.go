package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// platIsTermux — true kalau jalan di Termux (Android).
func platIsTermux() bool {
	return strings.Contains(os.Getenv("PREFIX"), "com.termux")
}

// platIsWSL — true kalau jalan di Windows Subsystem for Linux.
func platIsWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	b, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// platTermuxPrefix — prefix Termux, kosong kalau bukan Termux.
func platTermuxPrefix() string {
	if p := os.Getenv("PREFIX"); strings.Contains(p, "com.termux") {
		return p
	}
	return ""
}

// defaultApkOutDir — folder output APK sesuai platform.
// Override dengan env NETRA_OUT.
func defaultApkOutDir() string {
	if p := os.Getenv("NETRA_OUT"); p != "" {
		return p
	}
	if platIsTermux() {
		if st, err := os.Stat("/sdcard"); err == nil && st.IsDir() {
			return "/sdcard/NetRatest1/apk-output"
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "NetRatest1", "apk-output")
}

// defaultAgentWorkspace — folder kerja agent sesuai platform.
// Override dengan env NETRA_WORKSPACE.
func defaultAgentWorkspace() string {
	if p := os.Getenv("NETRA_WORKSPACE"); p != "" {
		return p
	}
	if platIsTermux() {
		return "/sdcard/AiNef"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "netra-projects")
}

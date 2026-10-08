// build.go — convert URL jadi APK/EXE
package main

import (
	"fmt"
	"os"
	"strings"
)

// cmdBuildAPK — POST ke pwa2apk.com, simpan APK ke /sdcard/NetRatest1/apk-output/
func cmdBuildAPK(targetURL, appName string) (string, error) {
	// Kalau targetURL diawali "@", baca dari file
	if len(targetURL) > 1 && targetURL[0] == '@' {
		data, err := os.ReadFile(targetURL[1:])
		if err != nil {
			return "", fmt.Errorf("gagal baca file %s: %w", targetURL[1:], err)
		}
		targetURL = strings.TrimSpace(string(data))
	}
	return cmdBuildAPKFromURL(targetURL, appName)
}

// cmdBuildEXE — placeholder, pake Pake CLI nanti
func cmdBuildEXE(targetURL, appName string) error {
	outDir := "/sdcard/NetRatest1/exe-output"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	fmt.Println("⚠️ Build EXE belum diimplementasi. Pakai Pake CLI nanti.")
	return nil
}

func sanitizeName(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, s)
}

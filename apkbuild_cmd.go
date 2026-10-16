// apkbuild_cmd.go -- command APK build jalur cepat (template).
// Gak butuh Java/d8/aapt. Jalan di NeferOS/Android.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cmdBuildAPKFast -- build APK pakai template embedded.
func cmdBuildAPKFast(targetURL, appName string) (string, error) {
	var log strings.Builder

	if targetURL == "" {
		return "", fmt.Errorf("URL kosong")
	}
	if appName == "" {
		appName = "NetraApp"
	}

	home, _ := os.UserHomeDir()
	iconPath := filepath.Join(home, ".netra-ai", "icon.png")
	if _, err := os.Stat(iconPath); err != nil {
		iconPath = filepath.Join(home, ".netra-ai", "icon.jpg")
		if _, err := os.Stat(iconPath); err != nil {
			iconPath = ""
		}
	}

	outputDir := "/sdcard/NetRatest1"
	os.MkdirAll(outputDir, 0755)
	outPath := filepath.Join(outputDir, sanitizeName(appName)+".apk")

	iconStatus := "default"
	if iconPath != "" {
		iconStatus = "custom"
	}

	log.WriteString(fmt.Sprintf("Building APK (template): %s\n\n", appName))
	log.WriteString(fmt.Sprintf("URL : %s\n", targetURL))
	log.WriteString(fmt.Sprintf("Icon: %s\n\n", iconStatus))

	start := time.Now()

	log.WriteString("  -> build    ")
	apkBytes, err := buildAPKFromTemplate(targetURL, appName, iconPath)
	if err != nil {
		log.WriteString("FAIL\n\n")
		log.WriteString(fmt.Sprintf("Error:\n```\n%v\n```\n", err))
		return log.String(), err
	}
	log.WriteString("OK\n")

	log.WriteString("  -> write    ")
	if err := os.WriteFile(outPath, apkBytes, 0644); err != nil {
		log.WriteString("FAIL\n\n")
		log.WriteString(fmt.Sprintf("Error write:\n```\n%v\n```\n", err))
		return log.String(), err
	}
	log.WriteString("OK\n")

	dur := time.Since(start)
	log.WriteString(fmt.Sprintf("\nAPK siap! (%.2fs)\n", dur.Seconds()))
	log.WriteString(fmt.Sprintf("%s (%d KB)\n", outPath, len(apkBytes)/1024))
	return log.String(), nil
}

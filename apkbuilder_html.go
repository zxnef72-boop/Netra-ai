// apkbuilder_html.go — build APK dari folder HTML lokal.
// APK standalone: HTML di-embed ke assets/, WebView load file://.
// Cocok buat: game HTML, tool offline, portofolio, docs.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// copyDir — copy folder recursive.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		return err
	})
}

// cmdBuildAPKFromHTML — entry point: folder HTML + nama app → APK offline.
func cmdBuildAPKFromHTML(htmlFolder, appName string, fullscreen bool) (string, error) {
	var log strings.Builder

	// Expand ~ jadi home dir
	if strings.HasPrefix(htmlFolder, "~/") {
		home, _ := os.UserHomeDir()
		htmlFolder = filepath.Join(home, htmlFolder[2:])
	}

	// AUTO-DETECT: kalau path = file .html, bikin folder temp + copy as index.html
	if stat, statErr := os.Stat(htmlFolder); statErr == nil && !stat.IsDir() {
		low := strings.ToLower(htmlFolder)
		if strings.HasSuffix(low, ".html") || strings.HasSuffix(low, ".htm") {
			tmpFolder := filepath.Join(os.TempDir(), fmt.Sprintf("netra-html-%d", time.Now().Unix()))
			if mkErr := os.MkdirAll(tmpFolder, 0755); mkErr == nil {
				data, readErr := os.ReadFile(htmlFolder)
				if readErr == nil {
					os.WriteFile(filepath.Join(tmpFolder, "index.html"), data, 0644)
					htmlFolder = tmpFolder
				}
			}
		}
	}

	// 1. Cek folder
	info, err := os.Stat(htmlFolder)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("folder gak ketemu: %s", htmlFolder)
	}

	// 2. Cek index.html
	indexPath := filepath.Join(htmlFolder, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		// Coba .htm
		indexPath = filepath.Join(htmlFolder, "index.htm")
		if _, err := os.Stat(indexPath); err != nil {
			return "", fmt.Errorf("gak ada index.html / index.htm di folder %s", htmlFolder)
		}
	}

	home, _ := os.UserHomeDir()
	pkg := "com.netra." + sanitizeName(strings.ToLower(appName))
	workDir := filepath.Join(home, "apk-build", sanitizeName(appName)+"_html_"+fmt.Sprintf("%d", time.Now().Unix()))
	outDir := "/sdcard/NetRatest1/apk-output"
	os.MkdirAll(outDir, 0755)

	// 3. Cari icon custom
	iconPath := filepath.Join(home, ".netra-ai", "icon.png")
	if _, err := os.Stat(iconPath); err != nil {
		iconPath = filepath.Join(home, ".netra-ai", "icon.jpg")
		if _, err := os.Stat(iconPath); err != nil {
			iconPath = ""
		}
	}

	cfg := apkConfig{
		TargetURL:   "file:///android_asset/index.html",
		AppName:     appName,
		Package:     pkg,
		VersionName: "1.0",
		VersionCode: 1,
		MinSDK:      21,
		TargetSDK:   30,
		WorkDir:     workDir,
		OutputDir:   outDir,
		IconPath:    iconPath,
	}

	log.WriteString(fmt.Sprintf("📦 **Building APK (HTML mode): %s**\n\n", appName))
	log.WriteString(fmt.Sprintf("Folder : `%s`\n", htmlFolder))
	log.WriteString(fmt.Sprintf("Package: `%s`\n\n", pkg))

	// 4. Steps
	steps := []struct {
		name string
		fn   func(apkConfig) error
	}{
		{"prepare", prepareWorkdir},
		{"assets", func(c apkConfig) error {
			assetsDir := filepath.Join(c.WorkDir, "assets")
			if err := os.MkdirAll(assetsDir, 0755); err != nil {
				return err
			}
			return copyDir(htmlFolder, assetsDir)
		}},
		{"icon", generateIcons},
		{"java", func(c apkConfig) error {
			if fullscreen {
				return writeJavaSourcesFullscreen(c)
			}
			return writeJavaSources(c)
		}},
		{"manifest", writeManifest},
		{"styles", writeStyles},
		{"compile", compileJava},
		{"dex", convertToDex},
		{"package", packageAPKWithAssets},
		{"sign", signAPK},
	}

	for _, s := range steps {
		log.WriteString(fmt.Sprintf("  `→` %-10s ", s.name))
		if err := s.fn(cfg); err != nil {
			log.WriteString("❌\n\n")
			log.WriteString(fmt.Sprintf("**Error di step `%s`:**\n```\n%v\n```\n", s.name, err))
			return log.String(), err
		}
		log.WriteString("✓\n")
	}

	// 5. Copy ke output
	finalPath := filepath.Join(cfg.OutputDir, sanitizeName(appName)+".apk")
	if err := copyFile(filepath.Join(cfg.WorkDir, "app-signed.apk"), finalPath); err != nil {
		log.WriteString(fmt.Sprintf("\n❌ Gagal copy: %v\n", err))
		return log.String(), err
	}

	fi, _ := os.Stat(finalPath)
	log.WriteString(fmt.Sprintf("\n✅ **APK siap! (offline)**\n"))
	log.WriteString(fmt.Sprintf("📁 `%s` (%d KB)\n", finalPath, fi.Size()/1024))
	log.WriteString("📲 Install → jalan tanpa internet\n")
	return log.String(), nil
}

// packageAPKWithAssets — package APK dengan folder assets (-A flag).
func packageAPKWithAssets(cfg apkConfig) error {
	home, _ := os.UserHomeDir()
	androidJar := filepath.Join(home, "android-sdk", "android.jar")
	unsignedAPK := filepath.Join(cfg.WorkDir, "app-unsigned.apk")
	resDir := filepath.Join(cfg.WorkDir, "res")
	assetsDir := filepath.Join(cfg.WorkDir, "assets")

	// Step 1: aapt package dengan assets
	cmd1 := exec.Command("aapt", "package",
		"-f",
		"-M", filepath.Join(cfg.WorkDir, "AndroidManifest.xml"),
		"-S", resDir,
		"-A", assetsDir,
		"-I", androidJar,
		"-F", unsignedAPK,
	)
	out1, err := cmd1.CombinedOutput()
	if err != nil {
		fmt.Println(string(out1))
		return err
	}

	// Step 2: aapt add classes.dex
	cmd2 := exec.Command("aapt", "add", unsignedAPK, "classes.dex")
	cmd2.Dir = cfg.WorkDir
	out2, err := cmd2.CombinedOutput()
	if err != nil {
		fmt.Println(string(out2))
		return err
	}
	return nil
}

// writeJavaSourcesFullscreen — versi fullscreen (tanpa toolbar).
func writeJavaSourcesFullscreen(cfg apkConfig) error {
	pkgPath := strings.ReplaceAll(cfg.Package, ".", "/")
	srcDir := filepath.Join(cfg.WorkDir, "src", pkgPath)

	home, _ := os.UserHomeDir()
	templatePath := filepath.Join(home, "mainactivity_fullscreen.txt")
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("template fullscreen gak ketemu: %w", err)
	}
	template := string(templateBytes)
	template = strings.ReplaceAll(template, "__TARGETURL__", cfg.TargetURL)
	template = strings.ReplaceAll(template, "__APPNAME__", cfg.AppName)

	// Splash dulu (sama kayak mode normal)
	if err := writeJavaSources(cfg); err != nil {
		return err
	}

	// Ganti MainActivity dengan versi fullscreen
	mainSrc := fmt.Sprintf(template, cfg.Package)
	return os.WriteFile(filepath.Join(srcDir, "MainActivity.java"), []byte(mainSrc), 0644)
}

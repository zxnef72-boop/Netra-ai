// apkbuild_htmlfast.go — build APK offline dari folder HTML tanpa Java/d8/aapt.
// Pakai template base.apk: HTML dimasukin ke assets/www/, config.json diarahin
// ke file:///android_asset/www/index.html, lalu tanda tangan ulang (pure Go).
package main

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func htmlFastBuild(htmlFolder, indexPath, appName string, fullscreen bool, log *strings.Builder) (string, error) {
	home, _ := os.UserHomeDir()
	outDir := "/sdcard/NetRatest1/apk-output"
	os.MkdirAll(outDir, 0755)

	iconPath := filepath.Join(home, ".netra-ai", "icon.png")
	if _, err := os.Stat(iconPath); err != nil {
		iconPath = filepath.Join(home, ".netra-ai", "icon.jpg")
		if _, err := os.Stat(iconPath); err != nil {
			iconPath = ""
		}
	}

	log.WriteString(fmt.Sprintf("Building APK (HTML, tanpa Java): %s\n\n", appName))
	log.WriteString(fmt.Sprintf("Folder: %s\n", htmlFolder))
	if fullscreen {
		log.WriteString("Mode  : fullscreen\n")
	}
	log.WriteString("\n")

	start := time.Now()
	fail := func(step string, err error) (string, error) {
		log.WriteString("FAIL\n\n")
		log.WriteString(fmt.Sprintf("Error di step %s:\n```\n%v\n```\n", step, err))
		return log.String(), err
	}

	log.WriteString("  -> baca file   ")
	var assets []APKEntry
	total := 0
	werr := filepath.Walk(htmlFolder, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(htmlFolder, p)
		assets = append(assets, APKEntry{Name: "assets/www/" + filepath.ToSlash(rel), Data: data, Method: zip.Deflate})
		total += len(data)
		return nil
	})
	if werr != nil {
		return fail("baca file", werr)
	}
	if total > 50*1024*1024 {
		return fail("baca file", fmt.Errorf("folder terlalu besar (%d MB, maks 50 MB)", total/1024/1024))
	}
	log.WriteString(fmt.Sprintf("%d file\n", len(assets)))

	entryName := strings.ToLower(filepath.Base(indexPath))
	log.WriteString("  -> build       ")
	apkBytes, err := buildAPKFromTemplateAssets("file:///android_asset/www/"+entryName, appName, iconPath, assets, fullscreen)
	if err != nil {
		return fail("build", err)
	}
	log.WriteString("OK\n")

	finalPath := filepath.Join(outDir, sanitizeName(appName)+".apk")
	log.WriteString("  -> tulis       ")
	if err := os.WriteFile(finalPath, apkBytes, 0644); err != nil {
		return fail("tulis", err)
	}
	log.WriteString("OK\n")

	log.WriteString(fmt.Sprintf("\nAPK siap! (%.2fs)\n%s (%d KB)\n", time.Since(start).Seconds(), finalPath, len(apkBytes)/1024))
	return log.String(), nil
}

// buildAPKFromTemplateAssets — sama seperti buildAPKFromTemplate, plus file assets.
func buildAPKFromTemplateAssets(targetURL, appName, iconPath string, assets []APKEntry, fullscreen bool) ([]byte, error) {
	key, err := loadPrivateKey(debugKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}
	cert, err := loadCert(debugCertPEM)
	if err != nil {
		return nil, fmt.Errorf("load cert: %w", err)
	}
	entries, err := extractAPK(baseAPKBytes)
	if err != nil {
		return nil, fmt.Errorf("extract template: %w", err)
	}

	patchConfigJSON(entries, targetURL)
	icons, err := generateIconsBytes(appName, iconPath)
	if err != nil {
		return nil, fmt.Errorf("gen icons: %w", err)
	}
	patchIcons(entries, icons)
	entries = append(entries, assets...)
	if fullscreen {
		if err := patchManifestTheme(entries, 0x01030007); err != nil { // android:Theme.NoTitleBar.Fullscreen
			return nil, fmt.Errorf("fullscreen: %w", err)
		}
	}

	unsigned, err := packAPK(entries)
	if err != nil {
		return nil, fmt.Errorf("pack: %w", err)
	}
	signed, err := signAPKv1(unsigned, key, cert)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	return signed, nil
}

// patchManifestTheme — ganti nilai android:theme (attr 0x01010000) di AndroidManifest.xml
// biner jadi resource id framework. Tanpa aapt: edit langsung 4 byte di AXML.
func patchManifestTheme(entries []APKEntry, themeID uint32) error {
	for i := range entries {
		if entries[i].Name != "AndroidManifest.xml" {
			continue
		}
		d := entries[i].Data
		le := binary.LittleEndian
		if len(d) < 8 {
			return fmt.Errorf("manifest kependekan")
		}
		var resMap []uint32
		patched := 0
		for off := 8; off+8 <= len(d); {
			t := le.Uint16(d[off:])
			sz := int(le.Uint32(d[off+4:]))
			if sz < 8 || off+sz > len(d) {
				break
			}
			switch t {
			case 0x0180:
				for p := off + 8; p+4 <= off+sz; p += 4 {
					resMap = append(resMap, le.Uint32(d[p:]))
				}
			case 0x0102:
				cnt := int(le.Uint16(d[off+28:]))
				for k := 0; k < cnt; k++ {
					a := off + 36 + k*20
					if a+20 > off+sz {
						break
					}
					name := int(le.Uint32(d[a+4:]))
					dataType := d[a+15]
					if name < len(resMap) && resMap[name] == 0x01010000 && dataType == 0x01 {
						le.PutUint32(d[a+16:], themeID)
						patched++
					}
				}
			}
			off += sz
		}
		if patched == 0 {
			return fmt.Errorf("atribut theme gak ketemu di manifest template")
		}
		return nil
	}
	return fmt.Errorf("AndroidManifest.xml gak ada di template")
}

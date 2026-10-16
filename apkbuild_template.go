// apkbuild_template.go — orchestrator build APK pakai template embedded.
// Ganti full pipeline lama (java+dex+aapt+apksigner) jadi pure Go.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
)

// buildAPKFromTemplate — full pipeline:
//
//	extract base.apk → patch config.json + icon → pack → sign v1
//
// Return: APK bytes siap install.
func buildAPKFromTemplate(targetURL, appName, iconPath string) ([]byte, error) {
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

// generateIconsBytes — generate 4 icon mipmap sebagai map[string][]byte.
// Key: "hdpi", "xhdpi", "xxhdpi", "xxxhdpi" (sesuai template base.apk).
func generateIconsBytes(appName, iconPath string) (map[string][]byte, error) {
	sizes := map[string]int{
		"hdpi":    72,
		"xhdpi":   96,
		"xxhdpi":  144,
		"xxxhdpi": 192,
	}

	var src image.Image
	var err error
	if iconPath != "" {
		src, err = loadIconImage(iconPath)
		if err != nil {
			return nil, err
		}
	} else {
		src = makeDefaultIcon(appName, 512)
	}

	out := make(map[string][]byte, len(sizes))
	for name, size := range sizes {
		resized := resizeIcon(src, size, size)
		var buf bytes.Buffer
		if err := png.Encode(&buf, resized); err != nil {
			return nil, fmt.Errorf("encode %s: %w", name, err)
		}
		out[name] = buf.Bytes()
	}
	return out, nil
}

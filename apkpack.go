// apkpack.go — extract + patch + repack APK dari template embedded.
// Gak butuh Java compiler / d8 / aapt. Semua pure Go stdlib.
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// APKEntry — 1 file di dalam APK.
type APKEntry struct {
	Name   string
	Data   []byte
	Method uint16 // zip.Store atau zip.Deflate
}

// extractAPK — baca semua entry dari APK bytes.
func extractAPK(apkBytes []byte) ([]APKEntry, error) {
	reader, err := zip.NewReader(bytes.NewReader(apkBytes), int64(len(apkBytes)))
	if err != nil {
		return nil, fmt.Errorf("baca zip: %w", err)
	}
	var entries []APKEntry
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("buka %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("baca %s: %w", f.Name, err)
		}
		entries = append(entries, APKEntry{
			Name:   f.Name,
			Data:   data,
			Method: f.Method,
		})
	}
	return entries, nil
}

// packAPK — bikin APK bytes dari entries.
// resources.arsc + AndroidManifest.xml wajib STORED (per Android spec).
func packAPK(entries []APKEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		hdr := &zip.FileHeader{
			Name:   e.Name,
			Method: e.Method,
		}
		if e.Name == "resources.arsc" || e.Name == "AndroidManifest.xml" {
			hdr.Method = zip.Store
		}
		hdr.SetMode(0644)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(e.Data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// patchConfigJSON — ganti URL di assets/config.json.
func patchConfigJSON(entries []APKEntry, url string) {
	safeURL := strings.ReplaceAll(url, `\`, `\\`)
	safeURL = strings.ReplaceAll(safeURL, `"`, `\"`)
	newContent := []byte(fmt.Sprintf(`{"url":"%s"}`, safeURL))
	for i := range entries {
		if entries[i].Name == "assets/config.json" {
			entries[i].Data = newContent
			return
		}
	}
	entries = append(entries, APKEntry{
		Name:   "assets/config.json",
		Data:   newContent,
		Method: zip.Deflate,
	})
}

// patchIcons — replace 4 icon mipmap. Key: "hdpi", "xhdpi", "xxhdpi", "xxxhdpi".
func patchIcons(entries []APKEntry, icons map[string][]byte) {
	for i := range entries {
		for size, data := range icons {
			if entries[i].Name == "res/mipmap-"+size+"-v4/ic_launcher.png" {
				entries[i].Data = data
				entries[i].Method = zip.Deflate
			}
		}
	}
}

// removeSignatureFiles — hapus META-INF/*.RSA, *.SF, MANIFEST.MF lama.
// Wajib dihapus sebelum re-sign.
func removeSignatureFiles(entries []APKEntry) []APKEntry {
	out := entries[:0]
	for _, e := range entries {
		name := strings.ToUpper(e.Name)
		if strings.HasPrefix(name, "META-INF/") {
			if strings.HasSuffix(name, ".RSA") ||
				strings.HasSuffix(name, ".SF") ||
				strings.HasSuffix(name, ".DSA") ||
				strings.HasSuffix(name, ".EC") ||
				name == "META-INF/MANIFEST.MF" {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

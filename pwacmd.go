// pwacmd.go — command /pwa di TUI Netra AI.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cmdPWA — handle "/pwa <template> [nama]"
// Contoh: /pwa toko  → bikin ~/pwa-output/toko/ dari templates/toko.html
func cmdPWA(args []string) string {
	if len(args) < 1 {
		return pwaHelp()
	}

	// Parse: /pwa <template> [nama app]
	templateName := args[0]
	appName := templateName
	if len(args) >= 2 {
		appName = strings.Join(args[1:], " ")
	}

	// Cari file template
	templateFile := findTemplateFile(templateName)
	if templateFile == "" {
		return fmt.Sprintf("❌ Template '%s' gak ketemu di folder templates/.\n\n%s",
			templateName, pwaListTemplates())
	}

	// Output dir — coba /sdcard dulu (biar keliatan di file manager)
	// fallback ke ~/pwa-output kalau gak bisa nulis
	outDir := ""
	var mkErrors []string
	candidates := []string{
		"/sdcard/NetRatest1/pwa-output",
		"/sdcard/NetRa-PWA",
	}
	for _, base := range candidates {
		testDir := filepath.Join(base, sanitizePWA(templateName))
		if err := os.MkdirAll(testDir, 0755); err == nil {
			// Test tulis — mastiin beneran bisa nulis
			testFile := filepath.Join(testDir, ".write_test")
			if werr := os.WriteFile(testFile, []byte("ok"), 0644); werr == nil {
				os.Remove(testFile)
				outDir = testDir
				break
			} else {
				mkErrors = append(mkErrors, fmt.Sprintf("%s: write fail: %v", testDir, werr))
			}
		} else {
			mkErrors = append(mkErrors, fmt.Sprintf("%s: mkdir fail: %v", testDir, err))
		}
	}
	if outDir == "" {
		home, _ := os.UserHomeDir()
		outDir = filepath.Join(home, "pwa-output", sanitizePWA(templateName))
		os.MkdirAll(outDir, 0755)
	}

	if err := buildPWA(templateFile, outDir, appName); err != nil {
		return fmt.Sprintf("❌ Gagal bikin PWA: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ **PWA berhasil dibuat!**\n\n"))
	sb.WriteString(fmt.Sprintf("📁 Folder: `%s`\n\n", outDir))
	if len(mkErrors) > 0 {
		sb.WriteString("⚠️ **Debug (folder /sdcard gagal):**\n")
		for _, e := range mkErrors {
			sb.WriteString(fmt.Sprintf("  • `%s`\n", e))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Isi folder:\n")
	sb.WriteString("  ├─ `index.html`    (template + PWA tags)\n")
	sb.WriteString("  ├─ `manifest.json` (nama, warna, ikon)\n")
	sb.WriteString("  ├─ `sw.js`         (service worker, offline-ready)\n")
	sb.WriteString("  └─ `icon.svg`      (ikon otomatis dari inisial)\n\n")
	sb.WriteString("**Cara install ke HP:**\n")
	sb.WriteString(fmt.Sprintf("```\ncd %s\npython3 -m http.server 8080\n```\n", outDir))
	sb.WriteString("1. Buka Chrome HP → `http://127.0.0.1:8080`\n")
	sb.WriteString("2. Menu Chrome (⋮) → **Add to Home screen**\n")
	sb.WriteString("3. Buka dari home screen → jalan kayak app\n\n")
	sb.WriteString("💡 Hosting lokal cuma buat tes. Kalau mau bisa diakses dari HP lain:\n")
	sb.WriteString(fmt.Sprintf("   `python3 -m http.server 8080 --bind 0.0.0.0` → akses `http://<IP-HP>:8080`"))

	return sb.String()
}

func pwaHelp() string {
	var sb strings.Builder
	sb.WriteString("**/pwa** — bikin PWA (Progressive Web App) dari template.\n\n")
	sb.WriteString("**Pakai:** `/pwa <template> [nama-app]`\n\n")
	sb.WriteString("**Contoh:**\n")
	sb.WriteString("  `/pwa toko`\n")
	sb.WriteString("  `/pwa toko-pro Toko Saya`\n")
	sb.WriteString("  `/pwa portfolio-pro Portfolio Anzar`\n\n")
	sb.WriteString(pwaListTemplates())
	return sb.String()
}

// pwaListTemplates — list template HTML yang ada.
func pwaListTemplates() string {
	dirs := []string{
		filepath.Join(elizaTemplateBaseDir(), "templates"),
		"templates",
	}
	var files []string
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".htm") {
				files = append(files, strings.TrimSuffix(name, filepath.Ext(name)))
			}
		}
		if len(files) > 0 {
			break
		}
	}

	if len(files) == 0 {
		return "_(Gak ada template HTML di folder templates/)_"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%d template tersedia:**\n\n", len(files)))
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("  • `%s`\n", f))
	}
	return sb.String()
}

// findTemplateFile — cari file template berdasarkan nama (tanpa ekstensi).
func findTemplateFile(name string) string {
	// Kalo udah ada ekstensi, cek langsung
	if strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".htm") {
		candidates := []string{
			filepath.Join(elizaTemplateBaseDir(), "templates", name),
			filepath.Join("templates", name),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
		return ""
	}

	// Coba .html dan .htm
	dirs := []string{
		filepath.Join(elizaTemplateBaseDir(), "templates"),
		"templates",
	}
	for _, d := range dirs {
		for _, ext := range []string{".html", ".htm"} {
			p := filepath.Join(d, name+ext)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func sanitizePWA(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "pwa"
	}
	return b.String()
}

// pwa.go — convert template HTML jadi PWA (Progressive Web App).
// Hasilnya folder berisi: index.html, manifest.json, sw.js, icon.svg
// User tinggal serve + "Add to Home Screen" di Chrome.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PWAManifest struct {
	Name            string            `json:"name"`
	ShortName       string            `json:"short_name"`
	StartURL        string            `json:"start_url"`
	Display         string            `json:"display"`
	BackgroundColor string            `json:"background_color"`
	ThemeColor      string            `json:"theme_color"`
	Description     string            `json:"description,omitempty"`
	Icons           []PWAManifestIcon `json:"icons"`
}

type PWAManifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose,omitempty"`
}

// buildPWA — bikin folder PWA dari template HTML.
func buildPWA(templatePath, outputDir, appName string) error {
	// Baca template
	htmlData, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("gak bisa baca template: %w", err)
	}

	// Bikin folder output
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// 1. Tulis index.html — inject manifest + SW registration
	htmlStr := string(htmlData)
	htmlStr = injectPWATags(htmlStr, appName)
	if err := os.WriteFile(filepath.Join(outputDir, "index.html"), []byte(htmlStr), 0644); err != nil {
		return err
	}

	// 2. manifest.json
	manifest := PWAManifest{
		Name:            appName,
		ShortName:       truncate(appName, 12),
		StartURL:        "./index.html",
		Display:         "standalone",
		BackgroundColor: "#120d1f",
		ThemeColor:      "#8b5cf6",
		Description:     "PWA dari NetRa — dibuat otomatis",
		Icons: []PWAManifestIcon{
			{Src: "icon.svg", Sizes: "any", Type: "image/svg+xml", Purpose: "any maskable"},
		},
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(outputDir, "manifest.json"), manifestBytes, 0644); err != nil {
		return err
	}

	// 3. sw.js — service worker minimal (cache-first)
	if err := os.WriteFile(filepath.Join(outputDir, "sw.js"), []byte(serviceWorkerJS), 0644); err != nil {
		return err
	}

	// 4. icon.svg — SVG dengan inisial app
	icon := buildIconSVG(appName)
	if err := os.WriteFile(filepath.Join(outputDir, "icon.svg"), []byte(icon), 0644); err != nil {
		return err
	}

	return nil
}

// injectPWATags — tambah <link manifest> + register SW ke HTML.
// Kalo HTML gak punya <head>, fallback: prepend aja.
func injectPWATags(htmlStr, appName string) string {
	// Update <title> kalau ada
	if strings.Contains(htmlStr, "<title>") {
		// gak wajib diubah, biarin apa adanya
	}

	pwaTags := `<link rel="manifest" href="manifest.json">
<meta name="theme-color" content="#8b5cf6">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
<script>
  if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
      navigator.serviceWorker.register('sw.js').catch(() => {});
    });
  }
</script>
`

	// Inject sebelum </head> kalau ada
	if idx := strings.Index(strings.ToLower(htmlStr), "</head>"); idx != -1 {
		return htmlStr[:idx] + pwaTags + htmlStr[idx:]
	}
	// Fallback: prepend
	return pwaTags + htmlStr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func buildIconSVG(name string) string {
	initial := "N"
	for _, r := range name {
		if r != ' ' {
			initial = strings.ToUpper(string(r))
			break
		}
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 192 192">
  <defs>
    <linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%%" stop-color="#8b5cf6"/>
      <stop offset="100%%" stop-color="#ec4899"/>
    </linearGradient>
  </defs>
  <rect width="192" height="192" rx="36" fill="url(#g)"/>
  <text x="96" y="130" font-family="system-ui,sans-serif" font-size="110" font-weight="bold"
        fill="white" text-anchor="middle">%s</text>
</svg>`, initial)
}

const serviceWorkerJS = `// sw.js — service worker minimal (cache-first, offline-friendly)
const CACHE = 'netra-pwa-v1';
const ASSETS = ['./', './index.html', './manifest.json', './icon.svg'];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(ASSETS)).catch(() => {}));
  self.skipWaiting();
});

self.addEventListener('activate', (e) => {
  e.waitUntil(self.clients.claim());
});

self.addEventListener('fetch', (e) => {
  if (e.request.method !== 'GET') return;
  e.respondWith(
    caches.match(e.request).then(cached => {
      if (cached) return cached;
      return fetch(e.request).then(res => {
        if (res.ok && new URL(e.request.url).origin === location.origin) {
          const clone = res.clone();
          caches.open(CACHE).then(c => c.put(e.request, clone));
        }
        return res;
      }).catch(() => caches.match('./index.html'));
    })
  );
});
`

// htmloffline.go — snapshot HTML jadi 1 file offline (SingleFile-style).
// Inline CSS, JS, dan gambar (base64) ke dalam HTML.
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	htmlOffMaxHTML  = 3 * 1024 * 1024 // 3MB
	htmlOffMaxCSS   = 1 * 1024 * 1024 // 1MB per CSS
	htmlOffMaxJS    = 2 * 1024 * 1024 // 2MB per JS
	htmlOffMaxAsset = 2 * 1024 * 1024 // 2MB per image/font
)

var (
	reLinkCSS   = regexp.MustCompile(`(?is)<link[^>]+rel\s*=\s*["']stylesheet["'][^>]*>`)
	reLinkHref  = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)
	reScrSrc    = regexp.MustCompile(`(?is)<script[^>]+src\s*=\s*["']([^"']+)["'][^>]*>\s*</script>`)
	reImgTag    = regexp.MustCompile(`(?is)<img[^>]+src\s*=\s*["']([^"']+)["'][^>]*>`)
	reCSSUrlRef = regexp.MustCompile(`url\s*\(\s*["']?([^"')]+)["']?\s*\)`)
	reSrcset    = regexp.MustCompile(`(?is)\s+srcset\s*=\s*["'][^"']*["']`)
	reDataSrc   = regexp.MustCompile(`(?is)\s+data-src\s*=\s*["'][^"']*["']`)
)

func tryHtmlOffline(input string) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(input))

	triggers := []string{
		"html offline ", "html 1 file ", "html one file ",
		"html single ", "html simpan ", "html local ", "html snapshot ",
	}
	matched := ""
	for _, t := range triggers {
		if strings.Contains(low, t) {
			matched = t
			break
		}
	}
	if matched == "" {
		return "", false
	}

	idx := strings.Index(low, matched)
	rest := strings.TrimSpace(input[idx+len(matched):])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "Pakai: `html offline <url>` -- contoh: `html offline example.com`", true
	}
	target := fields[0]

	var urlStr string
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		urlStr = target
	} else {
		urlStr = "https://" + target
	}

	baseParsed, err := url.Parse(urlStr)
	if err != nil || baseParsed.Host == "" {
		return "URL gak valid: " + target, true
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// 1. Fetch HTML
	html, err := fetchAssetSimple(client, urlStr, htmlOffMaxHTML)
	if err != nil {
		return fmt.Sprintf("Fetch HTML gagal: %v", err), true
	}
	origSize := len(html)

	var log strings.Builder
	log.WriteString(fmt.Sprintf("**Offline Snapshot `%s`**\n\n", urlStr))
	log.WriteString(fmt.Sprintf("_HTML asli: %d byte_\n\n", origSize))

	// 2. Inline CSS
	cssCount, cssSize := 0, 0
	html = reLinkCSS.ReplaceAllStringFunc(html, func(match string) string {
		hrefMatch := reLinkHref.FindStringSubmatch(match)
		if len(hrefMatch) < 2 {
			return match
		}
		cssURL := resolveURL(baseParsed, hrefMatch[1])
		if cssURL == "" {
			return match
		}
		css, err := fetchAssetSimple(client, cssURL, htmlOffMaxCSS)
		if err != nil {
			return match
		}
		css = inlineCSSUrls(client, css, cssURL)
		cssCount++
		cssSize += len(css)
		return "<style>\n" + css + "\n</style>"
	})

	// 2b. Inline url() di <style> tag
	html = inlineStyleTags(client, html, urlStr)

	// 3. Inline Script
	jsCount, jsSize := 0, 0
	html = reScrSrc.ReplaceAllStringFunc(html, func(match string) string {
		m := reScrSrc.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		jsURL := resolveURL(baseParsed, m[1])
		if jsURL == "" {
			return match
		}
		js, err := fetchAssetSimple(client, jsURL, htmlOffMaxJS)
		if err != nil {
			return match
		}
		jsCount++
		jsSize += len(js)
		safeJS := strings.ReplaceAll(js, "</script", "<\\/script")
		return "<script>\n" + safeJS + "\n</script>"
	})

	// 4. Inline Images
	imgCount, imgSize := 0, 0
	html = reImgTag.ReplaceAllStringFunc(html, func(match string) string {
		m := reImgTag.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		src := m[1]
		if strings.HasPrefix(src, "data:") {
			return match
		}
		imgURL := resolveURL(baseParsed, src)
		if imgURL == "" {
			return match
		}
		imgData, err := fetchAssetSimple(client, imgURL, htmlOffMaxAsset)
		if err != nil {
			return match
		}
		mime := detectMimeFromURL(imgURL)
		b64 := base64.StdEncoding.EncodeToString([]byte(imgData))
		dataURI := "data:" + mime + ";base64," + b64
		imgCount++
		imgSize += len(dataURI)
		// Buang srcset + data-src biar gak override src
		match = reSrcset.ReplaceAllString(match, "")
		match = reDataSrc.ReplaceAllString(match, "")
		return strings.Replace(match, src, dataURI, 1)
	})

	// 5. Save
	var outDir string
	for _, c := range []string{"/sdcard/AiNef/offline", "/storage/emulated/0/AiNef/offline"} {
		if _, err := os.Stat(filepath.Dir(c)); err == nil {
			outDir = c
			break
		}
	}
	if outDir == "" {
		home, _ := os.UserHomeDir()
		outDir = filepath.Join(home, "netra-ai", "downloads", "offline")
	}
	os.MkdirAll(outDir, 0755)

	host := strings.ReplaceAll(baseParsed.Host, ":", "_")
	if host == "" {
		host = "snapshot"
	}
	outFile := filepath.Join(outDir, host+".html")
	os.WriteFile(outFile, []byte(html), 0644)

	finalSize := len(html)
	ratio := float64(finalSize) / float64(origSize)

	log.WriteString("**Hasil inline:**\n")
	log.WriteString(fmt.Sprintf("- CSS: %d file (%s)\n", cssCount, humanSize(int64(cssSize))))
	log.WriteString(fmt.Sprintf("- JS: %d file (%s)\n", jsCount, humanSize(int64(jsSize))))
	log.WriteString(fmt.Sprintf("- Gambar: %d (%s)\n", imgCount, humanSize(int64(imgSize))))
	log.WriteString(fmt.Sprintf("\n**Total file: %s** (%s -> %s, %.1fx)\n\n",
		humanSize(int64(finalSize)),
		humanSize(int64(origSize)),
		humanSize(int64(finalSize)),
		ratio))
	log.WriteString(fmt.Sprintf("_File: `%s`_\n\n", outFile))
	log.WriteString("_Bisa dibuka offline (WiFi off)_")

	return log.String(), true
}

func fetchAssetSimple(client *http.Client, urlStr string, maxSize int) (string, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", metaUA)
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxSize)+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxSize {
		return "", fmt.Errorf("file > %s", humanSize(int64(maxSize)))
	}
	return string(body), nil
}

func resolveURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "data:") ||
		strings.HasPrefix(ref, "javascript:") ||
		strings.HasPrefix(ref, "mailto:") ||
		strings.HasPrefix(ref, "#") {
		return ""
	}
	if strings.HasPrefix(ref, "//") {
		return base.Scheme + ":" + ref
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func inlineCSSUrls(client *http.Client, css, cssURL string) string {
	cssBase, err := url.Parse(cssURL)
	if err != nil {
		return css
	}
	return reCSSUrlRef.ReplaceAllStringFunc(css, func(match string) string {
		m := reCSSUrlRef.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		ref := strings.TrimSpace(m[1])
		if strings.HasPrefix(ref, "data:") {
			return match
		}
		assetURL := resolveURL(cssBase, ref)
		if assetURL == "" {
			return match
		}
		data, err := fetchAssetSimple(client, assetURL, htmlOffMaxAsset)
		if err != nil {
			return match
		}
		mime := detectMimeFromURL(assetURL)
		b64 := base64.StdEncoding.EncodeToString([]byte(data))
		return "url(data:" + mime + ";base64," + b64 + ")"
	})
}

func detectMimeFromURL(u string) string {
	low := strings.ToLower(u)
	if idx := strings.Index(low, "?"); idx != -1 {
		low = low[:idx]
	}
	if idx := strings.Index(low, "#"); idx != -1 {
		low = low[:idx]
	}
	switch {
	case strings.HasSuffix(low, ".png"):
		return "image/png"
	case strings.HasSuffix(low, ".jpg"), strings.HasSuffix(low, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(low, ".gif"):
		return "image/gif"
	case strings.HasSuffix(low, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(low, ".webp"):
		return "image/webp"
	case strings.HasSuffix(low, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(low, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(low, ".woff"):
		return "font/woff"
	case strings.HasSuffix(low, ".ttf"):
		return "font/ttf"
	case strings.HasSuffix(low, ".otf"):
		return "font/otf"
	case strings.HasSuffix(low, ".css"):
		return "text/css"
	}
	return "application/octet-stream"
}

// inlineStyleTags — proses <style>...</style> di HTML, inline url() relative
func inlineStyleTags(client *http.Client, html, baseURL string) string {
	var result strings.Builder
	remaining := html
	for {
		lowerRem := strings.ToLower(remaining)
		startIdx := strings.Index(lowerRem, "<style")
		if startIdx == -1 {
			result.WriteString(remaining)
			break
		}
		openEnd := strings.Index(remaining[startIdx:], ">")
		if openEnd == -1 {
			result.WriteString(remaining)
			break
		}
		openEnd += startIdx + 1

		lowerRest := strings.ToLower(remaining[openEnd:])
		closeIdx := strings.Index(lowerRest, "</style>")
		if closeIdx == -1 {
			result.WriteString(remaining)
			break
		}
		closeIdx += openEnd

		result.WriteString(remaining[:openEnd])
		innerCSS := remaining[openEnd:closeIdx]
		inlined := inlineCSSUrls(client, innerCSS, baseURL)
		result.WriteString(inlined)
		result.WriteString("</style>")
		remaining = remaining[closeIdx+len("</style>"):]
	}
	return result.String()
}

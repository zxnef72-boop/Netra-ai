// apkscan.go — scan APK buat deteksi RAT/spyware.
// Cek: package name, permissions berbahaya, pola C2 (Telegram bot,
// Firebase, Discord webhook, IP mentah), keyword spyware.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Permission berbahaya — yang sering dipake RAT buat nyolong data.
var dangerousPerms = map[string]string{
	"android.permission.READ_SMS":                   "Baca SMS (OTP bank!)",
	"android.permission.RECEIVE_SMS":                "Terima SMS (intersep OTP)",
	"android.permission.SEND_SMS":                   "Kirim SMS diam-diam",
	"android.permission.READ_CONTACTS":              "Baca kontak",
	"android.permission.WRITE_CONTACTS":             "Edit kontak",
	"android.permission.RECORD_AUDIO":               "Rekam suara (mic)",
	"android.permission.CAMERA":                     "Akses kamera",
	"android.permission.ACCESS_FINE_LOCATION":       "Lokasi presisi (GPS)",
	"android.permission.ACCESS_BACKGROUND_LOCATION": "Lokasi background",
	"android.permission.READ_CALL_LOG":              "Baca log telepon",
	"android.permission.PROCESS_OUTGOING_CALLS":     "Sadap panggilan",
	"android.permission.READ_PHONE_STATE":           "Info SIM",
	"android.permission.SYSTEM_ALERT_WINDOW":        "Overlay (phishing UI)",
	"android.permission.REQUEST_INSTALL_PACKAGES":   "Install APK lain (bahaya!)",
	"android.permission.BIND_ACCESSIBILITY_SERVICE": "AKSESIBILITAS (kendali penuh!)",
	"android.permission.BIND_DEVICE_ADMIN":          "Admin HP (gak bisa uninstall)",
	"android.permission.RECEIVE_BOOT_COMPLETED":     "Auto-start pas HP nyala",
	"android.permission.WRITE_SETTINGS":             "Ubah setting HP",
	"android.permission.PACKAGE_USAGE_STATS":        "Spy app yang dipake",
	"android.permission.READ_EXTERNAL_STORAGE":      "Baca file HP",
	"android.permission.WRITE_EXTERNAL_STORAGE":     "Tulis file HP",
}

// Pola mencurigakan di strings classes.dex.
var suspiciousPatterns = []struct {
	Name  string
	Re    *regexp.Regexp
	Score int
}{
	{"Telegram Bot C2", regexp.MustCompile(`api\.telegram\.org/bot`), 3},
	{"Firebase C2", regexp.MustCompile(`firebaseio\.com|firebase\.google\.com`), 2},
	{"Discord Webhook", regexp.MustCompile(`discord(app)?\.com/api/webhooks`), 3},
	{"C2 IP mentah", regexp.MustCompile(`\b(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`), 1},
	{"Keyword spyware", regexp.MustCompile(`(?i)(screenshot|keylog|silent_record|hidden_cam|steal|hijack|intercept|spyware)`), 2},
	{"Dynamic class load", regexp.MustCompile(`DexClassLoader|PathClassLoader`), 1},
	{"Runtime exec", regexp.MustCompile(`Runtime\.getRuntime\(\)\.exec`), 1},
	{"Root check", regexp.MustCompile(`(?i)/system/xbin/su|magisk`), 1},
	{"HTTP endpoint hardcode", regexp.MustCompile(`https?://[a-zA-Z0-9\.\-]+\.[a-z]{2,}`), 1},
}

// isFakePackage — cek nama package yang mirip app terkenal atau pake kata "mod".
func isFakePackage(pkg string) bool {
	low := strings.ToLower(pkg)
	// Kata kunci curang
	badWords := []string{"free", "mod", "premium", "hack", "crack", "unlimited"}
	for _, w := range badWords {
		if strings.Contains(low, w) {
			return true
		}
	}
	// Tiru nama app terkenal TAPI bukan yang resmi
	official := map[string]bool{
		"com.whatsapp":           true,
		"com.facebook.katana":    true,
		"com.instagram.android":  true,
		"org.telegram.messenger": true,
		"com.google.android.gms": true,
	}
	famous := []string{"com.whatsapp.", "com.facebook.", "com.instagram.", "org.telegram.", "com.google.android."}
	for _, f := range famous {
		if strings.HasPrefix(low, f) && !official[pkg] {
			return true
		}
	}
	return false
}

// extractRawEvidence — tarik semua URL + IP mentah dari strings.
// Return: URLs unik, IPs unik, C2 kuat (Telegram/Discord/Pastebin).
func extractRawEvidence(content string) (urls []string, ips []string, strongC2 []string) {
	reURL := regexp.MustCompile(`https?://[a-zA-Z0-9\.\-_:]+(?:/[^\s"'<>\\]*)?`)
	reIP := regexp.MustCompile(`\b(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`)

	seenURL := make(map[string]bool)
	seenIP := make(map[string]bool)
	seenC2 := make(map[string]bool)

	// URL
	for _, m := range reURL.FindAllString(content, 2000) {
		m = strings.TrimRight(m, "\x00\r\n\t ")
		if len(m) > 120 {
			m = m[:117] + "..."
		}
		if seenURL[m] {
			continue
		}
		seenURL[m] = true
		urls = append(urls, m)

		// Cek C2 kuat
		low := strings.ToLower(m)
		if strings.Contains(low, "api.telegram.org/bot") ||
			strings.Contains(low, "discord.com/api/webhooks/") ||
			strings.Contains(low, "discordapp.com/api/webhooks/") ||
			strings.Contains(low, "pastebin.com/raw/") {
			if !seenC2[m] {
				seenC2[m] = true
				strongC2 = append(strongC2, m)
			}
		}
	}

	// IP
	for _, m := range reIP.FindAllString(content, 2000) {
		if isBogusIP(m) || seenIP[m] {
			continue
		}
		seenIP[m] = true
		ips = append(ips, m)
	}

	// Batasi output
	if len(urls) > 40 {
		urls = urls[:40]
	}
	if len(ips) > 20 {
		ips = ips[:20]
	}
	return
}

// scanAPKFile — fungsi utama. Return hasil sebagai string (buat ditampilin di chat).
func scanAPKFile(apkPath string) string {
	var sb strings.Builder

	fi, err := os.Stat(apkPath)
	if err != nil {
		return fmt.Sprintf("❌ File gak ketemu: %s", apkPath)
	}

	sb.WriteString(fmt.Sprintf("🔍 **Scan APK: %s**\n", filepath.Base(apkPath)))
	sb.WriteString(fmt.Sprintf("📊 Size: %.1f KB\n\n", float64(fi.Size())/1024))

	// Sanity: beneran APK (ZIP)?
	typeOut, _ := exec.Command("file", apkPath).Output()
	if !strings.Contains(string(typeOut), "Zip") {
		sb.WriteString("⚠️ **Bukan file ZIP/APK valid** — scan dilanjut, hasil mungkin aneh\n\n")
	}

	riskScore := 0

	// === 1. aapt dump badging ===
	badging, aerr := exec.Command("aapt", "dump", "badging", apkPath).Output()
	if aerr != nil {
		sb.WriteString("⚠️ `aapt` gagal — APK mungkin corrupt\n")
	} else {
		lines := strings.Split(string(badging), "\n")
		var perms []string
		var pkgName, launchActivity string
		reName := regexp.MustCompile(`name='([^']+)'`)

		for _, ln := range lines {
			if strings.HasPrefix(ln, "package:") {
				if m := reName.FindStringSubmatch(ln); m != nil {
					pkgName = m[1]
				}
			}
			if strings.HasPrefix(ln, "launchable-activity:") {
				if m := reName.FindStringSubmatch(ln); m != nil {
					launchActivity = m[1]
				}
			}
			if strings.HasPrefix(ln, "uses-permission:") {
				if m := reName.FindStringSubmatch(ln); m != nil {
					perms = append(perms, m[1])
				}
			}
		}

		sb.WriteString("📦 **Info dasar**\n")
		sb.WriteString(fmt.Sprintf("  Package  : `%s`\n", pkgName))
		sb.WriteString(fmt.Sprintf("  Activity : `%s`\n", launchActivity))

		if isFakePackage(pkgName) {
			sb.WriteString("  🚨 **NAMA MENCURIGAKAN!** Mirip app terkenal / pake kata 'mod/free'\n")
			riskScore += 3
		}
		sb.WriteString("\n")

		// === Permissions ===
		sb.WriteString(fmt.Sprintf("🔐 **Permissions (%d total)**\n", len(perms)))
		dangerCount := 0
		for _, p := range perms {
			short := strings.TrimPrefix(p, "android.permission.")
			if desc, ok := dangerousPerms[p]; ok {
				sb.WriteString(fmt.Sprintf("  🚨 `%s` → %s\n", short, desc))
				riskScore++
				dangerCount++
			} else if strings.Contains(p, "INTERNET") || strings.Contains(p, "NETWORK") {
				sb.WriteString(fmt.Sprintf("  ✅ `%s` (wajar)\n", short))
			}
		}
		if dangerCount == 0 {
			sb.WriteString("  ✅ Gak ada permission berbahaya\n")
		}
		sb.WriteString("\n")
	}

	// === 2. Extract classes.dex + strings ===
	tmpDir, err := os.MkdirTemp("", "apkscan-*")
	if err != nil {
		sb.WriteString("⚠️ Gagal bikin temp dir\n")
	} else {
		defer os.RemoveAll(tmpDir)

		exec.Command("unzip", "-o", "-q", apkPath, "classes.dex", "-d", tmpDir).Run()
		dexPath := filepath.Join(tmpDir, "classes.dex")

		if _, err := os.Stat(dexPath); err != nil {
			sb.WriteString("⚠️ Gak ada `classes.dex` — kemungkinan APK kosong / corrupt\n\n")
		} else {
			stringsOut, _ := exec.Command("strings", dexPath).Output()
			content := string(stringsOut)

			// === BUKTI MENTAH — semua URL & IP dari classes.dex ===
			urls, ips, strongC2 := extractRawEvidence(content)

			// C2 KUAT (paling bahaya)
			if len(strongC2) > 0 {
				sb.WriteString("🚨 **C2 KUAT — BUKTI NYATA**\n")
				for _, c := range strongC2 {
					sb.WriteString(fmt.Sprintf("  ❌ `%s`\n", c))
					riskScore += 5
				}
				sb.WriteString("\n")
			}

			// SEMUA URL MENTAH
			if len(urls) > 0 {
				sb.WriteString(fmt.Sprintf("🌐 **Semua URL (%d unik, dari classes.dex):**\n", len(urls)))
				for _, u := range urls {
					mark := "  ❓ "
					if isWhitelistedDomain(u) {
						mark = "  ✅ "
					} else if strings.HasPrefix(strings.ToLower(u), "http://") {
						mark = "  ⚠️ "
					}
					sb.WriteString(fmt.Sprintf("%s`%s`\n", mark, u))
				}
				sb.WriteString("\n")
			}

			// SEMUA IP MENTAH (non-private)
			if len(ips) > 0 {
				sb.WriteString(fmt.Sprintf("📡 **IP mentah (%d unik):**\n", len(ips)))
				for _, ip := range ips {
					sb.WriteString(fmt.Sprintf("  ❓ `%s`\n", ip))
				}
				sb.WriteString("\n")
			}
		}
	}

	// === 3. Risk score + verdict ===
	if riskScore > 10 {
		riskScore = 10
	}

	var icon, verdict string
	switch {
	case riskScore >= 7:
		icon = "🚨"
		verdict = "**BAHAYA TINGGI** — kemungkinan besar RAT/spyware"
	case riskScore >= 4:
		icon = "⚠️"
		verdict = "**MENCURIGAKAN** — cek manual sebelum install"
	case riskScore >= 1:
		icon = "🟡"
		verdict = "**RENDAH** — kemungkinan aman, tapi ada yang aneh"
	default:
		icon = "✅"
		verdict = "**AMAN** — gak ada indikasi bahaya"
	}

	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf("%s **RISK SCORE: %d/10**\n", icon, riskScore))
	sb.WriteString(fmt.Sprintf("%s\n", verdict))

	return sb.String()
}

// domainWhitelist — domain umum yang sering muncul di APK legit.
var domainWhitelist = []string{
	"apache.org", "android.com", "google.com", "googleapis.com",
	"gstatic.com", "github.com", "githubusercontent.com", "cloudflare.com",
	"cloudflare-dns.com", "cloudflareclient.com", "one.one.one.one",
	"w3.org", "schemas.android.com", "json-schema.org", "kotlinlang.org",
	"oracle.com", "mozilla.org", "jquery.com", "facebook.com",
	"twitter.com", "instagram.com", "whatsapp.com", "telegram.org",
	"apple.com", "amazonaws.com", "microsoft.com", "windows.com",
	"bing.com", "yahoo.com", "youtube.com",
	"digicert.com", "letsencrypt.org", "symantec.com", "globalsign.com",
	"verisign.com", "comodo.com", "thawte.com", "entrust.net",
	"measurement.com", "app-measurement.com", "crashlytics.com",
	"jitpack.io", "goo.gl", "goog.gl", "gvt1.com", "gvt2.com",
	"issuetracker.google.com", "tools.android.com",
}

// isWhitelistedDomain — cek domain legit.
func isWhitelistedDomain(url string) bool {
	low := strings.ToLower(url)
	for _, d := range domainWhitelist {
		if strings.Contains(low, d) {
			return true
		}
	}
	return false
}

// isBogusIP — IP yang sering muncul tapi bukan C2 (wildcard, loopback, private).
func isBogusIP(ip string) bool {
	// IP khusus / wildcard / netmask
	skipExact := []string{
		"0.0.0.0", "0.1.0.0", "1.0.0.0", "127.0.0.1", "255.255.255.255",
		"255.255.255.0", "255.255.0.0",
	}
	for _, s := range skipExact {
		if ip == s {
			return true
		}
	}

	// Private range
	if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "172.16.") || strings.HasPrefix(ip, "169.254.") {
		return true
	}

	// RFC 5737 — IP test (buat dokumentasi)
	if strings.HasPrefix(ip, "192.0.2.") || strings.HasPrefix(ip, "198.51.100.") ||
		strings.HasPrefix(ip, "203.0.113.") {
		return true
	}

	// Cloudflare DNS (app 1.1.1.1 legit pake ini)
	if ip == "1.1.1.1" || ip == "1.0.0.1" || ip == "1.0.0.2" ||
		ip == "1.0.0.3" || ip == "1.1.1.2" || ip == "1.1.1.3" {
		return true
	}

	return false
}

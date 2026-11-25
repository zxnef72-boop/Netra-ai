// wininstaller_icon.go — ikon installer EXE: --icon=, ~/.netra-ai/icon.png|jpg, atau huruf otomatis.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// splitIconArg — ambil token --icon=<path> dari appName, kembalikan appName bersih dan path ikon.
func splitIconArg(appName string) (string, string) {
	iconArg := ""
	for _, tok := range strings.Fields(appName) {
		if strings.HasPrefix(strings.ToLower(tok), "--icon=") {
			iconArg = tok[7:]
			appName = strings.TrimSpace(strings.ReplaceAll(appName, tok, ""))
		}
	}
	return appName, iconArg
}

func iconFirstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n"); i != -1 {
		s = s[:i]
	}
	return s
}

// installerPrepIcon — bikin icon.ico di payload dan sisipkan <link rel="icon"> ke HTML.
// Gagal bikin ikon bukan error fatal: build lanjut tanpa ikon.
func installerPrepIcon(appName, iconArg, indexPath, workDir, payloadDir string, data []byte) ([]byte, string) {
	icoPath := ""
	if iconArg != "" {
		src := iconArg
		if strings.HasPrefix(src, "~/") {
			if home, herr := os.UserHomeDir(); herr == nil {
				src = filepath.Join(home, src[2:])
			}
		} else if !filepath.IsAbs(src) {
			src = filepath.Join(filepath.Dir(indexPath), src)
		}
		if _, serr := os.Stat(src); serr != nil {
			return data, fmt.Sprintf("  `→` icon        dilewati: file ikon gak ketemu (%s)\n", src)
		}
		icoPath = filepath.Join(workDir, "icon.ico")
		out, cerr := exec.Command("convert", src, "-background", "none", "-resize", "256x256",
			"-gravity", "center", "-extent", "256x256",
			"-define", "icon:auto-resize=256,128,64,48,32,16", icoPath).CombinedOutput()
		if cerr != nil {
			return data, fmt.Sprintf("  `→` icon        dilewati: convert gagal: %s\n", iconFirstLine(string(out)))
		}
	} else {
		p, perr := prepareICO(appName, workDir)
		if perr != nil {
			return data, fmt.Sprintf("  `→` icon        dilewati: %s\n", iconFirstLine(perr.Error()))
		}
		icoPath = p
	}
	if cerr := copyFile(icoPath, filepath.Join(payloadDir, "icon.ico")); cerr != nil {
		return data, fmt.Sprintf("  `→` icon        dilewati: copy gagal: %v\n", cerr)
	}
	htmlStr := string(data)
	linkTag := `<link rel="icon" href="icon.ico">`
	low := strings.ToLower(htmlStr)
	if idx := strings.Index(low, "</title>"); idx != -1 {
		end := idx + len("</title>")
		data = []byte(htmlStr[:end] + linkTag + htmlStr[end:])
	} else if h := strings.Index(low, "<head"); h != -1 {
		if gt := strings.Index(htmlStr[h:], ">"); gt != -1 {
			pos := h + gt + 1
			data = []byte(htmlStr[:pos] + linkTag + htmlStr[pos:])
		}
	}
	return data, "  `→` icon        OK (icon.ico)\n"
}

// winfavicon.go — sisipkan favicon (data URI) ke HTML supaya jendela Edge/Chrome --app=
// memakai ikon app di taskbar dan title bar (bukan ikon globe).
package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

func injectFavicon(html []byte, appName, srcPath string) []byte {
	low := strings.ToLower(string(html))
	if strings.Contains(low, `rel="icon"`) || strings.Contains(low, `rel='icon'`) || strings.Contains(low, "shortcut icon") {
		return html
	}
	var src image.Image
	if srcPath != "" {
		if im, err := loadIconImage(srcPath); err == nil {
			src = im
		}
	}
	if src == nil {
		if home, err := os.UserHomeDir(); err == nil {
			for _, n := range []string{"icon.png", "icon.jpg", "icon.jpeg"} {
				if im, err := loadIconImage(filepath.Join(home, ".netra-ai", n)); err == nil {
					src = im
					break
				}
			}
		}
	}
	if src == nil {
		src = makeDefaultIcon(appName, 256)
	}
	var pb bytes.Buffer
	if err := png.Encode(&pb, squareCanvas(src, 64)); err != nil {
		return html
	}
	tag := `<link rel="icon" type="image/png" href="data:image/png;base64,` + base64.StdEncoding.EncodeToString(pb.Bytes()) + `">`
	s := string(html)
	if h := strings.Index(low, "<head"); h != -1 {
		if gt := strings.Index(s[h:], ">"); gt != -1 {
			pos := h + gt + 1
			return []byte(s[:pos] + tag + s[pos:])
		}
	}
	return []byte(tag + s)
}

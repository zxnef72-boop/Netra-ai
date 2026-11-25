package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// portableTemplate — stub Windows: ekstrak payload embedded ke temp, buka index.html.
const portableTemplate = `package main

import (
	"embed"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

//go:embed all:payload
var payloadFS embed.FS

const AppName = __APPNAME__
const SafeName = __SAFENAME__
const BuildID = __BUILDID__

var (
	user32     = syscall.NewLazyDLL("user32.dll")
	procMsgBox = user32.NewProc("MessageBoxW")
)

func messageBox(title, text string, flags uintptr) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	procMsgBox.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags)
}

func extract(dir string) error {
	marker := filepath.Join(dir, ".netra-ok")
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	err := fs.WalkDir(payloadFS, "payload", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("payload", path)
		target := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := payloadFS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(BuildID), 0644)
}

func main() {
	dir := filepath.Join(os.TempDir(), "netra-"+SafeName+"-"+BuildID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		messageBox(AppName, "Gagal bikin folder temp:\n"+err.Error(), 0x10)
		return
	}
	if err := extract(dir); err != nil {
		messageBox(AppName, "Gagal ekstrak data app:\n"+err.Error(), 0x10)
		return
	}
	u := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(filepath.Join(dir, "index.html"))}
	target := u.String()
	candidates := []string{
		"C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
		"C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe",
		"msedge.exe",
		"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
		"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
		"chrome.exe",
	}
	for _, ep := range candidates {
		if err := exec.Command(ep, "--app="+target, "--window-size=1200,800").Start(); err == nil {
			return
		}
	}
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start(); err != nil {
		messageBox(AppName, "Gagal buka app. Install Edge atau Chrome dulu.", 0x10)
	}
}
`

// cmdBuildEXEPortable — folder HTML jadi satu EXE Windows mandiri.
// Pakai: /build exe-portable <folder> <nama-app>
func cmdBuildEXEPortable(htmlFolder, appName, iconPath string) (string, error) {
	var log strings.Builder

	if strings.HasPrefix(htmlFolder, "~/") {
		home, _ := os.UserHomeDir()
		htmlFolder = filepath.Join(home, htmlFolder[2:])
	}
	info, err := os.Stat(htmlFolder)
	if err != nil {
		return "", fmt.Errorf("path gak ketemu: %s", htmlFolder)
	}
	var indexPath string
	singleFile := !info.IsDir()
	if singleFile {
		lower := strings.ToLower(info.Name())
		if !strings.HasSuffix(lower, ".html") && !strings.HasSuffix(lower, ".htm") {
			return "", fmt.Errorf("harus folder atau file .html/.htm: %s", htmlFolder)
		}
		indexPath = htmlFolder
	} else {
		indexPath = filepath.Join(htmlFolder, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			indexPath = ""
			items, _ := os.ReadDir(htmlFolder)
			for _, e := range items {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".html") {
					indexPath = filepath.Join(htmlFolder, e.Name())
					break
				}
			}
			if indexPath == "" {
				return "", fmt.Errorf("gak ada .html di %s", htmlFolder)
			}
		}
	}
	if _, err := exec.LookPath("go"); err != nil {
		return "", fmt.Errorf("go belum terpasang di PATH")
	}

	safeName := sanitizeName(appName)
	base := defaultAgentWorkspace()
	stamp := time.Now().Unix()
	workDir := filepath.Join(base, "exe-build", fmt.Sprintf("%s_%d", safeName, stamp))
	outDir := filepath.Join(base, "exe-output")
	payloadDir := filepath.Join(workDir, "payload")
	if err := os.MkdirAll(payloadDir, 0755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	log.WriteString(fmt.Sprintf("**Building EXE mandiri: %s**\n\n", appName))
	log.WriteString(fmt.Sprintf("Sumber : `%s`\n", htmlFolder))
	log.WriteString(fmt.Sprintf("WorkDir: `%s`\n\n", workDir))

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return log.String(), err
	}
	if iconPath != "" {
		if strings.HasPrefix(iconPath, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				iconPath = filepath.Join(home, iconPath[2:])
			}
		} else if !filepath.IsAbs(iconPath) {
			iconPath = filepath.Join(filepath.Dir(indexPath), iconPath)
		}
		if iconData, err := os.ReadFile(iconPath); err == nil {
			ext := strings.ToLower(filepath.Ext(iconPath))
			if ext == "" {
				ext = ".png"
			}
			iconFile := "icon" + ext
			if err := os.WriteFile(filepath.Join(payloadDir, iconFile), iconData, 0644); err == nil {
				htmlStr := string(data)
				linkTag := `<link rel="icon" href="` + iconFile + `">`
				if idx := strings.Index(strings.ToLower(htmlStr), "</title>"); idx != -1 {
					endIdx := idx + len("</title>")
					data = []byte(htmlStr[:endIdx] + linkTag + htmlStr[endIdx:])
				} else if hIdx := strings.Index(strings.ToLower(htmlStr), "<head"); hIdx != -1 {
					if gt := strings.Index(htmlStr[hIdx:], ">"); gt != -1 {
						pos := hIdx + gt + 1
						data = []byte(htmlStr[:pos] + linkTag + htmlStr[pos:])
					}
				}
			}
		}
	}
	data = injectFavicon(data, appName, iconPath)
	if err := os.WriteFile(filepath.Join(payloadDir, "index.html"), data, 0644); err != nil {
		return log.String(), err
	}
	if !singleFile {
		items, _ := os.ReadDir(htmlFolder)
		for _, e := range items {
			name := e.Name()
			if filepath.Join(htmlFolder, name) == indexPath {
				continue
			}
			src := filepath.Join(htmlFolder, name)
			dst := filepath.Join(payloadDir, name)
			if e.IsDir() {
				copyDir(src, dst)
			} else {
				copyFile(src, dst)
			}
		}
	}
	log.WriteString("  `→` payload     OK\n")

	stub := portableTemplate
	stub = strings.ReplaceAll(stub, "__APPNAME__", strconv.Quote(appName))
	stub = strings.ReplaceAll(stub, "__SAFENAME__", strconv.Quote(safeName))
	stub = strings.ReplaceAll(stub, "__BUILDID__", strconv.Quote(strconv.FormatInt(stamp, 10)))
	if err := os.WriteFile(filepath.Join(workDir, "main.go"), []byte(stub), 0644); err != nil {
		return log.String(), err
	}
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte("module netraportable\n\ngo 1.21\n"), 0644); err != nil {
		return log.String(), err
	}
	log.WriteString("  `→` stub        OK\n")
	if err := embedIconFor(appName, workDir, iconPath); err != nil {
		log.WriteString("  `→` ikon        dilewati: " + err.Error() + "\n")
	} else {
		log.WriteString("  `→` ikon        OK\n")
	}

	outExe := filepath.Join(outDir, safeName+".exe")
	cmd := exec.Command("go", "build", "-o", outExe, "-ldflags", "-s -w -H=windowsgui", ".")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.WriteString(fmt.Sprintf("  `→` compile     GAGAL\n\n```\n%s\n```\n", string(out)))
		return log.String(), err
	}
	log.WriteString("  `→` compile     OK\n")

	fi, err := os.Stat(outExe)
	if err != nil {
		return log.String(), err
	}
	os.RemoveAll(workDir)

	log.WriteString("\n**EXE mandiri siap!**\n")
	log.WriteString(fmt.Sprintf("`%s` (%d KB)\n", outExe, fi.Size()/1024))
	log.WriteString("\nSatu file, tanpa installer. Butuh Edge atau Chrome di PC.\n")
	log.WriteString("WSL: salin ke Windows, contoh `cp <file> /mnt/c/Users/Public/`\n")
	return log.String(), nil
}

// wininstaller_builder.go — bikin Setup.exe murni pake Go.
// Gak butuh NSIS. Bukan portable. Beneran installer:
//   - extract payload ke %LOCALAPPDATA%\NamaApp\
//   - bikin shortcut Desktop + Start Menu
//   - registry entry (uninstall from Control Panel)
//   - auto-run viewer
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// installerTemplate — source code installer Go yang di-generate.
// Placeholder __APPNAME__, __VIEWEREXE__ di-replace runtime.
const installerTemplate = `// Setup.exe — installer otomatis buat __APPNAME__
package main

import (
        "embed"
        "fmt"
        "io/fs"
        "os"
        "os/exec"
        "path/filepath"
        "syscall"
        "unsafe"
)

//go:embed payload
var payloadFS embed.FS

const AppName = "__APPNAME__"
const ViewerExe = "__VIEWEREXE__"

var (
        user32     = syscall.NewLazyDLL("user32.dll")
        procMsgBox = user32.NewProc("MessageBoxW")
)

const (
        MB_OK              = 0x00000000
        MB_ICONERROR       = 0x00000010
        MB_ICONINFORMATION = 0x00000040
)

func messageBox(title, text string, flags uintptr) {
        t, _ := syscall.UTF16PtrFromString(title)
        m, _ := syscall.UTF16PtrFromString(text)
        procMsgBox.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags)
}

func fail(msg string) {
        messageBox(AppName, msg, MB_OK|MB_ICONERROR)
}

func main() {
        // 1. Dapatkan folder install
        localAppData := os.Getenv("LOCALAPPDATA")
        if localAppData == "" {
                fail("Error: LOCALAPPDATA gak ada. Bukan Windows?")
                return
        }
        installDir := filepath.Join(localAppData, AppName)

        if err := os.MkdirAll(installDir, 0755); err != nil {
                fail("Error bikin folder: " + err.Error())
                return
        }

        // 2. Extract payload
        count := 0
        err := fs.WalkDir(payloadFS, "payload", func(path string, d fs.DirEntry, err error) error {
                if err != nil { return err }
                rel, _ := filepath.Rel("payload", path)
                target := filepath.Join(installDir, rel)

                if d.IsDir() {
                        return os.MkdirAll(target, 0755)
                }
                data, _ := payloadFS.ReadFile(path)
                if err := os.WriteFile(target, data, 0644); err != nil {
                        return err
                }
                count++
                return nil
        })
        if err != nil {
                fail("Error extract: " + err.Error())
                return
        }

        // 3. Bikin shortcut Desktop + Start Menu via PowerShell
        exePath := filepath.Join(installDir, ViewerExe)
        userProfile := os.Getenv("USERPROFILE")
        desktop := filepath.Join(userProfile, "Desktop")
        startMenu := filepath.Join(userProfile, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs", AppName)

        os.MkdirAll(startMenu, 0755)

        psScript := fmt.Sprintf(` + "`" + `
$WshShell = New-Object -ComObject WScript.Shell
$Desktop = $WshShell.CreateShortcut('%s\%s.lnk')
$Desktop.TargetPath = '%s'
$Desktop.WorkingDirectory = '%s'
$Desktop.Save()
$Start = $WshShell.CreateShortcut('%s\%s.lnk')
$Start.TargetPath = '%s'
$Start.WorkingDirectory = '%s'
$Start.Save()
` + "`" + `, desktop, AppName, exePath, installDir, startMenu, AppName, exePath, installDir)

        cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
        _ = cmd.Run()

        // 4. Registry entry buat Control Panel (uninstall)
        regBase := fmt.Sprintf(` + "`" + `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\%s` + "`" + `, AppName)
        regEntries := [][]string{
                {"DisplayName", AppName},
                {"DisplayVersion", "1.0"},
                {"Publisher", "NetRa"},
                {"InstallLocation", installDir},
                {"UninstallString", fmt.Sprintf(` + "`" + `cmd /c del "%s"` + "`" + `, exePath)},
        }
        for _, e := range regEntries {
                exec.Command("reg", "add", regBase, "/v", e[0], "/t", "REG_SZ", "/d", e[1], "/f").Run()
        }

        // 5. Jalanin app
        _ = exec.Command(exePath).Start()

        // 6. Notif selesai
        messageBox(AppName, "Install selesai!\n\nShortcut ada di Desktop.\nUninstall lewat Settings > Apps.", MB_OK|MB_ICONINFORMATION)
}
`

// cmdBuildWinInstaller — build Setup.exe Go (bukan NSIS).
// Input: folder HTML + nama app
func cmdBuildWinInstaller(htmlFolder, appName string) (string, error) {
	var log strings.Builder

	// Expand ~
	if strings.HasPrefix(htmlFolder, "~/") {
		home, _ := os.UserHomeDir()
		htmlFolder = filepath.Join(home, htmlFolder[2:])
	}

	// Cek folder + index.html (atau auto-detect HTML)
	info, err := os.Stat(htmlFolder)
	if err != nil {
		return "", fmt.Errorf("path gak ketemu: %s", htmlFolder)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("harus folder, bukan file: %s", htmlFolder)
	}

	indexPath := filepath.Join(htmlFolder, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		// Cari .html pertama
		entries, _ := os.ReadDir(htmlFolder)
		found := false
		for _, e := range entries {
			if strings.HasSuffix(strings.ToLower(e.Name()), ".html") {
				indexPath = filepath.Join(htmlFolder, e.Name())
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("gak ada .html di %s", htmlFolder)
		}
	}

	safeName := sanitizeName(appName)
	viewerExe := safeName + ".exe"

	// Workdir di /sdcard (accessible)
	workDir := fmt.Sprintf("/sdcard/AiNef/win-installer-build/%s_%d", safeName, time.Now().Unix())
	outDir := "/sdcard/AiNef/exe-output"

	os.MkdirAll(workDir, 0755)
	os.MkdirAll(outDir, 0755)

	log.WriteString(fmt.Sprintf("🏗️  **Building Go Installer: %s**\n\n", appName))
	log.WriteString(fmt.Sprintf("Folder : `%s`\n", htmlFolder))
	log.WriteString(fmt.Sprintf("WorkDir: `%s`\n\n", workDir))

	// 1. Bikin struktur installer
	installerDir := filepath.Join(workDir, "installer")
	payloadDir := filepath.Join(installerDir, "payload")
	os.MkdirAll(payloadDir, 0755)
	log.WriteString("  `→` folders     ✓\n")

	// 2. Compile viewer EXE (yang buka HTML)
	viewerWork := filepath.Join(workDir, "viewer")
	os.MkdirAll(viewerWork, 0755)

	viewerMain := `package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	htmlPath := filepath.Join(exeDir, "index.html")

	if _, err := os.Stat(htmlPath); err != nil {
		fmt.Println("Error: index.html gak ketemu di:", exeDir)
		fmt.Scanln()
		return
	}

	htmlURL := "file:///" + filepath.ToSlash(htmlPath)
	candidates := []string{
		"C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
		"C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe",
		"msedge.exe",
		"chrome.exe",
	}

	for _, ep := range candidates {
		if err := exec.Command(ep, "--app="+htmlURL, "--window-size=1200,800").Start(); err == nil {
			return
		}
	}

	exec.Command("rundll32", "url.dll,FileProtocolHandler", htmlURL).Start()
}
`
	os.WriteFile(filepath.Join(viewerWork, "main.go"), []byte(viewerMain), 0644)
	os.WriteFile(filepath.Join(viewerWork, "go.mod"), []byte("module netraviewer\n\ngo 1.21\n"), 0644)

	viewerCmd := exec.Command("go", "build",
		"-o", filepath.Join(payloadDir, viewerExe),
		"-ldflags", "-s -w",
		".")
	viewerCmd.Dir = viewerWork
	viewerCmd.Env = append(os.Environ(),
		"GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")

	if out, err := viewerCmd.CombinedOutput(); err != nil {
		log.WriteString(fmt.Sprintf("  `→` viewer      ❌\n\n```\n%s\n```\n", string(out)))
		return log.String(), err
	}
	log.WriteString("  `→` viewer      ✓\n")

	// 3. Copy HTML + assets ke payload
	data, _ := os.ReadFile(indexPath)
	os.WriteFile(filepath.Join(payloadDir, "index.html"), data, 0644)

	// Copy semua file lain di folder HTML
	entries, _ := os.ReadDir(htmlFolder)
	for _, e := range entries {
		name := e.Name()
		if name == "index.html" || name == "index.htm" {
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
	log.WriteString("  `→` payload     ✓\n")

	// 4. Generate installer main.go
	installerSrc := strings.ReplaceAll(installerTemplate, "__APPNAME__", appName)
	installerSrc = strings.ReplaceAll(installerSrc, "__VIEWEREXE__", viewerExe)
	os.WriteFile(filepath.Join(installerDir, "main.go"), []byte(installerSrc), 0644)
	os.WriteFile(filepath.Join(installerDir, "go.mod"), []byte("module netrainstaller\n\ngo 1.21\n"), 0644)
	log.WriteString("  `→` installer   ✓\n")

	// 5. Compile Setup.exe
	setupPath := filepath.Join(workDir, safeName+"-Setup.exe")
	installerCmd := exec.Command("go", "build",
		"-o", setupPath,
		"-ldflags", "-s -w -H=windowsgui",
		".")
	installerCmd.Dir = installerDir
	installerCmd.Env = append(os.Environ(),
		"GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")

	if out, err := installerCmd.CombinedOutput(); err != nil {
		log.WriteString(fmt.Sprintf("  `→` compile     ❌\n\n```\n%s\n```\n", string(out)))
		return log.String(), err
	}
	log.WriteString("  `→` compile     ✓\n")

	// 6. Copy ke output
	finalPath := filepath.Join(outDir, safeName+"-Setup.exe")
	if err := copyFile(setupPath, finalPath); err != nil {
		log.WriteString(fmt.Sprintf("  `→` copy        ❌ %v\n", err))
		return log.String(), err
	}
	log.WriteString("  `→` copy        ✓\n")

	fi, _ := os.Stat(finalPath)
	log.WriteString(fmt.Sprintf("\n✅ **Setup.exe siap! (Go installer, tanpa NSIS)**\n"))
	log.WriteString(fmt.Sprintf("📁 `%s` (%d KB)\n", finalPath, fi.Size()/1024))
	log.WriteString("\n**Cara pakai di PC:**\n")
	log.WriteString("1. Copy Setup.exe ke PC\n")
	log.WriteString("2. Klik 2x → auto install\n")
	log.WriteString("3. Shortcut di Desktop → klik → jalan\n")
	log.WriteString("4. Uninstall via Setting > Apps\n")
	return log.String(), nil
}

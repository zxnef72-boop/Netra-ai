// exebuilder.go — build EXE portable + installer Windows dari folder HTML.
// Butuh: Go (cross-compile) + makensis (di proot netra-sandbox).
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// viewerSource — source Go buat HTML viewer Windows.
// Di-embed ke EXE, buka HTML via Edge/Chrome app mode.
const viewerSource = `package main

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
		fmt.Println("Tekan Enter buat keluar...")
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

// nsisTemplate — template script NSIS. Pakai placeholder biar aman.
const nsisTemplate = `!include "MUI2.nsh"

Name "__APPNAME__"
OutFile "Setup.exe"
InstallDir "$LOCALAPPDATA\__APPNAME__"
InstallDirRegKey HKCU "Software\__APPNAME__" "InstallDir"
RequestExecutionLevel user

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section "Install"
    SetOutPath "$INSTDIR"
    File "__EXENAME__"
    File "index.html"

    WriteRegStr HKCU "Software\__APPNAME__" "InstallDir" "$INSTDIR"

    CreateShortcut "$DESKTOP\__APPNAME__.lnk" "$INSTDIR\__EXENAME__"
    CreateDirectory "$SMPROGRAMS\__APPNAME__"
    CreateShortcut "$SMPROGRAMS\__APPNAME__\__APPNAME__.lnk" "$INSTDIR\__EXENAME__"
    CreateShortcut "$SMPROGRAMS\__APPNAME__\Uninstall.lnk" "$INSTDIR\uninstall.exe"

    WriteUninstaller "$INSTDIR\uninstall.exe"

    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\__APPNAME__" "DisplayName" "__APPNAME__"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\__APPNAME__" "UninstallString" "$INSTDIR\uninstall.exe"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\__APPNAME__" "DisplayIcon" "$INSTDIR\__EXENAME__"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\__APPNAME__" "Publisher" "NetRa"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\__APPNAME__" "DisplayVersion" "1.0"
SectionEnd

Section "Uninstall"
    Delete "$INSTDIR\__EXENAME__"
    Delete "$INSTDIR\index.html"
    Delete "$INSTDIR\uninstall.exe"
    RMDir "$INSTDIR"

    Delete "$DESKTOP\__APPNAME__.lnk"
    Delete "$SMPROGRAMS\__APPNAME__\__APPNAME__.lnk"
    Delete "$SMPROGRAMS\__APPNAME__\Uninstall.lnk"
    RMDir "$SMPROGRAMS\__APPNAME__"

    DeleteRegKey HKCU "Software\__APPNAME__"
    DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\__APPNAME__"
SectionEnd
`

// buildEXEViewer — compile viewer Go → EXE Windows.
func buildEXEViewer(workDir, exeName string) error {
	viewerDir := filepath.Join(workDir, "viewer")
	if err := os.MkdirAll(viewerDir, 0755); err != nil {
		return err
	}

	// Tulis main.go
	if err := os.WriteFile(filepath.Join(viewerDir, "main.go"), []byte(viewerSource), 0644); err != nil {
		return err
	}
	// Tulis go.mod
	gomod := "module netraviewer\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(viewerDir, "go.mod"), []byte(gomod), 0644); err != nil {
		return err
	}

	// Cross-compile ke Windows
	cmd := exec.Command("go", "build", "-o", filepath.Join(workDir, exeName), "-ldflags", "-s -w", ".")
	cmd.Dir = viewerDir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compile EXE gagal: %v\n%s", err, string(out))
	}
	return nil
}

// cmdBuildInstallerEXE — entry point. Folder HTML → Setup.exe installer.
func cmdBuildInstallerEXE(htmlFolder, appName string) (string, error) {
	var log strings.Builder

	// Expand ~
	if strings.HasPrefix(htmlFolder, "~/") {
		home, _ := os.UserHomeDir()
		htmlFolder = filepath.Join(home, htmlFolder[2:])
	}

	// Cek folder + index.html
	if info, err := os.Stat(htmlFolder); err != nil || !info.IsDir() {
		return "", fmt.Errorf("folder gak ketemu: %s", htmlFolder)
	}
	indexSrc := filepath.Join(htmlFolder, "index.html")
	if _, err := os.Stat(indexSrc); err != nil {
		indexSrc = filepath.Join(htmlFolder, "index.htm")
		if _, err := os.Stat(indexSrc); err != nil {
			return "", fmt.Errorf("gak ada index.html di %s", htmlFolder)
		}
	}

	safeName := sanitizeName(appName)
	exeName := safeName + ".exe"

	// Workdir — auto-detect OS
	var baseDir, outDir string
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, "netra-ai", "exe-build")
		outDir = filepath.Join(home, "netra-ai", "exe-output")
	} else {
		// Termux/Linux — pake /sdcard biar accessible dari proot
		baseDir = "/sdcard/AiNef/exe-build"
		outDir = "/sdcard/AiNef/exe-output"
	}
	workDir := filepath.Join(baseDir, fmt.Sprintf("%s_%d", safeName, time.Now().Unix()))

	os.MkdirAll(baseDir, 0755)
	os.MkdirAll(workDir, 0755)
	os.MkdirAll(outDir, 0755)

	log.WriteString(fmt.Sprintf("📦 **Building EXE Installer: %s**\n\n", appName))
	log.WriteString(fmt.Sprintf("Folder : `%s`\n", htmlFolder))
	log.WriteString(fmt.Sprintf("Workdir: `%s`\n\n", workDir))

	// Step 1: compile viewer EXE
	log.WriteString("  `→` compile viewer ")
	if err := buildEXEViewer(workDir, exeName); err != nil {
		log.WriteString("❌\n\n")
		log.WriteString(fmt.Sprintf("**Error:**\n```\n%v\n```\n", err))
		return log.String(), err
	}
	log.WriteString("✓\n")

	// Step 2: copy index.html
	log.WriteString("  `→` copy html     ")
	srcData, err := os.ReadFile(indexSrc)
	if err != nil {
		log.WriteString("❌\n")
		return log.String(), err
	}
	if err := os.WriteFile(filepath.Join(workDir, "index.html"), srcData, 0644); err != nil {
		log.WriteString("❌\n")
		return log.String(), err
	}
	log.WriteString("✓\n")

	// Step 3: copy extra files (css, js, images)
	entries, _ := os.ReadDir(htmlFolder)
	for _, e := range entries {
		if e.Name() == "index.html" || e.Name() == "index.htm" {
			continue
		}
		src := filepath.Join(htmlFolder, e.Name())
		dst := filepath.Join(workDir, e.Name())
		if e.IsDir() {
			copyDir(src, dst)
		} else {
			copyFile(src, dst)
		}
	}
	log.WriteString("  `→` copy assets   ✓\n")

	// Step 4: tulis installer.nsi
	log.WriteString("  `→` nsi script    ")
	nsi := strings.ReplaceAll(nsisTemplate, "__APPNAME__", appName)
	nsi = strings.ReplaceAll(nsi, "__EXENAME__", exeName)
	if err := os.WriteFile(filepath.Join(workDir, "installer.nsi"), []byte(nsi), 0644); err != nil {
		log.WriteString("❌\n")
		return log.String(), err
	}
	log.WriteString("✓\n")

	// Step 5: panggil makensis — auto-detect OS
	log.WriteString("  `→` makensis      ")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows — panggil makensis.exe langsung (butuh NSIS terinstall)
		cmd = exec.Command("makensis", "installer.nsi")
		cmd.Dir = workDir
	} else {
		// Termux/Linux — panggil via proot-distro
		prootPath := "/root/hp-storage" + workDir[len("/sdcard"):] + "/installer.nsi"
		cmd = exec.Command("proot-distro", "login", "netra-sandbox",
			"--shared-tmp",
			"--bind", "/storage/emulated/0:/root/hp-storage",
			"--",
			"bash", "-c", fmt.Sprintf("cd '%s' && makensis installer.nsi",
				filepath.Dir(prootPath)))
		cmd.Dir = workDir
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WriteString("❌\n\n")
		log.WriteString(fmt.Sprintf("**makensis gagal:**\n```\n%s\n```\n", string(out)))
		if runtime.GOOS == "windows" {
			log.WriteString("\n**Info:** Pastikan NSIS terinstall di Windows.\n")
			log.WriteString("Download: https://nsis.sourceforge.io\n")
			log.WriteString("Tambahkan `C:\\Program Files (x86)\\NSIS\\` ke PATH.\n")
		}
		return log.String(), err
	}
	log.WriteString("✓\n")

	// Step 6: copy Setup.exe ke output
	log.WriteString("  `→` copy output   ")
	setupSrc := filepath.Join(workDir, "Setup.exe")
	setupDst := filepath.Join(outDir, safeName+"-Setup.exe")
	if err := copyFile(setupSrc, setupDst); err != nil {
		log.WriteString("❌\n")
		return log.String(), err
	}
	log.WriteString("✓\n")

	fi, _ := os.Stat(setupDst)
	log.WriteString(fmt.Sprintf("\n✅ **Installer siap!**\n"))
	log.WriteString(fmt.Sprintf("📁 `%s` (%d KB)\n", setupDst, fi.Size()/1024))
	log.WriteString(fmt.Sprintf("📁 Portable: `%s`\n", filepath.Join(workDir, exeName)))
	log.WriteString("📲 Kirim Setup.exe ke PC → klik 2x → install\n")
	return log.String(), nil
}

// copyFile — helper (kalo belum ada di file lain)
func copyFileEXE(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

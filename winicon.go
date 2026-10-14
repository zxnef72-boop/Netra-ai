// winicon.go — helper embed icon ke EXE Windows.
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// generateDefaultPNG — bikin icon default pakai ImageMagick.
func generateDefaultPNG(appName, outPath string) error {
	initial := "?"
	for _, r := range appName {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			initial = string(r)
			break
		}
	}
	if len(initial) == 1 && initial[0] >= 'a' && initial[0] <= 'z' {
		initial = string(initial[0] - 32)
	}

	h := sha256.Sum256([]byte(appName))
	bg := fmt.Sprintf("#%02x%02x%02x", h[0]&0x7f+0x40, h[1]&0x7f+0x40, h[2]&0x7f+0x40)

	cmd := exec.Command("convert",
		"-size", "256x256",
		"xc:"+bg,
		"-gravity", "center",
		"-pointsize", "140",
		"-fill", "white",
		"-annotate", "0", initial,
		outPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("convert gagal: %v\n%s", err, string(out))
	}
	return nil
}

// prepareICO — cari icon.png di ~/.netra-ai/, atau generate default.
func prepareICO(appName, workDir string) (string, error) {
	home, _ := os.UserHomeDir()
	pngPath := filepath.Join(home, ".netra-ai", "icon.png")
	if _, err := os.Stat(pngPath); err != nil {
		pngPath = filepath.Join(home, ".netra-ai", "icon.jpg")
		if _, err := os.Stat(pngPath); err != nil {
			pngPath = filepath.Join(workDir, "icon_default.png")
			if err := generateDefaultPNG(appName, pngPath); err != nil {
				return "", err
			}
		}
	}

	icoPath := filepath.Join(workDir, "icon.ico")
	cmd := exec.Command("convert", pngPath,
		"-define", "icon:auto-resize=256,128,64,48,32,16",
		icoPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("convert ico gagal: %v\n%s", err, string(out))
	}
	return icoPath, nil
}

// embedIcon — copy .ico ke targetDir + generate rsrc.syso.
func embedIcon(targetDir, icoPath string) error {
	if icoPath == "" {
		return nil
	}
	localIco := filepath.Join(targetDir, "icon.ico")
	if err := copyFile(icoPath, localIco); err != nil {
		return fmt.Errorf("copy ico: %w", err)
	}

	rsrcBin := filepath.Join(os.Getenv("HOME"), "go", "bin", "rsrc")
	if _, err := os.Stat(rsrcBin); err != nil {
		rsrcBin = "rsrc"
	}

	cmd := exec.Command(rsrcBin, "-ico", localIco, "-o", filepath.Join(targetDir, "rsrc.syso"))
	cmd.Dir = targetDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsrc gagal: %v\n%s", err, string(out))
	}
	return nil
}

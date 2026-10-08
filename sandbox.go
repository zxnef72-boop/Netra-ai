// sandbox.go — jalanin command di environment terisolasi.
// Deteksi OS otomatis:
//   - Termux (Android) -> proot-distro
//   - Linux            -> docker (kalau ada), fallback shell
//   - Windows          -> WSL (kalau ada), fallback cmd
//   - macOS            -> docker (kalau ada), fallback shell
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const sandboxDistro = "netra-sandbox"
const sandboxTimeout = 60 * time.Second

// isTermux — cek apakah lagi jalan di Termux (Android).
func isTermux() bool {
	prefix := os.Getenv("PREFIX")
	if strings.Contains(prefix, "com.termux") {
		return true
	}
	// Cek juga file marker proot-distro
	if _, err := os.Stat("/data/data/com.termux/files/usr/bin/proot-distro"); err == nil {
		return true
	}
	return false
}

// hasCommand — cek apakah command ada di PATH.
func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// sandboxBackend — return (nama backend, args-template, deskripsi).
// Args-template dipanggil dengan args[0]=script.
func sandboxBackend(script string) (string, []string, string, error) {
	// 1. Termux -> proot-distro
	if isTermux() {
		if hasCommand("proot-distro") {
			return "proot-distro",
				[]string{"login", sandboxDistro, "--", "bash", "-c", script},
				"Kali (proot-distro)", nil
		}
		return "", nil, "", fmt.Errorf("proot-distro gak ketemu. Install: pkg install proot-distro")
	}

	// 2. Windows -> WSL
	if runtime.GOOS == "windows" {
		if hasCommand("wsl") {
			return "wsl",
				[]string{"bash", "-c", script},
				"WSL (Windows Subsystem Linux)", nil
		}
		// Fallback: cmd.exe
		if hasCommand("cmd") {
			return "cmd",
				[]string{"/C", script},
				"cmd.exe (no isolasi)", nil
		}
		return "", nil, "", fmt.Errorf("gak ada WSL atau cmd.exe")
	}

	// 3. Linux/macOS -> docker
	if hasCommand("docker") {
		// Cek docker daemon jalan gak
		check := exec.Command("docker", "info")
		if err := check.Run(); err == nil {
			return "docker",
				[]string{"run", "--rm", "kalilinux/kali-rolling", "bash", "-c", script},
				"Docker (kalilinux/kali-rolling)", nil
		}
	}

	// 4. Fallback: shell biasa
	shell := "/bin/bash"
	if runtime.GOOS == "darwin" {
		shell = "/bin/zsh"
	}
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}
	return shell,
		[]string{"-c", script},
		"host shell (no isolasi)", nil
}

// cmdSandboxRun — jalanin perintah shell di environment terisolasi.
func cmdSandboxRun(script string) (string, error) {
	var log strings.Builder

	script = strings.TrimSpace(script)
	if script == "" {
		return "", fmt.Errorf("pakai: /sandbox <command>")
	}

	// Pilih backend sesuai OS
	bin, args, desc, err := sandboxBackend(script)
	if err != nil {
		return "", err
	}

	log.WriteString("**Sandbox Run**\n\n")
	log.WriteString(fmt.Sprintf("Backend: `%s`\n", desc))
	log.WriteString(fmt.Sprintf("Command: `%s`\n\n", script))
	log.WriteString("```\n")

	ctx, cancel := context.WithTimeout(context.Background(), sandboxTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	out, runErr := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		log.WriteString("[TIMEOUT: >60s, proses dihentikan]\n")
		log.WriteString("```\n")
		return log.String(), fmt.Errorf("timeout 60 detik")
	}

	output := string(out)
	if output == "" {
		output = "(tidak ada output)\n"
	}
	log.WriteString(output)
	if !strings.HasSuffix(output, "\n") {
		log.WriteString("\n")
	}
	log.WriteString("```\n")

	if runErr != nil {
		log.WriteString("\nExit code: non-zero\n")
		return log.String(), runErr
	}

	log.WriteString("\nSelesai\n")
	return log.String(), nil
}

// pathJoinSafe — helper path portable (dipakai file lain juga).
func pathJoinSafe(parts ...string) string {
	return filepath.Join(parts...)
}

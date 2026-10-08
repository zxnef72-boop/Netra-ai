// sandbox.go — sandbox jujur.
// Kalau ada proot-distro + distro, pakai itu (isolasi penuh).
// Kalau gak ada, pakai sandbox logis (cwd dikurung, BUKAN isolasi).
// Header selalu bilang jujur level isolasinya.
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

const sandboxTimeout = 60 * time.Second

// deteksi distro proot yang tersedia (prioritas: alpine > debian > netra-sandbox > kali)
func detectProotDistro() string {
	if !hasCommand("proot-distro") {
		return ""
	}
	// Cek folder installed-rootfs langsung (lebih reliable dari parsing output)
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}
	rootfsDir := filepath.Join(prefix, "var", "lib", "proot-distro", "containers")
	prioritas := []string{"alpine", "debian", "netra-sandbox", "kali", "ubuntu"}
	for _, d := range prioritas {
		if _, err := os.Stat(filepath.Join(rootfsDir, d)); err == nil {
			return d
		}
	}
	// Fallback: parsing output (kalau folder gak ada)
	out, err := exec.Command("proot-distro", "list").Output()
	if err != nil {
		return ""
	}
	installed := string(out)
	for _, d := range prioritas {
		if strings.Contains(installed, d) {
			return d
		}
	}
	return ""
}

func isTermux() bool {
	prefix := os.Getenv("PREFIX")
	return strings.Contains(prefix, "com.termux")
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// sandboxBackend — return (bin, args, deskripsi, isolasi)
func sandboxBackend(script string) (string, []string, string, string) {
	// 1. Termux + proot-distro
	if isTermux() {
		if distro := detectProotDistro(); distro != "" {
			return "proot-distro",
				[]string{"login", distro, "--", "bash", "-c", script},
				"proot: " + distro,
				"penuh"
		}
	}

	// 2. Windows + WSL
	if runtime.GOOS == "windows" {
		if hasCommand("wsl") {
			return "wsl",
				[]string{"bash", "-c", script},
				"WSL",
				"penuh"
		}
		return "cmd",
			[]string{"/C", script},
			"cmd.exe",
			"tidak ada"
	}

	// 3. Linux/macOS + docker
	if hasCommand("docker") {
		check := exec.Command("docker", "info")
		if err := check.Run(); err == nil {
			return "docker",
				[]string{"run", "--rm", "alpine", "sh", "-c", script},
				"Docker alpine",
				"penuh"
		}
	}

	// 4. Fallback: sandbox logis (murni Go, cwd dikurung)
	return "", nil, "logis (cwd dikurung)", "tidak ada"
}

// runLogis — sandbox logis tanpa proot/docker.
// Cwd dikurung di ~/.netra-ai/sandbox/, tapi command tetap bisa akses sistem.
func runLogis(script string) string {
	home, _ := os.UserHomeDir()
	sandboxDir := filepath.Join(home, ".netra-ai", "sandbox")
	os.MkdirAll(sandboxDir, 0755)

	shell := "/bin/sh"
	if isTermux() {
		shell = "/data/data/com.termux/files/usr/bin/bash"
	}
	if _, err := os.Stat(shell); err != nil {
		shell = "sh"
	}

	c := exec.Command(shell, "-c", script)
	c.Dir = sandboxDir
	c.Env = []string{
		"HOME=" + sandboxDir,
		"PATH=" + os.Getenv("PATH"),
		"TERM=xterm",
	}
	out, _ := c.CombinedOutput()
	return string(out)
}

func cmdSandboxRun(script string) (string, error) {
	script = strings.TrimSpace(script)
	if script == "" {
		return "", fmt.Errorf("pakai: /sandbox <command>")
	}

	bin, args, desc, isolasi := sandboxBackend(script)

	var log strings.Builder
	log.WriteString("**Sandbox Run**\n\n")
	log.WriteString(fmt.Sprintf("Backend : `%s`\n", desc))
	log.WriteString(fmt.Sprintf("Isolasi : `%s`\n", isolasi))
	if isolasi == "tidak ada" {
		log.WriteString("_Catatan: cwd dikurung, tapi command tetap bisa akses sistem._\n")
	}
	log.WriteString(fmt.Sprintf("Command : `%s`\n\n", script))
	log.WriteString("```\n")

	var output string
	var runErr error

	if bin == "" {
		// Sandbox logis
		output = runLogis(script)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), sandboxTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, args...)
		out, err := cmd.CombinedOutput()
		output = string(out)
		runErr = err
		if ctx.Err() == context.DeadlineExceeded {
			log.WriteString("[TIMEOUT: >60s]\n```\n")
			return log.String(), fmt.Errorf("timeout")
		}
	}

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

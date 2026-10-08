// sandbox.go — jalanin command di dalam proot-distro netra-sandbox.
// Mini-Docker versi NetRa: isolasi environment, capture output, timeout.
package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const sandboxDistro = "netra-sandbox"
const sandboxTimeout = 60 * time.Second

// cmdSandboxRun — jalanin perintah shell di dalam netra-sandbox.
func cmdSandboxRun(script string) (string, error) {
	var log strings.Builder

	script = strings.TrimSpace(script)
	if script == "" {
		return "", fmt.Errorf("pakai: /sandbox <command>")
	}

	log.WriteString("**Sandbox Run**\n\n")
	log.WriteString(fmt.Sprintf("Distro : `%s` (Kali)\n", sandboxDistro))
	log.WriteString(fmt.Sprintf("Command: `%s`\n\n", script))
	log.WriteString("```\n")

	ctx, cancel := context.WithTimeout(context.Background(), sandboxTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "proot-distro", "login", sandboxDistro,
		"--", "bash", "-c", script)

	out, err := cmd.CombinedOutput()

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

	if err != nil {
		log.WriteString("\n⚠️  Exit code: non-zero\n")
		return log.String(), err
	}

	log.WriteString("\n✅ Selesai\n")
	return log.String(), nil
}

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const sandboxTimeout = 60 * time.Second

type sandboxPlan struct {
	bin             string
	args            []string
	description     string
	isolation       string
	dockerContainer string
}

func detectProotDistro() string {
	if !hasCommand("proot-distro") {
		return ""
	}

	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}

	rootfsDir := filepath.Join(prefix, "var", "lib", "proot-distro", "containers")
	priorities := []string{"alpine", "debian", "netra-sandbox", "kali", "ubuntu"}

	for _, distro := range priorities {
		if info, err := os.Stat(filepath.Join(rootfsDir, distro)); err == nil && info.IsDir() {
			return distro
		}
	}

	out, err := exec.Command("proot-distro", "list").Output()
	if err != nil {
		return ""
	}

	installed := strings.ToLower(string(out))
	for _, distro := range priorities {
		if strings.Contains(installed, distro) {
			return distro
		}
	}
	return ""
}

func isTermux() bool {
	return strings.Contains(strings.ToLower(os.Getenv("PREFIX")), "com.termux")
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func sandboxWorkspace() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("home directory tidak tersedia")
	}

	dir := filepath.Join(home, ".netra-ai", "sandbox")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("gagal membuat workspace: %w", err)
	}

	info, err := os.Lstat(dir)
	if err != nil {
		return "", fmt.Errorf("gagal memeriksa workspace: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("workspace harus berupa direktori biasa, bukan symlink")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return "", fmt.Errorf("gagal mengatur permission workspace: %w", err)
	}

	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("gagal resolve workspace: %w", err)
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("gagal menentukan path workspace: %w", err)
	}
	return dir, nil
}

func dockerReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		return false
	}

	ctxImage, cancelImage := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelImage()

	return exec.CommandContext(ctxImage, "docker", "image", "inspect", "alpine").Run() == nil
}

func bubblewrapPlan(script, workspace string) (sandboxPlan, error) {
	bin, err := exec.LookPath("bwrap")
	if err != nil {
		return sandboxPlan{}, err
	}

	required := []string{
		"/usr", "/etc", "/usr/bin", "/usr/sbin",
		"/usr/lib",
	}
	for _, path := range required {
		info, statErr := os.Stat(path)
		if statErr != nil || !info.IsDir() {
			return sandboxPlan{}, fmt.Errorf("path sistem untuk Bubblewrap tidak tersedia: %s", path)
		}
	}

	args := []string{
		"--unshare-all",
		"--die-with-parent",
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/etc", "/etc",
		"--symlink", "usr/bin", "/bin",
		"--symlink", "usr/sbin", "/sbin",
		"--symlink", "usr/lib", "/lib",
		"--symlink", "usr/lib64", "/lib64",
		"--dev", "/dev",
		"--proc", "/proc",
		"--tmpfs", "/tmp",
		"--bind", workspace, "/workspace",
		"--chdir", "/workspace",
		"--clearenv",
		"--setenv", "HOME", "/workspace",
		"--setenv", "PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"--setenv", "TERM", "xterm",
		"--setenv", "LANG", "C.UTF-8",
		"--",
		"/bin/sh", "-c", script,
	}

	return sandboxPlan{
		bin:         bin,
		args:        args,
		description: "Bubblewrap (Linux namespaces)",
		isolation:   "namespace + filesystem read-only + network terpisah",
	}, nil
}

func dockerPlan(script, workspace string) (sandboxPlan, error) {
	if !hasCommand("docker") || !dockerReady() {
		return sandboxPlan{}, fmt.Errorf("Docker daemon aktif dan image alpine diperlukan")
	}

	current, err := user.Current()
	if err != nil || current.Uid == "" || current.Gid == "" {
		return sandboxPlan{}, fmt.Errorf("identitas user tidak dapat ditentukan untuk Docker")
	}

	name := fmt.Sprintf("netra-sandbox-%d-%d", os.Getpid(), time.Now().UnixNano())

	args := []string{
		"run", "--rm",
		"--name", name,
		"--network", "none",
		"--read-only",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges:true",
		"--pids-limit", "128",
		"--memory", "512m",
		"--cpus", "1",
		"--tmpfs", "/tmp:rw,nosuid,nodev,size=64m",
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/workspace", workspace),
		"--workdir", "/workspace",
		"--user", current.Uid + ":" + current.Gid,
		"alpine", "sh", "-c", script,
	}

	return sandboxPlan{
		bin:             "docker",
		args:            args,
		description:     "Docker Alpine (restricted)",
		isolation:       "container + network none + read-only root + resource limits",
		dockerContainer: name,
	}, nil
}

func sandboxBackend(script string, allowWeak bool) (sandboxPlan, error) {
	workspace, err := sandboxWorkspace()
	if err != nil {
		return sandboxPlan{}, err
	}

	// Linux: Bubblewrap menjadi backend utama.
	if runtime.GOOS == "linux" && hasCommand("bwrap") {
		plan, bwrapErr := bubblewrapPlan(script, workspace)
		if bwrapErr == nil {
			return plan, nil
		}
	}

	// Docker hanya dipakai jika daemon aktif dan image alpine sudah tersedia.
	if (runtime.GOOS == "linux" || runtime.GOOS == "darwin") &&
		hasCommand("docker") && dockerReady() {
		return dockerPlan(script, workspace)
	}

	// Proot adalah isolasi userspace, bukan isolasi kernel.
	if isTermux() && allowWeak {
		if distro := detectProotDistro(); distro != "" {
			return sandboxPlan{
				bin:         "proot-distro",
				args:        []string{"login", distro, "--", "bash", "-c", script},
				description: "Proot-Distro: " + distro,
				isolation:   "userspace (bukan isolasi kernel)",
			}, nil
		}
	}

	// WSL memakai distro WSL yang sudah ada, bukan container khusus.
	if runtime.GOOS == "windows" && allowWeak && hasCommand("wsl") {
		return sandboxPlan{
			bin:         "wsl",
			args:        []string{"--exec", "bash", "-lc", script},
			description: "WSL (distro default)",
			isolation:   "distro WSL (bukan container khusus Netra)",
		}, nil
	}

	if allowWeak {
		return sandboxPlan{}, fmt.Errorf(
			"backend yang diizinkan tidak tersedia; Netra tidak akan menjalankan shell host sebagai fallback",
		)
	}

	return sandboxPlan{}, fmt.Errorf(
		"backend sandbox aman tidak tersedia; butuh Bubblewrap atau Docker aktif dengan image alpine",
	)
}

func cleanupSandbox(plan sandboxPlan) {
	if plan.dockerContainer == "" || !hasCommand("docker") {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", plan.dockerContainer).Run()
}

func sandboxIsLongListing(script string) bool {
	fields := strings.Fields(script)
	if len(fields) < 2 || len(fields) > 3 || fields[0] != "ls" {
		return false
	}
	flags := ""
	for _, field := range fields[1:] {
		if !strings.HasPrefix(field, "-") || strings.HasPrefix(field, "--") {
			return false
		}
		flags += strings.TrimLeft(field, "-")
	}
	hasA, hasL := false, false
	for _, flag := range flags {
		switch flag {
		case 'a':
			hasA = true
		case 'l':
			hasL = true
		default:
			return false
		}
	}
	return hasA && hasL
}

func sandboxListingMarkdown(output string) (string, bool) {
	var rows strings.Builder
	rows.WriteString("### Isi direktori\n\n")
	rows.WriteString("| Permission | Links | Owner | Group | Size | Modified | Name |\n")
	rows.WriteString("|---|---:|---|---|---:|---|---|\n")
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "total" {
			continue
		}
		if len(fields) < 9 {
			return "", false
		}
		cells := []string{fields[0], fields[1], fields[2], fields[3], fields[4], strings.Join(fields[5:8], " "), strings.Join(fields[8:], " ")}
		for i := range cells {
			cells[i] = strings.ReplaceAll(cells[i], "|", "¦")
		}
		rows.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		count++
	}
	if count == 0 {
		return "", false
	}
	return rows.String(), true
}

func cmdSandboxRun(script string) (string, error) {
	script = strings.TrimSpace(script)
	if script == "" {
		return "", fmt.Errorf("pakai: /sandbox [--allow-weak] <command>")
	}

	allowWeak := false
	fields := strings.Fields(script)
	if len(fields) > 0 && fields[0] == "--allow-weak" {
		allowWeak = true
		script = strings.TrimSpace(strings.TrimPrefix(script, fields[0]))
	}
	if script == "" {
		return "", fmt.Errorf("pakai: /sandbox --allow-weak <command>")
	}

	plan, err := sandboxBackend(script, allowWeak)
	if err != nil {
		return "", err
	}

	var log strings.Builder
	log.WriteString("**Sandbox Run**\n\n")
	log.WriteString(fmt.Sprintf("Backend : `%s`\n", plan.description))
	log.WriteString(fmt.Sprintf("Isolasi : `%s`\n", plan.isolation))
	log.WriteString(fmt.Sprintf("Command : `%s`\n\n", script))

	isListing := sandboxIsLongListing(script)

	ctx, cancel := context.WithTimeout(context.Background(), sandboxTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, plan.bin, plan.args...)
	out, runErr := cmd.CombinedOutput()
	output := string(out)

	if ctx.Err() == context.DeadlineExceeded {
		cleanupSandbox(plan)
		log.WriteString("```\n")
		log.WriteString(output)
		if output != "" && !strings.HasSuffix(output, "\n") {
			log.WriteString("\n")
		}
		log.WriteString("[TIMEOUT: lebih dari 60 detik]\n```\n")
		return log.String(), fmt.Errorf("sandbox timeout setelah %s", sandboxTimeout)
	}

	if output == "" {
		output = "(tidak ada output)\n"
	}

	renderedAsTable := false
	if isListing && runErr == nil {
		if table, ok := sandboxListingMarkdown(output); ok {
			log.WriteString(table)
			renderedAsTable = true
		}
	}

	if !renderedAsTable {
		log.WriteString("```\n")
		log.WriteString(output)
		if !strings.HasSuffix(output, "\n") {
			log.WriteString("\n")
		}
		log.WriteString("```\n")
	}

	if runErr != nil {
		log.WriteString("\nEksekusi gagal\n")
		return log.String(), runErr
	}

	log.WriteString("\nSelesai\n")
	return log.String(), nil
}

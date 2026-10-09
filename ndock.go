// ndock.go — NetraDock: Docker-like portable (pakai proot-distro backend)
package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ndockPath — lokasi containers proot-distro
func ndockPath() string {
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		prefix = "/data/data/com.termux/files/usr"
	}
	return filepath.Join(prefix, "var", "lib", "proot-distro", "containers")
}

// ndockProtect — container yang gak boleh diutak-atik
var ndockProtect = map[string]bool{
	"alpine":        true,
	"netra-sandbox": true,
}

func ndockImages() string {
	var log strings.Builder
	log.WriteString("**NetraDock Images**\n\n")
	entries, err := os.ReadDir(ndockPath())
	if err != nil {
		log.WriteString("(belum ada image)\n")
		return log.String()
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		marker := ""
		if ndockProtect[e.Name()] {
			marker = " _(system)_"
		}
		log.WriteString(fmt.Sprintf("- `%s`%s\n", e.Name(), marker))
		count++
	}
	if count == 0 {
		log.WriteString("(belum ada image)\n")
	}
	return log.String()
}

func ndockPull(name, url string) (string, error) {
	var log strings.Builder
	log.WriteString(fmt.Sprintf("**NetraDock Pull**: `%s`\n\n", name))
	log.WriteString(fmt.Sprintf("URL: `%s`\n\n", url))

	if ndockProtect[name] {
		return log.String(), fmt.Errorf("nama `%s` dilindungi, pakai nama lain", name)
	}

	containerDir := filepath.Join(ndockPath(), name)
	if _, err := os.Stat(containerDir); err == nil {
		os.RemoveAll(containerDir)
	}
	rootfsDir := filepath.Join(containerDir, "rootfs")
	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return log.String(), fmt.Errorf("mkdir: %w", err)
	}

	log.WriteString("Downloading...\n")
	tmpFile := filepath.Join(os.TempDir(), "ndock-"+name+".tar.gz")
	defer os.Remove(tmpFile)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return log.String(), fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return log.String(), fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(tmpFile)
	if err != nil {
		return log.String(), err
	}
	io.Copy(out, resp.Body)
	out.Close()

	log.WriteString("Extracting...\n")
	f, err := os.Open(tmpFile)
	if err != nil {
		return log.String(), err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return log.String(), fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return log.String(), fmt.Errorf("tar: %w", err)
		}
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue
		}
		target := filepath.Join(rootfsDir, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			out, err := os.Create(target)
			if err != nil {
				continue
			}
			io.Copy(out, tr)
			out.Close()
			os.Chmod(target, os.FileMode(hdr.Mode&0777))
			count++
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			os.Remove(target)
			os.Symlink(hdr.Linkname, target)
		}
	}

	// Bikin folder .l2s (marker proot-distro)
	os.MkdirAll(filepath.Join(rootfsDir, ".l2s"), 0755)

	// Copy manifest dari alpine (kalo ada) atau bikin minimal
	srcManifest := filepath.Join(ndockPath(), "alpine", "manifest.json")
	dstManifest := filepath.Join(containerDir, "manifest.json")
	if data, err := os.ReadFile(srcManifest); err == nil {
		os.WriteFile(dstManifest, data, 0644)
	} else {
		os.WriteFile(dstManifest, []byte(`{"image_ref":"`+name+`"}`), 0644)
	}

	log.WriteString(fmt.Sprintf("\nImage `%s` siap (%d file)\n", name, count))
	log.WriteString(fmt.Sprintf("Test: `ndock run %s \"uname -a\"`\n", name))
	return log.String(), nil
}

func ndockRun(name, cmdStr string) (string, error) {
	var log strings.Builder
	log.WriteString(fmt.Sprintf("**NetraDock Run**: `%s`\n", name))
	log.WriteString(fmt.Sprintf("Command: `%s`\n\n", cmdStr))

	containerDir := filepath.Join(ndockPath(), name)
	if _, err := os.Stat(containerDir); err != nil {
		return log.String(), fmt.Errorf("image `%s` gak ketemu. pull dulu.", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, "proot-distro", "login", name, "--", "bash", "-c", cmdStr)
	out, err := c.CombinedOutput()

	log.WriteString("```\n")
	log.WriteString(string(out))
	if !strings.HasSuffix(string(out), "\n") {
		log.WriteString("\n")
	}
	log.WriteString("```\n")

	if ctx.Err() == context.DeadlineExceeded {
		log.WriteString("\n_TIMEOUT >120s_\n")
		return log.String(), nil
	}
	if err != nil {
		log.WriteString(fmt.Sprintf("\n_Exit: %v_\n", err))
	}
	return log.String(), nil
}

func ndockRemove(name string) string {
	if ndockProtect[name] {
		return fmt.Sprintf("`%s` dilindungi, gak bisa dihapus.", name)
	}
	path := filepath.Join(ndockPath(), name)
	if _, err := os.Stat(path); err != nil {
		return fmt.Sprintf("Image `%s` gak ketemu.", name)
	}
	os.RemoveAll(path)
	return fmt.Sprintf("Image `%s` dihapus.", name)
}

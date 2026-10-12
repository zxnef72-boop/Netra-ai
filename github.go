// github.go — fetch file dari GitHub pakai raw URL.
// Gak butuh API key, gak ada rate limit ketat.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// tryGitHubFetch — deteksi intent ambil file GitHub
func tryGitHubFetch(input string) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(input))

	triggers := []string{
		"github ambil ", "github get ", "github fetch ",
		"github download ", "github tarik ",
	}
	matched := ""
	for _, t := range triggers {
		if strings.Contains(low, t) {
			matched = t
			break
		}
	}
	if matched == "" {
		return "", false
	}

	idx := strings.Index(low, matched)
	rest := strings.TrimSpace(input[idx+len(matched):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "Pakai: `github ambil <owner>/<repo>/<path>`\nContoh: `github ambil torvalds/linux/README`", true
	}

	spec := parts[0]
	segs := strings.SplitN(spec, "/", 4)
	if len(segs) < 3 {
		return "Format salah. Pakai: `owner/repo/path` atau `owner/repo/branch/path`", true
	}

	owner := segs[0]
	repo := segs[1]

	var branch, filePath string
	if len(segs) == 3 {
		// owner/repo/path -> coba branch default
		filePath = segs[2]
		branch = "" // auto-detect
	} else {
		// owner/repo/branch/path
		branch = segs[2]
		filePath = segs[3]
	}

	// Fetch dengan fallback branch
	content, usedBranch, err := fetchRawGitHub(owner, repo, branch, filePath)
	if err != nil {
		return fmt.Sprintf("Gagal ambil `%s/%s/%s`: %v", owner, repo, filePath, err), true
	}

	// Simpen ke /sdcard/AiNef/github/
	host := owner + "_" + repo
	var outDir string
	candidates := []string{"/sdcard/AiNef/github", "/storage/emulated/0/AiNef/github"}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Dir(c)); err == nil {
			outDir = c
			break
		}
	}
	if outDir == "" {
		home, _ := os.UserHomeDir()
		outDir = filepath.Join(home, "netra-ai", "downloads", "github")
	}
	os.MkdirAll(outDir, 0755)

	fname := strings.ReplaceAll(filePath, "/", "_")
	if fname == "" || fname == "." {
		fname = "file"
	}
	outFile := filepath.Join(outDir, host+"_"+fname)
	os.WriteFile(outFile, []byte(content), 0644)

	// Preview
	var b strings.Builder
	b.WriteString(fmt.Sprintf("**GitHub: `%s/%s/%s`**\n\n", owner, repo, filePath))
	b.WriteString(fmt.Sprintf("_Branch: `%s` | %d byte_\n\n", usedBranch, len(content)))

	// Deteksi bahasa buat code block
	lang := detectLang(filePath)
	preview := content
	truncated := false
	maxLen := 5000
	if len(preview) > maxLen {
		preview = preview[:maxLen]
		truncated = true
	}

	b.WriteString("```" + lang + "\n")
	b.WriteString(preview)
	if truncated {
		b.WriteString(fmt.Sprintf("\n\n... (dipotong, %d byte lagi)", len(content)-maxLen))
	}
	b.WriteString("\n```\n\n")
	b.WriteString(fmt.Sprintf("_Full: `%s`_", outFile))

	return b.String(), true
}

// fetchRawGitHub — coba ambil dari raw.githubusercontent.com
// branch kosong = auto-detect (coba main, master)
func fetchRawGitHub(owner, repo, branch, path string) (string, string, error) {
	var branches []string
	if branch != "" {
		branches = []string{branch}
	} else {
		branches = []string{"main", "master"}
	}

	var lastErr error
	for _, br := range branches {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
			owner, repo, br, path)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", metaUA)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 200 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
			resp.Body.Close()
			return string(body), br, nil
		}
		resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d di branch %s", resp.StatusCode, br)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("gak nemu branch/file")
	}
	return "", "", lastErr
}

// detectLang — tebak bahasa dari ekstensi file
func detectLang(path string) string {
	low := strings.ToLower(path)
	switch {
	case strings.HasSuffix(low, ".go"):
		return "go"
	case strings.HasSuffix(low, ".py"):
		return "python"
	case strings.HasSuffix(low, ".js"):
		return "javascript"
	case strings.HasSuffix(low, ".ts"):
		return "typescript"
	case strings.HasSuffix(low, ".lua"):
		return "lua"
	case strings.HasSuffix(low, ".md"):
		return "markdown"
	case strings.HasSuffix(low, ".json"):
		return "json"
	case strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml"):
		return "yaml"
	case strings.HasSuffix(low, ".html"):
		return "html"
	case strings.HasSuffix(low, ".css"):
		return "css"
	case strings.HasSuffix(low, ".sh"):
		return "bash"
	case strings.HasSuffix(low, ".c"):
		return "c"
	case strings.HasSuffix(low, ".cpp") || strings.HasSuffix(low, ".cc"):
		return "cpp"
	case strings.HasSuffix(low, ".rs"):
		return "rust"
	case strings.HasSuffix(low, ".java"):
		return "java"
	case strings.HasSuffix(low, ".sql"):
		return "sql"
	}
	return ""
}

// tryGitHubSearch — cari repo di GitHub berdasarkan keyword
func tryGitHubSearch(input string) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(input))

	triggers := []string{
		"github cari ", "cari github ", "cari repo ",
		"cari script ", "cari project ", "cari tools ",
	}
	matched := ""
	for _, t := range triggers {
		if strings.Contains(low, t) {
			matched = t
			break
		}
	}
	if matched == "" {
		return "", false
	}

	idx := strings.Index(low, matched)
	query := strings.TrimSpace(input[idx+len(matched):])
	if query == "" {
		return "Pakai: `github cari <keyword>`\nContoh: `github cari python scanner`", true
	}

	// Panggil Search API
	apiURL := "https://api.github.com/search/repositories?q=" +
		url.QueryEscape(query) + "&sort=stars&per_page=5"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "URL gak valid", true
	}
	req.Header.Set("User-Agent", metaUA)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("GitHub search gagal: %v", err), true
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Sprintf("GitHub search HTTP %d (mungkin rate limit)", resp.StatusCode), true
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))

	var data struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
			Language    string `json:"language"`
			HTMLURL     string `json:"html_url"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Sprintf("Parse JSON gagal: %v", err), true
	}

	if len(data.Items) == 0 {
		return fmt.Sprintf("Gak nemu repo buat `%s`", query), true
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("**GitHub Search: `%s`**\n", query))
	b.WriteString(fmt.Sprintf("_Total: %d repo, tampil 5 teratas (by stars)_\n\n", data.TotalCount))

	for i, item := range data.Items {
		desc := item.Description
		if desc == "" {
			desc = "(gak ada deskripsi)"
		}
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		b.WriteString(fmt.Sprintf("**%d. %s** ⭐ %d\n", i+1, item.FullName, item.Stars))
		b.WriteString(fmt.Sprintf("  Owner: `%s`", item.Owner.Login))
		if item.Language != "" {
			b.WriteString(fmt.Sprintf(" | Bahasa: %s", item.Language))
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("  %s\n", desc))
		b.WriteString(fmt.Sprintf("  %s\n\n", item.HTMLURL))
	}

	// Auto fetch README dari repo teratas
	top := data.Items[0]
	readmeContent, _, err := fetchRawGitHub(top.Owner.Login, repoShortName(top.FullName), "", "README.md")
	if err != nil {
		readmeContent, _, err = fetchRawGitHub(top.Owner.Login, repoShortName(top.FullName), "", "README")
	}
	if err == nil && readmeContent != "" {
		// Simpen README full ke file
		var outDir string
		for _, c := range []string{"/sdcard/AiNef/github", "/storage/emulated/0/AiNef/github"} {
			if _, err := os.Stat(filepath.Dir(c)); err == nil {
				outDir = c
				break
			}
		}
		if outDir == "" {
			home, _ := os.UserHomeDir()
			outDir = filepath.Join(home, "netra-ai", "downloads", "github")
		}
		os.MkdirAll(outDir, 0755)
		readmeFile := filepath.Join(outDir, top.Owner.Login+"_"+repoShortName(top.FullName)+"_README.md")
		os.WriteFile(readmeFile, []byte(readmeContent), 0644)

		// Preview 5000 char
		preview := readmeContent
		truncated := false
		if len(preview) > 5000 {
			preview = preview[:5000]
			truncated = true
		}
		b.WriteString(fmt.Sprintf("---\n\n**README `%s`:**\n\n", top.FullName))
		b.WriteString(preview)
		if truncated {
			b.WriteString(fmt.Sprintf("\n\n... (dipotong, %d byte lagi)", len(readmeContent)-5000))
		}
		b.WriteString(fmt.Sprintf("\n\n_Full README: `%s`_", readmeFile))
	}

	// Struktur folder repo #1
	tree, err := fetchRepoTree(top.Owner.Login, repoShortName(top.FullName))
	if err == nil && len(tree) > 0 {
		b.WriteString(fmt.Sprintf("\n\n---\n\n**Struktur `%s`:**\n\n", top.FullName))
		b.WriteString("```\n")
		var dirs, files []repoItem
		for _, item := range tree {
			if item.Type == "dir" {
				dirs = append(dirs, item)
			} else if item.Type == "file" {
				files = append(files, item)
			}
		}
		maxShow := 40
		shown := 0
		for _, d := range dirs {
			if shown >= maxShow {
				break
			}
			b.WriteString(fmt.Sprintf("%s/\n", d.Name))
			shown++
		}
		for _, f := range files {
			if shown >= maxShow {
				break
			}
			b.WriteString(fmt.Sprintf("%s  (%s)\n", f.Name, humanSize(int64(f.Size))))
			shown++
		}
		if len(tree) > maxShow {
			b.WriteString(fmt.Sprintf("\n... (%d item lagi)\n", len(tree)-maxShow))
		}
		b.WriteString("```\n")
		b.WriteString(fmt.Sprintf("\n_Total %d item di root_", len(tree)))
	}

	return b.String(), true
}

// repoShortName — "owner/repo" -> "repo"
func repoShortName(fullName string) string {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fullName
}

// fetchRepoTree — ambil list file/folder di root repo
func fetchRepoTree(owner, repo string) ([]repoItem, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/", owner, repo)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", metaUA)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	var items []repoItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	return items, nil
}

type repoItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
	Size int    `json:"size"`
}

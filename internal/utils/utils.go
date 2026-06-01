package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// PythonVersion represents a single Python release.
type PythonVersion struct {
	Version   string
	Filename  string
	URL       string
	Installed bool
	Active    bool
	Path      string
}

// FindVersionFile looks for a .python-version file in the current directory or parent directories.
func FindVersionFile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(dir, ".python-version")
		if _, err := os.Stat(path); err == nil {
			content, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(content)), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf(".python-version not found")
}

func SetupShimDirectory() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}
	for _, dir := range []string{
		filepath.Join(homeDir, ".pyvm"),
		filepath.Join(homeDir, ".pyvm", "shim"),
		filepath.Join(homeDir, ".pyvm", "versions"),
		filepath.Join(homeDir, ".pyvm", "downloads"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %v", dir, err)
		}
	}
	return nil
}

// IsShimInPath reports whether ~/.pyvm/shim is present in $PATH.
func IsShimInPath() bool {
	homeDir, _ := os.UserHomeDir()
	shimDir := filepath.Join(homeDir, ".pyvm", "shim")
	for _, entry := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if entry == shimDir {
			return true
		}
	}
	return false
}

// GetShimPathInstructions returns a human-readable line the user can paste.
func GetShimPathInstructions() string {
	if runtime.GOOS == "windows" {
		return `Add to PATH: %USERPROFILE%\.pyvm\shim`
	}
	return `Add to your shell config: export PATH="$HOME/.pyvm/shim:$PATH"`
}

// ─── Platform mapping ───────────────────────────────────────────────────────

// goarchToStandalone maps Go's GOARCH to python-build-standalone arch strings.
func goarchToStandalone() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "i686"
	default:
		return runtime.GOARCH
	}
}

// goosToStandalone maps Go's GOOS to python-build-standalone OS strings.
func goosToStandalone() string {
	switch runtime.GOOS {
	case "linux":
		return "unknown-linux-gnu"
	case "darwin":
		return "apple-darwin"
	case "windows":
		return "pc-windows-msvc"
	default:
		return runtime.GOOS
	}
}

// ─── Fetch ──────────────────────────────────────────────────────────────────

// FetchPythonVersions is a bubbletea command that hits the GitHub releases API
// for indygreg/python-build-standalone and returns a VersionsMsg.
//
// python-build-standalone ships pre-compiled Python binaries for every
// platform — this is the same source used by uv, rye, and pyenv-win.
// Assets follow the naming convention:
//
//	cpython-{VER}+{DATE}-{ARCH}-{OS}-install_only.tar.gz
func FetchPythonVersions() tea.Msg {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET",
		"https://api.github.com/repos/indygreg/python-build-standalone/releases?per_page=10",
		nil)
	if err != nil {
		return ErrMsg(err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "pyvm/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return ErrMsg(fmt.Errorf("failed to connect to GitHub API: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return ErrMsg(fmt.Errorf("GitHub API rate limit exceeded — try again in an hour"))
	}
	if resp.StatusCode != 200 {
		return ErrMsg(fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrMsg(err)
	}

	var releases []struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return ErrMsg(fmt.Errorf("failed to parse GitHub API response: %v", err))
	}

	targetArch := goarchToStandalone()
	targetOS := goosToStandalone()

	// ── Discover installed versions ──────────────────────────────────────────
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ErrMsg(err)
	}
	versionsDir := filepath.Join(homeDir, ".pyvm", "versions")
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return ErrMsg(err)
	}

	// Active version: prefer our own file, fall back to whatever `python3` reports.
	activeVersion := ""
	activeVersionFile := filepath.Join(homeDir, ".pyvm", "active_version")
	if b, err := os.ReadFile(activeVersionFile); err == nil {
		activeVersion = strings.TrimSpace(string(b))
	} else {
		activeVersion = GetCurrentPythonVersion()
	}

	// Map version → install path for every directory that has a python binary.
	installedVersions := map[string]string{}
	if entries, err := os.ReadDir(versionsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			versionPath := filepath.Join(versionsDir, e.Name())
			pythonBin := filepath.Join(versionPath, "bin", "python3")
			if runtime.GOOS == "windows" {
				pythonBin = filepath.Join(versionPath, "python.exe")
			}
			if _, err := os.Stat(pythonBin); err == nil {
				installedVersions[e.Name()] = versionPath
			}
		}
	}

	// ── Parse assets from all fetched releases ───────────────────────────────
	// We deduplicate by Python version, keeping only the first occurrence
	// (which comes from the most recent release date).
	versionMap := map[string]PythonVersion{}

	for _, release := range releases {
		for _, asset := range release.Assets {
			name := asset.Name

			// Must be an install_only archive (not a .sha256, not a debug build)
			if !strings.Contains(name, "install_only") ||
				strings.HasSuffix(name, ".sha256") ||
				strings.Contains(name, "debug") {
				continue
			}

			// Filter to current platform
			if !strings.Contains(name, targetArch) || !strings.Contains(name, targetOS) {
				continue
			}

			pyVersion := extractPythonVersion(name)
			if pyVersion == "" {
				continue
			}

			// Only keep the most recent release date's entry per Python version
			if _, exists := versionMap[pyVersion]; exists {
				continue
			}

			v := PythonVersion{
				Version:  pyVersion,
				Filename: name,
				URL:      asset.BrowserDownloadURL,
			}
			if path, ok := installedVersions[pyVersion]; ok {
				v.Installed = true
				v.Path = path
			}
			if activeVersion == pyVersion {
				v.Active = true
			}
			versionMap[pyVersion] = v
		}
	}

	// Convert to sorted slice (newest first)
	var versions []PythonVersion
	for _, v := range versionMap {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) > 0
	})

	return VersionsMsg(versions)
}

// extractPythonVersion pulls the Python version out of an asset filename.
// "cpython-3.12.7+20241016-x86_64-..." → "3.12.7"
func extractPythonVersion(filename string) string {
	s := strings.TrimPrefix(filename, "cpython-")
	plusIdx := strings.Index(s, "+")
	if plusIdx == -1 {
		return ""
	}
	return s[:plusIdx]
}

// GetCurrentPythonVersion runs python3 --version and returns the version string.
func GetCurrentPythonVersion() string {
	for _, bin := range []string{"python3", "python"} {
		out, err := exec.Command(bin, "--version").Output()
		if err != nil {
			continue
		}
		parts := strings.Fields(string(out))
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	return ""
}

// ─── Install ────────────────────────────────────────────────────────────────

// DownloadAndInstall returns a tea.Cmd that downloads and extracts a Python
// version from python-build-standalone into ~/.pyvm/versions/{version}/.
func DownloadAndInstall(version PythonVersion) tea.Cmd {
	return func() tea.Msg {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ErrMsg(err)
		}

		versionsDir := filepath.Join(homeDir, ".pyvm", "versions")
		downloadsDir := filepath.Join(homeDir, ".pyvm", "downloads")
		for _, dir := range []string{versionsDir, downloadsDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return ErrMsg(err)
			}
		}

		versionDir := filepath.Join(versionsDir, version.Version)
		if _, err := os.Stat(versionDir); err == nil {
			if err := os.RemoveAll(versionDir); err != nil {
				return ErrMsg(fmt.Errorf("failed to remove existing installation: %v", err))
			}
		}

		downloadPath := filepath.Join(downloadsDir, version.Filename)
		os.Remove(downloadPath)

		// ── Download ─────────────────────────────────────────────────────────
		resp, err := http.Get(version.URL)
		if err != nil {
			return ErrMsg(fmt.Errorf("download failed: %v", err))
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return ErrMsg(fmt.Errorf("download failed: HTTP %d", resp.StatusCode))
		}

		out, err := os.Create(downloadPath)
		if err != nil {
			return ErrMsg(err)
		}
		written, copyErr := io.Copy(out, resp.Body)
		out.Close()
		if copyErr != nil {
			return ErrMsg(fmt.Errorf("download error: %v", copyErr))
		}
		if written == 0 {
			return ErrMsg(fmt.Errorf("downloaded empty file"))
		}

		// ── Extract ──────────────────────────────────────────────────────────
		// python-build-standalone archives always extract to a "python/" dir.
		tempDir := filepath.Join(downloadsDir, "tmp_"+version.Version)
		os.RemoveAll(tempDir)
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return ErrMsg(err)
		}
		defer os.RemoveAll(tempDir)

		var extractCmd *exec.Cmd
		if runtime.GOOS == "windows" {
			extractCmd = exec.Command("powershell", "-Command",
				fmt.Sprintf(`Expand-Archive -Path "%s" -DestinationPath "%s" -Force`,
					downloadPath, tempDir))
		} else {
			extractCmd = exec.Command("tar", "-xzf", downloadPath, "-C", tempDir)
		}
		if output, err := extractCmd.CombinedOutput(); err != nil {
			return ErrMsg(fmt.Errorf("extraction failed: %v\n%s", err, output))
		}

		// The archive always extracts to tempDir/python/
		extractedPython := filepath.Join(tempDir, "python")
		if _, err := os.Stat(extractedPython); os.IsNotExist(err) {
			return ErrMsg(fmt.Errorf(
				"unexpected archive structure: python/ directory not found after extraction"))
		}

		// Move python/ → ~/.pyvm/versions/{version}/
		if err := os.Rename(extractedPython, versionDir); err != nil {
			// Cross-device rename (e.g. different filesystem) → fallback to copy
			if err := copyDir(extractedPython, versionDir); err != nil {
				return ErrMsg(fmt.Errorf("failed to move installation: %v", err))
			}
		}

		// Fix executable bits on Unix
		if runtime.GOOS != "windows" {
			binDir := filepath.Join(versionDir, "bin")
			if entries, err := os.ReadDir(binDir); err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						if err := os.Chmod(filepath.Join(binDir, e.Name()), 0755); err != nil {
							return ErrMsg(fmt.Errorf("failed to chmod: %v", err))
						}
					}
				}
			}
		}

		// ── Verify ───────────────────────────────────────────────────────────
		pythonBin := filepath.Join(versionDir, "bin", "python3")
		if runtime.GOOS == "windows" {
			pythonBin = filepath.Join(versionDir, "python.exe")
		}
		if _, err := os.Stat(pythonBin); os.IsNotExist(err) {
			return ErrMsg(fmt.Errorf("installation verification failed: python binary not found at %s", pythonBin))
		}
		if verifyOut, err := exec.Command(pythonBin, "--version").CombinedOutput(); err != nil {
			return ErrMsg(fmt.Errorf("python binary verification failed: %v\n%s", err, verifyOut))
		}

		os.Remove(downloadPath)
		return DownloadCompleteMsg{Version: version.Version, Path: versionDir}
	}
}

// ─── Switch ─────────────────────────────────────────────────────────────────

// SwitchVersion writes shell-script shims into ~/.pyvm/shim/ for every binary
// in the chosen version's bin/ directory, then updates active_version.
//
// Shim approach (identical to govm): each shim is a tiny bash script that
// hardcodes the full path to the real binary for the selected version.
// This is simpler than runtime resolution and perfect for a single-user tool.
func SwitchVersion(version PythonVersion) tea.Cmd {
	return func() tea.Msg {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ErrMsg(err)
		}
		if err := SetupShimDirectory(); err != nil {
			return ErrMsg(err)
		}

		shimDir := filepath.Join(homeDir, ".pyvm", "shim")

		// On Windows, python.exe lives in the version root, not a bin/ sub-dir.
		binDir := filepath.Join(version.Path, "bin")
		if runtime.GOOS == "windows" {
			binDir = version.Path
		}

		if _, err := os.Stat(binDir); os.IsNotExist(err) {
			return ErrMsg(fmt.Errorf("bin directory not found: %s", binDir))
		}

		entries, err := os.ReadDir(binDir)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to read bin directory: %v", err))
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			binName := entry.Name()
			targetBin := filepath.Join(binDir, binName)

			if runtime.GOOS == "windows" {
				// Strip .exe from the binary name before adding .bat
				name := strings.TrimSuffix(binName, ".exe")
				shimPath := filepath.Join(shimDir, name+".bat")
				os.Remove(shimPath)

				content := fmt.Sprintf("@echo off\n\"%s\" %%*\n", targetBin)
				if err := os.WriteFile(shimPath, []byte(content), 0755); err != nil {
					return ErrMsg(fmt.Errorf("failed to write shim for %s: %v", name, err))
				}
			} else {
				shimPath := filepath.Join(shimDir, binName)
				os.Remove(shimPath)

				content := fmt.Sprintf("#!/usr/bin/env bash\n\"%s\" \"$@\"\n", targetBin)
				if err := os.WriteFile(shimPath, []byte(content), 0755); err != nil {
					return ErrMsg(fmt.Errorf("failed to write shim for %s: %v", binName, err))
				}
				if err := os.Chmod(shimPath, 0755); err != nil {
					return ErrMsg(fmt.Errorf("failed to chmod shim: %v", err))
				}
			}
		}

		// python-build-standalone ships python3 but not python.
		// Create a "python" shim pointing to python3 for convenience.
		if runtime.GOOS != "windows" {
			pythonShim := filepath.Join(shimDir, "python")
			if _, err := os.Stat(pythonShim); os.IsNotExist(err) {
				python3Bin := filepath.Join(binDir, "python3")
				if _, err := os.Stat(python3Bin); err == nil {
					content := fmt.Sprintf("#!/usr/bin/env bash\n\"%s\" \"$@\"\n", python3Bin)
					if err := os.WriteFile(pythonShim, []byte(content), 0755); err != nil {
						return ErrMsg(fmt.Errorf("failed to write python shim: %v", err))
					}
					if err := os.Chmod(pythonShim, 0755); err != nil {
						return ErrMsg(fmt.Errorf("failed to chmod python shim: %v", err))
					}
				}
			}
		}

		activeVersionFile := filepath.Join(homeDir, ".pyvm", "active_version")
		if err := os.WriteFile(activeVersionFile, []byte(version.Version), 0644); err != nil {
			return ErrMsg(fmt.Errorf("failed to write active_version: %v", err))
		}

		return SwitchCompletedMsg{Version: version.Version, ShimInPath: IsShimInPath()}
	}
}

// ─── Delete ─────────────────────────────────────────────────────────────────

// DeleteVersion removes an installed version from disk.
func DeleteVersion(version PythonVersion) tea.Cmd {
	return func() tea.Msg {
		if !version.Installed {
			return ErrMsg(fmt.Errorf("version %s is not installed", version.Version))
		}
		if version.Active {
			return ErrMsg(fmt.Errorf("cannot delete the active version — switch to another version first"))
		}
		if err := os.RemoveAll(version.Path); err != nil {
			return ErrMsg(fmt.Errorf("failed to delete %s: %v", version.Version, err))
		}
		return DeleteCompleteMsg{Version: version.Version}
	}
}

// ─── Version comparison ──────────────────────────────────────────────────────

func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}
	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			p2, _ = strconv.Atoi(parts2[i])
		}
		if p1 != p2 {
			if p1 < p2 {
				return -1
			}
			return 1
		}
	}
	return 0
}

// ─── File helpers ────────────────────────────────────────────────────────────

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

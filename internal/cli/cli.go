package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/user/pyvm/internal/utils"
)

// InstallVersion installs a Python version by name or prefix.
// e.g. "3.12" installs the latest 3.12.x available.
func InstallVersion(version string) {
	fmt.Printf("🔍 Looking for Python version matching %s…\n", version)

	matched, err := findMatchingVersion(version)
	if err != nil {
		fmt.Printf("❌ %s\n", err)
		return
	}

	fmt.Printf("📥 Installing Python %s…\n", matched.Version)

	done := make(chan bool, 1)
	errCh := make(chan error, 1)

	go func() {
		msg := utils.DownloadAndInstall(matched)()
		switch msg := msg.(type) {
		case utils.ErrMsg:
			errCh <- msg
		case utils.DownloadCompleteMsg:
			done <- true
		default:
			errCh <- fmt.Errorf("unexpected message type")
		}
	}()

	spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinIdx := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			fmt.Printf("\r✅ Installed Python %s\n", matched.Version)
			fmt.Printf("👉 To activate, run: pyvm use %s\n", matched.Version)
			return
		case err := <-errCh:
			fmt.Printf("\r❌ Installation failed: %v\n", err)
			return
		case <-ticker.C:
			fmt.Printf("\r%s Installing Python %s…", spinChars[spinIdx], matched.Version)
			spinIdx = (spinIdx + 1) % len(spinChars)
		}
	}
}

// UseVersion switches the active Python version.
func UseVersion(version string) {
	if version == "" {
		detected, err := utils.FindVersionFile()
		if err == nil {
			fmt.Printf("📂 Found .python-version: %s\n", detected)
			version = detected
		} else {
			fmt.Println("❌ No version provided and no .python-version file found.")
			fmt.Println("Usage: pyvm use <version>")
			return
		}
	}

	fmt.Printf("🔍 Looking for installed Python version matching %s…\n", version)

	matched, err := findInstalledVersion(version)
	if err != nil {
		fmt.Printf("❌ %s\n", err)
		return
	}

	fmt.Printf("🔄 Switching to Python %s…\n", matched.Version)
	msg := utils.SwitchVersion(matched)()

	switch msg := msg.(type) {
	case utils.ErrMsg:
		fmt.Printf("❌ Failed to switch: %v\n", msg)
	case utils.SwitchCompletedMsg:
		fmt.Printf("✅ Switched to Python %s\n", matched.Version)
		if !utils.IsShimInPath() {
			fmt.Println("\n⚠️  PyVM shim is not in your PATH")
			fmt.Println(utils.GetShimPathInstructions())
		} else {
			fmt.Println("🚀 Run 'python3 --version' in a new terminal to verify")
		}
	}
}

// ListVersions prints all installed Python versions.
func ListVersions() {
	fmt.Println("📋 Installed Python Versions:")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}

	activeVersion := ""
	if b, err := os.ReadFile(filepath.Join(homeDir, ".pyvm", "active_version")); err == nil {
		activeVersion = strings.TrimSpace(string(b))
	}

	versionsDir := filepath.Join(homeDir, ".pyvm", "versions")
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		fmt.Println("  No versions installed yet.")
		return
	}

	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}

	found := false
	for _, entry := range entries {
		if entry.IsDir() {
			found = true
			v := entry.Name()
			if v == activeVersion {
				fmt.Printf("  %s  ✓ (active)\n", v)
			} else {
				fmt.Printf("  %s\n", v)
			}
		}
	}
	if !found {
		fmt.Println("  No versions installed yet.")
	}
	fmt.Println("\nTo install a version: pyvm install <version>")
	fmt.Println("To switch versions:   pyvm use <version>")
}

// DeleteVersion removes an installed version from disk.
func DeleteVersion(version string) {
	fmt.Printf("🔍 Looking for installed Python version matching %s…\n", version)

	matched, err := findInstalledVersion(version)
	if err != nil {
		fmt.Printf("❌ %s\n", err)
		return
	}

	homeDir, _ := os.UserHomeDir()
	activeVersion := ""
	if b, err := os.ReadFile(filepath.Join(homeDir, ".pyvm", "active_version")); err == nil {
		activeVersion = strings.TrimSpace(string(b))
	}

	if matched.Version == activeVersion {
		fmt.Println("❌ Cannot delete the active version. Switch to another version first.")
		return
	}

	fmt.Printf("⚠️  Delete Python %s? (y/N): ", matched.Version)
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		fmt.Println("🛑 Error reading input.")
		return
	}
	if strings.ToLower(response) != "y" {
		fmt.Println("🛑 Cancelled.")
		return
	}

	msg := utils.DeleteVersion(matched)()
	switch msg := msg.(type) {
	case utils.ErrMsg:
		fmt.Printf("❌ %v\n", msg)
	case utils.DeleteCompleteMsg:
		fmt.Printf("✅ Deleted Python %s\n", msg.Version)
	}
}

// ─── Internal helpers ────────────────────────────────────────────────────────

// findMatchingVersion fetches the remote version list and finds the best match
// for the given version prefix (e.g. "3.12" → "3.12.7").
func findMatchingVersion(version string) (utils.PythonVersion, error) {
	msg := utils.FetchPythonVersions()
	versions, ok := msg.(utils.VersionsMsg)
	if !ok {
		if errMsg, isErr := msg.(utils.ErrMsg); isErr {
			return utils.PythonVersion{}, fmt.Errorf("failed to fetch versions: %v", errMsg)
		}
		return utils.PythonVersion{}, fmt.Errorf("failed to fetch versions")
	}

	// Exact match first
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	// Prefix match — pick the highest patch
	prefix := version + "."
	var best utils.PythonVersion
	found := false
	for _, v := range versions {
		if strings.HasPrefix(v.Version, prefix) {
			if !found || compareVersions(v.Version, best.Version) > 0 {
				best = v
				found = true
			}
		}
	}
	if found {
		return best, nil
	}
	return utils.PythonVersion{}, fmt.Errorf("no version matching '%s' found", version)
}

// findInstalledVersion scans ~/.pyvm/versions/ for the best match.
func findInstalledVersion(version string) (utils.PythonVersion, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return utils.PythonVersion{}, fmt.Errorf("failed to get home directory: %v", err)
	}
	versionsDir := filepath.Join(homeDir, ".pyvm", "versions")

	// Exact match
	exactPath := filepath.Join(versionsDir, version)
	if _, err := os.Stat(exactPath); err == nil {
		return utils.PythonVersion{
			Version:   version,
			Path:      exactPath,
			Installed: true,
		}, nil
	}

	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return utils.PythonVersion{}, fmt.Errorf("failed to read versions directory: %v", err)
	}

	// Prefix match — pick the highest patch
	prefix := version + "."
	var best utils.PythonVersion
	found := false
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			versionPath := filepath.Join(versionsDir, entry.Name())
			if !found || compareVersions(entry.Name(), best.Version) > 0 {
				best = utils.PythonVersion{
					Version:   entry.Name(),
					Path:      versionPath,
					Installed: true,
				}
				found = true
			}
		}
	}
	if found {
		return best, nil
	}
	return utils.PythonVersion{}, fmt.Errorf("no installed version matching '%s' found", version)
}

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

// SetColor validates a hex string, saves it to config, and prints confirmation.
func SetColor(hex string) {
	if !strings.HasPrefix(hex, "#") {
		hex = "#" + hex
	}
	if !utils.IsValidHex(hex) {
		fmt.Printf("❌ Invalid hex color: %s\n", hex)
		fmt.Println("   Format: #RRGGBB or #RGB  (e.g. pyvm color '#4B8BBE')")
		return
	}
	cfg := utils.LoadConfig()
	cfg.AccentColor = hex
	if err := utils.SaveConfig(cfg); err != nil {
		fmt.Printf("❌ Failed to save config: %v\n", err)
		return
	}
	fmt.Printf("✅ Accent color set to %s\n", hex)
	fmt.Println("   Restart the TUI to see the change.")
}

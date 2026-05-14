package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

// Config holds all persistent pyvm settings.
type Config struct {
	AccentColor string `json:"accent_color"`
}

var defaultConfig = Config{
	AccentColor: "#F7C948",
}

func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".pyvm", "config.json"), nil
}

// LoadConfig reads ~/.pyvm/config.json. Missing file → returns defaults.
func LoadConfig() Config {
	path, err := configPath()
	if err != nil {
		return defaultConfig
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultConfig
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig
	}
	if cfg.AccentColor == "" {
		cfg.AccentColor = defaultConfig.AccentColor
	}
	return cfg
}

// SaveConfig writes cfg to ~/.pyvm/config.json (creates dir if needed).
func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// IsValidHex checks that a string looks like a CSS hex color (#RGB or #RRGGBB).
func IsValidHex(s string) bool {
	matched, _ := regexp.MatchString(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$`, s)
	return matched
}

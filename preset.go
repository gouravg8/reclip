package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// creditPreset is persisted so the user adds their 10s credit video once.
type creditPreset struct {
	CreditPath string `json:"creditPath"`
}

// configDir returns the Reclip config dir. RECLIP_CONFIG_DIR overrides it
// (used by tests to avoid touching the real user config).
func configDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("RECLIP_CONFIG_DIR")); v != "" {
		return v, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "reclip"), nil
}

func presetPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "preset.json"), nil
}

// SaveCreditPreset stores the credit video path as the default.
func (a *App) SaveCreditPreset(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("empty credit path")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("credit file not found: %w", err)
	}
	p, err := presetPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creditPreset{CreditPath: path}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// GetCreditPreset returns the saved credit path, or "" if none/invalid.
func (a *App) GetCreditPreset() string {
	p, err := presetPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	var preset creditPreset
	if err := json.Unmarshal(data, &preset); err != nil {
		return ""
	}
	preset.CreditPath = strings.TrimSpace(preset.CreditPath)
	if preset.CreditPath == "" {
		return ""
	}
	if _, err := os.Stat(preset.CreditPath); err != nil {
		return ""
	}
	return preset.CreditPath
}

// ClearCreditPreset removes the saved default.
func (a *App) ClearCreditPreset() error {
	p, err := presetPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

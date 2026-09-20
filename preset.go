package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// appConfig is persisted so the user sets things once: default credit clip
// plus optional explicit paths to the external engines. Older config files
// holding only {creditPath} still load fine.
type appConfig struct {
	CreditPath string            `json:"creditPath"`
	Engines    map[string]string `json:"engines,omitempty"`
	// Cookies selects Instagram auth for downloads:
	// "" (none), "browser:chrome", or "file:/path/to/cookies.txt".
	Cookies string `json:"cookies,omitempty"`
	// DownloadDelaySec pauses this long between downloads (0 = off).
	DownloadDelaySec int `json:"downloadDelaySec,omitempty"`
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

// loadConfig reads the config file, tolerating missing files and the
// legacy {creditPath}-only shape.
func loadConfig() appConfig {
	cfg := appConfig{Engines: map[string]string{}}
	p, err := presetPath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return appConfig{Engines: map[string]string{}}
	}
	if cfg.Engines == nil {
		cfg.Engines = map[string]string{}
	}
	return cfg
}

func saveConfig(cfg appConfig) error {
	p, err := presetPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if cfg.Engines == nil {
		cfg.Engines = map[string]string{}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// configEngine returns a user-saved engine path ("ffmpeg" | "ytdlp").
func configEngine(tool string) string {
	return strings.TrimSpace(loadConfig().Engines[tool])
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
	cfg := loadConfig()
	cfg.CreditPath = path
	return saveConfig(cfg)
}

// GetCreditPreset returns the saved credit path, or "" if none/invalid.
func (a *App) GetCreditPreset() string {
	credit := strings.TrimSpace(loadConfig().CreditPath)
	if credit == "" {
		return ""
	}
	if _, err := os.Stat(credit); err != nil {
		return ""
	}
	return credit
}

// cookieBrowsers are the --cookies-from-browser choices offered in Setup.
var cookieBrowsers = []string{"chrome", "chromium", "firefox", "edge", "brave"}

// GetCookieSetting returns the saved Instagram auth ("", "browser:x", "file:...").
func (a *App) GetCookieSetting() string {
	return strings.TrimSpace(loadConfig().Cookies)
}

// SetCookieSetting saves Instagram auth. Accepts "" (none),
// "browser:<chrome|chromium|firefox|edge|brave>", or "file:<cookies.txt path>".
func (a *App) SetCookieSetting(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		if rest, ok := strings.CutPrefix(value, "browser:"); ok {
			known := false
			for _, b := range cookieBrowsers {
				if rest == b {
					known = true
					break
				}
			}
			if !known {
				return fmt.Errorf("unsupported browser %q", rest)
			}
		} else if rest, ok := strings.CutPrefix(value, "file:"); ok {
			if st, err := os.Stat(rest); err != nil || st.IsDir() {
				return fmt.Errorf("cookies file not found: %q", rest)
			}
		} else {
			return fmt.Errorf("use browser:<name> or file:<path>")
		}
	}
	cfg := loadConfig()
	cfg.Cookies = value
	return saveConfig(cfg)
}

// cookieArgs translates the saved setting into yt-dlp flags.
func cookieArgs() []string {
	setting := strings.TrimSpace(loadConfig().Cookies)
	if name, ok := strings.CutPrefix(setting, "browser:"); ok && name != "" {
		return []string{"--cookies-from-browser", name}
	}
	if path, ok := strings.CutPrefix(setting, "file:"); ok && strings.TrimSpace(path) != "" {
		return []string{"--cookies", strings.TrimSpace(path)}
	}
	return nil
}

// GetDownloadDelay returns the saved pause between downloads (seconds, 0 = off).
func (a *App) GetDownloadDelay() int {
	d := loadConfig().DownloadDelaySec
	if d < 0 || d > 3600 {
		return 0
	}
	return d
}

// SetDownloadDelay saves the pause between downloads (0–3600 seconds).
func (a *App) SetDownloadDelay(sec int) error {
	if sec < 0 || sec > 3600 {
		return fmt.Errorf("delay must be 0–3600 seconds")
	}
	cfg := loadConfig()
	cfg.DownloadDelaySec = sec
	return saveConfig(cfg)
}

var engineTools = []string{"ffmpeg", "ytdlp"}

func validEngineTool(tool string) bool {
	for _, t := range engineTools {
		if tool == t {
			return true
		}
	}
	return false
}

// GetEnginePaths returns user-saved engine overrides (missing keys = "").
func (a *App) GetEnginePaths() map[string]string {
	cfg := loadConfig()
	out := map[string]string{}
	for _, t := range engineTools {
		out[t] = cfg.Engines[t]
	}
	return out
}

// SetEnginePath saves an explicit path for an engine. Empty path clears it.
func (a *App) SetEnginePath(tool, path string) error {
	if !validEngineTool(tool) {
		return fmt.Errorf("unknown engine %q", tool)
	}
	path = strings.TrimSpace(path)
	cfg := loadConfig()
	if path == "" {
		delete(cfg.Engines, tool)
		return saveConfig(cfg)
	}
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if st.IsDir() {
		return fmt.Errorf("not a file: %q", path)
	}
	cfg.Engines[tool] = path
	return saveConfig(cfg)
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

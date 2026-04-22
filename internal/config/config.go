package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Config stores runtime configuration and supports thread-safe reads and writes.
type Config struct {
	mu   sync.RWMutex
	path string

	MaxConcurrentQueue     int    `json:"max_concurrent_queue"`
	MaxConcurrentNow       int    `json:"max_concurrent_now"`
	DownloadDir            string `json:"download_dir"`
	OutputTemplate         string `json:"output_template"`
	OutputTemplatePlaylist string `json:"output_template_playlist"`
	DefaultQuality         string `json:"default_quality"`
	AudioFormat            string `json:"audio_format"`
	AudioQuality           string `json:"audio_quality"`
	AutoCategorize         bool   `json:"auto_categorize"`
	LogLevel               string `json:"log_level"`
	LogFile                string `json:"log_file"`
	APIPort                int    `json:"api_port"`
	APIHost                string `json:"api_host"`
	Retries                int    `json:"retries"`
	TrimFilenames          int    `json:"trim_filenames"`
	FFmpegLocation         string `json:"ffmpeg_location"`
	CookieFile             string `json:"cookie_file"`
	Proxy                  string `json:"proxy"`
	UserAgent              string `json:"user_agent"`
	GeoBypass              bool   `json:"geo_bypass"`
	APIToken               string `json:"api_token"`
	CORSAllowedOrigins     string `json:"cors_allowed_origins"`
}

// New returns a Config with project default values.
func New() *Config {
	return &Config{
		MaxConcurrentQueue:     4,
		MaxConcurrentNow:       2,
		DownloadDir:            defaultDownloadDir(),
		OutputTemplate:         "%(title)s.%(ext)s",
		OutputTemplatePlaylist: "%(playlist)s/%(playlist_index)s - %(title)s.%(ext)s",
		DefaultQuality:         "best",
		AudioFormat:            "mp3",
		AudioQuality:           "0",
		AutoCategorize:         true,
		LogLevel:               "info",
		LogFile:                "./pdv.log",
		APIPort:                8787,
		APIHost:                "0.0.0.0",
		Retries:                10,
		TrimFilenames:          200,
		FFmpegLocation:         "",
		CookieFile:             "",
		Proxy:                  "",
		UserAgent:              "",
		GeoBypass:              true,
		APIToken:               "",
		CORSAllowedOrigins:     "http://localhost,http://127.0.0.1,http://localhost:8787,http://127.0.0.1:8787",
	}
}

func defaultDownloadDir() string {
	return defaultDownloadDirFor(runtime.GOOS, os.Getenv, os.UserHomeDir, os.Stat)
}

func defaultDownloadDirFor(
	goos string,
	getenv func(string) string,
	userHomeDir func() (string, error),
	stat func(string) (os.FileInfo, error),
) string {
	candidates := make([]string, 0, 4)
	addCandidate := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		for _, existing := range candidates {
			if existing == path {
				return
			}
		}
		candidates = append(candidates, path)
	}

	addHomeDownloads := func() {
		home, err := userHomeDir()
		if err != nil {
			return
		}
		home = strings.TrimSpace(home)
		if home == "" {
			return
		}
		addCandidate(filepath.Join(home, "Downloads"))
	}

	addAndroidCandidates := func() {
		addCandidate("/storage/emulated/0/Download")
		home, err := userHomeDir()
		if err != nil {
			return
		}
		home = strings.TrimSpace(home)
		if home == "" {
			return
		}
		addCandidate(filepath.Join(home, "storage", "downloads"))
		addCandidate(filepath.Join(home, "Downloads"))
	}

	switch goos {
	case "windows":
		if profile := strings.TrimSpace(getenv("USERPROFILE")); profile != "" {
			addCandidate(filepath.Join(profile, "Downloads"))
		}
		addHomeDownloads()
	case "android":
		addAndroidCandidates()
	default:
		if goos == "linux" && isTermuxEnv(getenv) {
			addAndroidCandidates()
		} else {
			addHomeDownloads()
		}
	}

	for _, path := range candidates {
		if info, err := stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "./downloads"
}

func isTermuxEnv(getenv func(string) string) bool {
	return strings.Contains(getenv("PREFIX"), "com.termux") || strings.TrimSpace(getenv("TERMUX_VERSION")) != ""
}

// Load reads configuration from path and merges it over defaults.
func Load(path string) (*Config, error) {
	cfg := New()
	cfg.path = path
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}
	return cfg, nil
}

// Save persists the current configuration atomically to path.
func (c *Config) Save(path string) error {
	c.mu.RLock()
	data, err := json.MarshalIndent(c, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: ensure dir %q: %w", dir, err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("config: write temp %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("config: replace %q: %w", path, err)
	}
	c.mu.Lock()
	c.path = path
	c.mu.Unlock()
	return nil
}

// Get returns the string value of key.
func (c *Config) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getLocked(key)
}

// Set updates key with value and persists to the loaded config path.
func (c *Config) Set(key, value string) error {
	c.mu.Lock()
	prev, ok := c.getLocked(key)
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("config: set key %q: unknown key", key)
	}
	if err := c.setLocked(key, value); err != nil {
		c.mu.Unlock()
		return err
	}
	if err := c.validateLocked(); err != nil {
		_ = c.setLocked(key, prev)
		c.mu.Unlock()
		return err
	}
	path := c.path
	c.mu.Unlock()
	if path == "" {
		return fmt.Errorf("config: set key %q: no config path available", key)
	}
	if err := c.Save(path); err != nil {
		return fmt.Errorf("config: set key %q: %w", key, err)
	}
	return nil
}

// Validate checks whether config values satisfy runtime constraints.
func (c *Config) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.validateLocked()
}

func (c *Config) getLocked(key string) (string, bool) {
	switch key {
	case "max_concurrent_queue":
		return strconv.Itoa(c.MaxConcurrentQueue), true
	case "max_concurrent_now":
		return strconv.Itoa(c.MaxConcurrentNow), true
	case "download_dir":
		return c.DownloadDir, true
	case "output_template":
		return c.OutputTemplate, true
	case "output_template_playlist":
		return c.OutputTemplatePlaylist, true
	case "default_quality":
		return c.DefaultQuality, true
	case "audio_format":
		return c.AudioFormat, true
	case "audio_quality":
		return c.AudioQuality, true
	case "auto_categorize":
		return strconv.FormatBool(c.AutoCategorize), true
	case "log_level":
		return c.LogLevel, true
	case "log_file":
		return c.LogFile, true
	case "api_port":
		return strconv.Itoa(c.APIPort), true
	case "api_host":
		return c.APIHost, true
	case "retries":
		return strconv.Itoa(c.Retries), true
	case "trim_filenames":
		return strconv.Itoa(c.TrimFilenames), true
	case "ffmpeg_location":
		return c.FFmpegLocation, true
	case "cookie_file":
		return c.CookieFile, true
	case "proxy":
		return c.Proxy, true
	case "user_agent":
		return c.UserAgent, true
	case "geo_bypass":
		return strconv.FormatBool(c.GeoBypass), true
	case "api_token":
		return c.APIToken, true
	case "cors_allowed_origins":
		return c.CORSAllowedOrigins, true
	default:
		return "", false
	}
}

func (c *Config) setLocked(key, value string) error {
	intVal := func(name string) (int, error) {
		v, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("config: set key %q: parse int for %q: %w", key, name, err)
		}
		return v, nil
	}
	boolVal := func(name string) (bool, error) {
		v, err := strconv.ParseBool(value)
		if err != nil {
			return false, fmt.Errorf("config: set key %q: parse bool for %q: %w", key, name, err)
		}
		return v, nil
	}
	switch key {
	case "max_concurrent_queue":
		v, err := intVal(key)
		if err != nil {
			return err
		}
		c.MaxConcurrentQueue = v
	case "max_concurrent_now":
		v, err := intVal(key)
		if err != nil {
			return err
		}
		c.MaxConcurrentNow = v
	case "download_dir":
		c.DownloadDir = value
	case "output_template":
		c.OutputTemplate = value
	case "output_template_playlist":
		c.OutputTemplatePlaylist = value
	case "default_quality":
		c.DefaultQuality = value
	case "audio_format":
		c.AudioFormat = value
	case "audio_quality":
		c.AudioQuality = value
	case "auto_categorize":
		v, err := boolVal(key)
		if err != nil {
			return err
		}
		c.AutoCategorize = v
	case "log_level":
		c.LogLevel = value
	case "log_file":
		c.LogFile = value
	case "api_port":
		v, err := intVal(key)
		if err != nil {
			return err
		}
		c.APIPort = v
	case "api_host":
		c.APIHost = value
	case "retries":
		v, err := intVal(key)
		if err != nil {
			return err
		}
		c.Retries = v
	case "trim_filenames":
		v, err := intVal(key)
		if err != nil {
			return err
		}
		c.TrimFilenames = v
	case "ffmpeg_location":
		c.FFmpegLocation = value
	case "cookie_file":
		c.CookieFile = value
	case "proxy":
		c.Proxy = value
	case "user_agent":
		c.UserAgent = value
	case "geo_bypass":
		v, err := boolVal(key)
		if err != nil {
			return err
		}
		c.GeoBypass = v
	case "api_token":
		c.APIToken = value
	case "cors_allowed_origins":
		c.CORSAllowedOrigins = value
	default:
		return fmt.Errorf("config: set key %q: unknown key", key)
	}
	return nil
}

func (c *Config) validateLocked() error {
	var errs []error
	add := func(field, msg string) {
		errs = append(errs, fmt.Errorf("config: %s: %s", field, msg))
	}

	if c.MaxConcurrentQueue < 1 || c.MaxConcurrentQueue > 128 {
		add("max_concurrent_queue", "must be between 1 and 128")
	}
	if c.MaxConcurrentNow < 1 {
		add("max_concurrent_now", "must be at least 1")
	}
	if c.MaxConcurrentQueue >= 1 && c.MaxConcurrentNow > c.MaxConcurrentQueue {
		add("max_concurrent_now", "must be less than or equal to max_concurrent_queue")
	}
	if strings.TrimSpace(c.DownloadDir) == "" {
		add("download_dir", "must not be empty")
	}
	if strings.TrimSpace(c.DefaultQuality) == "" {
		add("default_quality", "must not be empty")
	}
	switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
	case "debug", "info", "warn", "error":
	default:
		add("log_level", "must be one of: debug, info, warn, error")
	}
	if c.APIPort < 1 || c.APIPort > 65535 {
		add("api_port", "must be between 1 and 65535")
	}
	if strings.TrimSpace(c.APIHost) == "" {
		add("api_host", "must not be empty")
	}
	if c.Retries < 0 || c.Retries > 100 {
		add("retries", "must be between 0 and 100")
	}
	if c.TrimFilenames < 0 || c.TrimFilenames > 512 {
		add("trim_filenames", "must be between 0 and 512")
	}
	if strings.ContainsAny(c.APIToken, " \t\r\n") {
		add("api_token", "must not contain whitespace")
	}
	if c.APIToken != "" && len(c.APIToken) < 8 {
		add("api_token", "must be at least 8 characters when set")
	}
	if err := validateCORSAllowedOrigins(c.CORSAllowedOrigins); err != nil {
		add("cors_allowed_origins", err.Error())
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func validateCORSAllowedOrigins(v string) error {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return fmt.Errorf("must not be empty")
	}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			return fmt.Errorf("contains empty origin entry")
		}
		if origin == "*" {
			continue
		}
		if strings.ContainsAny(origin, " \t\r\n") {
			return fmt.Errorf("origin %q contains whitespace", origin)
		}
		u, err := url.Parse(origin)
		if err != nil || !u.IsAbs() {
			return fmt.Errorf("origin %q must be an absolute http(s) origin", origin)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("origin %q must use http or https", origin)
		}
		if u.Host == "" {
			return fmt.Errorf("origin %q must include host", origin)
		}
		if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("origin %q must not include path/query/fragment", origin)
		}
	}
	return nil
}

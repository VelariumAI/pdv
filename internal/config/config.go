package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
}

// New returns a Config with project default values.
func New() *Config {
	return &Config{
		MaxConcurrentQueue:     4,
		MaxConcurrentNow:       2,
		DownloadDir:            "./downloads",
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
	}
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
	if err := c.setLocked(key, value); err != nil {
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
	default:
		return fmt.Errorf("config: set key %q: unknown key", key)
	}
	return nil
}

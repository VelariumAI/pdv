package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "pdv.json")
	content := `{"max_concurrent_queue":8,"download_dir":"./data","api_port":8989}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MaxConcurrentQueue != 8 {
		t.Fatalf("MaxConcurrentQueue = %d, want 8", cfg.MaxConcurrentQueue)
	}
	if cfg.DownloadDir != "./data" {
		t.Fatalf("DownloadDir = %q, want ./data", cfg.DownloadDir)
	}
	if cfg.APIPort != 8989 {
		t.Fatalf("APIPort = %d, want 8989", cfg.APIPort)
	}
}

func TestSaveAndReload(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "pdv.json")
	cfg := New()
	cfg.MaxConcurrentQueue = 6
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if reloaded.MaxConcurrentQueue != 6 {
		t.Fatalf("MaxConcurrentQueue = %d, want 6", reloaded.MaxConcurrentQueue)
	}
}

func TestGetAndSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "pdv.json")
	cfg := New()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := cfg.Set("max_concurrent_queue", "9"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	val, ok := cfg.Get("max_concurrent_queue")
	if !ok {
		t.Fatal("Get(max_concurrent_queue) ok = false, want true")
	}
	if val != "9" {
		t.Fatalf("Get(max_concurrent_queue) = %q, want 9", val)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.MaxConcurrentQueue != 9 {
		t.Fatalf("persisted MaxConcurrentQueue = %d, want 9", loaded.MaxConcurrentQueue)
	}
}

func TestDefaults(t *testing.T) {
	t.Parallel()
	cfg := New()
	if cfg.MaxConcurrentQueue != 4 {
		t.Fatalf("MaxConcurrentQueue default = %d, want 4", cfg.MaxConcurrentQueue)
	}
	if cfg.APIHost != "0.0.0.0" {
		t.Fatalf("APIHost default = %q, want 0.0.0.0", cfg.APIHost)
	}
	if cfg.APIPort != 8787 {
		t.Fatalf("APIPort default = %d, want 8787", cfg.APIPort)
	}
	if cfg.OutputTemplate != "%(title)s.%(ext)s" {
		t.Fatalf("OutputTemplate default = %q, want %%(title)s.%%(ext)s", cfg.OutputTemplate)
	}
}

func TestSetUnknownKey(t *testing.T) {
	t.Parallel()
	cfg := New()
	if err := cfg.Set("unknown_key", "x"); err == nil {
		t.Fatal("Set(unknown_key) error = nil, want non-nil")
	}
}

func TestSetWithoutPath(t *testing.T) {
	t.Parallel()
	cfg := New()
	if err := cfg.Set("max_concurrent_queue", "4"); err == nil {
		t.Fatal("Set() without path error = nil, want non-nil")
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("Load(missing) error = nil, want non-nil")
	}
}

func TestSetAllKnownKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "pdv.json")
	cfg := New()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	cases := map[string]string{
		"max_concurrent_queue":     "3",
		"max_concurrent_now":       "2",
		"download_dir":             "./dl",
		"output_template":          "%(title)s",
		"output_template_playlist": "%(playlist)s/%(title)s",
		"default_quality":          "1080p",
		"audio_format":             "aac",
		"audio_quality":            "2",
		"auto_categorize":          "false",
		"log_level":                "debug",
		"log_file":                 "./x.log",
		"api_port":                 "9090",
		"api_host":                 "127.0.0.1",
		"retries":                  "5",
		"trim_filenames":           "100",
		"ffmpeg_location":          "/usr/bin/ffmpeg",
		"cookie_file":              "./cookies.txt",
		"proxy":                    "http://127.0.0.1:8080",
		"user_agent":               "pdv-test",
		"geo_bypass":               "false",
	}
	for key, value := range cases {
		if err := cfg.Set(key, value); err != nil {
			t.Fatalf("Set(%s) error = %v", key, err)
		}
		got, ok := cfg.Get(key)
		if !ok {
			t.Fatalf("Get(%s) ok = false, want true", key)
		}
		if got != value {
			t.Fatalf("Get(%s) = %q, want %q", key, got, value)
		}
	}
}

func TestSetParseErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "pdv.json")
	cfg := New()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	for key := range map[string]struct{}{
		"max_concurrent_queue": {},
		"max_concurrent_now":   {},
		"api_port":             {},
		"retries":              {},
		"trim_filenames":       {},
	} {
		if err := cfg.Set(key, "not-an-int"); err == nil {
			t.Fatalf("Set(%s, not-an-int) error = nil, want non-nil", key)
		}
	}
	for key := range map[string]struct{}{
		"auto_categorize": {},
		"geo_bypass":      {},
	} {
		if err := cfg.Set(key, "not-a-bool"); err == nil {
			t.Fatalf("Set(%s, not-a-bool) error = nil, want non-nil", key)
		}
	}
}

func TestSaveInvalidPath(t *testing.T) {
	t.Parallel()
	cfg := New()
	if err := cfg.Save("/proc/pdv/config.json"); err == nil {
		t.Fatal("Save(invalid path) error = nil, want non-nil")
	}
}

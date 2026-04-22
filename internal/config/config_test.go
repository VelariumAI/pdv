package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if cfg.DownloadDir != defaultDownloadDir() {
		t.Fatalf("DownloadDir default = %q, want %q", cfg.DownloadDir, defaultDownloadDir())
	}
	if cfg.CORSAllowedOrigins == "" {
		t.Fatal("CORSAllowedOrigins default is empty, want localhost allowlist")
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
		"api_token":                "secret-token",
		"cors_allowed_origins":     "http://localhost,http://127.0.0.1",
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

func TestValidateDefaults(t *testing.T) {
	t.Parallel()
	cfg := New()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(defaults) error = %v, want nil", err)
	}
}

func TestValidateInvalidFields(t *testing.T) {
	t.Parallel()
	cfg := New()
	cfg.MaxConcurrentQueue = 0
	cfg.MaxConcurrentNow = 0
	cfg.APIPort = 70000
	cfg.LogLevel = "trace"
	cfg.CORSAllowedOrigins = "localhost:8787"
	cfg.APIToken = "short"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate(invalid) error = nil, want non-nil")
	}
	msg := err.Error()
	for _, want := range []string{
		"max_concurrent_queue",
		"max_concurrent_now",
		"api_port",
		"log_level",
		"cors_allowed_origins",
		"api_token",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("Validate(invalid) error %q missing %q", msg, want)
		}
	}
}

func TestSetRollbackOnValidationFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "pdv.json")
	cfg := New()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	before, _ := cfg.Get("api_port")
	if err := cfg.Set("api_port", "70000"); err == nil {
		t.Fatal("Set(api_port=70000) error = nil, want non-nil")
	}
	after, _ := cfg.Get("api_port")
	if after != before {
		t.Fatalf("Set rollback failed: api_port after=%q before=%q", after, before)
	}
}

func TestValidateMaxConcurrentNowGreaterThanQueue(t *testing.T) {
	t.Parallel()
	cfg := New()
	cfg.MaxConcurrentQueue = 2
	cfg.MaxConcurrentNow = 3
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate(max_now>queue) error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "max_concurrent_now") {
		t.Fatalf("Validate(max_now>queue) error %q missing max_concurrent_now", err.Error())
	}
}

func TestValidateCORSAllowedOriginsVariants(t *testing.T) {
	t.Parallel()
	cfg := New()

	cfg.CORSAllowedOrigins = "https://app.example.com,http://127.0.0.1:8787,*"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(valid cors variants) error = %v, want nil", err)
	}

	cfg.CORSAllowedOrigins = "https://app.example.com/path"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate(cors path) error = nil, want non-nil")
	}

	cfg.CORSAllowedOrigins = "https://app.example.com?x=1"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate(cors query) error = nil, want non-nil")
	}

	cfg.CORSAllowedOrigins = "ftp://app.example.com"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate(cors scheme) error = nil, want non-nil")
	}

	cfg.CORSAllowedOrigins = "https://ok.example.com,   "
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate(cors empty entry) error = nil, want non-nil")
	}
}

func TestValidateAPITokenWhitespace(t *testing.T) {
	t.Parallel()
	cfg := New()
	cfg.APIToken = "token with spaces"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate(api token whitespace) error = nil, want non-nil")
	}
}

func TestDefaultDownloadDirForLinuxUsesHomeDownloads(t *testing.T) {
	t.Parallel()
	path := defaultDownloadDirFor(
		"linux",
		func(string) string { return "" },
		func() (string, error) { return "/home/tester", nil },
		fakeDirStat("/home/tester/Downloads"),
	)
	if path != "/home/tester/Downloads" {
		t.Fatalf("defaultDownloadDirFor(linux) = %q, want /home/tester/Downloads", path)
	}
}

func TestDefaultDownloadDirForTermuxPrefersSharedStorage(t *testing.T) {
	t.Parallel()
	path := defaultDownloadDirFor(
		"linux",
		func(key string) string {
			if key == "PREFIX" {
				return "/data/data/com.termux/files/usr"
			}
			return ""
		},
		func() (string, error) { return "/data/data/com.termux/files/home", nil },
		fakeDirStat("/storage/emulated/0/Download"),
	)
	if path != "/storage/emulated/0/Download" {
		t.Fatalf("defaultDownloadDirFor(termux) = %q, want /storage/emulated/0/Download", path)
	}
}

func TestDefaultDownloadDirForTermuxFallsBackToHomeStorage(t *testing.T) {
	t.Parallel()
	path := defaultDownloadDirFor(
		"linux",
		func(key string) string {
			if key == "TERMUX_VERSION" {
				return "0.118.0"
			}
			return ""
		},
		func() (string, error) { return "/data/data/com.termux/files/home", nil },
		fakeDirStat("/data/data/com.termux/files/home/storage/downloads"),
	)
	if path != "/data/data/com.termux/files/home/storage/downloads" {
		t.Fatalf("defaultDownloadDirFor(termux fallback) = %q, want /data/data/com.termux/files/home/storage/downloads", path)
	}
}

func TestDefaultDownloadDirForWindowsUsesUserProfile(t *testing.T) {
	t.Parallel()
	path := defaultDownloadDirFor(
		"windows",
		func(key string) string {
			if key == "USERPROFILE" {
				return `C:\Users\tester`
			}
			return ""
		},
		func() (string, error) { return `C:\Users\fallback`, nil },
		fakeDirStat(`C:\Users\tester/Downloads`),
	)
	if path != `C:\Users\tester/Downloads` {
		t.Fatalf("defaultDownloadDirFor(windows) = %q, want C:\\Users\\tester/Downloads", path)
	}
}

func TestDefaultDownloadDirForFallsBackWhenUnknown(t *testing.T) {
	t.Parallel()
	path := defaultDownloadDirFor(
		"unknown",
		func(string) string { return "" },
		func() (string, error) { return "", os.ErrNotExist },
		fakeDirStat(),
	)
	if path != "./downloads" {
		t.Fatalf("defaultDownloadDirFor(unknown) = %q, want ./downloads", path)
	}
}

func fakeDirStat(paths ...string) func(string) (os.FileInfo, error) {
	set := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		set[path] = struct{}{}
	}
	return func(path string) (os.FileInfo, error) {
		if _, ok := set[path]; ok {
			return fakeFileInfo{isDir: true}, nil
		}
		return nil, os.ErrNotExist
	}
}

type fakeFileInfo struct {
	isDir bool
}

func (f fakeFileInfo) Name() string       { return "fake" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o755 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() any           { return nil }

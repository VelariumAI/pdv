package download

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/velariumai/pdv/pkg/output"
)

func TestProbeParsesDumpJSON(t *testing.T) {
	withFakeYTDLP(t, `#!/bin/sh
if [ "$1" = "--dump-json" ]; then
  echo '{"title":"Test","uploader":"U","duration":42,"upload_date":"20260101","thumbnail":"http://x","formats":[{"format_id":"18","ext":"mp4","width":1280,"height":720,"vcodec":"h264","filesize":1000}],"subtitles":{"en":[]}}'
  exit 0
fi
echo "unexpected args" >&2
exit 1
`)
	got, err := Probe(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if got.Title != "Test" || got.Duration != 42 {
		t.Fatalf("Probe() parsed wrong result: %#v", got)
	}
	if len(got.Formats) != 1 || got.Formats[0].FormatID != "18" {
		t.Fatalf("Probe() formats mismatch: %#v", got.Formats)
	}
	if len(got.Subtitles) != 1 || got.Subtitles[0] != "en" {
		t.Fatalf("Probe() subtitles mismatch: %#v", got.Subtitles)
	}
}

func TestProbeErrors(t *testing.T) {
	if _, err := Probe(context.Background(), ""); err == nil {
		t.Fatal("Probe(empty url) error = nil, want non-nil")
	}
	withFakeYTDLP(t, `#!/bin/sh
echo "failure" >&2
exit 2
`)
	if _, err := Probe(context.Background(), "https://example.com"); err == nil {
		t.Fatal("Probe(non-zero exit) error = nil, want non-nil")
	}
	withFakeYTDLP(t, `#!/bin/sh
echo "not-json"
exit 0
`)
	if _, err := Probe(context.Background(), "https://example.com"); err == nil {
		t.Fatal("Probe(invalid json) error = nil, want non-nil")
	}
}

func TestDownloadWithProgress(t *testing.T) {
	withFakeYTDLP(t, `#!/bin/sh
echo "pdv-progress:100|1000|1000|1MiB/s|10|10.0%"
echo "pdv-progress:1000|1000|1000|2MiB/s|0|100.0%"
exit 0
`)
	entry := &output.QueueEntry{ID: 7, URL: "https://example.com/video"}
	var events []output.ProgressEvent
	err := DownloadWithProgress(context.Background(), entry, &DownloadOpts{Template: "%(title)s.%(ext)s"}, func(e output.ProgressEvent) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("DownloadWithProgress() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("progress events = %d, want 2", len(events))
	}
	if events[1].Percentage < 99 {
		t.Fatalf("final percentage = %.2f, want near 100", events[1].Percentage)
	}
}

func TestDownloadErrors(t *testing.T) {
	if err := Download(context.Background(), nil, nil); err == nil {
		t.Fatal("Download(nil entry) error = nil, want non-nil")
	}
	if err := Download(context.Background(), &output.QueueEntry{}, nil); err == nil {
		t.Fatal("Download(empty URL) error = nil, want non-nil")
	}
	withFakeYTDLP(t, `#!/bin/sh
echo "nope" >&2
exit 4
`)
	err := Download(context.Background(), &output.QueueEntry{ID: 1, URL: "https://example.com"}, &DownloadOpts{})
	if err == nil {
		t.Fatal("Download(non-zero exit) error = nil, want non-nil")
	}
}

func TestBuildDownloadArgsAndParseProgress(t *testing.T) {
	args := buildDownloadArgs("https://example.com", &DownloadOpts{
		Format:    "18",
		Template:  "%(title)s.%(ext)s",
		Cookies:   "/tmp/cookies.txt",
		Proxy:     "http://127.0.0.1:8080",
		UserAgent: "pdv-test",
	})
	joined := strings.Join(args, " ")
	for _, part := range []string{"-f 18", "-o %(title)s.%(ext)s", "--cookies /tmp/cookies.txt", "--proxy http://127.0.0.1:8080", "--user-agent pdv-test"} {
		if !strings.Contains(joined, part) {
			t.Fatalf("args missing %q: %s", part, joined)
		}
	}
	ev, ok := parseProgressLine("pdv-progress:500|1000|1000|1MiB/s|3|50.0%", 42)
	if !ok || ev.ID != 42 || ev.Percentage != 50 {
		t.Fatalf("parseProgressLine mismatch: ok=%v ev=%#v", ok, ev)
	}
	if _, ok := parseProgressLine("random", 1); ok {
		t.Fatal("parseProgressLine(random) ok = true, want false")
	}
}

func TestCleanup(t *testing.T) {
	if err := Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}

func withFakeYTDLP(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	orig := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+":"+orig); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", orig)
	})
}

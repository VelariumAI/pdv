package download

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/velariumai/pdv/pkg/output"
)

var execCommandContext = exec.CommandContext

// DownloadOpts contains options for a download operation.
type DownloadOpts struct {
	Quality   string
	Format    string
	Template  string
	Cookies   string
	Proxy     string
	UserAgent string
}

// Probe calls yt-dlp --dump-json to retrieve metadata about a URL.
func Probe(ctx context.Context, url string) (*output.ProbeResult, error) {
	if url == "" {
		return nil, fmt.Errorf("download: probe: url is empty")
	}
	cmd := execCommandContext(ctx, "yt-dlp", "--dump-json", url)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("download: probe %q: %w", url, err)
		}
		return nil, fmt.Errorf("download: probe %q: %s: %w", url, msg, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("download: probe %q: parse JSON: %w", url, err)
	}
	return parseProbeJSON(raw), nil
}

// Download executes a yt-dlp download and consumes progress output.
func Download(ctx context.Context, entry *output.QueueEntry, opts *DownloadOpts) error {
	return DownloadWithProgress(ctx, entry, opts, nil)
}

// DownloadWithProgress executes a yt-dlp download and invokes onProgress for parsed progress events.
func DownloadWithProgress(
	ctx context.Context,
	entry *output.QueueEntry,
	opts *DownloadOpts,
	onProgress func(output.ProgressEvent),
) error {
	if entry == nil {
		return fmt.Errorf("download: entry is nil")
	}
	if entry.URL == "" {
		return fmt.Errorf("download: url is empty")
	}
	if opts == nil {
		opts = &DownloadOpts{}
	}
	args := buildDownloadArgs(entry.URL, opts)
	cmd := execCommandContext(ctx, "yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("download: create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("download: create stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("download: start yt-dlp: %w", err)
	}
	var stderrMu sync.Mutex
	stderrLines := make([]string, 0, 8)
	errDone := make(chan struct{})
	go consumeLines(stderr, func(line string) {
		if line == "" {
			return
		}
		stderrMu.Lock()
		stderrLines = append(stderrLines, line)
		stderrMu.Unlock()
	}, errDone)
	consumeLines(stdout, func(line string) {
		if onProgress == nil {
			return
		}
		if ev, ok := parseProgressLine(line, entry.ID); ok {
			onProgress(ev)
		}
	}, nil)
	if err := cmd.Wait(); err != nil {
		<-errDone
		stderrMu.Lock()
		msg := strings.Join(stderrLines, "; ")
		stderrMu.Unlock()
		if msg == "" {
			return fmt.Errorf("download: yt-dlp failed: %w", err)
		}
		return fmt.Errorf("download: yt-dlp failed: %s: %w", msg, err)
	}
	<-errDone
	return nil
}

// Cleanup attempts to terminate stale yt-dlp processes.
func Cleanup() error {
	pkill, err := exec.LookPath("pkill")
	if err != nil {
		return nil
	}
	cmd := exec.Command(pkill, "-f", "yt-dlp")
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("download: cleanup stale processes: %w", err)
	}
	return nil
}

func buildDownloadArgs(url string, opts *DownloadOpts) []string {
	args := []string{
		"--newline",
		"--progress-template",
		"pdv-progress:%(progress.downloaded_bytes)s|%(progress.total_bytes_estimate)s|%(progress.total_bytes)s|%(progress.speed)s|%(progress.eta)s|%(progress._percent_str)s",
	}
	if opts.Format != "" {
		args = append(args, "-f", opts.Format)
	} else if opts.Quality != "" {
		args = append(args, "-S", opts.Quality)
	} else {
		args = append(args, "-S", "res,ext:mp4:m4a")
	}
	if opts.Template != "" {
		args = append(args, "-o", opts.Template)
	}
	if opts.Cookies != "" {
		args = append(args, "--cookies", opts.Cookies)
	}
	if opts.Proxy != "" {
		args = append(args, "--proxy", opts.Proxy)
	}
	if opts.UserAgent != "" {
		args = append(args, "--user-agent", opts.UserAgent)
	}
	args = append(args, url)
	return args
}

func consumeLines(pipe io.Reader, fn func(string), done chan struct{}) {
	if done != nil {
		defer close(done)
	}
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fn(strings.TrimSpace(scanner.Text()))
	}
}

func parseProgressLine(line string, id int64) (output.ProgressEvent, bool) {
	const prefix = "pdv-progress:"
	if !strings.HasPrefix(line, prefix) {
		return output.ProgressEvent{}, false
	}
	parts := strings.Split(strings.TrimPrefix(line, prefix), "|")
	if len(parts) != 6 {
		return output.ProgressEvent{}, false
	}
	downloaded, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	estimated, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	total, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	speed := strings.TrimSpace(parts[3])
	eta := strings.TrimSpace(parts[4])
	percentText := strings.TrimSpace(strings.TrimSuffix(parts[5], "%"))
	percent, err := strconv.ParseFloat(percentText, 64)
	if err != nil {
		percent = 0
	}
	if percent <= 0 && total > 0 {
		percent = (downloaded / total) * 100
	}
	if percent <= 0 && estimated > 0 {
		percent = (downloaded / estimated) * 100
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return output.ProgressEvent{
		ID:         id,
		Percentage: percent,
		Speed:      speed,
		ETA:        eta,
	}, true
}

// parseProbeJSON extracts relevant fields from yt-dlp JSON output.
func parseProbeJSON(data map[string]any) *output.ProbeResult {
	result := &output.ProbeResult{
		Formats:   make([]output.Format, 0),
		Subtitles: make([]string, 0),
	}
	if v, ok := data["title"].(string); ok {
		result.Title = v
	}
	if v, ok := data["uploader"].(string); ok {
		result.Uploader = v
	}
	if v, ok := data["duration"].(float64); ok {
		result.Duration = int(v)
	}
	if v, ok := data["upload_date"].(string); ok {
		result.UploadDate = v
	}
	if v, ok := data["thumbnail"].(string); ok {
		result.ThumbnailURL = v
	}
	if formats, ok := data["formats"].([]any); ok {
		for _, f := range formats {
			if fmap, ok := f.(map[string]any); ok {
				result.Formats = append(result.Formats, parseFormat(fmap))
			}
		}
	}
	if subs, ok := data["subtitles"].(map[string]any); ok {
		for lang := range subs {
			result.Subtitles = append(result.Subtitles, lang)
		}
	}
	return result
}

// parseFormat extracts format information from yt-dlp format object.
func parseFormat(fmap map[string]any) output.Format {
	format := output.Format{}
	if v, ok := fmap["format_id"].(string); ok {
		format.FormatID = v
	}
	if v, ok := fmap["ext"].(string); ok {
		format.Ext = v
	}
	if height, ok := fmap["height"].(float64); ok {
		if width, ok := fmap["width"].(float64); ok {
			format.Resolution = fmt.Sprintf("%.0fx%.0f", width, height)
		}
	}
	if v, ok := fmap["vcodec"].(string); ok && v != "none" {
		format.Codec = v
	} else if v, ok := fmap["acodec"].(string); ok && v != "none" {
		format.Codec = v
	}
	if v, ok := fmap["filesize"].(float64); ok {
		format.FileSizeEstimate = int64(v)
	} else if v, ok := fmap["filesize_approx"].(float64); ok {
		format.FileSizeEstimate = int64(v)
	}
	return format
}

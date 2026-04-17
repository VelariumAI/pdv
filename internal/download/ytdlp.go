package download

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/velariumai/pdv/pkg/output"
)

// Probe calls yt-dlp --dump-json to retrieve metadata about a URL.
// Returns a ProbeResult with title, formats, subtitles, and thumbnail information.
func Probe(ctx context.Context, url string) (*output.ProbeResult, error) {
	if url == "" {
		return nil, fmt.Errorf("download: probe: url is empty")
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", "--dump-json", url)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderr := stderr.String()
		if stderr != "" {
			return nil, fmt.Errorf("download: probe %q: yt-dlp error: %s", url, stderr)
		}
		return nil, fmt.Errorf("download: probe %q: %w", url, err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &rawData); err != nil {
		return nil, fmt.Errorf("download: probe %q: parse JSON: %w", url, err)
	}

	result := parseProbeJSON(rawData)
	return result, nil
}

// parseProbeJSON extracts relevant fields from yt-dlp JSON output.
func parseProbeJSON(data map[string]interface{}) *output.ProbeResult {
	result := &output.ProbeResult{
		Formats:   make([]output.Format, 0),
		Subtitles: make([]string, 0),
	}

	// Title
	if v, ok := data["title"].(string); ok {
		result.Title = v
	}

	// Uploader
	if v, ok := data["uploader"].(string); ok {
		result.Uploader = v
	}

	// Duration (in seconds)
	if v, ok := data["duration"].(float64); ok {
		result.Duration = int(v)
	}

	// Upload date (YYYYMMDD format)
	if v, ok := data["upload_date"].(string); ok {
		result.UploadDate = v
	}

	// Thumbnail
	if v, ok := data["thumbnail"].(string); ok {
		result.ThumbnailURL = v
	}

	// Formats
	if formats, ok := data["formats"].([]interface{}); ok {
		for _, f := range formats {
			if fmap, ok := f.(map[string]interface{}); ok {
				fmt := parseFormat(fmap)
				result.Formats = append(result.Formats, fmt)
			}
		}
	}

	// Subtitles (extract language codes)
	if subs, ok := data["subtitles"].(map[string]interface{}); ok {
		for lang := range subs {
			result.Subtitles = append(result.Subtitles, lang)
		}
	}

	return result
}

// parseFormat extracts format information from yt-dlp format object.
func parseFormat(fmap map[string]interface{}) output.Format {
	format := output.Format{}

	if v, ok := fmap["format_id"].(string); ok {
		format.FormatID = v
	}

	if v, ok := fmap["ext"].(string); ok {
		format.Ext = v
	}

	// Resolution (height + width)
	height, hasHeight := fmap["height"].(float64)
	width, hasWidth := fmap["width"].(float64)
	if hasHeight && hasWidth {
		format.Resolution = fmt.Sprintf("%.0fx%.0f", width, height)
	}

	// Codec (vcodec or acodec)
	if v, ok := fmap["vcodec"].(string); ok && v != "none" {
		format.Codec = v
	} else if v, ok := fmap["acodec"].(string); ok && v != "none" {
		format.Codec = v
	}

	// File size estimate
	if v, ok := fmap["filesize"].(float64); ok {
		format.FileSizeEstimate = int64(v)
	} else if v, ok := fmap["filesize_approx"].(float64); ok {
		format.FileSizeEstimate = int64(v)
	}

	return format
}

// DownloadOpts contains options for a download operation.
type DownloadOpts struct {
	Quality  string
	Format   string
	Template string
	Cookies  string
	Proxy    string
	UserAgent string
}

// Download executes a yt-dlp download and reads progress from stdout.
// Returns error on subprocess failure or parse error.
func Download(ctx context.Context, url string, opts *DownloadOpts) error {
	if url == "" {
		return fmt.Errorf("download: url is empty")
	}
	if opts == nil {
		opts = &DownloadOpts{}
	}

	args := []string{
		"--progress-template", "%(progress.total_bytes)s|%(progress.downloaded_bytes)s|%(progress.elapsed)s|%(progress.eta)s",
		"--newline",
	}

	// Quality/format selection
	if opts.Format != "" {
		args = append(args, "-f", opts.Format)
	} else if opts.Quality != "" {
		args = append(args, "-S", opts.Quality)
	} else {
		args = append(args, "-S", "res,ext:mp4:m4a")
	}

	// Output template
	if opts.Template != "" {
		args = append(args, "-o", opts.Template)
	}

	// Cookies
	if opts.Cookies != "" {
		args = append(args, "--cookies", opts.Cookies)
	}

	// Proxy
	if opts.Proxy != "" {
		args = append(args, "--proxy", opts.Proxy)
	}

	// User agent
	if opts.UserAgent != "" {
		args = append(args, "--user-agent", opts.UserAgent)
	}

	args = append(args, url)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
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

	// Read progress and errors from stdout/stderr (consume them to avoid blocking)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			// Progress events are available but not processed yet (Tranche 3+)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// Stderr messages (warnings, info) are consumed but not processed
		}
	}()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("download: yt-dlp failed: %w", err)
	}

	return nil
}

// Cleanup attempts to kill any orphaned yt-dlp processes.
// In practice, this is defensive; context cancellation should handle cleanup.
func Cleanup() error {
	// On most systems, orphaned processes are cleaned up by the OS.
	// For defensive cleanup, we could find and kill yt-dlp processes by name,
	// but this is not typically necessary if context cancellation is used correctly.
	// Tranche 2 leaves this as a no-op; robust signal handling happens in Stop().
	return nil
}

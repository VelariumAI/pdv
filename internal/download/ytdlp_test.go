package download

import (
	"context"
	"testing"

	"github.com/velariumai/pdv/pkg/output"
)

func TestProbe(t *testing.T) {
	// Note: These tests require yt-dlp to be installed.
	// For isolated unit testing, Tranche 2+ could add testable interfaces.
	// For now, we verify the function signature and error handling.

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "empty url error",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := Probe(ctx, tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Probe() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseProbeJSON(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want *output.ProbeResult
	}{
		{
			name: "empty data",
			data: map[string]interface{}{},
			want: &output.ProbeResult{
				Formats:   []output.Format{},
				Subtitles: []string{},
			},
		},
		{
			name: "basic metadata",
			data: map[string]interface{}{
				"title":    "Test Video",
				"duration": 120.5,
				"uploader": "Test User",
			},
			want: &output.ProbeResult{
				Title:     "Test Video",
				Duration:  120,
				Uploader:  "Test User",
				Formats:   []output.Format{},
				Subtitles: []string{},
			},
		},
		{
			name: "with formats",
			data: map[string]interface{}{
				"title": "Video",
				"formats": []interface{}{
					map[string]interface{}{
						"format_id": "best",
						"ext":       "mp4",
						"height":    1080.0,
						"width":     1920.0,
					},
				},
			},
			want: &output.ProbeResult{
				Title: "Video",
				Formats: []output.Format{
					{
						FormatID:   "best",
						Ext:        "mp4",
						Resolution: "1920x1080",
					},
				},
				Subtitles: []string{},
			},
		},
		{
			name: "with subtitles",
			data: map[string]interface{}{
				"title": "Translated Video",
				"subtitles": map[string]interface{}{
					"en": []interface{}{},
					"es": []interface{}{},
					"fr": []interface{}{},
				},
			},
			want: &output.ProbeResult{
				Title:     "Translated Video",
				Formats:   []output.Format{},
				Subtitles: []string{"en", "es", "fr"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseProbeJSON(tt.data)

			if got.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Duration != tt.want.Duration {
				t.Errorf("Duration = %d, want %d", got.Duration, tt.want.Duration)
			}
			if got.Uploader != tt.want.Uploader {
				t.Errorf("Uploader = %q, want %q", got.Uploader, tt.want.Uploader)
			}
			if len(got.Formats) != len(tt.want.Formats) {
				t.Errorf("Formats count = %d, want %d", len(got.Formats), len(tt.want.Formats))
			}
			if len(got.Subtitles) != len(tt.want.Subtitles) {
				t.Errorf("Subtitles count = %d, want %d", len(got.Subtitles), len(tt.want.Subtitles))
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		name string
		fmap map[string]interface{}
		want output.Format
	}{
		{
			name: "complete format",
			fmap: map[string]interface{}{
				"format_id": "best",
				"ext":       "mp4",
				"height":    720.0,
				"width":     1280.0,
				"vcodec":    "h264",
				"filesize":  104857600.0,
			},
			want: output.Format{
				FormatID:         "best",
				Ext:              "mp4",
				Resolution:       "1280x720",
				Codec:            "h264",
				FileSizeEstimate: 104857600,
			},
		},
		{
			name: "audio format",
			fmap: map[string]interface{}{
				"format_id": "best",
				"ext":       "m4a",
				"acodec":    "aac",
				"filesize":  5242880.0,
			},
			want: output.Format{
				FormatID:         "best",
				Ext:              "m4a",
				Codec:            "aac",
				FileSizeEstimate: 5242880,
			},
		},
		{
			name: "minimal format",
			fmap: map[string]interface{}{
				"format_id": "basic",
				"ext":       "webm",
			},
			want: output.Format{
				FormatID: "basic",
				Ext:      "webm",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFormat(tt.fmap)

			if got.FormatID != tt.want.FormatID {
				t.Errorf("FormatID = %q, want %q", got.FormatID, tt.want.FormatID)
			}
			if got.Ext != tt.want.Ext {
				t.Errorf("Ext = %q, want %q", got.Ext, tt.want.Ext)
			}
			if got.Resolution != tt.want.Resolution {
				t.Errorf("Resolution = %q, want %q", got.Resolution, tt.want.Resolution)
			}
			if got.Codec != tt.want.Codec {
				t.Errorf("Codec = %q, want %q", got.Codec, tt.want.Codec)
			}
			if got.FileSizeEstimate != tt.want.FileSizeEstimate {
				t.Errorf("FileSizeEstimate = %d, want %d", got.FileSizeEstimate, tt.want.FileSizeEstimate)
			}
		})
	}
}

func TestDownloadOpts(t *testing.T) {
	// Verify DownloadOpts struct fields are accessible
	opts := &DownloadOpts{
		Quality:   "best",
		Format:    "best[ext=mp4]",
		Template:  "%(title)s.%(ext)s",
		Cookies:   "/path/to/cookies",
		Proxy:     "http://proxy:8080",
		UserAgent: "Custom/1.0",
	}

	if opts.Quality != "best" {
		t.Errorf("Quality = %q, want %q", opts.Quality, "best")
	}
	if opts.Format != "best[ext=mp4]" {
		t.Errorf("Format = %q, want %q", opts.Format, "best[ext=mp4]")
	}
	if opts.Template != "%(title)s.%(ext)s" {
		t.Errorf("Template = %q, want %q", opts.Template, "%(title)s.%(ext)s")
	}
}

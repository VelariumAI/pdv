package download

import (
	"strings"
	"testing"
)

func TestBuildOutputPath(t *testing.T) {
	tests := []struct {
		name     string
		template string
		title    string
		ext      string
		playlist string
		want     string
	}{
		{
			name:     "default template with title and ext",
			template: "%(title)s.%(ext)s",
			title:    "My Video",
			ext:      "mp4",
			playlist: "",
			want:     "My Video.mp4",
		},
		{
			name:     "playlist template",
			template: "%(playlist)s/%(title)s.%(ext)s",
			title:    "Episode 5",
			ext:      "mkv",
			playlist: "Series One",
			want:     "Series One/Episode 5.mkv",
		},
		{
			name:     "empty template uses default",
			template: "",
			title:    "Video Title",
			ext:      "webm",
			playlist: "",
			want:     "Video Title.webm",
		},
		{
			name:     "title with slashes cleaned",
			template: "%(title)s.%(ext)s",
			title:    "My/Video/Title",
			ext:      "mp4",
			playlist: "",
			want:     "My-Video-Title.mp4",
		},
		{
			name:     "ext with leading dot trimmed",
			template: "%(title)s.%(ext)s",
			title:    "Test",
			ext:      ".m4a",
			playlist: "",
			want:     "Test.m4a",
		},
		{
			name:     "empty ext defaults to mp4",
			template: "%(title)s.%(ext)s",
			title:    "Silent Video",
			ext:      "",
			playlist: "",
			want:     "Silent Video.mp4",
		},
		{
			name:     "empty title defaults to untitled",
			template: "%(title)s.%(ext)s",
			title:    "",
			ext:      "mp4",
			playlist: "",
			want:     "untitled.mp4",
		},
		{
			name:     "empty playlist defaults to downloads",
			template: "%(playlist)s/%(title)s.%(ext)s",
			title:    "Video",
			ext:      "mp4",
			playlist: "",
			want:     "downloads/Video.mp4",
		},
		{
			name:     "title with control characters",
			template: "%(title)s.%(ext)s",
			title:    "Title\x00\x01\x1fWith\x7fChars",
			ext:      "mp4",
			playlist: "",
			want:     "TitleWithChars.mp4",
		},
		{
			name:     "title with multiple spaces normalized",
			template: "%(title)s.%(ext)s",
			title:    "Title    With    Spaces",
			ext:      "mp4",
			playlist: "",
			want:     "Title With Spaces.mp4",
		},
		{
			name:     "very long title truncated",
			template: "%(title)s.%(ext)s",
			title:    "a" + strings.Repeat("b", 250),
			ext:      "mp4",
			playlist: "",
			want:     "a" + strings.Repeat("b", 199) + ".mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildOutputPath(tt.template, tt.title, tt.ext, tt.playlist)
			if got != tt.want {
				t.Errorf("BuildOutputPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanFilename(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "normal string unchanged",
			s:    "Normal Filename",
			want: "Normal Filename",
		},
		{
			name: "slashes become dashes",
			s:    "Part/1/2",
			want: "Part-1-2",
		},
		{
			name: "backslashes become dashes",
			s:    "Path\\To\\File",
			want: "Path-To-File",
		},
		{
			name: "empty string defaults to untitled",
			s:    "",
			want: "untitled",
		},
		{
			name: "spaces preserved",
			s:    "File With Spaces",
			want: "File With Spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanFilename(tt.s)
			if got != tt.want {
				t.Errorf("cleanFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanExtension(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "simple extension",
			s:    "mp4",
			want: "mp4",
		},
		{
			name: "extension with leading dot",
			s:    ".mkv",
			want: "mkv",
		},
		{
			name: "uppercase becomes lowercase",
			s:    "MP4",
			want: "mp4",
		},
		{
			name: "extension with spaces becomes empty defaults to mp4",
			s:    "   ",
			want: "mp4",
		},
		{
			name: "empty defaults to mp4",
			s:    "",
			want: "mp4",
		},
		{
			name: "non-alphanumeric chars removed",
			s:    ".m@p#4!",
			want: "mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanExtension(tt.s)
			if got != tt.want {
				t.Errorf("cleanExtension() = %q, want %q", got, tt.want)
			}
		})
	}
}

package download

import (
	"regexp"
	"strings"
)

// BuildOutputPath applies variable substitution to a yt-dlp output template.
// Supports: %(title)s, %(ext)s, %(playlist)s, %(playlist_index)s, %(uploader)s, %(upload_date)s
// Unknown variables are passed through unchanged (yt-dlp handles them).
func BuildOutputPath(template, title, ext, playlist string) string {
	if template == "" {
		template = "%(title)s.%(ext)s"
	}

	// Ensure playlist has a sensible default
	if playlist == "" {
		playlist = "downloads"
	}

	result := template

	// Replace %(title)s
	result = strings.ReplaceAll(result, "%(title)s", cleanFilename(title))

	// Replace %(ext)s
	result = strings.ReplaceAll(result, "%(ext)s", cleanExtension(ext))

	// Replace %(playlist)s
	result = strings.ReplaceAll(result, "%(playlist)s", cleanFilename(playlist))

	// Replace %(playlist_index)s (placeholder, will be 0 for now)
	result = strings.ReplaceAll(result, "%(playlist_index)s", "0")

	// Unknown variables like %(uploader)s, %(upload_date)s are left as-is
	// for yt-dlp to handle or ignore

	return result
}

// cleanFilename removes or replaces problematic characters in filenames.
func cleanFilename(s string) string {
	if s == "" {
		return "untitled"
	}

	// Replace path separators
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")

	// Remove control characters
	re := regexp.MustCompile(`[\x00-\x1f\x7f]`)
	s = re.ReplaceAllString(s, "")

	// Collapse multiple spaces
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")

	// Trim spaces
	s = strings.TrimSpace(s)

	// Limit length (most filesystems support 255 bytes)
	if len(s) > 200 {
		s = s[:200]
	}

	return s
}

// cleanExtension ensures the extension is safe and properly formatted.
func cleanExtension(s string) string {
	if s == "" {
		return "mp4"
	}

	// Remove leading dot if present
	s = strings.TrimPrefix(s, ".")

	// Lowercase
	s = strings.ToLower(s)

	// Only allow alphanumeric
	re := regexp.MustCompile(`[^a-z0-9]`)
	s = re.ReplaceAllString(s, "")

	if s == "" {
		return "mp4"
	}

	return s
}

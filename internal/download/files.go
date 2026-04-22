package download

import (
	"os"
	"path/filepath"
	"strings"
)

func categorize(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "mp3", "m4a", "aac", "wav", "flac", "ogg":
		return "audio"
	case "mp4", "mkv", "webm", "mov", "avi", "m4v":
		return "videos"
	case "jpg", "jpeg", "png", "gif", "webp":
		return "images"
	default:
		return "downloads"
	}
}

func categoryDir(category, baseDir string) string {
	base := strings.TrimSpace(baseDir)
	if base == "" {
		base = "."
	}
	cat := strings.TrimSpace(category)
	if cat == "" {
		cat = "downloads"
	}
	return filepath.Join(base, cat)
}

func moveFile(srcPath, destDir string) (string, error) {
	src := strings.TrimSpace(srcPath)
	destRoot := strings.TrimSpace(destDir)
	if src == "" || destRoot == "" {
		return "", nil
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return "", err
	}
	destPath := filepath.Join(destRoot, filepath.Base(src))
	if err := os.Rename(src, destPath); err != nil {
		return "", err
	}
	return destPath, nil
}

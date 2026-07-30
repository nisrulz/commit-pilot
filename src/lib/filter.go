package lib

import (
	"os"
	"path/filepath"
	"strings"
)

func FilterFiles(files []FileDiff, includes, excludes []string, allowSensitive bool) []FileDiff {
	ignored := append([]string{}, excludes...)
	if data, err := os.ReadFile(".commitpilotignore"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				ignored = append(ignored, line)
			}
		}
	}
	var out []FileDiff
	for _, file := range files {
		if matches(file.Path, ignored) || (!allowSensitive && IsSensitivePath(file.Path)) {
			continue
		}
		if len(includes) == 0 || matches(file.Path, includes) {
			out = append(out, file)
		}
	}
	return out
}

func matches(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
	}
	return false
}
func IsSensitivePath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == ".env" || strings.Contains(name, "secret") || strings.Contains(name, "key") || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".p12") || strings.HasSuffix(name, ".pfx")
}

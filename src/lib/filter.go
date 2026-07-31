package lib

import (
	"os"
	"path/filepath"
	"strings"
)

func FilterFiles(files []FileDiff, includes, excludes []string, allowSensitive bool) []FileDiff {
	var out []FileDiff
	for _, file := range files {
		if ShouldIncludePath(file.Path, includes, excludes, allowSensitive) {
			out = append(out, file)
		}
	}
	return out
}

func FilterChanges(changes *Changes, includes, excludes []string, allowSensitive bool) {
	changes.FilesWithDiffs = FilterFiles(changes.FilesWithDiffs, includes, excludes, allowSensitive)
	changes.BinaryFiles = FilterPaths(changes.BinaryFiles, includes, excludes, allowSensitive)
	changes.AllFiles = AllFilePaths(changes)
}

func FilterPaths(paths, includes, excludes []string, allowSensitive bool) []string {
	var out []string
	for _, path := range paths {
		if ShouldIncludePath(path, includes, excludes, allowSensitive) {
			out = append(out, path)
		}
	}
	return out
}

func ShouldIncludePath(path string, includes, excludes []string, allowSensitive bool) bool {
	ignored := append([]string{}, excludes...)
	ignored = append(ignored, IgnorePatterns()...)
	return !matches(path, ignored) && (allowSensitive || !IsSensitivePath(path)) &&
		(len(includes) == 0 || matches(path, includes))
}

func IgnorePatterns() []string {
	path := ".commitpilotignore"
	if root, err := GitRun("rev-parse", "--show-toplevel"); err == nil {
		path = filepath.Join(strings.TrimSpace(root), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}
	return patterns
}

func matches(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
		if !strings.Contains(pattern, "/") {
			if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
				return true
			}
		}
	}
	return false
}
func IsSensitivePath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if name == ".env" || strings.HasPrefix(name, ".env.") || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".p12") || strings.HasSuffix(name, ".pfx") {
		return true
	}
	if strings.Contains(name, "secret") || strings.Contains(name, "credential") || strings.HasSuffix(name, ".key") || strings.HasPrefix(name, "id_") || strings.Contains(name, "private_key") || strings.Contains(name, "api_key") || strings.Contains(name, "apikey") {
		return true
	}
	switch name {
	case "cargo.lock", "composer.lock", "gemfile.lock", "package-lock.json", "pnpm-lock.yaml", "yarn.lock":
		return true
	default:
		return strings.HasSuffix(name, ".lock")
	}
}

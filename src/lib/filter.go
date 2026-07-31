package lib

import (
	"os"
	"path/filepath"
	"strings"
)

func FilterFiles(files []FileDiff, includes, excludes []string, allowSensitive bool) []FileDiff {
	return filterFiles(files, includes, ignoredPatterns(excludes), allowSensitive)
}

func filterFiles(files []FileDiff, includes, ignored []string, allowSensitive bool) []FileDiff {
	var out []FileDiff
	for _, file := range files {
		if shouldIncludePath(file.Path, includes, ignored, allowSensitive) {
			out = append(out, file)
		}
	}
	return out
}

func FilterChanges(changes *Changes, includes, excludes []string, allowSensitive bool) []string {
	before := append([]string(nil), changes.AllFiles...)
	ignored := ignoredPatterns(excludes)
	changes.FilesWithDiffs = filterFiles(changes.FilesWithDiffs, includes, ignored, allowSensitive)
	changes.BinaryFiles = filterPaths(changes.BinaryFiles, includes, ignored, allowSensitive)
	changes.AllFiles = AllFilePaths(changes)

	selected := make(map[string]bool, len(changes.AllFiles))
	for _, path := range changes.AllFiles {
		selected[path] = true
	}
	var filtered []string
	for _, path := range before {
		if !selected[path] {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func FilterPaths(paths, includes, excludes []string, allowSensitive bool) []string {
	return filterPaths(paths, includes, ignoredPatterns(excludes), allowSensitive)
}

func filterPaths(paths, includes, ignored []string, allowSensitive bool) []string {
	var out []string
	for _, path := range paths {
		if shouldIncludePath(path, includes, ignored, allowSensitive) {
			out = append(out, path)
		}
	}
	return out
}

func ShouldIncludePath(path string, includes, excludes []string, allowSensitive bool) bool {
	return shouldIncludePath(path, includes, ignoredPatterns(excludes), allowSensitive)
}

func shouldIncludePath(path string, includes, ignored []string, allowSensitive bool) bool {
	return !matches(path, ignored) && (allowSensitive || !IsSensitivePath(path)) &&
		(len(includes) == 0 || matches(path, includes))
}

func ignoredPatterns(excludes []string) []string {
	ignored := append([]string{}, excludes...)
	ignored = append(ignored, IgnorePatterns()...)
	return ignored
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
	case ".netrc", ".npmrc", ".pypirc", "auth.json":
		return true
	default:
		return strings.HasSuffix(name, ".jks") || strings.HasSuffix(name, ".keystore")
	}
}

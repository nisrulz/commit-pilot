package e2e_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// mockRelease serves a fake GitHub release (tag lookup, archive download, and
// checksums) over a real HTTP server so install.sh runs against a genuine
// curl/tar/checksum pipeline.
type mockRelease struct {
	server  *httptest.Server
	apiBase string
	dlBase  string
}

// installOSArch maps the current platform to the names install.sh derives from
// `uname`; ok is false for platforms the script does not support.
func installOSArch() (osName, archName string, ok bool) {
	switch runtime.GOOS {
	case "darwin":
		osName = "darwin"
	case "linux":
		osName = "linux"
	default:
		return "", "", false
	}
	switch runtime.GOARCH {
	case "amd64":
		archName = "amd64"
	case "arm64":
		archName = "arm64"
	default:
		return "", "", false
	}
	return osName, archName, true
}

// buildReleaseArchive packs the given binary into the tar.gz layout GoReleaser
// produces for commit-pilot.
func buildReleaseArchive(t *testing.T, binBytes []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	entries := []struct {
		name string
		mode int64
		data []byte
	}{
		{"commit-pilot", 0o755, binBytes},
		{"LICENSE", 0o644, []byte("MIT\n")},
		{"README.md", 0o644, []byte("commit-pilot\n")},
	}
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: e.mode, Size: int64(len(e.data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newMockRelease starts an httptest server serving the given archive and
// checksums under the path layout install.sh requests. It stops the server when
// the test finishes.
func newMockRelease(t *testing.T, tag, archiveName string, archiveBytes []byte, checksumsContent string) *mockRelease {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "{\"tag_name\": \"%s\"}\n", tag)
	})
	mux.HandleFunc("/dl/releases/download/", func(w http.ResponseWriter, r *http.Request) {
		asset := strings.TrimPrefix(r.URL.Path, "/dl/releases/download/")
		if _, rest, ok := strings.Cut(asset, "/"); ok {
			asset = rest
		}
		switch asset {
		case archiveName:
			_, _ = w.Write(archiveBytes)
		case "checksums.txt":
			_, _ = io.WriteString(w, checksumsContent)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &mockRelease{
		server:  srv,
		apiBase: srv.URL + "/api",
		dlBase:  srv.URL + "/dl",
	}
}

// runInstall executes scripts/install.sh in cwd with a controlled HOME and the
// release sources pointed at the mock server. It returns combined output and
// the command error.
func runInstall(t *testing.T, cwd, home string, release *mockRelease) (string, error) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", filepath.Join(root, "scripts", "install.sh"))
	cmd.Dir = cwd
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=" + os.Getenv("PATH"),
		"COMMIT_PILOT_INSTALL_API_BASE=" + release.apiBase,
		"COMMIT_PILOT_INSTALL_DOWNLOAD_BASE=" + release.dlBase,
	}
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	return out.String(), err
}

// dirNames lists the entries of dir, sorted, and fails the test on error.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestEndToEndInstallScript(t *testing.T) {
	// Use the real CLI binary as the "release" so the installed artifact can be
	// smoke-tested afterwards.
	bin := cliBinary(t)
	binBytes, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}

	osName, archName, ok := installOSArch()
	if !ok {
		t.Skipf("install.sh does not support %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	version := "1.2.3"
	archiveName := fmt.Sprintf("commit-pilot_%s_%s_%s.tar.gz", version, osName, archName)
	archiveBytes := buildReleaseArchive(t, binBytes)
	sum := sha256.Sum256(archiveBytes)
	checksums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), archiveName)

	release := newMockRelease(t, "v"+version, archiveName, archiveBytes, checksums)

	t.Run("installs binary without touching cwd", func(t *testing.T) {
		home := t.TempDir()
		cwd := t.TempDir()

		// Regression: the cwd already contains a commit-pilot/ directory, which
		// used to make the script fail with "Is a directory".
		marker := filepath.Join(cwd, "commit-pilot", "keep.txt")
		if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		// A stray file so we can assert nothing extra is written to the cwd
		// (no archive, no checksums.txt, no binary).
		if err := os.WriteFile(filepath.Join(cwd, "stray.txt"), []byte("stray"), 0o644); err != nil {
			t.Fatal(err)
		}
		// A stale previous install at the destination that must be replaced.
		if err := os.MkdirAll(filepath.Join(home, "go", "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "go", "bin", "commit-pilot"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}

		out, err := runInstall(t, cwd, home, release)
		if err != nil {
			t.Fatalf("install failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "Installed commit-pilot") {
			t.Fatalf("missing success message:\n%s", out)
		}

		installed := filepath.Join(home, "go", "bin", "commit-pilot")
		got, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("binary not installed: %v\n%s", err, out)
		}
		if !bytes.Equal(got, binBytes) {
			t.Fatal("installed binary does not match the released binary")
		}
		fi, err := os.Stat(installed)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode()&0o111 == 0 {
			t.Fatalf("installed binary is not executable: %v", fi.Mode())
		}

		if got := dirNames(t, cwd); !reflect.DeepEqual(got, []string{"commit-pilot", "stray.txt"}) {
			t.Fatalf("install polluted the cwd: %v", got)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("pre-existing cwd content was removed: %v", err)
		}

		// The installed artifact actually runs.
		help := exec.Command(installed, "--help")
		if output, err := help.CombinedOutput(); err != nil {
			t.Fatalf("installed binary --help failed: %v\n%s", err, output)
		}
	})

	t.Run("aborts on checksum mismatch", func(t *testing.T) {
		home := t.TempDir()
		cwd := t.TempDir()

		bad := fmt.Sprintf("%s  %s\n", strings.Repeat("0", 64), archiveName)
		badRelease := newMockRelease(t, "v"+version, archiveName, archiveBytes, bad)

		out, err := runInstall(t, cwd, home, badRelease)
		if err == nil {
			t.Fatalf("expected checksum failure\n%s", out)
		}
		if !strings.Contains(out, "Checksum mismatch") {
			t.Fatalf("missing mismatch message:\n%s", out)
		}
		if _, err := os.Stat(filepath.Join(home, "go", "bin", "commit-pilot")); !os.IsNotExist(err) {
			t.Fatalf("binary must not be installed on checksum failure: %v", err)
		}
		if got := dirNames(t, cwd); len(got) != 0 {
			t.Fatalf("checksum failure polluted the cwd: %v", got)
		}
	})

	t.Run("aborts when checksum entry is missing", func(t *testing.T) {
		home := t.TempDir()
		cwd := t.TempDir()
		missing := newMockRelease(t, "v"+version, archiveName, archiveBytes, "")

		out, err := runInstall(t, cwd, home, missing)
		if err == nil {
			t.Fatalf("expected missing checksum failure\n%s", out)
		}
		if !strings.Contains(out, "Missing or invalid checksum") {
			t.Fatalf("missing checksum error:\n%s", out)
		}
		if _, err := os.Stat(filepath.Join(home, "go", "bin", "commit-pilot")); !os.IsNotExist(err) {
			t.Fatalf("binary must not be installed without a checksum: %v", err)
		}
	})

	t.Run("aborts when destination is a directory", func(t *testing.T) {
		home := t.TempDir()
		cwd := t.TempDir()

		if err := os.MkdirAll(filepath.Join(home, "go", "bin", "commit-pilot"), 0o755); err != nil {
			t.Fatal(err)
		}

		out, err := runInstall(t, cwd, home, release)
		if err == nil {
			t.Fatalf("expected failure when destination is a directory\n%s", out)
		}
		if !strings.Contains(out, "exists as a directory") {
			t.Fatalf("missing destination message:\n%s", out)
		}
	})
}

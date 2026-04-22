package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// PinnedCrocVersion is the croc version managed by qim-data setup.
	PinnedCrocVersion = "v10.4.2"
)

// EnsureCroc returns a usable croc path. It prefers an existing v10+ binary,
// otherwise downloads a pinned prebuilt release into qim-data managed storage.
func EnsureCroc(preferredPath string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(preferredPath) != "" {
		candidates = append(candidates, strings.TrimSpace(preferredPath))
	}
	if path, err := exec.LookPath("croc"); err == nil {
		candidates = append(candidates, path)
	}

	for _, candidate := range candidates {
		if isUsableV10(candidate) {
			return candidate, nil
		}
	}

	managedPath, err := managedBinaryPath()
	if err != nil {
		return "", err
	}
	if isUsableV10(managedPath) {
		return managedPath, nil
	}

	if err := installManagedBinary(managedPath); err != nil {
		return "", err
	}
	if !isUsableV10(managedPath) {
		return "", fmt.Errorf("installed croc is not usable or not v10+: %s", managedPath)
	}
	return managedPath, nil
}

func managedBinaryPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	base := filepath.Join(cacheDir, "qim-data", "bin")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("create managed bin directory %s: %w", base, err)
	}
	name := "croc"
	if runtime.GOOS == "windows" {
		name = "croc.exe"
	}
	return filepath.Join(base, name), nil
}

func installManagedBinary(destPath string) error {
	url, archiveKind, err := releaseAssetURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	tmpDir := filepath.Dir(destPath)
	archivePath := filepath.Join(tmpDir, "croc-"+PinnedCrocVersion+".download")
	if err := downloadFile(url, archivePath); err != nil {
		return err
	}
	defer os.Remove(archivePath)

	tmpExtractPath := destPath + ".tmp"
	if err := os.RemoveAll(tmpExtractPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cleanup temp binary %s: %w", tmpExtractPath, err)
	}

	switch archiveKind {
	case "tar.gz":
		if err := extractTarGzBinary(archivePath, tmpExtractPath); err != nil {
			return err
		}
	case "zip":
		if err := extractZipBinary(archivePath, tmpExtractPath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported archive kind: %s", archiveKind)
	}

	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.Chmod(tmpExtractPath, mode); err != nil {
		return fmt.Errorf("chmod temp binary %s: %w", tmpExtractPath, err)
	}

	if runtime.GOOS == "windows" {
		_ = os.Remove(destPath)
	}
	if err := os.Rename(tmpExtractPath, destPath); err != nil {
		return fmt.Errorf("place installed binary %s: %w", destPath, err)
	}
	return nil
}

func releaseAssetURL(goos, goarch string) (url string, archiveKind string, err error) {
	base := "https://github.com/schollz/croc/releases/download/" + PinnedCrocVersion + "/"

	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return base + "croc_" + PinnedCrocVersion + "_Linux-64bit.tar.gz", "tar.gz", nil
		case "arm64":
			return base + "croc_" + PinnedCrocVersion + "_Linux-ARM64.tar.gz", "tar.gz", nil
		case "386":
			return base + "croc_" + PinnedCrocVersion + "_Linux-32bit.tar.gz", "tar.gz", nil
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return base + "croc_" + PinnedCrocVersion + "_macOS-64bit.tar.gz", "tar.gz", nil
		case "arm64":
			return base + "croc_" + PinnedCrocVersion + "_macOS-ARM64.tar.gz", "tar.gz", nil
		}
	case "windows":
		switch goarch {
		case "amd64":
			return base + "croc_" + PinnedCrocVersion + "_Windows-64bit.zip", "zip", nil
		case "arm64":
			return base + "croc_" + PinnedCrocVersion + "_Windows-ARM64.zip", "zip", nil
		case "386":
			return base + "croc_" + PinnedCrocVersion + "_Windows-32bit.zip", "zip", nil
		}
	}

	return "", "", fmt.Errorf("no pinned croc build for platform %s/%s", goos, goarch)
}

func downloadFile(url, destPath string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s failed: status %s", url, resp.Status)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", destPath, err)
	}
	return nil
}

func extractTarGzBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive %s: %w", archivePath, err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read gzip %s: %w", archivePath, err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar %s: %w", archivePath, err)
		}

		base := filepath.Base(hdr.Name)
		if base != "croc" && base != "croc.exe" {
			continue
		}

		out, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("create extracted binary %s: %w", destPath, err)
		}
		defer out.Close()

		if _, err := io.Copy(out, tr); err != nil {
			return fmt.Errorf("extract binary to %s: %w", destPath, err)
		}
		return nil
	}

	return fmt.Errorf("could not find croc binary in %s", archivePath)
}

func extractZipBinary(archivePath, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", archivePath, err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if base != "croc.exe" && base != "croc" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open entry %s in %s: %w", f.Name, archivePath, err)
		}
		defer rc.Close()

		out, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("create extracted binary %s: %w", destPath, err)
		}
		defer out.Close()

		if _, err := io.Copy(out, rc); err != nil {
			return fmt.Errorf("extract binary to %s: %w", destPath, err)
		}
		return nil
	}

	return fmt.Errorf("could not find croc binary in %s", archivePath)
}

func isUsableV10(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}

	cmd := exec.Command(path, "--version")
	b, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	out := strings.TrimSpace(string(b))
	return strings.Contains(out, "v10.")
}

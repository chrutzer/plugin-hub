package source

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Fetch retrieves a zip file from a location into destZip. The location is
// treated as a URL if it starts with "http://" or "https://", otherwise as
// a local file path.
func Fetch(location, destZip string) error {
	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		return fetchURL(location, destZip)
	}
	return fetchFile(location, destZip)
}

func fetchURL(url, destZip string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}

	out, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("save downloaded zip: %w", err)
	}
	return nil
}

func fetchFile(path, destZip string) error {
	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer in.Close()

	out, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s: %w", path, err)
	}
	return nil
}

// Extract unzips zipPath into destDir, rejecting any entry that would
// escape destDir (zip-slip protection).
func Extract(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	destDir, err = filepath.Abs(destDir)
	if err != nil {
		return err
	}

	for _, f := range r.File {
		if err := extractOne(f, destDir); err != nil {
			return err
		}
	}
	return nil
}

func extractOne(f *zip.File, destDir string) error {
	cleanName := filepath.Clean(f.Name)
	if cleanName == "." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) || cleanName == ".." {
		return fmt.Errorf("zip entry escapes destination: %q", f.Name)
	}

	targetPath := filepath.Join(destDir, cleanName)
	rel, err := filepath.Rel(destDir, targetPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("zip entry escapes destination: %q", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(targetPath, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %q: %w", f.Name, err)
	}
	defer rc.Close()

	out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode().Perm()|0o600)
	if err != nil {
		return fmt.Errorf("create %q: %w", targetPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, io.LimitReader(rc, maxEntrySize)); err != nil {
		return fmt.Errorf("write %q: %w", targetPath, err)
	}
	return nil
}

const maxEntrySize = 200 * 1024 * 1024

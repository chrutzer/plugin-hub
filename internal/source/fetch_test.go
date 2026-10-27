package source

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func zipEntries(t *testing.T, path string) []string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

func writeFiles(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFetchDirFiltersPluginFiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir,
		"plugin.json", "mcp.json", "icon.svg", "README.md",
		"skills/a/SKILL.md", "skills/a/references/doc.md",
		"agent/prompt.md", ".git/config")

	dest := filepath.Join(t.TempDir(), "out.zip")
	if err := Fetch(dir, dest); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"icon.svg", "mcp.json", "plugin.json", "skills/", "skills/a/",
		"skills/a/SKILL.md", "skills/a/references/", "skills/a/references/doc.md",
	}
	got := zipEntries(t, dest)
	if len(got) != len(want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entries = %v, want %v", got, want)
		}
	}
}

func TestFetchDirWithoutManifestIsNotFiltered(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "a.zip", "other/b.zip")

	dest := filepath.Join(t.TempDir(), "out.zip")
	if err := Fetch(dir, dest); err != nil {
		t.Fatal(err)
	}
	if got := zipEntries(t, dest); len(got) != 3 {
		t.Fatalf("entries = %v, want 3", got)
	}
}

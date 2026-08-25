package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDetectsIcon(t *testing.T) {
	root := t.TempDir()

	manifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"icon-test"}`
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "icon.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(resolvedRoot, "icon.svg")
	if p.IconPath != want {
		t.Fatalf("IconPath = %q, want %q", p.IconPath, want)
	}
}

func TestLoadPrefersPNGOverOtherIconFormats(t *testing.T) {
	root := t.TempDir()

	manifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"icon-priority-test"}`
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "icon.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "icon.png"), []byte("fake-png"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(resolvedRoot, "icon.png")
	if p.IconPath != want {
		t.Fatalf("IconPath = %q, want %q", p.IconPath, want)
	}
}

func TestLoadRejectsSymlinkEscapeForIcon(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	secretPath := filepath.Join(outside, "secret.png")
	if err := os.WriteFile(secretPath, []byte("leaked"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"icon-escape-test"}`
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secretPath, filepath.Join(root, "icon.png")); err != nil {
		t.Fatal(err)
	}

	p, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if p.IconPath != "" {
		t.Fatalf("expected the symlinked icon to be skipped, got %q", p.IconPath)
	}
}

func TestLoadRejectsSymlinkEscapeForSkill(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	secretPath := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secretPath, []byte("---\nname: leaked\ndescription: should not be read.\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"escape-test"}`
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(root, "skills", "leaked")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secretPath, filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	p, err := Load(root)
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(p.Skills) != 0 {
		t.Fatalf("expected the symlinked skill to be skipped, got %d skills", len(p.Skills))
	}

	found := false
	for _, w := range p.Warnings {
		if w != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a warning about the escaping skill")
	}
}

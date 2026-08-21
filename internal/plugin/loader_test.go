package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

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

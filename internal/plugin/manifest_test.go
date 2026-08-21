package plugin

import "testing"

func TestNonObjectExtensionsIsNonFatal(t *testing.T) {
	data := []byte(`{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin",
		"extensions": ["a", "b"]
	}`)
	m, warnings, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("expected non-fatal handling, got error: %v", err)
	}
	if m == nil {
		t.Fatal("expected manifest to be returned")
	}
	if m.Extensions != nil {
		t.Fatalf("expected extensions to be nil, got %v", m.Extensions)
	}
	if len(warnings) == 0 {
		t.Fatal("expected a warning about non-object extensions")
	}
}

func TestExtensionsMemberMustBeObject(t *testing.T) {
	data := []byte(`{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin",
		"extensions": {
			"com.example.client": "not-an-object"
		}
	}`)
	_, _, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for non-object extension namespace value")
	}
}

func TestExtensionsMemberObjectAccepted(t *testing.T) {
	data := []byte(`{
		"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
		"name": "test-plugin",
		"extensions": {
			"com.example.client": {"setting": true}
		}
	}`)
	m, _, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Extensions) != 1 {
		t.Fatalf("expected 1 extension namespace, got %d", len(m.Extensions))
	}
}

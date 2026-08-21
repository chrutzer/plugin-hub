package plugin

import (
	"encoding/json"
	"fmt"
	"regexp"
)

const SchemaPluginV1 = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"

var pluginNameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-.]*[a-z0-9])?$`)

type Author struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Manifest struct {
	Schema      string                     `json:"$schema"`
	Name        string                     `json:"name"`
	Version     string                     `json:"version,omitempty"`
	Description string                     `json:"description,omitempty"`
	Author      *Author                    `json:"author,omitempty"`
	Homepage    string                     `json:"homepage,omitempty"`
	Repository  string                     `json:"repository,omitempty"`
	License     string                     `json:"license,omitempty"`
	Keywords    []string                   `json:"keywords,omitempty"`
	Extensions  map[string]json.RawMessage `json:"extensions,omitempty"`
}

// ParseManifest parses and validates plugin.json per the Agent Plugins v1
// specification. It returns non-fatal warnings for reportable-but-ignorable
// violations, and an error for anything that must reject the whole plugin.
func ParseManifest(data []byte) (*Manifest, []string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("plugin.json is not a valid JSON object: %w", err)
	}

	allowed := map[string]bool{
		"$schema": true, "name": true, "version": true, "description": true,
		"author": true, "homepage": true, "repository": true, "license": true,
		"keywords": true, "extensions": true,
	}

	var warnings []string
	for field := range raw {
		if !allowed[field] {
			warnings = append(warnings, fmt.Sprintf("unknown top-level field %q ignored", field))
			delete(raw, field)
		}
	}

	if extRaw, ok := raw["extensions"]; ok {
		var probe interface{}
		if err := json.Unmarshal(extRaw, &probe); err == nil {
			if _, isObj := probe.(map[string]interface{}); !isObj {
				warnings = append(warnings, "extensions field is not an object, ignored")
				delete(raw, "extensions")
			}
		}
	}

	cleaned, err := json.Marshal(raw)
	if err != nil {
		return nil, warnings, err
	}

	var m Manifest
	if err := json.Unmarshal(cleaned, &m); err != nil {
		return nil, warnings, fmt.Errorf("plugin.json schema violation: %w", err)
	}

	for namespace, value := range m.Extensions {
		var probe interface{}
		if err := json.Unmarshal(value, &probe); err != nil {
			return nil, warnings, fmt.Errorf("extensions.%s: invalid JSON: %w", namespace, err)
		}
		if _, isObj := probe.(map[string]interface{}); !isObj {
			return nil, warnings, fmt.Errorf("extensions.%s: must be an object", namespace)
		}
	}

	if m.Schema != SchemaPluginV1 {
		return nil, warnings, fmt.Errorf("unsupported or missing $schema %q", m.Schema)
	}

	if err := validateName(m.Name); err != nil {
		return nil, warnings, fmt.Errorf("invalid plugin name: %w", err)
	}

	if m.Author != nil {
		var authorRaw map[string]json.RawMessage
		if ar, ok := raw["author"]; ok {
			_ = json.Unmarshal(ar, &authorRaw)
			for field := range authorRaw {
				if field != "name" && field != "email" && field != "url" {
					return nil, warnings, fmt.Errorf("author has invalid field %q", field)
				}
			}
		}
	}

	return &m, warnings, nil
}

func validateName(name string) error {
	if len(name) < 1 || len(name) > 64 {
		return fmt.Errorf("must be 1-64 characters, got %d", len(name))
	}
	if !pluginNameRe.MatchString(name) {
		return fmt.Errorf("must be lowercase alphanumeric, hyphens, or periods, starting/ending alphanumeric: %q", name)
	}
	if containsConsecutive(name, "--") || containsConsecutive(name, "..") {
		return fmt.Errorf("must not contain consecutive hyphens or periods: %q", name)
	}
	return nil
}

func containsConsecutive(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

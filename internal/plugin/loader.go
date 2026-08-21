package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Plugin struct {
	Manifest *Manifest
	Skills   []Skill
	MCP      *MCPConfig
	Warnings []string

	SourceName string
	ZipPath    string
}

// Load reads a plugin from an extracted directory (rootDir), applying the
// discovery and failure-isolation rules from the Agent Plugins v1 spec.
func Load(rootDir string) (*Plugin, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	rootDir, err = filepath.EvalSymlinks(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve plugin root: %w", err)
	}

	manifestPath := filepath.Join(rootDir, "plugin.json")
	if !isWithin(rootDir, manifestPath) {
		return nil, fmt.Errorf("plugin.json escapes plugin root")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read plugin.json: %w", err)
	}
	manifest, warnings, err := ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin.json: %w", err)
	}

	p := &Plugin{Manifest: manifest, Warnings: warnings}

	skillsDir := filepath.Join(rootDir, "skills")
	if info, err := os.Stat(skillsDir); err == nil {
		if info.IsDir() && isWithin(rootDir, skillsDir) {
			p.Skills, p.Warnings = loadSkills(rootDir, skillsDir, p.Warnings)
		} else {
			p.Warnings = append(p.Warnings, "skills/ is not a directory, skipped")
		}
	}

	mcpPath := filepath.Join(rootDir, "mcp.json")
	if info, err := os.Stat(mcpPath); err == nil {
		if !info.Mode().IsRegular() || !resolvesWithin(rootDir, mcpPath) {
			p.Warnings = append(p.Warnings, "mcp.json is not a regular file, skipped")
		} else {
			mcpData, err := os.ReadFile(mcpPath)
			if err != nil {
				p.Warnings = append(p.Warnings, fmt.Sprintf("could not read mcp.json: %v", err))
			} else {
				mcpCfg, mcpWarnings, err := ParseMCPConfig(mcpData)
				p.Warnings = append(p.Warnings, mcpWarnings...)
				if err != nil {
					p.Warnings = append(p.Warnings, fmt.Sprintf("mcp.json disabled: %v", err))
				} else if schemaVersion(mcpCfg.Schema) != schemaVersion(manifest.Schema) {
					p.Warnings = append(p.Warnings, "mcp.json disabled: $schema version does not match plugin.json")
				} else {
					p.MCP = mcpCfg
				}
			}
		}
	}

	return p, nil
}

func loadSkills(rootDir, skillsDir string, warnings []string) ([]Skill, []string) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, append(warnings, fmt.Sprintf("could not read skills/: %v", err))
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillMDPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if !resolvesWithin(rootDir, skillMDPath) {
			warnings = append(warnings, fmt.Sprintf("skill %q SKILL.md escapes plugin root, skipped", entry.Name()))
			continue
		}
		info, err := os.Stat(skillMDPath)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(skillMDPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skill %q: could not read SKILL.md: %v", entry.Name(), err))
			continue
		}
		skill, err := ParseSkill(data, entry.Name())
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skill %q invalid, skipped: %v", entry.Name(), err))
			continue
		}
		skills = append(skills, *skill)
	}
	return skills, warnings
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !filepathHasPrefix(rel, "../")
}

// resolvesWithin reports whether path, after resolving any symlinks, stays
// within root. A path that doesn't exist is treated as within root, since
// the caller's own os.Stat will subsequently fail on it.
func resolvesWithin(root, path string) bool {
	if !isWithin(root, path) {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return true
	}
	return isWithin(root, resolved)
}

func filepathHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// schemaVersion extracts the version segment (e.g. "1.0.0") from a canonical
// Agent Plugins schema URL such as .../schemas/1.0.0/plugin.schema.json.
func schemaVersion(schemaURL string) string {
	parts := strings.Split(schemaURL, "/")
	for i, part := range parts {
		if part == "schemas" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return schemaURL
}

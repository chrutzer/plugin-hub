package plugin

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var skillNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Skill struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	AllowedTools  string            `json:"allowedTools,omitempty"`
}

type skillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
}

// ParseSkill parses a SKILL.md file's YAML frontmatter and validates it
// against the Agent Skills specification. dirName is the skill's parent
// directory name, which the frontmatter name must match.
func ParseSkill(data []byte, dirName string) (*Skill, error) {
	frontmatter, _, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var fm skillFrontmatter
	if err := yaml.Unmarshal(frontmatter, &fm); err != nil {
		return nil, fmt.Errorf("invalid frontmatter YAML: %w", err)
	}

	if err := validateSkillName(fm.Name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}
	if fm.Name != dirName {
		return nil, fmt.Errorf("name %q must match parent directory name %q", fm.Name, dirName)
	}
	if len(fm.Description) < 1 || len(fm.Description) > 1024 {
		return nil, fmt.Errorf("description must be 1-1024 characters, got %d", len(fm.Description))
	}
	if len(fm.Compatibility) > 500 {
		return nil, fmt.Errorf("compatibility must be at most 500 characters")
	}

	return &Skill{
		Name:          fm.Name,
		Description:   fm.Description,
		License:       fm.License,
		Compatibility: fm.Compatibility,
		Metadata:      fm.Metadata,
		AllowedTools:  fm.AllowedTools,
	}, nil
}

func validateSkillName(name string) error {
	if len(name) < 1 || len(name) > 64 {
		return fmt.Errorf("must be 1-64 characters, got %d", len(name))
	}
	if !skillNameRe.MatchString(name) {
		return fmt.Errorf("must be lowercase alphanumeric and hyphens, no leading/trailing/consecutive hyphens: %q", name)
	}
	return nil
}

func splitFrontmatter(data []byte) (frontmatter, body []byte, err error) {
	const delim = "---"
	s := string(data)
	if !strings.HasPrefix(s, delim) {
		return nil, nil, fmt.Errorf("missing YAML frontmatter delimiter")
	}
	rest := s[len(delim):]
	idx := strings.Index(rest, "\n"+delim)
	if idx == -1 {
		return nil, nil, fmt.Errorf("unterminated YAML frontmatter")
	}
	return []byte(rest[:idx]), []byte(rest[idx+len(delim)+1:]), nil
}

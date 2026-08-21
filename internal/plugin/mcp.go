package plugin

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const SchemaMCPV1 = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"

type MCPConfig struct {
	Schema     string               `json:"$schema"`
	MCPServers map[string]MCPServer `json:"mcpServers"`
}

type MCPServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Cwd     string            `json:"cwd,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ParseMCPConfig parses and validates mcp.json. Invalid server entries are
// dropped (with a warning) rather than failing the whole document, per
// §7.2.2 of the spec. A top-level failure disables MCP entirely (nil, warnings, err).
func ParseMCPConfig(data []byte) (*MCPConfig, []string, error) {
	var raw struct {
		Schema     string                     `json:"$schema"`
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	var rawTop map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawTop); err != nil {
		return nil, nil, fmt.Errorf("mcp.json is not valid JSON: %w", err)
	}
	for field := range rawTop {
		if field != "$schema" && field != "mcpServers" {
			return nil, nil, fmt.Errorf("mcp.json has unknown top-level field %q", field)
		}
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("mcp.json schema violation: %w", err)
	}
	if raw.Schema != SchemaMCPV1 {
		return nil, nil, fmt.Errorf("unsupported or missing $schema %q", raw.Schema)
	}
	if raw.MCPServers == nil {
		return nil, nil, fmt.Errorf("mcp.json missing required field %q", "mcpServers")
	}

	cfg := &MCPConfig{Schema: raw.Schema, MCPServers: map[string]MCPServer{}}
	var warnings []string

	for name, entryRaw := range raw.MCPServers {
		server, err := parseMCPServer(entryRaw)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("mcp server %q invalid, skipped: %v", name, err))
			continue
		}
		cfg.MCPServers[name] = *server
	}

	return cfg, warnings, nil
}

func parseMCPServer(data json.RawMessage) (*MCPServer, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("not an object: %w", err)
	}

	var s MCPServer
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	switch s.Type {
	case "stdio":
		for f := range fields {
			switch f {
			case "type", "command", "args", "env", "cwd":
			default:
				return nil, fmt.Errorf("field %q not valid for stdio", f)
			}
		}
		if s.Command == "" {
			return nil, fmt.Errorf("command is required")
		}
		if s.Env != nil {
			if _, ok := s.Env["PLUGIN_ROOT"]; ok {
				return nil, fmt.Errorf("env must not contain reserved PLUGIN_ROOT")
			}
			if _, ok := s.Env["PLUGIN_DATA"]; ok {
				return nil, fmt.Errorf("env must not contain reserved PLUGIN_DATA")
			}
		}
		if s.Cwd != "" {
			if !strings.HasPrefix(s.Cwd, "./") && s.Cwd != "${PLUGIN_ROOT}" && !strings.HasPrefix(s.Cwd, "${PLUGIN_ROOT}/") &&
				s.Cwd != "${PLUGIN_DATA}" && !strings.HasPrefix(s.Cwd, "${PLUGIN_DATA}/") {
				return nil, fmt.Errorf("cwd %q has an unsupported form", s.Cwd)
			}
		}
		return &s, nil

	case "streamable-http", "sse":
		for f := range fields {
			switch f {
			case "type", "url", "headers":
			default:
				return nil, fmt.Errorf("field %q not valid for %s", f, s.Type)
			}
		}
		if s.URL == "" {
			return nil, fmt.Errorf("url is required")
		}
		u, err := url.Parse(s.URL)
		if err != nil || !u.IsAbs() {
			return nil, fmt.Errorf("url must be absolute: %q", s.URL)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("url must use http or https: %q", s.URL)
		}
		if u.User != nil || u.Fragment != "" {
			return nil, fmt.Errorf("url must not contain user info or a fragment")
		}
		if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
			return nil, fmt.Errorf("non-loopback url must use https: %q", s.URL)
		}
		seen := map[string]bool{}
		for h, v := range s.Headers {
			lower := strings.ToLower(h)
			if seen[lower] {
				return nil, fmt.Errorf("duplicate header name (case-insensitive): %q", h)
			}
			seen[lower] = true
			if !isValidHeaderName(h) {
				return nil, fmt.Errorf("invalid header name: %q", h)
			}
			if !isValidHeaderValue(v) {
				return nil, fmt.Errorf("invalid header value for %q", h)
			}
		}
		return &s, nil

	default:
		return nil, fmt.Errorf("unknown type %q", s.Type)
	}
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// isValidHeaderName reports whether name is a valid HTTP header field-name
// (RFC 7230 token characters).
func isValidHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !isTokenChar(c) {
			return false
		}
	}
	return true
}

// isValidHeaderValue reports whether value contains only characters
// permitted in an HTTP header field-value (RFC 7230 field-content),
// rejecting CR/LF and other control characters.
func isValidHeaderValue(value string) bool {
	for _, c := range value {
		if c == '\r' || c == '\n' {
			return false
		}
		if c < 0x20 && c != '\t' {
			return false
		}
		if c == 0x7f {
			return false
		}
	}
	return true
}

func isTokenChar(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case strings.ContainsRune("!#$%&'*+-.^_`|~", c):
		return true
	default:
		return false
	}
}

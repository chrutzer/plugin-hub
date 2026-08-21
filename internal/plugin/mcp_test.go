package plugin

import "testing"

const mcpSchemaHeader = `"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",`

func TestMCPHeaderInjectionRejected(t *testing.T) {
	data := []byte(`{
		` + mcpSchemaHeader + `
		"mcpServers": {
			"bad": {
				"type": "streamable-http",
				"url": "https://example.com/mcp",
				"headers": { "X-Tenant": "value\r\nX-Injected: evil" }
			}
		}
	}`)
	cfg, warnings, err := ParseMCPConfig(data)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if len(cfg.MCPServers) != 0 {
		t.Fatalf("expected the malicious server entry to be dropped, got %d servers", len(cfg.MCPServers))
	}
	if len(warnings) == 0 {
		t.Fatal("expected a warning about the invalid header")
	}
}

func TestMCPHeaderValidAccepted(t *testing.T) {
	data := []byte(`{
		` + mcpSchemaHeader + `
		"mcpServers": {
			"good": {
				"type": "streamable-http",
				"url": "https://example.com/mcp",
				"headers": { "X-Tenant": "public-tenant" }
			}
		}
	}`)
	cfg, _, err := ParseMCPConfig(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.MCPServers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(cfg.MCPServers))
	}
}

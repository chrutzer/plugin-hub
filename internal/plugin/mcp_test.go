package plugin

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

const mcpSchemaHeader = `"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",`

func TestMCPOAuthConfigPreserved(t *testing.T) {
	for _, transport := range []string{"streamable-http", "sse"} {
		for _, oauth := range []string{
			`{"client_id":"grafana-client","scopes":["read","write"],"provider":{"audience":"grafana"},"enabled":true}`,
			`{}`,
		} {
			t.Run(transport+"/"+oauth, func(t *testing.T) {
				data := []byte(fmt.Sprintf(`{%s "mcpServers":{"mcp-grafana":{"type":%q,"url":"https://example.com/mcp","oauth":%s}}}`, mcpSchemaHeader, transport, oauth))
				cfg, warnings, err := ParseMCPConfig(data)
				if err != nil || len(warnings) != 0 {
					t.Fatalf("ParseMCPConfig: err=%v, warnings=%v", err, warnings)
				}
				server, ok := cfg.MCPServers["mcp-grafana"]
				if !ok {
					t.Fatal("OAuth server was dropped")
				}
				var want map[string]interface{}
				if err := json.Unmarshal([]byte(oauth), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(server.OAuth, want) {
					t.Fatalf("OAuth = %#v, want %#v", server.OAuth, want)
				}
			})
		}
	}
}

func TestMCPInvalidOAuthServerSkipped(t *testing.T) {
	for _, invalid := range []string{
		`{"type":"streamable-http","url":"https://example.com/mcp","oauth":"invalid"}`,
		`{"type":"sse","url":"https://example.com/mcp","oauth":[]}`,
		`{"type":"stdio","command":"example","oauth":{}}`,
		`{"type":"streamable-http","url":"https://example.com/mcp","oauth":{},"unknown":true}`,
	} {
		t.Run(invalid, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{%s "mcpServers":{"bad":%s,"good":{"type":"stdio","command":"example"}}}`, mcpSchemaHeader, invalid))
			cfg, warnings, err := ParseMCPConfig(data)
			if err != nil {
				t.Fatal(err)
			}
			if len(warnings) != 1 {
				t.Fatalf("warnings = %v, want one invalid server warning", warnings)
			}
			if len(cfg.MCPServers) != 1 || cfg.MCPServers["good"].Command != "example" {
				t.Fatalf("expected only the valid server, got %#v", cfg.MCPServers)
			}
		})
	}
}

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

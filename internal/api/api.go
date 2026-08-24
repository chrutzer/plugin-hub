package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/chrutzer/plugin-hub/internal/plugin"
	"github.com/chrutzer/plugin-hub/internal/registry"
)

type Server struct {
	reg *registry.Registry
}

var _ ServerInterface = (*Server)(nil)

func New(reg *registry.Registry) *Server {
	return &Server{reg: reg}
}

func (s *Server) Handler() http.Handler {
	return corsMiddleware(Handler(s))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toAuthor(a *plugin.Author) *Author {
	if a == nil {
		return nil
	}
	return &Author{
		Name:  stringPtr(a.Name),
		Email: stringPtr(a.Email),
		Url:   stringPtr(a.URL),
	}
}

// toExtensions re-decodes each namespace's raw JSON into a generic object.
// ParseManifest already rejects non-object extension values, so this only
// fails on programmer error.
func toExtensions(ext map[string]json.RawMessage) *map[string]map[string]interface{} {
	if len(ext) == 0 {
		return nil
	}
	out := make(map[string]map[string]interface{}, len(ext))
	for namespace, raw := range ext {
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		out[namespace] = obj
	}
	return &out
}

func toSkills(skills []plugin.Skill) *[]Skill {
	if len(skills) == 0 {
		return nil
	}
	out := make([]Skill, len(skills))
	for i, sk := range skills {
		s := Skill{
			Name:          sk.Name,
			Description:   sk.Description,
			License:       stringPtr(sk.License),
			Compatibility: stringPtr(sk.Compatibility),
			AllowedTools:  stringPtr(sk.AllowedTools),
		}
		if len(sk.Metadata) > 0 {
			m := sk.Metadata
			s.Metadata = &m
		}
		out[i] = s
	}
	return &out
}

func toMCP(servers map[string]plugin.MCPServer) *map[string]MCPServer {
	if len(servers) == 0 {
		return nil
	}
	out := make(map[string]MCPServer, len(servers))
	for name, srv := range servers {
		m := MCPServer{
			Type:    MCPServerType(srv.Type),
			Command: stringPtr(srv.Command),
			Cwd:     stringPtr(srv.Cwd),
			Url:     stringPtr(srv.URL),
		}
		if len(srv.Args) > 0 {
			args := srv.Args
			m.Args = &args
		}
		if len(srv.Env) > 0 {
			env := srv.Env
			m.Env = &env
		}
		if len(srv.Headers) > 0 {
			headers := srv.Headers
			m.Headers = &headers
		}
		out[name] = m
	}
	return &out
}

func toPlugin(p *plugin.Plugin) Plugin {
	pl := Plugin{
		Name:        p.Manifest.Name,
		Version:     stringPtr(p.Manifest.Version),
		Description: stringPtr(p.Manifest.Description),
		Author:      toAuthor(p.Manifest.Author),
		Homepage:    stringPtr(p.Manifest.Homepage),
		Repository:  stringPtr(p.Manifest.Repository),
		License:     stringPtr(p.Manifest.License),
		Source:      p.SourceName,
		Extensions:  toExtensions(p.Manifest.Extensions),
		Skills:      toSkills(p.Skills),
	}
	if len(p.Manifest.Keywords) > 0 {
		keywords := p.Manifest.Keywords
		pl.Keywords = &keywords
	}
	if len(p.Warnings) > 0 {
		warnings := p.Warnings
		pl.Warnings = &warnings
	}
	if p.MCP != nil {
		pl.Mcp = toMCP(p.MCP.MCPServers)
	}
	return pl
}

func (s *Server) ListPlugins(w http.ResponseWriter, r *http.Request) {
	list := s.reg.List()
	out := make([]Plugin, 0, len(list))
	for _, p := range list {
		out = append(out, toPlugin(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) DownloadPlugin(w http.ResponseWriter, r *http.Request, name string) {
	name, ok := strings.CutSuffix(name, ".zip")
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "plugin not found"})
		return
	}
	p, ok := s.reg.Get(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "plugin not found"})
		return
	}
	if _, err := os.Stat(p.ZipPath); err != nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "plugin archive not available"})
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(name)+`.zip"`)
	http.ServeFile(w, r, p.ZipPath)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

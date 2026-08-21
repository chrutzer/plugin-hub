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
	mux *http.ServeMux
}

func New(reg *registry.Registry) *Server {
	s := &Server{reg: reg, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /", s.handleList)
	s.mux.HandleFunc("GET /{name}", s.handleGet)
	return s
}

func (s *Server) Handler() http.Handler {
	return corsMiddleware(s.mux)
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

type pluginSummary struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source"`
	Skills      []string `json:"skills,omitempty"`
	MCPServers  []string `json:"mcpServers,omitempty"`
}

type pluginDetail struct {
	pluginSummary
	Manifest manifestView                `json:"manifest"`
	Skills   []plugin.Skill              `json:"skills,omitempty"`
	MCP      map[string]plugin.MCPServer `json:"mcp,omitempty"`
	Warnings []string                    `json:"warnings,omitempty"`
}

type manifestView struct {
	Name        string                     `json:"name"`
	Version     string                     `json:"version,omitempty"`
	Description string                     `json:"description,omitempty"`
	Author      *plugin.Author             `json:"author,omitempty"`
	Homepage    string                     `json:"homepage,omitempty"`
	Repository  string                     `json:"repository,omitempty"`
	License     string                     `json:"license,omitempty"`
	Keywords    []string                   `json:"keywords,omitempty"`
	Extensions  map[string]json.RawMessage `json:"extensions,omitempty"`
}

func toManifestView(m *plugin.Manifest) manifestView {
	return manifestView{
		Name:        m.Name,
		Version:     m.Version,
		Description: m.Description,
		Author:      m.Author,
		Homepage:    m.Homepage,
		Repository:  m.Repository,
		License:     m.License,
		Keywords:    m.Keywords,
		Extensions:  m.Extensions,
	}
}

func summarize(p *plugin.Plugin) pluginSummary {
	var skillNames []string
	for _, sk := range p.Skills {
		skillNames = append(skillNames, sk.Name)
	}
	var mcpNames []string
	if p.MCP != nil {
		for name := range p.MCP.MCPServers {
			mcpNames = append(mcpNames, name)
		}
	}
	return pluginSummary{
		Name:        p.Manifest.Name,
		Version:     p.Manifest.Version,
		Description: p.Manifest.Description,
		Source:      p.SourceName,
		Skills:      skillNames,
		MCPServers:  mcpNames,
	}
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	list := s.reg.List()
	out := make([]pluginSummary, 0, len(list))
	for _, p := range list {
		out = append(out, summarize(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if rest, ok := strings.CutSuffix(name, ".zip"); ok {
		s.handleDownload(w, r, rest)
		return
	}

	p, ok := s.reg.Get(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found"})
		return
	}
	detail := pluginDetail{
		pluginSummary: summarize(p),
		Manifest:      toManifestView(p.Manifest),
		Skills:        p.Skills,
		Warnings:      p.Warnings,
	}
	if p.MCP != nil {
		detail.MCP = p.MCP.MCPServers
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request, name string) {
	p, ok := s.reg.Get(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin not found"})
		return
	}
	if _, err := os.Stat(p.ZipPath); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "plugin archive not available"})
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

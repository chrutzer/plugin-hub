package registry

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/chrutzer/plugin-hub/internal/config"
	"github.com/chrutzer/plugin-hub/internal/plugin"
	"github.com/chrutzer/plugin-hub/internal/source"
)

type Registry struct {
	cacheDir string
	sources  []string

	mu      sync.RWMutex
	plugins map[string]*plugin.Plugin
}

func New(cfg *config.Config) (*Registry, error) {
	cacheDir, err := os.MkdirTemp("", "plugin-hub-")
	if err != nil {
		return nil, fmt.Errorf("create temp cache dir: %w", err)
	}
	return &Registry{
		cacheDir: cacheDir,
		sources:  cfg.Sources,
		plugins:  map[string]*plugin.Plugin{},
	}, nil
}

// Reload fetches every configured source, extracts it, and loads the plugin.
// A failure on one source is logged and skipped; it does not affect others.
func (r *Registry) Reload() {
	plugins := map[string]*plugin.Plugin{}

	for i, src := range r.sources {
		p, zipPath, err := r.loadSource(i, src)
		if err != nil {
			log.Printf("source %q: %v", src, err)
			continue
		}
		p.SourceName = src
		p.ZipPath = zipPath
		for _, w := range p.Warnings {
			log.Printf("source %q: %s", src, w)
		}
		plugins[p.Manifest.Name] = p
	}

	r.mu.Lock()
	r.plugins = plugins
	r.mu.Unlock()
}

func (r *Registry) loadSource(index int, location string) (*plugin.Plugin, string, error) {
	srcDir := filepath.Join(r.cacheDir, fmt.Sprintf("source-%d", index))
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("create cache dir: %w", err)
	}

	zipPath := filepath.Join(srcDir, "plugin.zip")
	if err := source.Fetch(location, zipPath); err != nil {
		return nil, "", err
	}

	extractDir := filepath.Join(srcDir, "extracted")
	if err := os.RemoveAll(extractDir); err != nil {
		return nil, "", fmt.Errorf("clear extract dir: %w", err)
	}
	if err := source.Extract(zipPath, extractDir); err != nil {
		return nil, "", fmt.Errorf("extract zip: %w", err)
	}

	root, err := findPluginRoot(extractDir)
	if err != nil {
		return nil, "", err
	}

	p, err := plugin.Load(root)
	if err != nil {
		return nil, "", fmt.Errorf("load plugin: %w", err)
	}

	return p, zipPath, nil
}

// findPluginRoot returns extractDir itself if it contains plugin.json, or
// its single top-level subdirectory if that contains plugin.json instead
// (common when a zip wraps its contents in one directory).
func findPluginRoot(extractDir string) (string, error) {
	if _, err := os.Stat(filepath.Join(extractDir, "plugin.json")); err == nil {
		return extractDir, nil
	}

	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", fmt.Errorf("read extracted dir: %w", err)
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 1 {
		candidate := filepath.Join(extractDir, dirs[0])
		if _, err := os.Stat(filepath.Join(candidate, "plugin.json")); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("plugin.json not found in zip")
}

func (r *Registry) List() []*plugin.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*plugin.Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		list = append(list, p)
	}
	return list
}

func (r *Registry) Get(name string) (*plugin.Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

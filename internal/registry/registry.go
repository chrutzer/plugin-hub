package registry

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// Reload fetches every configured source, extracts it, and loads the
// plugin(s) it contains. A source's zip may either be a single plugin, or a
// bundle of nested zips (each in turn a plugin or another bundle). A failure
// on one source, or one plugin within a bundle, is logged and skipped; it
// does not affect others.
func (r *Registry) Reload() {
	plugins := map[string]*plugin.Plugin{}

	for i, src := range r.sources {
		loaded, err := r.loadSource(i, src)
		if err != nil {
			log.Printf("source %q: %v", src, err)
			continue
		}
		for _, p := range loaded {
			p.SourceName = src
			for _, w := range p.Warnings {
				log.Printf("source %q: %s", src, w)
			}
			plugins[p.Manifest.Name] = p
		}
	}

	r.mu.Lock()
	r.plugins = plugins
	r.mu.Unlock()
}

func (r *Registry) loadSource(index int, location string) ([]*plugin.Plugin, error) {
	srcDir := filepath.Join(r.cacheDir, fmt.Sprintf("source-%d", index))
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	zipPath := filepath.Join(srcDir, "source.zip")
	if err := source.Fetch(location, zipPath); err != nil {
		return nil, err
	}

	found, err := resolvePluginZips(zipPath)
	if err != nil {
		return nil, err
	}

	var plugins []*plugin.Plugin
	for _, f := range found {
		p, err := plugin.Load(f.root)
		if err != nil {
			log.Printf("source %q: load %q: %v", location, filepath.Base(f.zipPath), err)
			continue
		}
		p.ZipPath = f.zipPath
		plugins = append(plugins, p)
	}
	if len(plugins) == 0 {
		return nil, fmt.Errorf("no valid plugins found")
	}

	return plugins, nil
}

type foundPlugin struct {
	root    string
	zipPath string
}

// resolvePluginZips extracts zipPath and, if it directly contains a plugin
// (plugin.json at its root or single top-level subdirectory), returns it.
// Otherwise it treats zipPath as a bundle of nested zips and recurses into
// each of them, so bundles of bundles of plugins are also supported.
func resolvePluginZips(zipPath string) ([]foundPlugin, error) {
	extractDir := strings.TrimSuffix(zipPath, filepath.Ext(zipPath)) + "-extracted"
	if err := os.RemoveAll(extractDir); err != nil {
		return nil, fmt.Errorf("clear extract dir: %w", err)
	}
	if err := source.Extract(zipPath, extractDir); err != nil {
		return nil, fmt.Errorf("extract zip: %w", err)
	}

	if root, err := findPluginRoot(extractDir); err == nil {
		return []foundPlugin{{root: root, zipPath: zipPath}}, nil
	}

	nested, err := findNestedZips(extractDir)
	if err != nil {
		return nil, fmt.Errorf("plugin.json not found and no nested zips found")
	}

	var found []foundPlugin
	for _, nz := range nested {
		sub, err := resolvePluginZips(nz)
		if err != nil {
			log.Printf("bundle entry %q: %v", filepath.Base(nz), err)
			continue
		}
		found = append(found, sub...)
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("bundle contained no valid plugins")
	}
	return found, nil
}

// findNestedZips looks for *.zip files directly inside dir, or (if none are
// found there) inside dir's single top-level subdirectory, mirroring
// findPluginRoot's handling of zips that wrap their contents in one folder.
func findNestedZips(dir string) ([]string, error) {
	zips, err := zipsInDir(dir)
	if err != nil {
		return nil, err
	}
	if len(zips) > 0 {
		return zips, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read extracted dir: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 1 {
		return zipsInDir(filepath.Join(dir, dirs[0]))
	}
	return nil, fmt.Errorf("no nested zip files found")
}

func zipsInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var zips []string
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".zip") {
			zips = append(zips, filepath.Join(dir, e.Name()))
		}
	}
	return zips, nil
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

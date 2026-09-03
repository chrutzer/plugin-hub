package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultRefreshInterval = 5 * time.Minute

type Source struct {
	URL      string
	Token    string
	Branch   string
	Path     string
	Keywords []string
}

func (s *Source) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		s.URL = value.Value
		return nil
	}
	type plain struct {
		URL      string   `yaml:"url"`
		Token    string   `yaml:"token"`
		Branch   string   `yaml:"branch"`
		Path     string   `yaml:"path"`
		Keywords []string `yaml:"keywords"`
	}
	var p plain
	if err := value.Decode(&p); err != nil {
		return err
	}
	s.URL = p.URL
	s.Token = os.ExpandEnv(p.Token)
	s.Branch = p.Branch
	s.Path = p.Path
	s.Keywords = p.Keywords
	return nil
}

type Config struct {
	Sources         []Source `yaml:"sources"`
	RefreshInterval time.Duration
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	for i, s := range cfg.Sources {
		if s.URL == "" {
			return nil, fmt.Errorf("source %d: url must not be empty", i)
		}
	}

	cfg.RefreshInterval = defaultRefreshInterval
	if s := os.Getenv("REFRESH_INTERVAL"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("REFRESH_INTERVAL: %w", err)
		}
		cfg.RefreshInterval = d
	}

	return &cfg, nil
}

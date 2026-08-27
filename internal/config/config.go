package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Source struct {
	URL    string
	Token  string
	Branch string
}

func (s *Source) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		s.URL = value.Value
		return nil
	}
	type plain struct {
		URL    string `yaml:"url"`
		Token  string `yaml:"token"`
		Branch string `yaml:"branch"`
	}
	var p plain
	if err := value.Decode(&p); err != nil {
		return err
	}
	s.URL = p.URL
	s.Token = os.ExpandEnv(p.Token)
	s.Branch = p.Branch
	return nil
}

type Config struct {
	Sources []Source `yaml:"sources"`
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

	return &cfg, nil
}

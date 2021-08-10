package config

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// DefaultConfig creates a copy of the default config
func DefaultConfig() *Config {
	return &Config{
		Model:  DirConfig{DirName: "models", Package: "models"},
		Helper: DirConfig{DirName: "helpers", Package: "helpers"},
		Graph:  DirConfig{DirName: "graph", Package: "graph"},
		Schema: SchemaConfig{DirName: "schema", Package: "schema"},
	}
}

type PackageConfig struct {
	Filename string `yaml:"filename,omitempty"`
	Package  string `yaml:"package,omitempty"`
}

type DirConfig struct {
	DirName string `yaml:"dir"`
	Package string `yaml:"package,omitempty"`
}

type ResolverConfig struct {
	DirName string `yaml:"dir"`
	Package string `yaml:"package,omitempty"`
	Type    string `yaml:"type,omitempty"`
}

type FederationConfig struct {
	DirName       string           `yaml:"dir"`
	Package       string           `yaml:"package,omitempty"`
	CurrentSchema string           `yaml:"currentSchema,omitempty"`
	ForeignIDs    ForeignIDColumns `yaml:"foreignIds,omitempty"`
}

type ForeignIDColumns struct {
	Column string
	Table  string
}

type SchemaConfig struct {
	DirName    string   `yaml:"dir"`
	Package    string   `yaml:"package,omitempty"`
	Directives []string `yaml:"directives,omitempty"`
}

type DatabaseConfig struct {
	ActivateSoftDeletes bool `yaml:"activateSoftDeletes"`
}

type Config struct {
	Model      DirConfig        `yaml:"model,omitempty"`
	Helper     DirConfig        `yaml:"helper,omitempty"`
	Graph      DirConfig        `yaml:"graph,omitempty"`
	Schema     SchemaConfig     `yaml:"schema,omitempty"`
	Resolver   ResolverConfig   `yaml:"resolver,omitempty"`
	Federation FederationConfig `yaml:"federation,omitempty"`
	Database   DatabaseConfig   `yaml:"database,omitempty"`
}

// LoadConfig reads the sinatra.yml config file
func LoadConfig(filename string) (*Config, error) {
	config := DefaultConfig()

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read config")
	}

	if err := yaml.UnmarshalStrict(b, config); err != nil {
		return nil, errors.Wrap(err, "unable to parse config")
	}

	return config, nil
}

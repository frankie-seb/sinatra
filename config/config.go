package config

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var cfgFilenames = []string{".sinatra.yml", "sinatra.yml", "sinatra.yaml"}

type PackageConfig struct {
	Filename string `yaml:"filename,omitempty"`
	Package  string `yaml:"package,omitempty"`
}

type DirConfig struct {
	DirName string `yaml:"dirname"`
	Package string `yaml:"package,omitempty"`
}

type ResolverConfig struct {
	DirName string `yaml:"dirname"`
	Package string `yaml:"package,omitempty"`
	Type    string `yaml:"type,omitempty"`
}

type FederationConfig struct {
	DirName       string           `yaml:"dirname"`
	Package       string           `yaml:"package,omitempty"`
	CurrentSchema string           `yaml:"currentSchema,omitempty"`
	ForeignIDs    ForeignIDColumns `yaml:"foreignIds,omitempty"`
}

type ForeignIDColumns struct {
	Column string
	Table  string
}

type SchemaConfig struct {
	DirName    string   `yaml:"dirname"`
	Package    string   `yaml:"package,omitempty"`
	Directives []string `yaml:"directives,omitempty"`
}

type ModelConfig struct {
	DirName string `yaml:"dir"`
	Package string `yaml:"package,omitempty"`
}

type DatabaseConfig struct {
	DBDriver         string   `yaml:"dbDriver"`
	DBName           string   `yaml:"dbName"`
	Schema           string   `yaml:"debug,omitempty"`
	Host             string   `yaml:"host,omitempty"`
	Port             string   `yaml:"port,omitempty"`
	UserName         string   `yaml:"user,omitempty"`
	Password         string   `yaml:"pass,omitempty"`
	SSLMode          string   `yaml:"sslMode,omitempty"`
	Blacklist        []string `yaml:"blacklist,omitempty"`
	Whitelist        []string `yaml:"whitelist,omitempty"`
	Debug            bool     `yaml:"debug,omitempty"`
	AddGlobal        bool     `yaml:"addGlobal,omitempty"`
	AddPanic         bool     `yaml:"addPanic,omitempty"`
	NoContext        bool     `yaml:"noContext,omitempty"`
	NoTests          bool     `yaml:"noTests,omitempty"`
	NoHooks          bool     `yaml:"noHooks,omitempty"`
	NoRowsAffected   bool     `yaml:"noRowsAffected,omitempty"`
	NoAutoTimestamps bool     `yaml:"noAutoTimestamps,omitempty"`
	Wipe             bool     `yaml:"wipe,omitempty"`
	AddSoftDeletes   bool     `yaml:"addSoftDeletes,omitempty"`
	StructTagCasing  string   `yaml:"noAutoTimestamps,omitempty"`
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

// DefaultConfig creates a copy of the default config
func DefaultConfig() *Config {
	return &Config{
		Model:    DirConfig{DirName: "models", Package: "models"},
		Helper:   DirConfig{DirName: "helpers", Package: "helpers"},
		Graph:    DirConfig{DirName: "graph", Package: "graph"},
		Schema:   SchemaConfig{DirName: "schema", Package: "schema"},
		Database: DatabaseConfig{DBDriver: "psql", Debug: false, AddGlobal: true, AddPanic: false, NoContext: false, NoTests: false, NoHooks: false, NoRowsAffected: false, NoAutoTimestamps: false, AddSoftDeletes: true, Wipe: true, StructTagCasing: "camel"},
	}
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

// LoadDefaultConfig loads the default config so that it is ready to be used
func LoadDefaultConfig() (*Config, error) {
	config := DefaultConfig()
	return config, nil
}

// LoadConfigFromDefaultLocations looks for a config file in the current directory, and all parent directories
// walking up the tree. The closest config file will be returned.
func LoadConfigFromDefaultLocations() (*Config, error) {
	cfgFile, err := findCfg()
	if err != nil {
		return nil, err
	}

	err = os.Chdir(filepath.Dir(cfgFile))
	if err != nil {
		return nil, errors.Wrap(err, "unable to enter config dir")
	}
	return LoadConfig(cfgFile)
}

// findCfg searches for the config file in this directory and all parents up the tree
// looking for the closest match
func findCfg() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "unable to get working dir to findCfg")
	}

	cfg := findCfgInDir(dir)

	for cfg == "" && dir != filepath.Dir(dir) {
		dir = filepath.Dir(dir)
		cfg = findCfgInDir(dir)
	}

	if cfg == "" {
		return "", os.ErrNotExist
	}

	return cfg, nil
}

func findCfgInDir(dir string) string {
	for _, cfgName := range cfgFilenames {
		path := filepath.Join(dir, cfgName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
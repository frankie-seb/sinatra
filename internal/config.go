package internal

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	gqlcon "github.com/99designs/gqlgen/codegen/config"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/ast"
	"gopkg.in/yaml.v2"
)

var cfgFilenames = []string{".sinatra.yml", "sinatra.yml", "sinatra.yaml"}

type PackageConfig struct {
	Filename string `yaml:"filename,omitempty"`
	Package  string `yaml:"package,omitempty"`
}

type BaseConfig struct {
	DirName string `yaml:"dirname"`
	Package string `yaml:"package,omitempty"`
}

type ResolverConfig struct {
	DirName string `yaml:"dirname"`
	Package string `yaml:"package,omitempty"`
	Type    string `yaml:"type,omitempty"`
}

type FederationConfig struct {
	Activate   bool               `yaml:"activate,omitempty"`
	ForeignIDs *[]ForeignIDColumn `yaml:"foreignids,omitempty"`
}

type ForeignIDColumn struct {
	Column string
	Table  string
}

type SchemaConfig struct {
	DirName         string   `yaml:"dirname"`
	Package         string   `yaml:"package,omitempty"`
	Directives      []string `yaml:"directives,omitempty"`
	SkipInputFields []string `yaml:"skipinputfields,omitempty"`
}

type ModelConfig struct {
	DirName string `yaml:"dir"`
	Package string `yaml:"package,omitempty"`
}

type DatabaseConfig struct {
	DBDriver         string   `yaml:"dbdriver"`
	DBName           string   `yaml:"dbname"`
	Schema           string   `yaml:"schema,omitempty"`
	Host             string   `yaml:"host,omitempty"`
	Port             string   `yaml:"port,omitempty"`
	User             string   `yaml:"user,omitempty"`
	Password         string   `yaml:"password,omitempty"`
	SSLMode          string   `yaml:"sslmode,omitempty"`
	Blacklist        []string `yaml:"blacklist,omitempty"`
	Whitelist        []string `yaml:"whitelist,omitempty"`
	Debug            bool     `yaml:"debug,omitempty"`
	AddGlobal        bool     `yaml:"addglobal,omitempty"`
	AddPanic         bool     `yaml:"addpanic,omitempty"`
	NoContext        bool     `yaml:"nocontext,omitempty"`
	NoTests          bool     `yaml:"notests,omitempty"`
	NoHooks          bool     `yaml:"nohooks,omitempty"`
	NoRowsAffected   bool     `yaml:"norowsaffected,omitempty"`
	NoAutoTimestamps bool     `yaml:"noautotimestamps,omitempty"`
	Wipe             bool     `yaml:"wipe,omitempty"`
	AddSoftDeletes   bool     `yaml:"addsoftdeletes,omitempty"`
	StructTagCasing  string   `yaml:"structtagcasing,omitempty"`
}

type Config struct {
	Model      BaseConfig       `yaml:"model,omitempty"`
	Helper     BaseConfig       `yaml:"helper,omitempty"`
	Graph      BaseConfig       `yaml:"graph,omitempty"`
	Schema     SchemaConfig     `yaml:"schema,omitempty"`
	Resolver   ResolverConfig   `yaml:"resolver,omitempty"`
	Federation FederationConfig `yaml:"federation,omitempty"`
	Database   DatabaseConfig   `yaml:"database,omitempty"`
}

var path2regex = strings.NewReplacer(
	`.`, `\.`,
	`*`, `.+`,
	`\`, `[\\/]`,
	`/`, `[\\/]`,
)

// DefaultConfig creates a copy of the default config
func DefaultConfig() *Config {
	return &Config{
		Model:    BaseConfig{DirName: "models", Package: "models"},
		Helper:   BaseConfig{DirName: "helpers", Package: "helpers"},
		Graph:    BaseConfig{DirName: "graph", Package: "graph"},
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

// GenerateGqlgenConfig
func LoadGqlgenConfig(cfg *Config) (*gqlcon.Config, error) {
	config := gqlcon.DefaultConfig()

	defaultDirectives := map[string]gqlcon.DirectiveConfig{
		"skip":       {SkipRuntime: true},
		"include":    {SkipRuntime: true},
		"deprecated": {SkipRuntime: true},
	}

	config.SchemaFilename = gqlcon.StringList{cfg.Schema.DirName + "/*.graphql"}
	config.Exec.Filename = cfg.Graph.DirName + "/exec.go"
	config.Exec.Package = cfg.Graph.Package
	config.Model.Filename = cfg.Model.DirName + "models.go"
	config.Model.Package = cfg.Model.Package
	config.Resolver.Filename = cfg.Resolver.DirName + "/resolver.go"
	config.Models = gqlcon.TypeMap{
		"ConnectionBackwardPagination": gqlcon.TypeMapEntry{
			Model: gqlcon.StringList{"github.com/FrankieHealth/be-base/helpers.ConnectionBackwardPagination"},
		},
		"ConnectionForwardPagination": gqlcon.TypeMapEntry{
			Model: gqlcon.StringList{"github.com/FrankieHealth/be-base/helpers.ConnectionBackwardPagination"},
		},
		"ConnectionPagination": gqlcon.TypeMapEntry{
			Model: gqlcon.StringList{"github.com/FrankieHealth/be-base/helpers.ConnectionBackwardPagination"},
		},
		"SortDirection": gqlcon.TypeMapEntry{
			Model: gqlcon.StringList{"github.com/FrankieHealth/be-base/helpers.ConnectionBackwardPagination"},
		},
	}

	if cfg.Federation.Activate {
		config.AutoBind = gqlcon.StringList{cfg.Graph.DirName}
		config.Federation.Filename = cfg.Graph.DirName + "/federation.go"
		config.Federation.Package = cfg.Graph.Package
	}

	preGlobbing := config.SchemaFilename

	for key, value := range defaultDirectives {
		if _, defined := config.Directives[key]; !defined {
			config.Directives[key] = value
		}
	}

	config.SchemaFilename = gqlcon.StringList{}
	for _, f := range preGlobbing {
		var matches []string
		var err error

		// for ** we want to override default globbing patterns and walk all
		// subdirectories to match schema files.
		if strings.Contains(f, "**") {
			pathParts := strings.SplitN(f, "**", 2)
			rest := strings.TrimPrefix(strings.TrimPrefix(pathParts[1], `\`), `/`)
			// turn the rest of the glob into a regex, anchored only at the end because ** allows
			// for any number of dirs in between and walk will let us match against the full path name
			globRe := regexp.MustCompile(path2regex.Replace(rest) + `$`)

			if err := filepath.Walk(pathParts[0], func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if globRe.MatchString(strings.TrimPrefix(path, pathParts[0])) {
					matches = append(matches, path)
				}

				return nil
			}); err != nil {
				return nil, errors.Wrapf(err, "failed to walk schema at root %s", pathParts[0])
			}
		} else {
			matches, err = filepath.Glob(f)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to glob schema filename %s", f)
			}
		}

		for _, m := range matches {
			if config.SchemaFilename.Has(m) {
				continue
			}
			config.SchemaFilename = append(config.SchemaFilename, m)
		}
	}

	for _, filename := range config.SchemaFilename {
		filename = filepath.ToSlash(filename)
		var err error
		var schemaRaw []byte
		schemaRaw, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, errors.Wrap(err, "unable to open schema")
		}

		config.Sources = append(config.Sources, &ast.Source{Name: filename, Input: string(schemaRaw)})
	}

	return config, nil
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

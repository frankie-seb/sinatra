package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/ast"
	"gopkg.in/yaml.v2"
)

// ImportPathForDir takes a path and returns a golang import path for the package
func ImportPathForDir(dir string) (res string) {
	dir, err := filepath.Abs(dir)

	if err != nil {
		panic(err)
	}
	dir = filepath.ToSlash(dir)

	modDir, ok := goModuleRoot(dir)
	if ok {
		return modDir
	}

	for _, gopath := range gopaths {
		if len(gopath) < len(dir) && strings.EqualFold(gopath, dir[0:len(gopath)]) {
			return dir[len(gopath)+1:]
		}
	}

	return ""
}

// goModuleRoot returns the root of the current go module if there is a go.mod file in the directory tree
// If not, it returns false
func goModuleRoot(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}
	dir = filepath.ToSlash(dir)
	modDir := dir
	assumedPart := ""
	for {
		f, err := ioutil.ReadFile(filepath.Join(modDir, "go.mod"))
		if err == nil {
			// found it, stop searching
			return string(modregex.FindSubmatch(f)[1]) + assumedPart, true
		}

		assumedPart = "/" + filepath.Base(modDir) + assumedPart
		parentDir, err := filepath.Abs(filepath.Join(modDir, ".."))
		if err != nil {
			panic(err)
		}

		if parentDir == modDir {
			// Walked all the way to the root and didnt find anything :'(
			break
		}
		modDir = parentDir
	}
	return "", false
}

var modregex = regexp.MustCompile(`module ([^\s]*)`)

// LoadConfig reads the gqlgen.yml config file
func LoadConfig(filename string) (*Config, error) {
	config := DefaultConfig()

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read config")
	}

	if err := yaml.UnmarshalStrict(b, config); err != nil {
		return nil, errors.Wrap(err, "unable to parse config")
	}

	defaultDirectives := map[string]DirectiveConfig{
		"skip":       {SkipRuntime: true},
		"include":    {SkipRuntime: true},
		"deprecated": {SkipRuntime: true},
	}

	for key, value := range defaultDirectives {
		if _, defined := config.Directives[key]; !defined {
			config.Directives[key] = value
		}
	}

	preGlobbing := config.SchemaFilename
	config.SchemaFilename = StringList{}
	for _, f := range preGlobbing {
		var matches []string

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

func (c *Config) Init() error {
	if c.Packages == nil {
		c.Packages = &code.Packages{}
	}

	if c.Schema == nil {
		if err := c.LoadSchema(); err != nil {
			return err
		}
	}

	err := c.injectTypesFromSchema()
	if err != nil {
		return err
	}

	err = c.autobind()
	if err != nil {
		return err
	}

	c.injectBuiltins()

	// prefetch all packages in one big packages.Load call
	pkgs := []string{
		"github.com/99designs/gqlgen/graphql",
		"github.com/99designs/gqlgen/graphql/introspection",
	}
	pkgs = append(pkgs, c.Models.ReferencedPackages()...)
	pkgs = append(pkgs, c.AutoBind...)
	c.Packages.LoadAll(pkgs...)

	//  check everything is valid on the way out
	err = c.check()
	if err != nil {
		return err
	}

	return nil
}

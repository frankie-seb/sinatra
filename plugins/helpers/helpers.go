package helpers

import (
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/frankie-seb/sinatra/internal"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin"
	"github.com/iancoleman/strcase"
	"github.com/rs/zerolog/log"
	"github.com/vektah/gqlparser/v2/ast"
)

var pathRegex *regexp.Regexp

func init() {
	pathRegex = regexp.MustCompile(`src/(.*)`)
}

func NewHelperPlugin(cfg *internal.Config) plugin.Plugin {
	return &HelperPlugin{
		cfg:            cfg,
		rootImportPath: internal.GetRootImportPath(),
	}
}

type HelperPlugin struct {
	cfg            *internal.Config
	rootImportPath string
}

type FederationConfig struct {
	Activate bool
	Schema   string
}

type ModelBuild struct {
	DbModels    internal.DirConfig
	GraphModels internal.DirConfig
	PackageName string
	Federation  FederationConfig
	Interfaces  []*Interface
	Models      []*internal.Model
	Enums       []*internal.Enum
	Scalars     []string
}

func (t ModelBuild) Imports() []internal.Import {
	return []internal.Import{
		{
			Alias:      t.GraphModels.PackageName,
			ImportPath: t.GraphModels.Directory,
		},
		{
			Alias:      t.DbModels.PackageName,
			ImportPath: t.DbModels.Directory,
		},
	}
}

type Interface struct {
	Description string
	Name        string
}

type SchemaCols struct {
	Column    string
	TableName string
}

var _ plugin.ConfigMutator = &HelperPlugin{}

func (m *HelperPlugin) Name() string {
	return "sinatra-helpers"
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func copyConfig(cfg config.Config) *config.Config {
	return &cfg
}

func (m *HelperPlugin) MutateConfig(originalCfg *config.Config) error {
	b := &ModelBuild{
		DbModels: internal.DirConfig{
			Directory:   path.Join(m.rootImportPath, m.cfg.Model.DirName),
			PackageName: m.cfg.Model.Package,
		},
		GraphModels: internal.DirConfig{
			Directory:   path.Join(m.rootImportPath, m.cfg.Graph.DirName),
			PackageName: m.cfg.Graph.Package,
		},
		PackageName: m.cfg.Helper.Package,
		Federation: FederationConfig{
			Activate: m.cfg.Federation.Activate,
			Schema:   m.cfg.Database.Schema,
		},
	}

	cfg := copyConfig(*originalCfg)
	if err := os.MkdirAll(m.cfg.Helper.DirName, os.ModePerm); err != nil {
		log.Error().Err(err).Str("directory", m.cfg.Helper.DirName).Msg("could not create directories")
	}
	// log.Debug().Msg("[customization] looking for *_customized files")

	// log.Debug().Msg("[convert] get boiler models")
	boilerModels, boilerEnums := internal.GetBoilerModels(m.cfg.Model.DirName)

	// log.Debug().Msg("[convert] get extra's from schema")
	interfaces, enums, scalars := getExtrasFromSchema(cfg.Schema, boilerEnums)

	// log.Debug().Msg("[convert] get model with information")
	models := internal.GetModelsWithInformation(m.cfg.Model.Package, enums, originalCfg, boilerModels, []string{m.cfg.Graph.Package, m.cfg.Model.Package, "base_helpers"}, m.cfg.Federation.ForeignIDs)

	b.Models = models
	sort.Slice(b.Models, func(i, j int) bool { return b.Models[i].Name < b.Models[j].Name })
	b.Interfaces = interfaces
	sort.Slice(b.Interfaces, func(i, j int) bool { return b.Interfaces[i].Name < b.Interfaces[j].Name })
	b.Enums = enumsWithout(enums, []string{"SortDirection", "Sort"})
	sort.Slice(b.Enums, func(i, j int) bool { return b.Enums[i].Name < b.Enums[j].Name })
	b.Scalars = scalars
	sort.Slice(b.Scalars, func(i, j int) bool { return b.Scalars[i] < b.Scalars[j] })
	if len(b.Models) == 0 {
		log.Warn().Msg("no models found in graphql so skipping generation")
		return nil
	}

	// Manage join relations subqueries
	for _, model := range models {
		var js []internal.JoinRelationship
		if model.IsWhere {
			for _, v := range *m.cfg.Federation.JoinRelationships {
				if v.To == model.BoilerModel.TableName {
					// Convert to Snake Case
					j := internal.JoinRelationship{
						From:       ToSnakeCase(v.From),
						To:         ToSnakeCase(v.To),
						Via:        v.Via,
						FromColumn: v.FromColumn,
						ToColumn:   v.ToColumn,
					}
					js = append(js, j)
				}
				// Do the opposite
				if v.From == model.BoilerModel.TableName {
					// Convert to Snake Case
					j := internal.JoinRelationship{
						From:       ToSnakeCase(v.To),
						To:         ToSnakeCase(v.From),
						Via:        v.Via,
						FromColumn: v.ToColumn,
						ToColumn:   v.FromColumn,
					}
					js = append(js, j)
				}
			}
			model.JoinArray = js
		}
	}

	filesToGenerate := []string{
		"base.go",
		"lib.go",
		"common_filter.go",
		"preload.go",
	}

	// We get all function names from helper repository to check if any customizations are available
	// we ignore the files we generated by this plugin
	userDefinedFunctions, err := internal.GetFunctionNamesFromDir(m.cfg.Helper.Package, filesToGenerate)
	if err != nil {
		log.Err(err).Msg("could not parse user defined functions")
	}

	for _, fileName := range filesToGenerate {
		templateName := fileName + "tpl"

		templateContent, err := internal.GetTemplateContent(templateName)
		if err != nil {
			log.Err(err).Msg("error when reading " + templateName)
			continue
		}

		if renderError := internal.WriteTemplateFile(
			m.cfg.Helper.DirName+"/"+fileName,
			internal.Options{
				Template:             templateContent,
				PackageName:          m.cfg.Helper.Package,
				Data:                 b,
				UserDefinedFunctions: userDefinedFunctions,
			}); renderError != nil {
			log.Err(renderError).Msg("error while rendering " + templateName)
		}
	}

	return nil
}

func enumsWithout(enums []*internal.Enum, skip []string) []*internal.Enum {
	var a []*internal.Enum
	for _, e := range enums {
		var skipped bool
		for _, skip := range skip {
			if strings.HasSuffix(e.Name, skip) {
				skipped = true
			}
		}
		if !skipped {
			a = append(a, e)
		}
	}
	return a
}

func getExtrasFromSchema(schema *ast.Schema, boilerEnums []*internal.BoilerEnum) (interfaces []*Interface, enums []*internal.Enum, scalars []string) {
	for _, schemaType := range schema.Types {
		switch schemaType.Kind {
		case ast.Interface, ast.Union:
			interfaces = append(interfaces, &Interface{
				Description: schemaType.Description,
				Name:        schemaType.Name,
			})
		case ast.Enum:
			boilerEnum := internal.FindBoilerEnum(boilerEnums, schemaType.Name)
			it := &internal.Enum{
				Name:          schemaType.Name,
				PluralName:    internal.Plural(schemaType.Name),
				Description:   schemaType.Description,
				HasBoilerEnum: boilerEnum != nil,
				BoilerEnum:    boilerEnum,
			}
			for _, v := range schemaType.EnumValues {
				it.Values = append(it.Values, &internal.EnumValue{
					Name:            v.Name,
					NameLower:       strcase.ToLowerCamel(strings.ToLower(v.Name)),
					Description:     v.Description,
					BoilerEnumValue: internal.FindBoilerEnumValue(boilerEnum, v.Name),
				})
			}
			if strings.HasPrefix(it.Name, "_") {
				continue
			}
			enums = append(enums, it)
		case ast.Scalar:
			scalars = append(scalars, schemaType.Name)
		}
	}
	return
}

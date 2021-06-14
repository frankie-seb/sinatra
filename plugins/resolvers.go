package plugins

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/FrankieHealth/be-base/internal/utils"

	"github.com/rs/zerolog/log"

	"github.com/iancoleman/strcase"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/plugin"
)

func NewResolverPlugin(helpers, boilerModels, gqlModels Config, resolverPluginConfig ResolverPluginConfig) plugin.Plugin {
	return &ResolverPlugin{
		helpers:        helpers,
		boilerModels:   boilerModels,
		gqlModels:      gqlModels,
		pluginConfig:   resolverPluginConfig,
		rootImportPath: utils.GetRootImportPath(),
	}
}

type AuthorizationScope struct {
	ImportPath        string
	ImportAlias       string
	ScopeResolverName string
	BoilerColumnName  string
	AddHook           func(model *utils.BoilerModel, resolver *Resolver, templateKey string) bool
}

type ResolverPluginConfig struct {
	AuthorizationScopes []*AuthorizationScope
}

type ResolverPlugin struct {
	helpers        Config
	boilerModels   Config
	gqlModels      Config
	pluginConfig   ResolverPluginConfig
	rootImportPath string
}

var _ plugin.CodeGenerator = &ResolverPlugin{}

func (m *ResolverPlugin) Name() string {
	return "resolver-frankie-health"
}

func (m *ResolverPlugin) GenerateCode(data *codegen.Data) error {
	if !data.Config.Resolver.IsDefined() {
		return nil
	}

	// Get all models information
	log.Debug().Msg("[resolver] get boiler models")
	boilerModels, _ := utils.GetBoilerModels(m.boilerModels.Directory)
	log.Debug().Msg("[resolver] get models with information")
	models := GetModelsWithInformation(m.boilerModels, nil, data.Config, boilerModels, nil)
	log.Debug().Msg("[resolver] generate file")
	return m.generatePerSchema(data, models, boilerModels)
}

func groupByBoilerModelName(list []*Model) [][]*Model {
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	r := make([][]*Model, 0)
	i := 0
	var j int
	for {
		if i >= len(list) {
			break
		}
		for j = i + 1; j < len(list) && getFirstWord(list[i].Name) == getFirstWord(list[j].Name); j++ {
		}

		r = append(r, list[i:j])
		i = j
	}
	return r
}

func (m *ResolverPlugin) generatePerSchema(data *codegen.Data, models []*Model, _ []*utils.BoilerModel) error {
	file := File{}

	file.Imports = append(file.Imports, Import{
		Alias:      "dm",
		ImportPath: path.Join(m.rootImportPath, m.boilerModels.Directory),
	})

	file.Imports = append(file.Imports, Import{
		Alias:      "fm",
		ImportPath: path.Join(m.rootImportPath, m.gqlModels.Directory),
	})

	file.Imports = append(file.Imports, Import{
		Alias:      "gm",
		ImportPath: buildImportPath(m.rootImportPath, data.Config.Exec.ImportPath()),
	})

	for _, scope := range m.pluginConfig.AuthorizationScopes {
		file.Imports = append(file.Imports, Import{
			Alias:      scope.ImportAlias,
			ImportPath: scope.ImportPath,
		})
	}

	// Get model resolver template
	templateName := "resolver_sep.gotpl"
	templateContent, err := getTemplateContent(templateName)
	if err != nil {
		log.Err(err).Msg("error when reading " + templateName)
		return err
	}

	// Get common resolver template
	commonTemplateName := "common_resolver.gotpl"
	commonTemplateContent, err := getTemplateContent(commonTemplateName)
	if err != nil {
		log.Err(err).Msg("error when reading " + templateName)
		return err
	}

	dir, err := os.Getwd()

	if err != nil {
		log.Err(err).Msg("error when reading " + templateName)
	}

	// Sort the models
	rMod := groupByBoilerModelName(models)

	// Get the extension files
	extendedFiles := []string{"directives.go", "resolver.go", "auth.go"}

	for _, v := range rMod {
		extendedFiles = append(extendedFiles, "gqlgen_"+strings.ToLower(getFirstWord(v[0].Name))+".go")
	}

	extendedFunctions, err := utils.GetFunctionNamesFromDir("resolvers", extendedFiles)
	if err != nil {
		log.Err(err).Msg("could not parse user defined functions in resolver")
	}

	resolverBuild := &ResolverBuild{
		File:                &file,
		PackageName:         data.Config.Resolver.Package,
		ResolverType:        data.Config.Resolver.Type,
		HasRoot:             true,
		IsFederatedServer:   data.Config.Federation.IsDefined(),
		Models:              models,
		AuthorizationScopes: m.pluginConfig.AuthorizationScopes,
	}

	// Write Common Resolver
	utils.WriteTemplateFile(dir+"/resolvers/resolver.go", utils.Options{
		Template:             commonTemplateContent,
		PackageName:          data.Config.Resolver.Package,
		Data:                 resolverBuild,
		UserDefinedFunctions: extendedFunctions,
	})

	// Add in helper import
	file.Imports = append(file.Imports, Import{
		Alias:      ".",
		ImportPath: path.Join(m.rootImportPath, m.helpers.Directory),
	})

	// Model Resolver
	for _, v := range rMod {
		file.Resolvers = []*Resolver{}

		for _, o := range data.Objects {
			if o.HasResolvers() {
				file.Objects = append(file.Objects, o)
			}
			for _, f := range o.Fields {
				if !f.IsResolver {
					continue
				}
				n := f.Name

				replace := map[string]string{
					"create": "",
					"update": "",
					"delete": "",
				}

				for s, r := range replace {
					n = strings.Replace(n, s, r, -1)
				}

				if strings.EqualFold(getFirstWord(v[0].Name), getFirstWord(n)) || strings.EqualFold(utils.Plural(getFirstWord(v[0].Name)), getFirstWord(n)) {
					resolver := &Resolver{
						Object:         o,
						Field:          f,
						Implementation: `panic("not implemented yet")`,
					}
					enhanceResolver(resolver, models)
					if resolver.Model.BoilerModel != nil && resolver.Model.BoilerModel.Name != "" {
						file.Resolvers = append(file.Resolvers, resolver)
					} else if resolver.Field.GoFieldName != "Node" {
						log.Debug().Str("resolver", resolver.Object.Name).Str("field", resolver.Field.GoFieldName).Msg(
							"skipping resolver since no model found")
					}
				}
			}

		}

		resolverBuild.Models = v

		utils.WriteTemplateFile(dir+"/resolvers/gqlgen_"+strings.ToLower(getFirstWord(v[0].Name))+".go", utils.Options{
			Template:             templateContent,
			PackageName:          data.Config.Resolver.Package,
			Data:                 resolverBuild,
			UserDefinedFunctions: extendedFunctions,
		})
	}
	// Replace text in resolvers
	err = filepath.Walk(dir+"/resolvers", ReplaceGeneratedText)
	if err != nil {
		panic(err)
	}

	return nil
}

func buildImportPath(rootImportPath, directory string) string {
	index := strings.Index(directory, rootImportPath)
	if index > 0 {
		return directory[index:]
	}
	return directory
}

type ResolverBuild struct {
	*File
	HasRoot             bool
	PackageName         string
	ResolverType        string
	IsFederatedServer   bool
	Models              []*Model
	AuthorizationScopes []*AuthorizationScope
	TryHook             func(string) bool
}

type File struct {
	// These are separated because the type definition of the resolver object may live in a different file from the
	// resolver method implementations, for example when extending a type in a different graphql schema file
	Objects         []*codegen.Object
	Resolvers       []*Resolver
	Imports         []Import
	RemainingSource string
}

type Resolver struct {
	Object *codegen.Object
	Field  *codegen.Field

	Implementation            string
	IsSingle                  bool
	IsList                    bool
	IsListForward             bool
	IsListBackward            bool
	IsCreate                  bool
	IsUpdate                  bool
	IsDelete                  bool
	IsBatchCreate             bool
	IsBatchUpdate             bool
	IsBatchDelete             bool
	ResolveOrganizationID     bool // TODO: something more pluggable
	ResolveUserOrganizationID bool // TODO: something more pluggable
	ResolveUserID             bool // TODO: something more pluggable
	Model                     Model
	InputModel                Model
	BoilerWhiteList           string
	PublicErrorKey            string
	PublicErrorMessage        string
}

func (rb *ResolverBuild) getResolverType(ty string) string {
	for _, imp := range rb.Imports {
		if strings.Contains(ty, imp.ImportPath) {
			if imp.Alias != "" {
				ty = strings.Replace(ty, imp.ImportPath, imp.Alias, -1)
			} else {
				ty = strings.Replace(ty, imp.ImportPath, "", -1)
			}
		}
	}
	return ty
}

func (rb *ResolverBuild) ShortResolverDeclaration(r *Resolver) string {
	res := "(ctx context.Context"

	if !r.Field.Object.Root {
		res += fmt.Sprintf(", obj %s", rb.getResolverType(r.Field.Object.Reference().String()))
	}
	for _, arg := range r.Field.Args {
		res += fmt.Sprintf(", %s %s", arg.VarName, rb.getResolverType(arg.TypeReference.GO.String()))
	}

	result := rb.getResolverType(r.Field.TypeReference.GO.String())
	if r.Field.Object.Stream {
		result = "<-chan " + result
	}

	res += fmt.Sprintf(") (%s, error)", result)
	return res
}

func enhanceResolver(r *Resolver, models []*Model) { //nolint:gocyclo
	nameOfResolver := r.Field.GoFieldName

	// get model names + model convert information
	modelName, inputModelName := getModelNames(nameOfResolver, false)
	// modelPluralName, _ := getModelNames(nameOfResolver, true)

	model := findModelOrEmpty(models, modelName)
	inputModel := findModelOrEmpty(models, inputModelName)

	// save for later inside file
	r.Model = model
	r.InputModel = inputModel

	switch r.Object.Name {
	case "Mutation":
		r.IsCreate = containsPrefixAndPartAfterThatIsSingle(nameOfResolver, "Create")
		r.IsUpdate = containsPrefixAndPartAfterThatIsSingle(nameOfResolver, "Update")
		r.IsDelete = containsPrefixAndPartAfterThatIsSingle(nameOfResolver, "Delete")
		r.IsBatchCreate = containsPrefixAndPartAfterThatIsPlural(nameOfResolver, "Create")
		r.IsBatchUpdate = containsPrefixAndPartAfterThatIsPlural(nameOfResolver, "Update")
		r.IsBatchDelete = containsPrefixAndPartAfterThatIsPlural(nameOfResolver, "Delete")
	case "Query":
		isPlural := utils.IsPlural(nameOfResolver)
		if isPlural {
			r.IsList = isPlural
			r.IsListBackward = strings.Contains(r.Field.GoFieldName, "first int") &&
				strings.Contains(r.Field.GoFieldName, "after *string")
			r.IsListBackward = strings.Contains(r.Field.GoFieldName, "last int") &&
				strings.Contains(r.Field.GoFieldName, "before *string")
		}

		r.IsSingle = !r.IsList
	case "Subscription":
		// TODO: generate helpers for subscription
	default:
		log.Warn().Str("unknown", r.Object.Name).Msg(
			"only Query and Mutation are handled we don't recognize the following")
	}

	lmName := strcase.ToLowerCamel(model.Name)
	lmpName := strcase.ToLowerCamel(model.PluralName)
	r.PublicErrorKey = "public"

	if (r.IsCreate || r.IsDelete || r.IsUpdate) && strings.HasSuffix(lmName, "Batch") {
		r.PublicErrorKey += "One"
	}
	r.PublicErrorKey += model.Name

	switch {
	case r.IsSingle:
		r.PublicErrorKey += "Single"
		r.PublicErrorMessage = "could not get " + lmName
	case r.IsList:
		r.PublicErrorKey += "List"
		r.PublicErrorMessage = "could not list " + lmpName
	case r.IsCreate:
		r.PublicErrorKey += "Create"
		r.PublicErrorMessage = "could not create " + lmName
	case r.IsUpdate:
		r.PublicErrorKey += "Update"
		r.PublicErrorMessage = "could not update " + lmName
	case r.IsDelete:
		r.PublicErrorKey += "Delete"
		r.PublicErrorMessage = "could not delete " + lmName
	case r.IsBatchCreate:
		r.PublicErrorKey += "BatchCreate"
		r.PublicErrorMessage = "could not create " + lmpName
	case r.IsBatchUpdate:
		r.PublicErrorKey += "BatchUpdate"
		r.PublicErrorMessage = "could not update " + lmpName
	case r.IsBatchDelete:
		r.PublicErrorKey += "BatchDelete"
		r.PublicErrorMessage = "could not delete " + lmpName
	}

	r.PublicErrorKey += "Error"
}

func findModelOrEmpty(models []*Model, modelName string) Model {
	if modelName == "" {
		return Model{}
	}
	for _, m := range models {
		if m.Name == modelName {
			return *m
		}
	}
	return Model{}
}

var InputTypes = []string{"Create", "Update", "Delete"} //nolint:gochecknoglobals

func getModelNames(v string, plural bool) (modelName, inputModelName string) {
	var prefix string
	var isInputType bool
	for _, inputType := range InputTypes {
		if strings.HasPrefix(v, inputType) {
			isInputType = true
			v = strings.TrimPrefix(v, inputType)
			prefix = inputType
		}
	}
	var s string
	if plural {
		s = utils.Plural(v)
	} else {
		s = utils.Singular(v)
	}

	if isInputType {
		return s, s + prefix + "Input"
	}

	return s, ""
}

func containsPrefixAndPartAfterThatIsSingle(v string, prefix string) bool {
	partAfterThat := strings.TrimPrefix(v, prefix)
	return strings.HasPrefix(v, prefix) && utils.IsSingular(partAfterThat)
}

func containsPrefixAndPartAfterThatIsPlural(v string, prefix string) bool {
	partAfterThat := strings.TrimPrefix(v, prefix)
	return strings.HasPrefix(v, prefix) && utils.IsPlural(partAfterThat)
}

func ReplaceGeneratedText(path string, fi os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	matched, err := filepath.Match("*.go", fi.Name())

	if err != nil {
		panic(err)
	}

	if matched {
		read, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err)
		}

		newContents := strings.Replace(string(read), "// OVERIDESTART", "/*", -1)
		newContents2 := strings.Replace(string(newContents), "// OVERIDEEND", "*/", -1)

		err = ioutil.WriteFile(path, []byte(newContents2), 0)
		if err != nil {
			panic(err)
		}

	}

	return nil
}

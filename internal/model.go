package internal

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"github.com/99designs/gqlgen/codegen/config"
	gqlgenTemplates "github.com/99designs/gqlgen/codegen/templates"
	"github.com/iancoleman/strcase"
	"github.com/rs/zerolog/log"
	"github.com/vektah/gqlparser/v2/ast"
)

const modelPackage = "base_helpers."

type Preload struct {
	Key           string
	ColumnSetting ColumnSetting
}

type IgnoreTypeMatch struct {
	gql    string
	boiler string
}

type ColumnSetting struct {
	Name                  string
	RelationshipModelName string
	IDAvailable           bool
}

type Model struct {
	Name           string
	PluralName     string
	BoilerModel    *BoilerModel
	PrimaryKeyType string
	Fields         []*Field
	IsNormal       bool
	IsInput        bool
	IsCreateInput  bool
	IsUpdateInput  bool
	IsNormalInput  bool
	IsPayload      bool
	IsConnection   bool
	IsEdge         bool
	IsOrdering     bool
	IsWhere        bool
	IsFilter       bool
	IsPreloadable  bool
	PreloadArray   []Preload
	JoinArray      []JoinRelationship

	HasPrimaryStringID bool
	Description        string
	PureFields         []*ast.FieldDefinition
	Implements         []string
}

type Field struct {
	Name               string
	JSONName           string
	PluralName         string
	Type               string
	TypeWithoutPointer string
	IsNumberID         bool
	IsPrimaryNumberID  bool
	IsPrimaryStringID  bool
	IsPrimaryID        bool
	IsRequired         bool
	IsPlural           bool
	ConvertConfig      FieldConfig
	Enum               *Enum
	// relation stuff
	IsRelation bool
	IsObject   bool
	// boiler relation stuff is inside this field
	BoilerField BoilerField
	// graphql relation ship can be found here
	Relationship *Model
	IsOr         bool
	IsAnd        bool
	IsJSON       bool

	// Some stuff
	Description  string
	OriginalType types.Type

	// For unmapped schema columns
	IsID      bool
	IsIDTable string
}

type FieldConfig struct {
	IsCustom         bool
	ToBoiler         string
	ToGraphQL        string
	GraphTypeAsText  string
	BoilerTypeAsText string
}

type Enum struct {
	Description   string
	Name          string
	PluralName    string
	Values        []*EnumValue
	HasBoilerEnum bool
	BoilerEnum    *BoilerEnum
}

type EnumValue struct {
	Description     string
	Name            string
	NameLower       string
	BoilerEnumValue *BoilerEnumValue
}

type Import struct {
	Alias      string
	ImportPath string
}

type DirConfig struct {
	Directory   string
	PackageName string
}

func GetModelsWithInformation(
	modelPackage string,
	enums []*Enum,
	cfg *config.Config,
	boilerModels []*BoilerModel,
	ignoreTypePrefixes []string,
	foreignIDs *[]ForeignIDColumn) []*Model {
	// get models based on the schema and sqlboiler structs
	models := getModelsFromSchema(cfg.Schema, boilerModels)

	// Now we have all model's let enhance them with fields
	enhanceModelsWithFields(enums, cfg.Schema, cfg, models, ignoreTypePrefixes, foreignIDs)

	// Add preload maps
	enhanceModelsWithPreloadArray(modelPackage, models)

	// Sort in same order
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	for _, m := range models {
		cfg.Models.Add(m.Name, cfg.Model.ImportPath()+"."+gqlgenTemplates.ToGo(m.Name))
	}
	return models
}

func enhanceModelsWithFields(enums []*Enum, schema *ast.Schema, cfg *config.Config,
	models []*Model, ignoreTypePrefixes []string, foreignIDs *[]ForeignIDColumn) {
	binder := cfg.NewBinder()

	// Generate the basic of the fields
	for _, m := range models {
		// Let's convert the pure ast fields to something usable for our templates
		for _, field := range m.PureFields {
			fieldDef := schema.Types[field.Type.Name()]

			// This calls some qglgen boilerType which gets the gqlgen type
			typ, err := getAstFieldType(binder, schema, cfg, field)
			if err != nil {
				log.Err(err).Msg("could not get field type from graphql schema")
			}
			jsonName := getGraphqlFieldName(cfg, m.Name, field)
			name := gqlgenTemplates.ToGo(jsonName)

			// just some (old) Relay clutter which is not needed anymore + we won't do anything with it
			// in our database converts.
			if strings.EqualFold(name, "clientMutationId") {
				continue
			}

			// override type struct with qqlgen code
			typ = binder.CopyModifiersFromAst(field.Type, typ)
			if isStruct(typ) && (fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject) {
				typ = types.NewPointer(typ)
			}

			// generate some booleans because these checks will be used a lot
			isObject := fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject

			shortType := GetShortType(typ.String(), ignoreTypePrefixes)

			isPrimaryID := strings.EqualFold(name, "id")

			// get sqlboiler information of the field
			boilerField := findBoilerFieldOrForeignKey(m.BoilerModel.Fields, name, isObject)
			isString := strings.Contains(strings.ToLower(boilerField.Type), "string")
			isNumberID := strings.HasSuffix(name, "ID") && !isString

			isPrimaryNumberID := isPrimaryID && !isString

			isPrimaryStringID := isPrimaryID && isString

			if isPrimaryStringID {
				m.HasPrimaryStringID = isPrimaryStringID
			}
			if isPrimaryNumberID || isPrimaryStringID {
				m.PrimaryKeyType = boilerField.Type
			}

			isEdges := strings.HasSuffix(m.Name, "Connection") && name == "Edges"
			isPageInfo := strings.HasSuffix(m.Name, "Connection") && name == "PageInfo"
			isSort := strings.HasSuffix(m.Name, "Ordering") && name == "Sort"
			isSortDirection := strings.HasSuffix(m.Name, "Ordering") && name == "Direction"
			isCursor := strings.HasSuffix(m.Name, "Edge") && name == "Cursor"
			isNode := strings.HasSuffix(m.Name, "Edge") && name == "Node"
			IsID := false
			IsIDTable := ""

			// Check if Schema ID
			if foreignIDs != nil {
				for _, s := range *foreignIDs {
					if s.Column == name {
						IsID = true
						IsIDTable = s.Table
						isNumberID = true
					}
				}
			}

			// log some warnings when fields could not be converted
			if boilerField.Type == "" {
				switch {
				case m.IsPayload:
				case IsPlural(name):
				case (m.IsFilter || m.IsWhere) && (strings.EqualFold(name, "and") ||
					strings.EqualFold(name, "or") ||
					strings.EqualFold(name, "search") ||
					strings.EqualFold(name, "where")) ||
					isEdges ||
					isSort ||
					isSortDirection ||
					isPageInfo ||
					isCursor ||
					isNode:
					// ignore
				default:
					continue
				}
			}

			if boilerField.Name == "" {
				if m.IsPayload || m.IsFilter || m.IsWhere || m.IsOrdering || m.IsEdge || isPageInfo || isEdges {
				} else {
					// log.Debug().Str("model.field", m.Name+"."+name).Msg("boiler type not available")
					continue
				}
			}

			enum := findEnum(enums, shortType)

			field := &Field{
				Name:               name,
				JSONName:           jsonName,
				Type:               shortType,
				TypeWithoutPointer: strings.Replace(strings.TrimPrefix(shortType, "*"), ".", "Dot", -1),
				BoilerField:        boilerField,
				IsNumberID:         isNumberID,
				IsPrimaryID:        isPrimaryID,
				IsPrimaryNumberID:  isPrimaryNumberID,
				IsPrimaryStringID:  isPrimaryStringID,
				IsRelation:         boilerField.IsRelation,
				IsObject:           isObject,
				IsOr:               strings.EqualFold(name, "or"),
				IsAnd:              strings.EqualFold(name, "and"),
				IsPlural:           IsPlural(name),
				IsJSON:             strings.Contains(boilerField.Type, "JSON"),
				PluralName:         Plural(name),
				OriginalType:       typ,
				Description:        field.Description,
				Enum:               enum,
				IsID:               IsID,
				IsIDTable:          IsIDTable,
			}

			field.ConvertConfig = getConvertConfig(enums, m, field)
			m.Fields = append(m.Fields, field)
		}
	}

	for _, m := range models {
		for _, f := range m.Fields {
			if f.BoilerField.Relationship != nil {
				f.Relationship = findModel(models, f.BoilerField.Relationship.Name)
			}
		}
	}
}

func enhanceModelsWithPreloadArray(modelPackage string, models []*Model) {
	// first adding basic first level relations
	for _, model := range models {
		if !model.IsPreloadable {
			continue
		}

		modelPreloadMap := getPreloadMapForModel(modelPackage, model)

		sortedPreloadKeys := make([]string, 0, len(modelPreloadMap))
		for k := range modelPreloadMap {
			sortedPreloadKeys = append(sortedPreloadKeys, k)
		}
		sort.Strings(sortedPreloadKeys)

		model.PreloadArray = make([]Preload, len(sortedPreloadKeys))
		for i, k := range sortedPreloadKeys {
			columnSetting := modelPreloadMap[k]
			model.PreloadArray[i] = Preload{
				Key:           k,
				ColumnSetting: columnSetting,
			}
		}
	}
}

func getPreloadMapForModel(modelPackage string, model *Model) map[string]ColumnSetting {
	preloadMap := map[string]ColumnSetting{}
	for _, field := range model.Fields {
		// only relations are preloadable
		if !field.IsObject || !field.BoilerField.IsRelation {
			continue
		}
		key := field.JSONName
		name := fmt.Sprintf("%v.%vRels.%v", modelPackage, model.Name, foreignKeyToRel(field.BoilerField.Name))
		setting := ColumnSetting{
			Name:                  name,
			IDAvailable:           !field.IsPlural,
			RelationshipModelName: field.BoilerField.Relationship.TableName,
		}

		preloadMap[key] = setting
	}
	return preloadMap
}

// getAstFieldType check's if user has defined a
func getAstFieldType(binder *config.Binder, schema *ast.Schema, cfg *config.Config, field *ast.FieldDefinition) (
	types.Type, error) {
	var typ types.Type
	var err error

	fieldDef := schema.Types[field.Type.Name()]
	if cfg.Models.UserDefined(field.Type.Name()) {
		typ, err = binder.FindTypeFromName(cfg.Models[field.Type.Name()].Model[0])
		if err != nil {
			return typ, err
		}
	} else {
		switch fieldDef.Kind {
		case ast.Scalar:
			// no user defined model, referencing a default scalar
			typ = types.NewNamed(
				types.NewTypeName(0, cfg.Model.Pkg(), "string", nil),
				nil,
				nil,
			)

		case ast.Interface, ast.Union:
			// no user defined model, referencing a generated interface type
			typ = types.NewNamed(
				types.NewTypeName(0, cfg.Model.Pkg(), gqlgenTemplates.ToGo(field.Type.Name()), nil),
				types.NewInterfaceType([]*types.Func{}, []types.Type{}),
				nil,
			)

		case ast.Enum:
			// no user defined model, must reference a generated enum
			typ = types.NewNamed(
				types.NewTypeName(0, cfg.Model.Pkg(), gqlgenTemplates.ToGo(field.Type.Name()), nil),
				nil,
				nil,
			)

		case ast.Object, ast.InputObject:
			// no user defined model, must reference a generated struct
			typ = types.NewNamed(
				types.NewTypeName(0, cfg.Model.Pkg(), gqlgenTemplates.ToGo(field.Type.Name()), nil),
				types.NewStruct(nil, nil),
				nil,
			)

		default:
			panic(fmt.Errorf("unknown ast type %s", fieldDef.Kind))
		}
	}

	return typ, err
}

func isStruct(t types.Type) bool {
	_, is := t.Underlying().(*types.Struct)
	return is
}

func findBoilerFieldOrForeignKey(fields []*BoilerField, golangGraphQLName string, isObject bool) BoilerField {
	// get database friendly struct for this model
	for _, field := range fields {
		if isObject {
			// If it a relation check to see if a foreign key is available
			if strings.EqualFold(field.Name, golangGraphQLName+"ID") {
				return *field
			}
		}
		if strings.EqualFold(field.Name, golangGraphQLName) {
			return *field
		}
	}
	return BoilerField{}
}

func getConvertConfig(enums []*Enum, model *Model, field *Field) (fc FieldConfig) { //nolint:gocognit,gocyclo
	graphType := field.Type
	boilType := field.BoilerField.Type

	enum := findEnum(enums, field.TypeWithoutPointer)
	if enum != nil { //nolint:nestif
		fc.IsCustom = true
		fc.ToBoiler = strings.TrimPrefix(
			getToBoiler(
				getBoilerTypeAsText(boilType),
				getGraphTypeAsText(graphType),
			), modelPackage)

		fc.ToGraphQL = strings.TrimPrefix(
			getToGraphQL(
				getBoilerTypeAsText(boilType),
				getGraphTypeAsText(graphType),
			), modelPackage)
		// Check if string array col or others
	} else if graphType != boilType && !checkInIgnoreTypes(boilType, graphType) {
		fc.IsCustom = true
		if field.IsPrimaryID || field.IsNumberID && field.BoilerField.IsRelation || field.IsID {
			// TODO: more dynamic and universal
			fc.ToGraphQL = "VALUE"
			fc.ToBoiler = "VALUE"

			// first unpointer json type if is pointer
			if strings.HasPrefix(graphType, "*") {
				fc.ToBoiler = modelPackage + "PointerStringToString(VALUE)"
			}

			goToUint := getBoilerTypeAsText(boilType) + "ToUint"
			if goToUint == "IntToUint" {
				fc.ToGraphQL = "uint(VALUE)"
			} else if goToUint != "UintToUint" {
				fc.ToGraphQL = modelPackage + goToUint + "(VALUE)"
			}

			if field.IsPrimaryID {
				fc.ToGraphQL = model.Name + "IDToGraphQL(" + fc.ToGraphQL + ")"
			} else if field.IsNumberID && field.BoilerField.IsRelation {
				fc.ToGraphQL = field.BoilerField.Relationship.Name + "IDToGraphQL(" + fc.ToGraphQL + ")"
			} else if field.IsNumberID && field.IsID && !strings.HasPrefix(graphType, "*") {
				fc.ToGraphQL = "base_helpers.IDToGraphQL(" + fc.ToGraphQL + ",\"" + field.IsIDTable + "\")"
			} else if field.IsNumberID && field.IsID && strings.HasPrefix(graphType, "*") {
				fc.ToGraphQL = "base_helpers.IDToGraphQLPointer(" + fc.ToGraphQL + ",\"" + field.IsIDTable + "\")"
			}

			isInt := strings.HasPrefix(strings.ToLower(boilType), "int") && !strings.HasPrefix(strings.ToLower(boilType), "uint")
			isTime := strings.HasPrefix(strings.ToLower(boilType), "time")

			if strings.HasPrefix(boilType, "null") {
				fc.ToBoiler = fmt.Sprintf("base_helpers.IDToBoilerNullInt(m.%v)", field.BoilerField.Name)
				if isInt {
					fc.ToBoiler = fmt.Sprintf("base_helpers.NullUintToNullInt(%v)", fc.ToBoiler)
				}
				if isTime {
					fc.ToBoiler = fmt.Sprintf("base_helpers.NullDotTimeToPointerTime(%v)", fc.ToBoiler)
				}
			} else {
				fc.ToBoiler = fmt.Sprintf("base_helpers.IDToBoiler(%v)", fc.ToBoiler)
				if isInt {
					fc.ToBoiler = fmt.Sprintf("int(%v)", fc.ToBoiler)
				}
			}

			fc.ToGraphQL = strings.Replace(fc.ToGraphQL, "VALUE", "m."+field.BoilerField.Name, -1)
			fc.ToBoiler = strings.Replace(fc.ToBoiler, "VALUE", "m."+field.Name, -1)
		} else {
			fc.ToBoiler = getToBoiler(getBoilerTypeAsText(boilType), getGraphTypeAsText(graphType))
			fc.ToGraphQL = getToGraphQL(getBoilerTypeAsText(boilType), getGraphTypeAsText(graphType))

		}
	}

	// JSON let the user convert how it looks in a custom file
	if strings.Contains(boilType, "JSON") {
		fc.ToBoiler = modelPackage + strings.TrimPrefix(fc.ToBoiler, modelPackage)
		fc.ToGraphQL = modelPackage + strings.TrimPrefix(fc.ToGraphQL, modelPackage)
	}

	fc.GraphTypeAsText = getGraphTypeAsText(graphType)
	fc.BoilerTypeAsText = getBoilerTypeAsText(boilType)

	return //nolint:nakedret
}

func getToBoiler(boilType, graphType string) string {
	return modelPackage + getGraphTypeAsText(graphType) + "To" + getBoilerTypeAsText(boilType)
}

func getBoilerTypeAsText(boilType string) string {
	if strings.HasPrefix(boilType, "types.") {
		boilType = strings.TrimPrefix(boilType, "types.")
		boilType = strcase.ToCamel(boilType)
		boilType = "Types" + boilType
	}
	boilType = strings.Replace(boilType, ".", "Dot", -1)

	return strcase.ToCamel(boilType)
}

func checkInIgnoreTypes(gqlType string, boilType string) bool {
	res := false
	for _, t := range ignoreMatchTypes {
		if t.gql == gqlType && t.boiler == boilType {
			res = true
		}
	}
	return res
}

var ignoreMatchTypes = []IgnoreTypeMatch{
	{
		gql:    "types.StringArray",
		boiler: "[]string",
	},
}

func FindBoilerEnumValue(enum *BoilerEnum, name string) *BoilerEnumValue {
	if enum != nil {
		for _, v := range enum.Values {
			if strings.EqualFold(strings.TrimPrefix(v.Name, enum.Name), name) {
				return v
			}
		}
		log.Error().Str(enum.Name, name).Msg("could sqlboiler enum value")
	}

	return nil
}

func foreignKeyToRel(v string) string {
	return strings.TrimSuffix(strcase.ToCamel(v), "ID")
}

func FindBoilerEnum(enums []*BoilerEnum, graphType string) *BoilerEnum {
	for _, enum := range enums {
		if enum.Name == graphType {
			return enum
		}
	}
	return nil
}

func findModel(models []*Model, search string) *Model {
	for _, m := range models {
		if m.Name == search {
			return m
		}
	}
	return nil
}

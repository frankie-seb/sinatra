package internal

import (
	"regexp"
	"strings"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/iancoleman/strcase"
	"github.com/vektah/gqlparser/v2/ast"
)

func getModelsFromSchema(schema *ast.Schema, boilerModels []*BoilerModel) (models []*Model) { //nolint:gocognit,gocyclo
	for _, schemaType := range schema.Types {
		// skip boiler plate from ggqlgen, we only want the models
		if strings.HasPrefix(schemaType.Name, "_") {
			continue
		}

		switch schemaType.Kind {
		case ast.Object, ast.InputObject:
			{
				if schemaType == schema.Query ||
					schemaType == schema.Mutation ||
					schemaType == schema.Subscription {
					continue
				}
				modelName := schemaType.Name

				if strings.HasPrefix(modelName, "_") {
					continue
				}

				// We will try to find a corresponding boiler struct
				boilerModel := FindBoilerModel(boilerModels, getBaseModelFromName(modelName))

				isInput := doesEndWith(modelName, "Input")
				isCreateInput := doesEndWith(modelName, "CreateInput")
				isUpdateInput := doesEndWith(modelName, "UpdateInput")
				isFilter := doesEndWith(modelName, "Filter")
				isWhere := doesEndWith(modelName, "Where")
				isPayload := doesEndWith(modelName, "Payload")
				isEdge := doesEndWith(modelName, "Edge")
				isConnection := doesEndWith(modelName, "Connection")
				isPageInfo := modelName == "PageInfo"
				isOrdering := doesEndWith(modelName, "Ordering")

				var isPagination bool
				paginationTriggers := []string{
					"ConnectionBackwardPagination",
					"ConnectionPagination",
					"ConnectionForwardPagination",
				}
				for _, p := range paginationTriggers {
					if modelName == p {
						isPagination = true
					}
				}

				// if no boiler model is found
				if boilerModel == nil || boilerModel.Name == "" {
					if isInput || isWhere || isFilter || isPayload || isPageInfo || isPagination {
						// silent continue
						continue
					}
					continue
				}

				isNormalInput := isInput && !isCreateInput && !isUpdateInput
				isNormal := !isInput && !isWhere && !isFilter && !isPayload && !isEdge && !isConnection && !isOrdering

				m := &Model{
					Name:          modelName,
					Description:   schemaType.Description,
					PluralName:    Plural(modelName),
					BoilerModel:   boilerModel,
					IsInput:       isInput,
					IsFilter:      isFilter,
					IsWhere:       isWhere,
					IsUpdateInput: isUpdateInput,
					IsCreateInput: isCreateInput,
					IsNormalInput: isNormalInput,
					IsConnection:  isConnection,
					IsEdge:        isEdge,
					IsPayload:     isPayload,
					IsOrdering:    isOrdering,
					IsNormal:      isNormal,
					IsPreloadable: isNormal,
				}

				for _, implementor := range schema.GetImplements(schemaType) {
					m.Implements = append(m.Implements, implementor.Name)
				}

				m.PureFields = append(m.PureFields, schemaType.Fields...)
				models = append(models, m)
			}
		}
	}
	return //nolint:nakedret
}

func getBaseModelFromName(v string) string {
	v = safeTrim(v, "CreateInput")
	v = safeTrim(v, "UpdateInput")
	v = safeTrim(v, "Input")
	v = safeTrim(v, "Payload")
	v = safeTrim(v, "Where")
	v = safeTrim(v, "Filter")
	v = safeTrim(v, "Ordering")
	v = safeTrim(v, "Edge")
	v = safeTrim(v, "Connection")

	return v
}

func safeTrim(v string, trimSuffix string) string {
	if v != trimSuffix {
		v = strings.TrimSuffix(v, trimSuffix)
	}
	return v
}

func doesEndWith(s string, suffix string) bool {
	return strings.HasSuffix(s, suffix) && s != suffix
}

func getGraphqlFieldName(cfg *config.Config, modelName string, field *ast.FieldDefinition) string {
	name := field.Name
	if nameOveride := cfg.Models[modelName].Fields[field.Name].FieldName; nameOveride != "" {
		// TODO: map overrides to sqlboiler the other way around?
		name = nameOveride
	}
	return name
}

func findEnum(enums []*Enum, graphType string) *Enum {
	for _, enum := range enums {
		if enum.Name == graphType {
			return enum
		}
	}
	return nil
}

func getToGraphQL(boilType, graphType string) string {
	return modelPackage + getBoilerTypeAsText(boilType) + "To" + getGraphTypeAsText(graphType)
}

func getGraphTypeAsText(graphType string) string {
	if strings.HasPrefix(graphType, "*") {
		graphType = strings.TrimPrefix(graphType, "*")
		graphType = strcase.ToCamel(graphType)
		graphType = "Pointer" + graphType
	}
	return strcase.ToCamel(graphType)
}

func GetFirstWord(str string) string {
	re := regexp.MustCompile(`^([aA-zZ][a-z0-9_\-]+)`)
	w := re.FindString(str)
	strW := strings.Trim(w, " ")
	return strW
}

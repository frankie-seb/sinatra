package schema

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/frankie-seb/sinatra/internal"
	"github.com/iancoleman/strcase"
	"github.com/rs/zerolog/log"
)

const (
	indent    = "  "
	lineBreak = "\n"
)

type SchemaArr struct {
	Name string
	Data string
}

type HooksConfig struct {
	HookShouldAddModel func(model SchemaModel) bool
	HookShouldAddField func(model SchemaModel, field SchemaField) bool
	HookChangeField    func(model *SchemaModel, field *SchemaField)
	HookChangeFields   func(model *SchemaModel, fields []*SchemaField, parenType ParentType) []*SchemaField
	HookChangeModel    func(model *SchemaModel)
}

type SchemaGenerateConfig struct {
	MergeSchema bool
}

type SchemaModel struct {
	Name   string
	Fields []*SchemaField
}

type SchemaField struct {
	Name                 string
	Type                 string // String, ID, Integer
	InputWhereType       string
	InputCreateType      string
	InputUpdateType      string
	InputBatchUpdateType string
	InputBatchCreateType string
	BoilerField          *internal.BoilerField
	SkipInput            bool
	SkipWhere            bool
	SkipSort             bool
	SkipCreate           bool
	SkipUpdate           bool
	SkipBatchUpdate      bool
	SkipBatchCreate      bool
	InputDirectives      []string
	Directives           []string
}

func NewSchemaField(name string, typ string, boilerField *internal.BoilerField) *SchemaField {
	return &SchemaField{
		Name:                 name,
		Type:                 typ,
		InputWhereType:       typ,
		InputCreateType:      typ,
		InputUpdateType:      typ,
		InputBatchUpdateType: typ,
		InputBatchCreateType: typ,
		BoilerField:          boilerField,
	}
}

func (s *SchemaField) SetInputTypeForAllInputs(v string) {
	s.InputWhereType = v
	s.InputCreateType = v
	s.InputUpdateType = v
	s.InputBatchUpdateType = v
	s.InputBatchCreateType = v
}

func (s *SchemaField) SetSkipForAllInputs(v bool) {
	s.SkipInput = v
	s.SkipWhere = v
	s.SkipCreate = v
	s.SkipUpdate = v
	s.SkipBatchUpdate = v
	s.SkipBatchCreate = v
}

type ParentType string

const (
	ParentTypeNormal      ParentType = "Normal"
	ParentTypeWhere       ParentType = "Where"
	ParentTypeCreate      ParentType = "Create"
	ParentTypeUpdate      ParentType = "Update"
	ParentTypeBatchUpdate ParentType = "BatchUpdate"
	ParentTypeBatchCreate ParentType = "BatchCreate"
)

func SchemaWrite(cfg *internal.Config, hooks *HooksConfig) error {
	// Generate schema based on config
	schema := SchemaGet(
		cfg,
		hooks,
	)

	ch := make(chan error)

	for _, s := range schema {
		go func(s SchemaArr) {
			if err := writeContentToFile(s.Data, "schema/"+strings.ToLower(s.Name)+"_gen.graphql"); err != nil {
				log.Err(err).Msg("Could not write schema to disk")
				ch <- err
				return
			}
			formatFile("schema/" + strings.ToLower(s.Name) + "_gen.graphql")
			ch <- nil
		}(s)
	}

	if err := <-ch; err != nil {
		return err
	}

	return nil
}

func getDirectivesAsString(va []string) string {
	a := make([]string, len(va))
	for i, v := range va {
		a[i] = "@" + v
	}
	return strings.Join(a, " ")
}

func groupByModelName(list []*SchemaModel) [][]*SchemaModel {
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	r := make([][]*SchemaModel, 0)
	i := 0
	var j int
	for {
		if i >= len(list) {
			break
		}
		for j = i + 1; j < len(list) && internal.GetFirstWord(list[i].Name) == internal.GetFirstWord(list[j].Name); j++ {
		}

		r = append(r, list[i:j])
		i = j
	}
	return r
}

//nolint:gocognit,gocyclo
func SchemaGet(
	cfg *internal.Config,
	hooks *HooksConfig,
) []SchemaArr {
	d := []SchemaArr{}
	g := &SimpleWriter{}
	e := &SimpleWriter{}

	// Parse models and their fields based on the sqlboiler model directory
	boilerModels, boilerEnums := internal.GetBoilerModels(cfg.Model.DirName)

	models := executeHooksOnModels(boilerModelsToModels(boilerModels, cfg.Federation.ForeignIDs), hooks)

	grpMod := groupByModelName(models)

	// Directives GraphQL
	fullDirectives := make([]string, len(cfg.Schema.Directives))
	for i, defaultDirective := range cfg.Schema.Directives {
		fullDirectives[i] = "@" + defaultDirective
	}

	joinedDirectives := strings.Join(fullDirectives, " ")

	// Common File
	g.l(`scalar Any`)
	g.l(`scalar AnyFilter`)
	g.l(`scalar Date`)
	g.l(`scalar DateTime`)
	g.l(`scalar Time`)
	g.l(`scalar JSON`)

	g.br()

	g.l(`schema {`)
	g.tl(`query: Query`)
	g.tl(`mutation: Mutation`)
	g.l(`}`)

	g.br()

	// Common Types
	g.l("type Query {")
	g.tl("node(id: ID!): Node")
	g.l(`}`)

	g.l(`interface Node {`)
	g.tl(`id: ID!`)
	g.l(`}`)

	g.br()

	g.l(`type PageInfo {`)
	g.tl(`hasNextPage: Boolean!`)
	g.tl(`hasPreviousPage: Boolean!`)
	g.tl(`startCursor: String`)
	g.tl(`endCursor: String`)
	g.l(`}`)

	g.br()

	// Add helpers for filtering lists
	g.l(queryHelperStructs)

	g.br()

	// Generate sorting helpers
	g.l("enum SortDirection { ASC, DESC }")

	gVal := SchemaArr{
		Name: "Common",
		Data: g.s.String(),
	}

	d = append(d, gVal)

	en := SchemaArr{
		Name: "Enum",
		Data: e.s.String(),
	}

	for _, enum := range boilerEnums {

		//	enum UserRoleFilter { ADMIN, USER }
		e.l(fmt.Sprintf(enumFilterHelper, enum.Name))

		//	enum UserRole { ADMIN, USER }
		e.l("enum " + enum.Name + " {")
		for _, v := range enum.Values {
			e.tl(strcase.ToScreamingSnake(strings.TrimPrefix(v.Name, enum.Name)))
		}
		e.l("}")

		e.br()
	}

	d = append(d, en)

	if len(grpMod) > 0 {
		for i, grp := range grpMod {
			w := &SimpleWriter{}
			w.l("extend type Query {")
			for _, model := range grp {
				modelPluralName := internal.Plural(model.Name)

				// single models
				w.tl(strcase.ToLowerCamel(model.Name) + "(id: ID!): " + model.Name + "!" + joinedDirectives)

				// lists
				arguments := []string{
					"first: Int!",
					"after: String",
					"ordering: [" + model.Name + "Ordering!]",
					"filter: " + model.Name + "Filter",
				}
				w.tl(
					strcase.ToLowerCamel(modelPluralName) + "(" + strings.Join(arguments, ", ") + "): " +
						model.Name + "Connection!" + joinedDirectives)
			}
			w.l("}")

			w.br()
			if i == 0 {
				w.l("type Mutation {")
			} else {
				w.l("extend type Mutation {")
			}
			for _, model := range grp {
				modelPluralName := internal.Plural(model.Name)
				// Generate mutation queries

				// create single
				// e.g createUser(input: UserInput!): UserPayload!
				w.tl("create" + model.Name + "(input: " + model.Name + "CreateInput!): " +
					model.Name + "Payload!" + joinedDirectives)

				// create multiple
				// e.g createUsers(input: [UsersInput!]!): UsersPayload!
				w.tl("create" + modelPluralName + "(input: " + modelPluralName + "CreateInput!): " +
					modelPluralName + "Payload!" + joinedDirectives)

				// update single
				// e.g updateUser(id: ID!, input: UserInput!): UserPayload!
				w.tl("update" + model.Name + "(id: ID!, input: " + model.Name + "UpdateInput!): " +
					model.Name + "Payload!" + joinedDirectives)

				// update multiple (batch update)
				// e.g updateUsers(filter: UserFilter, input: UsersInput!): UsersPayload!
				w.tl("update" + modelPluralName + "(filter: " + model.Name + "Filter, input: " +
					model.Name + "UpdateInput!): " + modelPluralName + "UpdatePayload!" + joinedDirectives)

				// delete single
				// e.g deleteUser(id: ID!): UserPayload!
				w.tl("delete" + model.Name + "(id: ID!): " + model.Name + "DeletePayload!" + joinedDirectives)

				// delete multiple
				// e.g deleteUsers(filter: UserFilter, input: [UsersInput!]!): UsersPayload!
				w.tl("delete" + modelPluralName + "(filter: " + model.Name + "Filter): " +
					modelPluralName + "DeletePayload!" + joinedDirectives)

			}
			w.l("}")
			w.br()

			for _, model := range grp {
				//	enum UserSort { FIRST_NAME, LAST_NAME }
				w.l("enum " + model.Name + "Sort {")
				for _, v := range fieldAsEnumStrings(model.Fields) {
					w.tl(v)
				}
				w.l("RANDOM")
				w.l("}")

				w.br()

				//	input UserOrdering {
				//		sort: UserSort!
				//		direction: SortDirection! = ASC
				//	}
				w.l("input " + model.Name + "Ordering {")
				w.tl("sort: " + model.Name + "Sort!")
				w.tl("direction: SortDirection! = ASC")
				w.l("}")

				w.br()

				// Create basic structs e.g.
				// type User {
				// 	firstName: String!
				// 	lastName: String
				// 	isProgrammer: Boolean!
				// 	organization: Organization!
				// }
				if cfg.Federation.Activate {
					keys := []string{}
					for _, field := range model.Fields {

						if strings.EqualFold(strings.ToLower(field.Name), "id") {
							keys = append(keys, "@key(fields: \""+field.Name+"\")")
						}
					}
					w.l("type " + model.Name + " implements Node " + strings.Join(keys, " ") + " {")
				} else {
					w.l("type " + model.Name + " implements Node {")
				}

				for _, field := range enhanceFields(hooks, model, model.Fields, ParentTypeNormal) {
					// e.g we have foreign key from user to organization
					// organizationID is clutter in your scheme
					// you only want Organization and OrganizationID should be skipped
					directives := getDirectivesAsString(field.Directives)
					if field.BoilerField != nil && field.BoilerField.IsRelation {
						w.tl(
							getRelationName(field) + ": " +
								getFinalFullTypeWithRelation(field, ParentTypeNormal) + directives,
						)
					} else {
						fullType := getFinalFullType(field, ParentTypeNormal)
						w.tl(field.Name + ": " + fullType + directives)
					}
				}
				w.l("}")

				w.br()

				//type UserEdge {
				//	cursor: String!
				//	node: User
				//}
				w.l("type " + model.Name + "Edge {")

				w.tl(`cursor: String!`)
				w.tl(`node: ` + model.Name)
				w.l("}")

				w.br()

				//type UserConnection {
				//	edges: [UserEdge]
				//	pageInfo: PageInfo!
				//}
				w.l("type " + model.Name + "Connection {")
				w.tl(`count: Int`)
				w.tl(`edges: [` + model.Name + `Edge]`)
				w.tl(`pageInfo: PageInfo!`)
				w.l("}")

				w.br()

				// generate filter structs per model
				// Ignore some specified input fields
				// Generate a type safe grapql filter

				// Generate the base filter
				// type UserFilter {
				// 	search: String
				// 	where: UserWhere
				// }
				w.l("input " + model.Name + "Filter {")
				w.tl("search: String")
				w.tl("where: " + model.Name + "Where")
				w.l("}")

				w.br()

				// Generate a where struct
				// type UserWhere {
				// 	id: IDFilter
				// 	title: StringFilter
				// 	organization: OrganizationWhere
				// 	or: FlowBlockWhere
				// 	and: FlowBlockWhere
				// }
				w.l("input " + model.Name + "Where {")

				for _, field := range enhanceFields(hooks, model, model.Fields, ParentTypeWhere) {
					directives := getDirectivesAsString(field.InputDirectives)
					if field.SkipInput || field.SkipWhere {
						continue
					}
					if field.BoilerField.IsRelation {
						// Support filtering in relationships (at least schema wise)
						relationName := getRelationName(field)
						w.tl(relationName + ": " + field.BoilerField.Relationship.Name + "Where" + directives)
					} else {
						w.tl(field.Name + ": " + field.Type + "Filter" + directives)
					}
				}
				w.tl("or: " + model.Name + "Where")
				w.tl("and: " + model.Name + "Where")
				w.l("}")

				w.br()

				// Generate input and payloads for mutatations
				filteredFields := fieldsWithout(model.Fields, cfg.Schema.SkipInputFields)

				modelPluralName := internal.Plural(model.Name)
				// input UserCreateInput {
				// 	firstName: String!
				// 	lastName: String
				//	organizationId: ID!
				// }
				w.l("input " + model.Name + "CreateInput {")

				for _, field := range enhanceFields(hooks, model, filteredFields, ParentTypeCreate) {
					if field.SkipInput || field.SkipCreate {
						continue
					}
					// id is not required in create and will be specified in update resolver
					if field.Name == "id" || field.Name == "createdAt" || field.Name == "updatedAt" || field.Name == "deletedAt" {
						continue
					}
					// not possible yet in input
					// TODO: make this possible for one-to-one structs?
					// only for foreign keys inside model itself
					if field.BoilerField.IsRelation && field.BoilerField.IsArray ||
						field.BoilerField.IsRelation && !strings.HasSuffix(field.BoilerField.Name, "ID") {
						continue
					}
					directives := getDirectivesAsString(field.InputDirectives)
					fullType := getFinalFullType(field, ParentTypeCreate)
					w.tl(field.Name + ": " + fullType + directives)
				}
				w.l("}")

				w.br()

				// input UserUpdateInput {
				// 	firstName: String!
				// 	lastName: String
				//	organizationId: ID!
				// }
				w.l("input " + model.Name + "UpdateInput {")

				for _, field := range enhanceFields(hooks, model, filteredFields, ParentTypeUpdate) {
					if field.SkipInput || field.SkipUpdate {
						continue
					}
					// id is not required in create and will be specified in update resolver
					if field.Name == "id" || field.Name == "createdAt" || field.Name == "updatedAt" || field.Name == "deletedAt" {
						continue
					}
					// not possible yet in input
					// TODO: make this possible for one-to-one structs?
					// only for foreign keys inside model itself
					if field.BoilerField.IsRelation && field.BoilerField.IsArray ||
						field.BoilerField.IsRelation && !strings.HasSuffix(field.BoilerField.Name, "ID") {
						continue
					}
					directives := getDirectivesAsString(field.InputDirectives)
					w.tl(field.Name + ": " + getFinalFullType(field, ParentTypeUpdate) + directives)
				}
				w.l("}")

				w.br()

				w.l("input " + modelPluralName + "CreateInput {")

				w.tl(strcase.ToLowerCamel(modelPluralName) + ": [" + model.Name + "CreateInput!]!")
				w.l("}")

				w.br()

				// if batchUpdate {
				// 	w.l("input " + modelPluralName + "UpdateInput {")
				// 	w.tl(strcase.ToLowerCamel(modelPluralName) + ": [" + model.Name + "UpdateInput!]!")
				// 	w.l("}")
				// 	w.br()
				// }

				// type UserPayload {
				// 	user: User!
				// }
				w.l("type " + model.Name + "Payload {")
				w.tl(strcase.ToLowerCamel(model.Name) + ": " + model.Name + "!")
				w.l("}")

				w.br()

				// TODO batch, delete input and payloads

				// type UserDeletePayload {
				// 	id: ID!
				// }
				w.l("type " + model.Name + "DeletePayload {")
				w.tl("id: ID!")
				w.l("}")

				w.br()

				// type UsersPayload {
				// 	users: [User!]!
				// }
				w.l("type " + modelPluralName + "Payload {")
				w.tl(strcase.ToLowerCamel(modelPluralName) + ": [" + model.Name + "!]!")
				w.l("}")

				w.br()

				// type UsersDeletePayload {
				// 	ids: [ID!]!
				// }
				w.l("type " + modelPluralName + "DeletePayload {")
				w.tl("ids: [ID!]!")
				w.l("}")

				w.br()
				// type UsersUpdatePayload {
				// 	ok: Boolean!
				// }
				w.l("type " + modelPluralName + "UpdatePayload {")
				w.tl("ok: Boolean!")
				w.l("}")

				w.br()

			}
			// Append to array
			mod := SchemaArr{
				Name: internal.GetFirstWord(grp[0].Name),
				Data: w.s.String(),
			}
			d = append(d, mod)
		}
	}

	return d
}

func enhanceFields(hooks *HooksConfig, model *SchemaModel, fields []*SchemaField, parentType ParentType) []*SchemaField {
	if hooks.HookChangeFields != nil {
		return hooks.HookChangeFields(model, fields, parentType)
	}
	return fields
}

func fieldAsEnumStrings(fields []*SchemaField) []string {
	var enums []string
	for _, field := range fields {
		if field.BoilerField != nil && !field.SkipSort && (!field.BoilerField.IsRelation && !field.BoilerField.IsForeignKey) {
			enums = append(enums, strcase.ToScreamingSnake(field.Name))
		}
	}
	return enums
}

func getFullType(fieldType string, isArray bool, isRequired bool) string {
	gType := fieldType

	if isArray {
		// To use a list type, surround the type in square brackets, so [Int] is a list of integers.
		gType = "[" + gType + "!]"
	}
	if isRequired {
		// Use an exclamation point to indicate a type cannot be nullable,
		// so String! is a non-nullable string.
		gType += "!"
	}
	return gType
}

func boilerModelsToModels(boilerModels []*internal.BoilerModel, foreignIDs *[]internal.ForeignIDColumn) []*SchemaModel {
	a := make([]*SchemaModel, len(boilerModels))
	for i, boilerModel := range boilerModels {
		a[i] = &SchemaModel{
			Name:   boilerModel.Name,
			Fields: boilerFieldsToFields(boilerModel.Fields, foreignIDs),
		}
	}
	return a
}

// executeHooksOnModels removes models and fields which the user hooked in into + it can change values
func executeHooksOnModels(models []*SchemaModel, hooks *HooksConfig) []*SchemaModel {
	var a []*SchemaModel
	for _, m := range models {
		if hooks.HookShouldAddModel != nil && !hooks.HookShouldAddModel(*m) {
			continue
		}
		var af []*SchemaField
		for _, f := range m.Fields {
			if hooks.HookShouldAddField != nil && !hooks.HookShouldAddField(*m, *f) {
				continue
			}
			if hooks.HookChangeField != nil {
				hooks.HookChangeField(m, f)
			}
			af = append(af, f)
		}
		m.Fields = af
		if hooks.HookChangeModel != nil {
			hooks.HookChangeModel(m)
		}

		a = append(a, m)

	}
	return a
}

func boilerFieldsToFields(boilerFields []*internal.BoilerField, foreignIDs *[]internal.ForeignIDColumn) []*SchemaField {
	fields := make([]*SchemaField, len(boilerFields))
	for i, boilerField := range boilerFields {
		fields[i] = boilerFieldToField(boilerField, foreignIDs)
	}
	return fields
}

func getRelationName(schemaField *SchemaField) string {
	return strcase.ToLowerCamel(schemaField.BoilerField.RelationshipName)
}

func getAlwaysOptional(parentType ParentType) bool {
	return parentType == ParentTypeUpdate || parentType == ParentTypeWhere || parentType == ParentTypeBatchUpdate
}

func getFinalFullTypeWithRelation(schemaField *SchemaField, parentType ParentType) string {
	boilerField := schemaField.BoilerField
	alwaysOptional := getAlwaysOptional(parentType)
	if boilerField.Relationship != nil {
		relationType := boilerField.Relationship.Name
		if alwaysOptional {
			return getFullType(
				relationType,
				boilerField.IsArray,
				false,
			)
		}
		return getFullType(
			relationType,
			boilerField.IsArray,
			boilerField.IsRequired,
		)
	}
	return getFinalFullType(schemaField, parentType)
}

func getFinalFullType(schemaField *SchemaField, parentType ParentType) string {

	alwaysOptional := getAlwaysOptional(parentType)
	isRequired := false
	isArray := false
	if schemaField.BoilerField != nil {
		boilerField := schemaField.BoilerField
		isRequired = boilerField.IsRequired
		isArray = boilerField.IsArray
	}
	if alwaysOptional {
		isRequired = false
	}

	return getFullType(getFieldType(schemaField, parentType), isArray, isRequired)
}

func getFieldType(schemaField *SchemaField, parentType ParentType) string {
	switch parentType {
	case ParentTypeNormal:
		return schemaField.Type
	case ParentTypeWhere:
		return schemaField.InputWhereType
	case ParentTypeCreate:
		return schemaField.InputCreateType
	case ParentTypeUpdate:
		return schemaField.InputUpdateType
	case ParentTypeBatchUpdate:
		return schemaField.InputBatchUpdateType
	case ParentTypeBatchCreate:
		return schemaField.InputBatchCreateType
	default:
		return ""
	}
}

func boilerFieldToField(boilerField *internal.BoilerField, foreignIDs *[]internal.ForeignIDColumn) *SchemaField {
	t := toGraphQLType(boilerField, foreignIDs)
	return NewSchemaField(toGraphQLName(boilerField.Name), t, boilerField)
}

func toGraphQLName(fieldName string) string {
	graphqlName := fieldName

	// Golang ID to Id the right way
	// Primary key
	if graphqlName == "ID" {
		graphqlName = "id"
	}

	if graphqlName == "URL" {
		graphqlName = "url"
	}

	// e.g. OrganizationID, TODO: more robust solution?
	graphqlName = strings.Replace(graphqlName, "ID", "Id", -1)
	graphqlName = strings.Replace(graphqlName, "URL", "Url", -1)

	return strcase.ToLowerCamel(graphqlName)
}

func toGraphQLType(boilerField *internal.BoilerField, foreignIDs *[]internal.ForeignIDColumn) string {
	lowerFieldName := strings.ToLower(boilerField.Name)
	lowerBoilerType := strings.ToLower(boilerField.Type)

	isCustomId := false

	if foreignIDs != nil {
		for _, s := range *foreignIDs {
			if strings.EqualFold(s.Column, boilerField.Name) {
				isCustomId = true
			}
		}
	}

	if boilerField.IsEnum {
		return boilerField.Enum.Name
	}

	if strings.HasSuffix(lowerFieldName, "id") || isCustomId {
		return "ID"
	}
	if strings.Contains(lowerBoilerType, "string") {
		return "String"
	}
	if strings.Contains(lowerBoilerType, "int") {
		return "Int"
	}
	if strings.Contains(lowerBoilerType, "byte") {
		return "String"
	}
	if strings.Contains(lowerBoilerType, "decimal") || strings.Contains(lowerBoilerType, "float") {
		return "Float"
	}
	if strings.Contains(lowerBoilerType, "bool") {
		return "Boolean"
	}

	// TODO: make this a scalar or something configurable?
	// I like to use unix here
	if strings.Contains(lowerBoilerType, "time") {
		return "Time"
	}

	// e.g. null.JSON let user define how it looks with their own struct
	// return strcase.ToCamel(fieldName)
	return "Any"
}

func fieldsWithout(fields []*SchemaField, skipFieldNames []string) []*SchemaField {
	var filteredFields []*SchemaField
	for _, field := range fields {
		if !internal.SliceContains(skipFieldNames, field.Name) {
			filteredFields = append(filteredFields, field)
		}
	}
	return filteredFields
}

func formatFile(filename string) error {
	name := "prettier"
	args := []string{filename, "--write"}

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("executing command: '%v %v' failed with: %v", name, strings.Join(args, " "), err)
	}
	return nil
}

func writeContentToFile(content string, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("could not write %v to disk: %v", filename, err)
	}

	// Close file if this functions returns early or at the end
	defer func() {
		closeErr := file.Close()
		if closeErr != nil {
			log.Err(closeErr).Msg("error while closing file")
		}
	}()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("could not write content to file %v: %v", filename, err)
	}

	return nil
}

type SimpleWriter struct {
	s strings.Builder
}

func (sw *SimpleWriter) l(v string) {
	sw.s.WriteString(v + lineBreak)
}

func (sw *SimpleWriter) br() {
	sw.s.WriteString(lineBreak)
}

func (sw *SimpleWriter) tl(v string) {
	sw.s.WriteString(indent + v + lineBreak)
}

const enumFilterHelper = `
input %[1]vFilter {
	isNull: Boolean
	notNull: Boolean
	equalTo: %[1]v
	notEqualTo: %[1]v
	in: [%[1]v!]
	notIn: [%[1]v!]
}
`

// TODO: only generate these if they are set
const queryHelperStructs = `
input IDFilter {
	isNull: Boolean
	notNull: Boolean
	equalTo: ID
	notEqualTo: ID
	in: [ID!]
	notIn: [ID!]
}

input StringFilter {
	isNullOrEmpty: Boolean
	isEmpty: Boolean
	isNull: Boolean
	notNullOrEmpty: Boolean
	notEmpty: Boolean
	notNull: Boolean

	equalTo: String
	notEqualTo: String

	in: [String!]
	notIn: [String!]

	startWith: String
	notStartWith: String

	endWith: String
	notEndWith: String

	contain: String
	notContain: String

	startWithStrict: String # Camel sensitive
	notStartWithStrict: String # Camel sensitive

	endWithStrict: String # Camel sensitive
	notEndWithStrict: String # Camel sensitive

	containStrict: String # Camel sensitive
	notContainStrict: String # Camel sensitive
}

input IntFilter {
	isNullOrZero: Boolean
	isNull: Boolean
	notNullOrZero: Boolean
	notNull: Boolean
	equalTo: Int
	notEqualTo: Int
	lessThan: Int
	lessThanOrEqualTo: Int
	moreThan: Int
	moreThanOrEqualTo: Int
	in: [Int!]
	notIn: [Int!]
}

input FloatFilter {
	isNullOrZero: Boolean
	isNull: Boolean
	notNullOrZero: Boolean
	notNull: Boolean
	equalTo: Float
	notEqualTo: Float
	lessThan: Float
	lessThanOrEqualTo: Float
	moreThan: Float
	moreThanOrEqualTo: Float
	in: [Float!]
	notIn: [Float!]
}

input BooleanFilter {
	isNull: Boolean
	notNull: Boolean
	equalTo: Boolean
	notEqualTo: Boolean
}

input TimeFilter {
	isNull: Boolean
	notNull: Boolean
	equalTo: Time
	notEqualTo: Time
	lessThan: Time
	lessThanOrEqualTo: Time
	moreThan: Time
	moreThanOrEqualTo: Time
}
`

package utils

import (
	"strings"
	"unicode"

	"github.com/iancoleman/strcase"
	"github.com/volatiletech/strmangle"
)

// TaskBlockedBies -> TaskBlockedBy
// People -> Person
func Singular(s string) string {
	singular := strmangle.Singular(strcase.ToSnake(s))

	singularTitle := strmangle.TitleCase(singular)
	if IsFirstCharacterLowerCase(s) {
		a := []rune(singularTitle)
		a[0] = unicode.ToLower(a[0])
		return string(a)
	}
	return singularTitle
}

// TaskBlockedBy -> TaskBlockedBies
// Person -> Persons
// Person -> People
func Plural(s string) string {
	plural := strmangle.Plural(strcase.ToSnake(s))

	pluralTitle := strmangle.TitleCase(plural)
	if IsFirstCharacterLowerCase(s) {
		a := []rune(pluralTitle)
		a[0] = unicode.ToLower(a[0])
		return string(a)
	}
	return pluralTitle
}

func IsPlural(s string) bool {
	return s == Plural(s)
}

func IsSingular(s string) bool {
	return s == Singular(s)
}

func GetShortType(longType string, ignoreTypePrefixes []string) string {
	// longType e.g = gitlab.com/decicify/app/backend/graphql_models.FlowWhere
	splittedBySlash := strings.Split(longType, "/")
	// gitlab.com, decicify, app, backend, graphql_models.FlowWhere

	lastPart := splittedBySlash[len(splittedBySlash)-1]
	isPointer := strings.HasPrefix(longType, "*")
	isStructInPackage := strings.Count(lastPart, ".") > 0

	if isStructInPackage {
		// if packages are deeper they don't have pointers but *time.Time will since it's not deep
		returnType := strings.TrimPrefix(lastPart, "*")
		for _, ignoreType := range ignoreTypePrefixes {
			fullIgnoreType := ignoreType + "."
			returnType = strings.TrimPrefix(returnType, fullIgnoreType)
		}

		if isPointer {
			return "*" + returnType
		}
		return returnType
	}

	return longType
}

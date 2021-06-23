package utils

import "strings"

func IsFieldId(fieldName string) bool {
	return strings.HasSuffix(strings.ToLower(fieldName), "id")
}

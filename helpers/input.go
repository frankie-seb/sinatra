package helpers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
)

// Export input from the current context
func GetInputFromContext(ctx context.Context, key string) map[string]interface{} {
	fieldContext := graphql.GetFieldContext(ctx)
	variables := map[string]interface{}{}
	args := graphql.GetOperationContext(ctx).Variables

	// If variables are used
	for k, arg := range args {
		if k != key {
			continue
		}
		fields, ok := arg.(map[string]interface{})

		if ok {
			for k, f := range fields {
				variables[k] = f
			}
		}

	}
	// If inline vars are used
	for _, arg := range fieldContext.Field.Arguments {
		if arg.Name != key {
			continue
		}
		for _, child := range arg.Value.Children {
			variables[child.Name] = child.Value
		}
	}

	return variables
}

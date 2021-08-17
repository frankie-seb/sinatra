// testing really need more improvment!

package plugins

import (
	"testing"

	utils "github.com/frankie-seb/sinatra/internal/utils"
)

func TestShortType(t *testing.T) {
	testShortType(t, "gitlab.com/product/app/backend/graphql_models.FlowWhere", "FlowWhere")
	testShortType(t, "*gitlab.com/product/app/backend/graphql_models.FlowWhere", "*FlowWhere")
	testShortType(t, "*utils.GeoPoint", "*GeoPoint")
	testShortType(t, "utils.GeoPoint", "GeoPoint")
	testShortType(t, "*string", "*string")
	testShortType(t, "string", "string")
	testShortType(t, "*time.Time", "*time.Time")
}

func testShortType(t *testing.T, input, output string) {
	result := utils.GetShortType(input, nil)
	if result != output {
		t.Errorf("%v should result in %v but did result in %v", input, output, result)
	}
}

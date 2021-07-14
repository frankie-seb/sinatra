package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	config "github.com/FrankieHealth/be-base/config"
	"github.com/FrankieHealth/be-base/internal/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/volatiletech/sqlboiler/v4/boilingcore"
	"github.com/volatiletech/sqlboiler/v4/drivers"
	"github.com/volatiletech/sqlboiler/v4/importers"
)

const version = "0.1"

var (
	cmdState  *boilingcore.State
	cmdConfig *boilingcore.Config
	rootCmd   *cobra.Command
)

var configTemplate = template.Must(template.New("name").Parse(
	`# Where are all the schema files located? globs are supported eg  src/**/*.graphqls
schema:
  - graph/*.graphqls
# Where should the generated server code go?
exec:
  filename: graph/generated/generated.go
  package: generated
# Uncomment to enable federation
# federation:
#   filename: graph/generated/federation.go
#   package: generated
# Where should any generated models go?
model:
  filename: graph/model/models_gen.go
  package: model
# Where should the resolver implementations go?
resolver:
  layout: follow-schema
  dir: graph
  package: graph
# Optional: turn on use ` + "`" + `gqlgen:"fieldName"` + "`" + ` tags in your models
# struct_tag: json
# Optional: turn on to use []Thing instead of []*Thing
# omit_slice_element_pointers: false
# Optional: set to speed up generation time by not performing a final validation pass.
# skip_validation: true
# gqlgen will search for any type names in the schema in these go packages
# if they match it will use them, otherwise it will generate them.
autobind:
  - "{{.}}/graph/model"
# This section declares type mapping between the GraphQL and go type systems
#
# The first line in each type will be used as defaults for resolver arguments and
# modelgen, the others will be allowed when binding to fields. Configure them to
# your liking
models:
  ID:
    model:
      - github.com/99designs/gqlgen/graphql.ID
      - github.com/99designs/gqlgen/graphql.Int
      - github.com/99designs/gqlgen/graphql.Int64
      - github.com/99designs/gqlgen/graphql.Int32
  Int:
    model:
      - github.com/99designs/gqlgen/graphql.Int
      - github.com/99designs/gqlgen/graphql.Int64
      - github.com/99designs/gqlgen/graphql.Int32
`))

func main() {
	// Set up the cobra root command
	rootCmd = &cobra.Command{
		Use:           "maddox [flags]",
		Short:         "Generate the Frankie ORM.",
		Long:          "Generate the Frankie ORM.",
		Example:       `maddox`,
		RunE:          run,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().StringP("output", "o", "models", "The name of the folder to output to")
	rootCmd.PersistentFlags().StringP("config", "c", "maddox.yml", "The config filename")
	rootCmd.PersistentFlags().StringP("pkgname", "p", "models", "The name you wish to assign to your generated package")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Debug mode prints stack traces on error")
	rootCmd.PersistentFlags().BoolP("no-auto-timestamps", "", false, "Disable automatic timestamps for created_at/updated_at")
	rootCmd.PersistentFlags().StringSliceP("templates", "", nil, "A templates directory, overrides the bindata'd template folders in sqlboiler")
	rootCmd.PersistentFlags().StringSliceP("tag", "t", nil, "Struct tags to be included on your models in addition to json, yaml, toml")
	rootCmd.PersistentFlags().StringSliceP("replace", "", nil, "Replace templates by directory: relpath/to_file.tpl:relpath/to_replacement.tpl")
	rootCmd.PersistentFlags().BoolP("no-context", "", false, "Disable context.Context usage in the generated code")
	rootCmd.PersistentFlags().BoolP("no-tests", "", false, "Disable generated go test files")
	rootCmd.PersistentFlags().BoolP("no-hooks", "", false, "Disable hooks feature for your models")
	rootCmd.PersistentFlags().BoolP("no-rows-affected", "", false, "Disable rows affected in the generated API")
	rootCmd.PersistentFlags().BoolP("add-global-variants", "", false, "Enable generation for global variants")
	rootCmd.PersistentFlags().BoolP("add-panic-variants", "", false, "Enable generation for panic variants")
	rootCmd.PersistentFlags().BoolP("no-soft-deletes", "", false, "Disable soft deletes where deleted_at exists")
	rootCmd.PersistentFlags().StringP("struct-tag-casing", "", "snake", "Decides the casing for go structure tag names. camel or snake (default snake)")
	rootCmd.PersistentFlags().BoolP("version", "", false, "Print the version")

	// hide flags not recommended for use
	rootCmd.PersistentFlags().MarkHidden("replace")

	if err := rootCmd.Execute(); err != nil {

		flags := rootCmd.PersistentFlags()
		configFilename := getStringP(flags, "config")
		pkgName := utils.ImportPathForDir(".")

		if !configExists(configFilename) {
			if err := initConfig(configFilename, pkgName); err != nil {
				os.Exit(1)
			}
		}

		if !getBoolP(flags, "debug") {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Error: %+v\n", err)
		}

		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	var err error

	// --version just prints the version and returns.
	flags := rootCmd.PersistentFlags()
	if getBoolP(flags, "version") {
		fmt.Println("maddox v" + version)
		return nil
	}

	// Only support PostgreSQL with SQL Boiler.
	driverName := "psql"
	driverPath := "sqlboiler-psql"
	dbName := "connect_db"
	if p, err := exec.LookPath(driverPath); err == nil {
		driverPath = p
	}
	driverPath, err = filepath.Abs(driverPath)
	if err != nil {
		return errors.Wrap(err, "could not find absolute path to driver")
	}
	drivers.RegisterBinary(driverName, driverPath)

	// Get the configuration for the driver.
	driverConfig, err := getPsqlDriverConfig(dbName)
	if err != nil {
		return errors.Wrap(err, "failed to create SQL Boiler driver config")
	}

	cmdConfig = &boilingcore.Config{
		DriverName:       driverName,
		DriverConfig:     driverConfig,
		OutFolder:        getStringP(flags, "output"),
		PkgName:          getStringP(flags, "pkgname"),
		Debug:            getBoolP(flags, "debug"),
		AddGlobal:        getBoolP(flags, "add-global-variants"),
		AddPanic:         getBoolP(flags, "add-panic-variants"),
		NoContext:        getBoolP(flags, "no-context"),
		NoTests:          getBoolP(flags, "no-tests"),
		NoHooks:          getBoolP(flags, "no-hooks"),
		NoRowsAffected:   getBoolP(flags, "no-rows-affected"),
		NoAutoTimestamps: getBoolP(flags, "no-auto-timestamps"),
		AddSoftDeletes:   getBoolP(flags, "no-auto-timestamps"),
		Wipe:             true,                                                    // always wipe
		StructTagCasing:  strings.ToLower(getStringP(flags, "struct-tag-casing")), // camel | snake
		TemplateDirs:     getStringSliceP(flags, "templates"),
		Tags:             getStringSliceP(flags, "tag"),
		Replacements:     getStringSliceP(flags, "replace"),
		Imports:          importers.NewDefaultImports(),
	}

	// Run SQL Boiler.
	cmdState, err = boilingcore.New(cmdConfig)
	if err != nil {
		return err
	}
	err = cmdState.Run()
	if err != nil {
		return err
	}
	return cmdState.Cleanup()
}

func getPsqlDriverConfig(name string) (map[string]interface{}, error) {
	config := map[string]interface{}{
		"blacklist": []string{"migrations"},
	}
	config = map[string]interface{}{
		"whitelist": []string{"account_commission"},
	}
	config["dbname"] = name
	config["schema"] = "connect"
	config["host"] = "localhost"
	config["port"] = "5432"
	config["user"] = "dev_user"
	config["pass"] = "Laqr67X7np5qMgu7"
	config["sslmode"] = "disable"

	return config, nil
}

func getBoolP(p *pflag.FlagSet, key string) bool {
	value, err := p.GetBool(key)
	if err != nil {
		panic(err)
	}
	return value
}

func getStringP(p *pflag.FlagSet, key string) string {
	value, err := p.GetString(key)
	if err != nil {
		panic(err)
	}
	return value
}

func getStringSliceP(p *pflag.FlagSet, key string) []string {
	value, err := p.GetStringSlice(key)
	if err != nil {
		panic(err)
	}
	return value
}

func configExists(configFilename string) bool {
	var cfg *config.Config

	if configFilename != "" {
		cfg, _ = config.LoadConfig(configFilename)
	} else {
		cfg, _ = config.LoadConfigFromDefaultLocations()
	}
	return cfg != nil
}

func initConfig(configFilename string, pkgName string) error {
	if configFilename == "" {
		configFilename = "maddox.yml"
	}

	if err := os.MkdirAll(filepath.Dir(configFilename), 0755); err != nil {
		return fmt.Errorf("unable to create config dir: " + err.Error())
	}

	var buf bytes.Buffer
	if err := configTemplate.Execute(&buf, pkgName); err != nil {
		panic(err)
	}

	if err := ioutil.WriteFile(configFilename, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("unable to write cfg file: " + err.Error())
	}

	return nil
}

package sqlboiler

import (
	"github.com/frankie-seb/sinatra/internal"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/boilingcore"
	_ "github.com/volatiletech/sqlboiler/v4/drivers/sqlboiler-psql/driver"
	"github.com/volatiletech/sqlboiler/v4/importers"
)

var (
	cmdState  *boilingcore.State
	cmdConfig *boilingcore.Config
)

func Run(cfg *internal.Config) error {
	// Get the configuration for the driver.
	driverConfig, err := getPsqlDriverConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "failed to create driver config")
	}

	// Create the configurations from flags.
	cmdConfig = &boilingcore.Config{
		DriverName:       cfg.Database.DBDriver,
		DriverConfig:     driverConfig,
		OutFolder:        cfg.Model.DirName,
		PkgName:          cfg.Model.Package,
		Debug:            cfg.Database.Debug,
		AddGlobal:        cfg.Database.AddGlobal,
		AddPanic:         cfg.Database.AddPanic,
		NoContext:        cfg.Database.NoContext,
		NoTests:          cfg.Database.NoTests,
		NoHooks:          cfg.Database.NoHooks,
		NoRowsAffected:   cfg.Database.NoRowsAffected,
		NoAutoTimestamps: cfg.Database.NoAutoTimestamps,
		AddSoftDeletes:   cfg.Database.AddSoftDeletes,
		Wipe:             cfg.Database.Wipe,
		StructTagCasing:  cfg.Database.StructTagCasing, // camel | snake
		// TemplateDirs:     getStringSliceP(flags, "templates"),
		// Tags:             getStringSliceP(flags, "tag"),
		// Replacements:     getStringSliceP(flags, "replace"),
		Imports: importers.NewDefaultImports(),
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

func getPsqlDriverConfig(cfg *internal.Config) (map[string]interface{}, error) {
	config := map[string]interface{}{
		"dbname":    cfg.Database.DBName,
		"host":      cfg.Database.Host,
		"port":      cfg.Database.Port,
		"user":      cfg.Database.User,
		"pass":      cfg.Database.Password,
		"sslmode":   cfg.Database.SSLMode,
		"blacklist": cfg.Database.Blacklist,
		"whitelist": cfg.Database.Whitelist,
	}

	if cfg.Database.Schema != "" {
		config["schema"] = cfg.Database.Schema
	}

	return config, nil
}

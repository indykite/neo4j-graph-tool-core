// Copyright (c) 2022 IndyKite
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	DefaultPort                = 8080
	DefaultLogLevel            = "warn"
	DefaultBaseFolder          = "import"
	DefaultDropCypherFile      = "drop.cypher"
	DefaultSchemaFolderName    = "schema"
	DefaultSchemaMigrationType = "up_down"
	DefaultNodeLabel           = "GraphToolMigration"
	DefaultNodeSubString       = "Version"
	DefaultFolderMigrationType = "change"
)

type (
	Config struct {
		Supervisor *Supervisor `mapstructure:"supervisor"`
		Planner    *Planner    `mapstructure:"planner"`
	}

	Supervisor struct {
		Port     int    `mapstructure:"port"`
		LogLevel string `mapstructure:"log_level"`
	}

	Planner struct {
		BaseFolder     string                   `mapstructure:"base_folder"`
		DropCypherFile string                   `mapstructure:"drop_cypher_file"`
		Batches        map[string]*BatchDetail  `mapstructure:"batches"`
		SchemaFolder   *SchemaFolder            `mapstructure:"schema_folder"`
		Folders        map[string]*FolderDetail `mapstructure:"folders"`
	}

	SchemaFolder struct {
		FolderName    string   `mapstructure:"folder_name"`
		MigrationType string   `mapstructure:"migration_type"`
		NodeLabels    []string `mapstructure:"node_labels"`
	}

	FolderDetail struct {
		MigrationType string   `mapstructure:"migration_type"`
		NodeLabels    []string `mapstructure:"node_labels"`
	}

	BatchDetail struct {
		Folders []string `mapstructure:"folders"`
	}
)

var (
	logLevelValues = []string{"fatal", "error", "warn", "info", "debug", "trace"}
	migrationTypes = []string{"change", "up_down"}
	labelCaser     = cases.Title(language.English)
)

// New creates a new config containing values from environment variables and default values
func New() (*Config, error) {
	return LoadFile("")
}

// LoadFile loads file into config, together with values from environment variables and default values
func LoadFile(fileName string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(fileName)
	if err := v.ReadInConfig(); err != nil {
		// If SetConfigFile is used, and config is not found, fs.PathError is returned, do not ignore this error
		// But if no config is found in ConfigPath with ConfigName, viper.ConfigFileNotFoundError is returned
		// and we want to ignore that error, as config is not really mandatory for us
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	v.SetDefault("supervisor", &Supervisor{})
	v.SetDefault("supervisor.port", DefaultPort)
	v.SetDefault("supervisor.log_level", DefaultLogLevel)
	v.SetDefault("planner.drop_cypher_file", DefaultDropCypherFile)
	v.SetDefault("planner.base_folder", DefaultBaseFolder)
	v.SetDefault("planner.schema_folder.folder_name", DefaultSchemaFolderName)
	v.SetDefault("planner.schema_folder.migration_type", DefaultSchemaMigrationType)

	c := &Config{}

	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}

	// Normalize will fail only when config is not fully initialized. But Unmarshal always initialize all.
	_ = c.Normalize()

	if err := c.validateValues(); err != nil {
		return nil, err
	}

	return c, nil
}

// Validate validates that data in Config struct are filled in correctly
func (c *Config) Validate() error {
	if err := c.validateStructure(); err != nil {
		return err
	}

	return c.validateValues()
}

func (c *Config) validateStructure() error {
	if c == nil {
		return errors.New("missing config")
	}
	if c.Supervisor == nil {
		return errors.New("missing config.Supervisor")
	}
	if c.Planner == nil {
		return errors.New("missing config.Planner")
	}
	if c.Planner.SchemaFolder == nil {
		return errors.New("missing config.Planner.SchemaFolder")
	}
	return nil
}

func (c *Config) validateValues() error {
	if err := c.validateRequiredParts(); err != nil {
		return err
	}
	return c.validateFoldersAndBatches()
}

func (c *Config) validateRequiredParts() error {
	// Supervisor part
	if c.Supervisor.Port < 1024 || c.Supervisor.Port > 65535 {
		return errors.New("port number must be in range 1024 - 65535")
	}

	if !stringInArray(logLevelValues, c.Supervisor.LogLevel) {
		return fmt.Errorf("logLevel value '%s' is invalid, must be one of '%s'",
			c.Supervisor.LogLevel, strings.Join(logLevelValues, ","))
	}

	// Planner part
	if c.Planner.BaseFolder == "" {
		return errors.New("base_folder cannot be empty")
	}
	if _, ok := c.Planner.Batches["schema"]; ok {
		return errors.New("the batch 'schema' is set automatically and cannot be overwritten")
	}

	// Planner.SchemaFolder part
	if c.Planner.SchemaFolder.FolderName == "" {
		return errors.New("folder_name of schema_folder cannot be empty")
	}
	if !stringInArray(migrationTypes, c.Planner.SchemaFolder.MigrationType) {
		return errors.New("in folder schema migration_type must be 'change' or 'up_down'")
	}
	if label, ok := duplicateElements(c.Planner.SchemaFolder.NodeLabels); ok {
		return fmt.Errorf("duplicate label '%s' in schemaFolder", label)
	}

	return nil
}

func (c *Config) validateFoldersAndBatches() error {
	possibleFolders := map[string]bool{}

	// Planner.Folders parts
	for folderName, folderDetail := range c.Planner.Folders {
		if folderName == c.Planner.SchemaFolder.FolderName {
			return fmt.Errorf(
				"folder '%s' is used as schema folder and cannot be used again in planner.folders", folderName)
		}

		if folderDetail == nil {
			return fmt.Errorf("empty configuration for folder '%s'", folderName)
		}

		if !stringInArray(migrationTypes, folderDetail.MigrationType) {
			return fmt.Errorf("in folder '%s' migration_type must be 'change' or 'up_down'", folderName)
		}

		if label, ok := duplicateElements(folderDetail.NodeLabels); ok {
			return fmt.Errorf("duplicate label '%s' in folder named '%s'", label, folderName)
		}
		possibleFolders[folderName] = true
	}

	for batchName, batchDetail := range c.Planner.Batches {
		if batchDetail == nil {
			return fmt.Errorf("empty configuration for batch '%s'", batchName)
		}

		for _, folder := range batchDetail.Folders {
			if folder == c.Planner.SchemaFolder.FolderName {
				return fmt.Errorf("folder '%s' is schema folder and thus implicit, cannot be specified in batch '%s'",
					folder, batchName)
			}

			if _, isDefined := possibleFolders[folder]; !isDefined {
				return fmt.Errorf("folder '%s' in batch '%s' is not defined in planner.folders", folder, batchName)
			}
		}
	}

	return nil
}

func duplicateElements(nodes []string) (string, bool) {
	uniqueLabels := make(map[string]bool)
	for _, label := range nodes {
		if uniqueLabels[label] {
			return label, true
		}
		uniqueLabels[label] = true
	}
	return "", false
}

func stringInArray(arrayString []string, searchString string) bool {
	for _, s := range arrayString {
		if s == searchString {
			return true
		}
	}

	return false
}

// Normalize will set default values for some dynamic parts of config
//
// It first calls Validate() to be sure all required structures are in place. This is only case,
// when function might return error. If you called Validate before, you are free to ignore this error.
func (c *Config) Normalize() error {
	if err := c.validateStructure(); err != nil {
		return err
	}

	if len(c.Planner.SchemaFolder.NodeLabels) == 0 {
		version := generateLabelName(c.Planner.SchemaFolder.FolderName)
		c.Planner.SchemaFolder.NodeLabels = []string{DefaultNodeLabel, version}
	}

	for folderName, folder := range c.Planner.Folders {
		if folder.MigrationType == "" {
			folder.MigrationType = DefaultFolderMigrationType
		}
		if len(folder.NodeLabels) == 0 {
			folder.NodeLabels = []string{DefaultNodeLabel, generateLabelName(folderName)}
		}
	}
	c.Supervisor.LogLevel = strings.ToLower(c.Supervisor.LogLevel)

	return nil
}

func generateLabelName(folderName string) string {
	return labelCaser.String(folderName) + DefaultNodeSubString
}

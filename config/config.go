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
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type (
	Config struct {
		Supervisor *SupervisorConfig `toml:"supervisor"`
		Planner    *PlannerConfig    `toml:"planner"`
	}

	SupervisorConfig struct {
		Port     int    `toml:"port"`
		LogLevel string `toml:"log_level"`
	}
	PlannerConfig struct {
		BaseFolder string         `toml:"base_folder"`
		Kinds      []*KindsConfig `toml:"kinds"`
	}
	KindsConfig struct {
		Name    string   `toml:"name"`
		Folders []string `toml:"folders"`
	}
)

var (
	LogLevelValues = []string{"fatal", "error", "warn", "info", "debug", "trace", "debug"}
)

// LoadConfig loads TOML data into a Config struct

func LoadConfig(data string) (*Config, error) {
	c := &Config{}

	content, err := ioutil.ReadFile(filepath.Clean(data))
	if err != nil {
		return nil, err
	}

	err = toml.Unmarshal(content, &c)
	if err != nil {
		return nil, err
	}

	c.FillMissingData()

	err = c.Validate()
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Validate validates that data in Config struct are filled in correctly

func (config *Config) Validate() error {

	if config.Supervisor.Port < 1024 || config.Supervisor.Port > 65535 {
		return errors.New("Port number out of range")
	}

	err := containsString(LogLevelValues, config.Supervisor.LogLevel)
	if err != nil {
		return err
	}

	if config.Planner.BaseFolder != "import" {
		return errors.New("Base Folder not filled in correctly")
	}

	if len(config.Planner.Kinds) == 0 {
		return errors.New("No planner.kinds filled in")
	}

	uniqueNames := make(map[string]bool)
	for _, kind := range config.Planner.Kinds {
		if uniqueNames[kind.Name] {
			return errors.New("Duplicate names in Planner Kinds")
		}
		uniqueNames[kind.Name] = true

		uniqueFolderElements := make(map[string]bool)
		for _, element := range kind.Folders {
			if uniqueFolderElements[element] {
				return errors.New("Duplicate elements in Folders array")
			}
			uniqueFolderElements[element] = true
		}
	}

	return nil
}

func containsString(arrayString []string, searchString string) error {
	for _, s := range arrayString {
		if s == searchString {
			return nil
		}
	}

	return errors.New("Log level not filled in correctly")
}

// FillMissingData fills in data that user didn't define

func (config *Config) FillMissingData() error {
	if config.Supervisor.Port == 0 {
		config.Supervisor.Port = 8080
	}

	if config.Supervisor.LogLevel == "" {
		config.Supervisor.LogLevel = "debug"
	} else {
		config.Supervisor.LogLevel = strings.ToLower(config.Supervisor.LogLevel)
	}

	if config.Planner.BaseFolder == "" {
		config.Planner.BaseFolder = "import"
	} else {
		config.Planner.BaseFolder = strings.ToLower(config.Planner.BaseFolder)
	}

	return nil
}

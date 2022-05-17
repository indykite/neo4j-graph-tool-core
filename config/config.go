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
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultPort       = 8080
	DefaultLogLevel   = "warn"
	DefaultBaseFolder = "import"
)

type (
	Config struct {
		Supervisor *Supervisor `toml:"supervisor"`
		Planner    *Planner    `toml:"planner"`
	}

	Supervisor struct {
		Port     int    `toml:"port"`
		LogLevel string `toml:"log_level"`
	}
	Planner struct {
		BaseFolder string   `toml:"base_folder"`
		Batches    []*Batch `toml:"batches"`
	}
	Batch struct {
		Name    string   `toml:"name"`
		Folders []string `toml:"folders"`
	}
)

var (
	LogLevelValues = []string{"fatal", "error", "warn", "info", "debug", "trace"}
)

// LoadConfig loads TOML data into a Config struct
func LoadConfig(data string) (*Config, error) {
	c := &Config{}

	content, err := ioutil.ReadFile(filepath.Clean(data))
	if err != nil {
		return nil, err
	}

	if err = toml.Unmarshal(content, &c); err != nil {
		return nil, err
	}

	if err = c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate validates that data in Config struct are filled in correctly
func (config *Config) Validate() error {
	if config.Planner == nil {
		return errors.New("planner field is missing")
	}
	config.normalizeData()

	if config.Supervisor.Port < 1024 || config.Supervisor.Port > 65535 {
		return errors.New("port number must be in range 1024 - 65535")
	}

	if logLevelIsCorrect := containsString(LogLevelValues, config.Supervisor.LogLevel); !logLevelIsCorrect {
		return fmt.Errorf("logLevel value '%s' invalid, must be one of '%s'",
			config.Supervisor.LogLevel, strings.Join(LogLevelValues, ","))
	}

	if len(config.Planner.Batches) == 0 {
		return errors.New("planner.batches array is missing")
	}

	uniqueNames := make(map[string]bool)
	for _, batch := range config.Planner.Batches {
		if uniqueNames[batch.Name] {
			return fmt.Errorf("duplicate name '%s' in planner.batches", batch.Name)
		}
		uniqueNames[batch.Name] = true

		uniqueFolderElements := make(map[string]bool)
		for _, element := range batch.Folders {
			if uniqueFolderElements[element] {
				return fmt.Errorf(
					"duplicate element '%s' in folders array in planner.batch named '%s'", element, batch.Name)
			}
			uniqueFolderElements[element] = true
		}
	}

	return nil
}

func containsString(arrayString []string, searchString string) bool {
	for _, s := range arrayString {
		if s == searchString {
			return true
		}
	}

	return false
}

func (config *Config) normalizeData() {
	if config.Supervisor == nil {
		config.Supervisor = &Supervisor{}
	}
	if config.Supervisor.Port == 0 {
		config.Supervisor.Port = DefaultPort
	}

	if config.Supervisor.LogLevel == "" {
		config.Supervisor.LogLevel = DefaultLogLevel
	}
	config.Supervisor.LogLevel = strings.ToLower(config.Supervisor.LogLevel)

	if config.Planner.BaseFolder == "" {
		config.Planner.BaseFolder = DefaultBaseFolder
	}
}

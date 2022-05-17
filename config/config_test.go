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

package config_test

import (
	"github.com/indykite/neo4j-graph-tool-core/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Load data errors", func() {
	It("Invalid folder", func() {
		_, err := config.LoadConfig("testdata/none")
		Expect(err).To(MatchError(ContainSubstring("open testdata/none: no such file or directory")))
	})
	It("Not a folder", func() {
		_, err := config.LoadConfig("config.go")

		Expect(err).To(MatchError(ContainSubstring("toml: line 1: expected '.' or '=', but got '/' instead")))
	})
})

var _ = Describe("Config", func() {
	res, err := config.LoadConfig("testdata/configData.toml")
	It("Loading data from file", func() {
		Expect(err).To(Succeed())
		Expect(res).To(PointTo(MatchAllFields(Fields{
			"Supervisor": PointTo(MatchAllFields(Fields{
				"Port":     Equal(config.DefaultPort),
				"LogLevel": Equal(config.DefaultLogLevel),
			})),
			"Planner": PointTo(MatchAllFields(Fields{
				"BaseFolder": Equal(config.DefaultBaseFolder),
				"Kinds": ConsistOf(PointTo(MatchAllFields(Fields{
					"Name":    Equal("schema"),
					"Folders": ConsistOf("schema"),
				})), PointTo(MatchAllFields(Fields{
					"Name":    Equal("data"),
					"Folders": ConsistOf("schema", "data"),
				})), PointTo(MatchAllFields(Fields{
					"Name":    Equal("performance"),
					"Folders": ConsistOf("schema", "data", "perf"),
				})),
				),
			})),
		})))
	})

})

var _ = Describe("Validation", func() {
	var configStruct *config.Config

	BeforeEach(func() {
		configStruct = &config.Config{
			Supervisor: &config.SupervisorConfig{
				Port:     config.DefaultPort,
				LogLevel: config.DefaultLogLevel,
			},
			Planner: &config.PlannerConfig{
				BaseFolder: config.DefaultBaseFolder,
				Kinds: []*config.KindsConfig{
					{
						Name:    "schema",
						Folders: []string{"schema"},
					},
					{
						Name:    "data",
						Folders: []string{"schema", "data"},
					},
					{
						Name:    "performance",
						Folders: []string{"schema", "data", "perf"},
					},
				},
			},
		}
	})

	It("Validate function", func() {
		err := configStruct.Validate()
		Expect(err).To(Succeed())
	})

	It("Fill Missing data", func() {
		configStruct.Supervisor.Port = 0
		configStruct.Supervisor.LogLevel = ""
		configStruct.Planner.BaseFolder = ""
		err := configStruct.Validate()
		Expect(err).To(Succeed())
		Expect(configStruct).To(PointTo(MatchAllFields(Fields{
			"Supervisor": PointTo(MatchAllFields(Fields{
				"Port":     Equal(config.DefaultPort),
				"LogLevel": Equal(config.DefaultLogLevel),
			})),
			"Planner": PointTo(MatchFields(IgnoreExtras, Fields{
				"BaseFolder": Equal(config.DefaultBaseFolder),
			})),
		})))

	})
	It("Case insensitive", func() {
		configStruct.Supervisor.LogLevel = "fAtAL"
		err := configStruct.Validate()
		Expect(err).To(Succeed())
		Expect(configStruct).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Supervisor": PointTo(MatchFields(IgnoreExtras, Fields{
				"LogLevel": Equal("fatal"),
			})),
		})))
	})
	It("Base Folder", func() {
		configStruct.Planner.BaseFolder = ""
		err := configStruct.Validate()
		Expect(err).To(Succeed())
		Expect(configStruct).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Planner": PointTo(MatchFields(IgnoreExtras, Fields{
				"BaseFolder": Equal(config.DefaultBaseFolder),
			})),
		})))

	})

	It("ConfigStruct.Supervisor is nil", func() {
		configStruct.Supervisor = nil
		err := configStruct.Validate()
		Expect(err).To(Succeed())
		Expect(configStruct).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Supervisor": PointTo(MatchAllFields(Fields{
				"Port":     Equal(config.DefaultPort),
				"LogLevel": Equal(config.DefaultLogLevel),
			})),
		})))

	})
})

var _ = Describe("Error tests", func() {
	var configStruct *config.Config

	BeforeEach(func() {
		configStruct = &config.Config{
			Supervisor: &config.SupervisorConfig{
				Port:     8080,
				LogLevel: "debug",
			},
			Planner: &config.PlannerConfig{
				BaseFolder: "import",
				Kinds: []*config.KindsConfig{
					{
						Name:    "schema",
						Folders: []string{"schema"},
					},
					{
						Name:    "data",
						Folders: []string{"schema", "data"},
					},
					{
						Name:    "performance",
						Folders: []string{"schema", "data", "perf"},
					},
				},
			},
		}
	})

	It("Port Number", func() {
		configStruct.Supervisor.Port = 1000
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp("port number must be in range 1024 - 65535")))

	})
	It("Log Level", func() {
		configStruct.Supervisor.LogLevel = "debugfatal"
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp(
			"logLevel value 'debugfatal' invalid, must be one of 'fatal,error,warn,info,debug,trace'")))

	})
	It("Planner kinds missing", func() {
		configStruct.Planner.Kinds = []*config.KindsConfig{}
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp("planner.kinds array is missing")))
	})
	It("Duplicate Names", func() {
		configStruct.Planner.Kinds[0].Name = configStruct.Planner.Kinds[1].Name
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp("duplicate name 'data' in planner.kinds")))
	})
	It("Duplicate elements of Folders", func() {
		configStruct.Planner.Kinds[1].Folders = []string{"schema", "schema", "data"}
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp(
			"duplicate element 'schema' in folders array in planner.kinds named 'data'")))
	})
	It("Config.Planner is nil", func() {
		configStruct.Planner = nil
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp("planner field is missing")))
	})

})

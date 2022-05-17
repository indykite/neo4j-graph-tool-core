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
		Expect(err).To(HaveOccurred())
	})
	It("Not a folder", func() {
		_, err := config.LoadConfig("config.go")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Config", func() {
	res, err := config.LoadConfig("testdata/configData.toml")
	It("Loading data from file", func() {
		Expect(err).To(Succeed())
		Expect(res).To(PointTo(MatchAllFields(Fields{
			"Supervisor": PointTo(MatchAllFields(Fields{
				"Port":     Equal(8080),
				"LogLevel": Equal("debug"),
			})),
			"Planner": PointTo(MatchAllFields(Fields{
				"BaseFolder": Equal("import"),
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

	It("Validate function", func() {
		err := configStruct.Validate()
		Expect(err).To(Succeed())
	})

	It("Fill Missing data", func() {
		configStruct.Supervisor.Port = 0
		configStruct.Supervisor.LogLevel = ""
		configStruct.Planner.BaseFolder = ""
		configStruct.FillMissingData()
		Expect(configStruct).To(PointTo(MatchAllFields(Fields{
			"Supervisor": PointTo(MatchAllFields(Fields{
				"Port":     Equal(8080),
				"LogLevel": Equal("debug"),
			})),
			"Planner": PointTo(MatchFields(IgnoreExtras, Fields{
				"BaseFolder": Equal("import"),
			})),
		})))

	})
	It("Case insensitive", func() {
		configStruct.Supervisor.LogLevel = "fAtAL"
		configStruct.Planner.BaseFolder = "ImPOrt"
		configStruct.FillMissingData()
		Expect(configStruct).To(PointTo(MatchAllFields(Fields{
			"Supervisor": PointTo(MatchFields(IgnoreExtras, Fields{
				"LogLevel": Equal("fatal"),
			})),
			"Planner": PointTo(MatchFields(IgnoreExtras, Fields{
				"BaseFolder": Equal("import"),
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
		Expect(err).To(MatchError(MatchRegexp("Port number out of range")))

	})
	It("Log Level", func() {
		configStruct.Supervisor.LogLevel = "debugfatal"
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp("Log level not filled in correctly")))

	})
	It("Base Folder", func() {
		configStruct.Planner.BaseFolder = "imp"
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp("Base Folder not filled in correctly")))

	})
	It("Planner kinds missing", func() {
		configStruct.Planner.Kinds = []*config.KindsConfig{}
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp("No planner.kinds filled in")))
	})
	It("Duplicate Names", func() {
		configStruct.Planner.Kinds[0].Name = configStruct.Planner.Kinds[1].Name
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp("Duplicate names in Planner.Kinds")))
	})
	It("Duplicate elements of Folders", func() {
		configStruct.Planner.Kinds[1].Folders = []string{"schema", "schema", "data"}
		err := configStruct.Validate()
		Expect(err).To(MatchError(MatchRegexp("Duplicate elements in Folders array")))
	})

})
